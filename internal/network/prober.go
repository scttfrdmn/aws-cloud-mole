package network

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// NetworkProber handles comprehensive network performance discovery
type NetworkProber struct {
	config       *ProbeConfig
	results      *ProbeResults
	awsEndpoints []string
	testServers  []TestServer
	mu           sync.RWMutex
}

// ProbeConfig defines probing parameters
type ProbeConfig struct {
	TestDuration       time.Duration `yaml:"test_duration"`        // 30 seconds per test
	MTUDiscoveryRange  []int         `yaml:"mtu_range"`            // [1200, 1500, 1800, 9000]
	BandwidthTestSizes []int64       `yaml:"bandwidth_test_sizes"` // [1MB, 10MB, 100MB]
	LatencyTestCount   int           `yaml:"latency_test_count"`   // 100 pings
	ParallelStreams    []int         `yaml:"parallel_streams"`     // [1, 2, 4, 8] streams
	TestRegions        []string      `yaml:"test_regions"`         // AWS regions to test
	EnableJumboFrames  bool          `yaml:"enable_jumbo_frames"`  // Test 9000 MTU
}

// ProbeResults contains comprehensive network analysis
type ProbeResults struct {
	Timestamp          time.Time
	LocalInterface     NetworkInterface
	OptimalMTU         int
	BaselineBandwidth  int64  // Sustained single-stream throughput (bps)
	BurstBandwidth     int64  // Peak multi-stream throughput (bps)
	OptimalStreams     int    // Best stream count for max throughput
	BottleneckLocation string // "local", "campus", "internet", "aws"
	AWSRegionLatencies map[string]time.Duration
	Recommendations    []string
	DetailedMetrics    NetworkMetrics
}

// NetworkInterface represents local network configuration
type NetworkInterface struct {
	Name       string
	Speed      int64           // Link speed in bps
	MTU        int             // Current MTU
	Driver     string          // NIC driver
	Offloading map[string]bool // TCP offloading features
	QueueCount int             // Multi-queue support
}

// NetworkMetrics contains detailed test results
type NetworkMetrics struct {
	MTUTests        []MTUTestResult
	BandwidthTests  []BandwidthTestResult
	LatencyTests    []LatencyTestResult
	CongestionTests []CongestionTestResult
}

// TestServer represents an AWS test endpoint
type TestServer struct {
	Region    string
	Endpoint  string
	PublicIP  string
	LatencyMs float64
}

// Supporting types for detailed metrics
type MTUTestResult struct {
	MTU        int
	Throughput int64
	Success    bool
}

type BandwidthTestResult struct {
	Streams    int
	Throughput int64
	Duration   time.Duration
}

type LatencyTestResult struct {
	Region     string
	MinLatency time.Duration
	AvgLatency time.Duration
	MaxLatency time.Duration
	PacketLoss float64
}

type CongestionTestResult struct {
	WindowSize int
	Throughput int64
	RTT        time.Duration
	Congestion string
}

// NewNetworkProber creates a comprehensive network prober
func NewNetworkProber() *NetworkProber {
	config := &ProbeConfig{
		TestDuration:       30 * time.Second,
		MTUDiscoveryRange:  []int{1200, 1500, 1800, 9000},
		BandwidthTestSizes: []int64{1024 * 1024, 10 * 1024 * 1024, 100 * 1024 * 1024},
		LatencyTestCount:   100,
		ParallelStreams:    []int{1, 2, 4, 8},
		TestRegions:        []string{"us-west-2", "us-east-1", "eu-west-1"},
		EnableJumboFrames:  true,
	}

	return &NetworkProber{
		config:       config,
		results:      &ProbeResults{},
		awsEndpoints: []string{},
	}
}

// ProbeNetwork performs comprehensive network performance discovery
func (np *NetworkProber) ProbeNetwork(ctx context.Context, targetRegion string) (*ProbeResults, error) {
	fmt.Println("🔍 Starting comprehensive network performance discovery...")

	// Phase 1: Local interface discovery
	fmt.Println("📡 Analyzing local network interface...")
	if err := np.discoverLocalInterface(); err != nil {
		return nil, fmt.Errorf("failed to discover local interface: %w", err)
	}

	// Phase 2: MTU discovery
	fmt.Println("📏 Discovering optimal MTU size...")
	if err := np.discoverOptimalMTU(ctx, targetRegion); err != nil {
		return nil, fmt.Errorf("MTU discovery failed: %w", err)
	}

	// Phase 3: Latency baseline
	fmt.Println("⏱️  Measuring latency to AWS regions...")
	if err := np.measureRegionLatencies(ctx); err != nil {
		return nil, fmt.Errorf("latency measurement failed: %w", err)
	}

	// Phase 4: Bandwidth discovery
	fmt.Println("🚀 Testing bandwidth capacity...")
	if err := np.discoverBandwidthCapacity(ctx, targetRegion); err != nil {
		return nil, fmt.Errorf("bandwidth testing failed: %w", err)
	}

	// Phase 5: Bottleneck identification
	fmt.Println("🔬 Identifying network bottlenecks...")
	np.identifyBottlenecks()

	// Phase 6: Generate recommendations
	np.generateRecommendations()

	np.results.Timestamp = time.Now()
	return np.results, nil
}

func (np *NetworkProber) discoverLocalInterface() error {
	// Get default interface
	defaultIface, err := np.getDefaultInterface()
	if err != nil {
		return err
	}

	np.results.LocalInterface = defaultIface

	fmt.Printf("  Interface: %s (%s)\n", defaultIface.Name, defaultIface.Driver)
	fmt.Printf("  Link Speed: %s\n", formatBandwidth(defaultIface.Speed))
	fmt.Printf("  Current MTU: %d\n", defaultIface.MTU)
	fmt.Printf("  Multi-queue: %d queues\n", defaultIface.QueueCount)

	return nil
}

func (np *NetworkProber) discoverOptimalMTU(ctx context.Context, region string) error {
	// Get a test endpoint in the target region
	testEndpoint, err := np.getTestEndpoint(region)
	if err != nil {
		return err
	}

	bestMTU := 1500
	bestThroughput := int64(0)

	for _, mtu := range np.config.MTUDiscoveryRange {
		// Skip jumbo frames if not enabled
		if mtu > 1500 && !np.config.EnableJumboFrames {
			continue
		}

		fmt.Printf("  Testing MTU %d... ", mtu)

		// Test with ping to verify path MTU
		if !np.testPathMTU(testEndpoint, mtu) {
			fmt.Printf("❌ Path MTU exceeded\n")
			continue
		}

		// Test throughput with this MTU
		throughput, err := np.testMTUThroughput(ctx, testEndpoint, mtu)
		if err != nil {
			fmt.Printf("❌ Test failed: %v\n", err)
			continue
		}

		fmt.Printf("✅ %s\n", formatBandwidth(throughput))

		if throughput > bestThroughput {
			bestThroughput = throughput
			bestMTU = mtu
		}

		// Store test result
		np.results.DetailedMetrics.MTUTests = append(np.results.DetailedMetrics.MTUTests, MTUTestResult{
			MTU:        mtu,
			Throughput: throughput,
			Success:    true,
		})
	}

	np.results.OptimalMTU = bestMTU
	fmt.Printf("  📏 Optimal MTU: %d bytes (throughput: %s)\n", bestMTU, formatBandwidth(bestThroughput))

	return nil
}

func (np *NetworkProber) measureRegionLatencies(ctx context.Context) error {
	np.results.AWSRegionLatencies = make(map[string]time.Duration)

	for _, region := range np.config.TestRegions {
		endpoint, err := np.getTestEndpoint(region)
		if err != nil {
			fmt.Printf("  ❌ Failed to get endpoint for %s: %v\n", region, err)
			continue
		}

		fmt.Printf("  Testing %s (%s)... ", region, endpoint)

		latency, err := np.measureLatency(endpoint)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
			continue
		}

		np.results.AWSRegionLatencies[region] = latency
		fmt.Printf("✅ %v\n", latency)
	}

	return nil
}

func (np *NetworkProber) discoverBandwidthCapacity(ctx context.Context, region string) error {
	endpoint, err := np.getTestEndpoint(region)
	if err != nil {
		return err
	}

	bestStreams := 1
	bestThroughput := int64(0)

	for _, streams := range np.config.ParallelStreams {
		fmt.Printf("  Testing %d parallel streams... ", streams)

		throughput, err := np.testBandwidth(ctx, endpoint, streams)
		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err)
			continue
		}

		fmt.Printf("✅ %s\n", formatBandwidth(throughput))

		if streams == 1 {
			np.results.BaselineBandwidth = throughput
		}

		if throughput > bestThroughput {
			bestThroughput = throughput
			bestStreams = streams
		}

		// Store test result
		np.results.DetailedMetrics.BandwidthTests = append(np.results.DetailedMetrics.BandwidthTests, BandwidthTestResult{
			Streams:    streams,
			Throughput: throughput,
			Duration:   np.config.TestDuration,
		})
	}

	np.results.BurstBandwidth = bestThroughput
	np.results.OptimalStreams = bestStreams

	fmt.Printf("  🚀 Baseline (1 stream): %s\n", formatBandwidth(np.results.BaselineBandwidth))
	fmt.Printf("  🚀 Peak (%d streams): %s\n", bestStreams, formatBandwidth(bestThroughput))

	return nil
}

func (np *NetworkProber) identifyBottlenecks() {
	localSpeed := np.results.LocalInterface.Speed
	baseline := np.results.BaselineBandwidth
	burst := np.results.BurstBandwidth

	// Analyze bottleneck location
	if baseline < localSpeed/10 {
		np.results.BottleneckLocation = "internet"
	} else if baseline < localSpeed/2 {
		np.results.BottleneckLocation = "campus"
	} else if burst/baseline < 2 {
		np.results.BottleneckLocation = "aws"
	} else {
		np.results.BottleneckLocation = "local"
	}

	fmt.Printf("  🔬 Primary bottleneck: %s\n", np.results.BottleneckLocation)
	fmt.Printf("  📊 Utilization: %.1f%% of local interface\n",
		float64(baseline)/float64(localSpeed)*100)
}

func (np *NetworkProber) generateRecommendations() {
	var recommendations []string

	// MTU recommendations
	if np.results.OptimalMTU > 1500 {
		recommendations = append(recommendations,
			fmt.Sprintf("Enable jumbo frames (MTU %d) for +%.1f%% performance",
				np.results.OptimalMTU,
				float64(np.results.OptimalMTU-1500)/1500*100))
	}

	// Tunnel count recommendations
	scalingEfficiency := float64(np.results.BurstBandwidth) / float64(np.results.BaselineBandwidth)
	optimalTunnels := int(scalingEfficiency * 0.8) // Conservative estimate

	if optimalTunnels > 1 {
		recommendations = append(recommendations,
			fmt.Sprintf("Use %d WireGuard tunnels for optimal throughput (%.1fx scaling efficiency)",
				optimalTunnels, scalingEfficiency))
	}

	// Instance type recommendations based on bottleneck
	switch np.results.BottleneckLocation {
	case "local":
		recommendations = append(recommendations, "Local interface is limiting - consider NIC upgrade")
	case "campus":
		recommendations = append(recommendations, "Campus network limiting - contact IT about bandwidth")
	case "internet":
		recommendations = append(recommendations, "Internet transit limiting - consider dedicated connection")
	case "aws":
		recommendations = append(recommendations, "AWS ingress limiting - use enhanced networking instances")
	}

	// TCP optimization recommendations
	if np.results.AWSRegionLatencies != nil {
		for region, latency := range np.results.AWSRegionLatencies {
			if latency > 100*time.Millisecond {
				recommendations = append(recommendations,
					fmt.Sprintf("High latency to %s (%v) - enable TCP BBR congestion control", region, latency))
			}
		}
	}

	np.results.Recommendations = recommendations

	fmt.Println("  💡 Recommendations:")
	for _, rec := range recommendations {
		fmt.Printf("     • %s\n", rec)
	}
}
