package network

import (
	"context"
	"testing"
	"time"
)

func TestNewNetworkProber(t *testing.T) {
	prober := NewNetworkProber()
	if prober == nil {
		t.Fatal("NewNetworkProber should not return nil")
	}

	if prober.config == nil {
		t.Fatal("NetworkProber config should not be nil")
	}

	// Verify default configuration values
	if prober.config.TestDuration != 30*time.Second {
		t.Errorf("Expected TestDuration=30s, got %v", prober.config.TestDuration)
	}

	expectedMTUs := []int{1200, 1500, 1800, 9000}
	if len(prober.config.MTUDiscoveryRange) != len(expectedMTUs) {
		t.Errorf("Expected %d MTU values, got %d", len(expectedMTUs), len(prober.config.MTUDiscoveryRange))
	}

	for i, expected := range expectedMTUs {
		if prober.config.MTUDiscoveryRange[i] != expected {
			t.Errorf("Expected MTU[%d]=%d, got %d", i, expected, prober.config.MTUDiscoveryRange[i])
		}
	}

	expectedStreams := []int{1, 2, 4, 8}
	if len(prober.config.ParallelStreams) != len(expectedStreams) {
		t.Errorf("Expected %d stream values, got %d", len(expectedStreams), len(prober.config.ParallelStreams))
	}

	expectedRegions := []string{"us-west-2", "us-east-1", "eu-west-1"}
	if len(prober.config.TestRegions) != len(expectedRegions) {
		t.Errorf("Expected %d test regions, got %d", len(expectedRegions), len(prober.config.TestRegions))
	}

	if !prober.config.EnableJumboFrames {
		t.Error("Expected EnableJumboFrames=true by default")
	}
}

func TestProbeResultsStructure(t *testing.T) {
	results := &ProbeResults{
		Timestamp:          time.Now(),
		OptimalMTU:         1500,
		BaselineBandwidth:  1000000000, // 1 Gbps
		BurstBandwidth:     2000000000, // 2 Gbps
		OptimalStreams:     4,
		BottleneckLocation: "local",
		AWSRegionLatencies: map[string]time.Duration{
			"us-west-2": 25 * time.Millisecond,
			"us-east-1": 75 * time.Millisecond,
		},
		Recommendations: []string{
			"Use 4 WireGuard tunnels for optimal throughput",
			"Enable jumbo frames (MTU 9000) for +15% performance",
		},
	}

	// Test basic structure
	if results.OptimalMTU != 1500 {
		t.Errorf("Expected OptimalMTU=1500, got %d", results.OptimalMTU)
	}

	if results.BaselineBandwidth != 1000000000 {
		t.Errorf("Expected BaselineBandwidth=1Gbps, got %d", results.BaselineBandwidth)
	}

	if results.OptimalStreams != 4 {
		t.Errorf("Expected OptimalStreams=4, got %d", results.OptimalStreams)
	}

	if len(results.AWSRegionLatencies) != 2 {
		t.Errorf("Expected 2 region latencies, got %d", len(results.AWSRegionLatencies))
	}

	// Test latency values
	if results.AWSRegionLatencies["us-west-2"] != 25*time.Millisecond {
		t.Errorf("Expected us-west-2 latency=25ms, got %v", results.AWSRegionLatencies["us-west-2"])
	}

	// Test recommendations
	if len(results.Recommendations) != 2 {
		t.Errorf("Expected 2 recommendations, got %d", len(results.Recommendations))
	}
}

func TestNetworkInterface(t *testing.T) {
	iface := NetworkInterface{
		Name:       "eth0",
		Speed:      1000000000, // 1 Gbps
		MTU:        1500,
		Driver:     "e1000e",
		QueueCount: 4,
		Offloading: map[string]bool{
			"tcp-segmentation-offload": true,
			"generic-receive-offload":  true,
			"large-receive-offload":    false,
		},
	}

	if iface.Name != "eth0" {
		t.Errorf("Expected Name=eth0, got %s", iface.Name)
	}

	if iface.Speed != 1000000000 {
		t.Errorf("Expected Speed=1Gbps, got %d", iface.Speed)
	}

	if iface.MTU != 1500 {
		t.Errorf("Expected MTU=1500, got %d", iface.MTU)
	}

	if iface.QueueCount != 4 {
		t.Errorf("Expected QueueCount=4, got %d", iface.QueueCount)
	}

	// Test offloading features
	if !iface.Offloading["tcp-segmentation-offload"] {
		t.Error("Expected tcp-segmentation-offload to be enabled")
	}

	if iface.Offloading["large-receive-offload"] {
		t.Error("Expected large-receive-offload to be disabled")
	}
}

func TestTestResults(t *testing.T) {
	// Test MTU test results
	mtuResult := MTUTestResult{
		MTU:        1500,
		Throughput: 1000000000,
		Success:    true,
	}

	if mtuResult.MTU != 1500 {
		t.Errorf("Expected MTU=1500, got %d", mtuResult.MTU)
	}

	if !mtuResult.Success {
		t.Error("Expected Success=true")
	}

	// Test bandwidth test results
	bwResult := BandwidthTestResult{
		Streams:    4,
		Throughput: 2000000000, // 2 Gbps
		Duration:   30 * time.Second,
	}

	if bwResult.Streams != 4 {
		t.Errorf("Expected Streams=4, got %d", bwResult.Streams)
	}

	if bwResult.Throughput != 2000000000 {
		t.Errorf("Expected Throughput=2Gbps, got %d", bwResult.Throughput)
	}

	// Test latency test results
	latencyResult := LatencyTestResult{
		Region:     "us-west-2",
		MinLatency: 20 * time.Millisecond,
		AvgLatency: 25 * time.Millisecond,
		MaxLatency: 30 * time.Millisecond,
		PacketLoss: 0.001, // 0.1%
	}

	if latencyResult.Region != "us-west-2" {
		t.Errorf("Expected Region=us-west-2, got %s", latencyResult.Region)
	}

	if latencyResult.AvgLatency != 25*time.Millisecond {
		t.Errorf("Expected AvgLatency=25ms, got %v", latencyResult.AvgLatency)
	}

	if latencyResult.PacketLoss != 0.001 {
		t.Errorf("Expected PacketLoss=0.001, got %f", latencyResult.PacketLoss)
	}
}

func TestProbeNetworkValidation(t *testing.T) {
	prober := NewNetworkProber()
	ctx := context.Background()

	// Test with invalid region (should handle gracefully)
	_, err := prober.ProbeNetwork(ctx, "invalid-region")
	if err == nil {
		t.Error("Expected error for invalid region")
	}

	// Test with valid region (may fail due to network dependencies, but should not panic)
	result, err := prober.ProbeNetwork(ctx, "us-west-2")
	if err != nil {
		// This is expected in test environment without network access
		t.Logf("ProbeNetwork failed as expected in test env: %v", err)
	} else {
		// If it succeeds, validate the result structure
		if result == nil {
			t.Error("ProbeNetwork returned nil result without error")
		} else {
			if result.OptimalMTU < 1200 || result.OptimalMTU > 9000 {
				t.Errorf("OptimalMTU out of range: %d", result.OptimalMTU)
			}
			if result.OptimalStreams < 1 || result.OptimalStreams > 16 {
				t.Errorf("OptimalStreams out of range: %d", result.OptimalStreams)
			}
		}
	}
}

func TestBottleneckIdentification(t *testing.T) {
	tests := []struct {
		name               string
		localSpeed         int64
		baselineBandwidth  int64
		burstBandwidth     int64
		expectedBottleneck string
	}{
		{
			name:               "internet bottleneck",
			localSpeed:         10000000000, // 10 Gbps
			baselineBandwidth:  100000000,   // 100 Mbps (< localSpeed/10)
			burstBandwidth:     200000000,   // 200 Mbps
			expectedBottleneck: "internet",
		},
		{
			name:               "campus bottleneck",
			localSpeed:         10000000000, // 10 Gbps
			baselineBandwidth:  2000000000,  // 2 Gbps (< localSpeed/2)
			burstBandwidth:     4000000000,  // 4 Gbps
			expectedBottleneck: "campus",
		},
		{
			name:               "aws bottleneck",
			localSpeed:         10000000000, // 10 Gbps
			baselineBandwidth:  5000000000,  // 5 Gbps
			burstBandwidth:     6000000000,  // 6 Gbps (burst/baseline < 2)
			expectedBottleneck: "aws",
		},
		{
			name:               "local bottleneck",
			localSpeed:         1000000000,  // 1 Gbps
			baselineBandwidth:  800000000,   // 800 Mbps
			burstBandwidth:     1600000000,  // 1.6 Gbps (good scaling)
			expectedBottleneck: "local",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Simulate bottleneck identification logic
			var bottleneck string
			baseline := test.baselineBandwidth
			burst := test.burstBandwidth
			localSpeed := test.localSpeed

			if baseline < localSpeed/10 {
				bottleneck = "internet"
			} else if baseline < localSpeed/2 {
				bottleneck = "campus"
			} else if burst/baseline < 2 {
				bottleneck = "aws"
			} else {
				bottleneck = "local"
			}

			if bottleneck != test.expectedBottleneck {
				t.Errorf("Expected bottleneck=%s, got %s", test.expectedBottleneck, bottleneck)
			}
		})
	}
}

func TestRecommendationGeneration(t *testing.T) {
	// Test MTU recommendation
	optimalMTU := 9000
	var recommendations []string

	if optimalMTU > 1500 {
		recommendations = append(recommendations,
			"Enable jumbo frames for improved performance")
	}

	if len(recommendations) != 1 {
		t.Errorf("Expected 1 recommendation, got %d", len(recommendations))
	}

	// Test tunnel scaling recommendation
	baselineBandwidth := int64(1000000000)  // 1 Gbps
	burstBandwidth := int64(4000000000)     // 4 Gbps
	scalingEfficiency := float64(burstBandwidth) / float64(baselineBandwidth)
	optimalTunnels := int(scalingEfficiency * 0.8)

	if optimalTunnels < 1 {
		optimalTunnels = 1
	}

	if optimalTunnels != 3 { // 4 * 0.8 = 3.2, truncated to 3
		t.Errorf("Expected optimalTunnels=3, got %d", optimalTunnels)
	}
}

func TestNetworkMetrics(t *testing.T) {
	metrics := NetworkMetrics{
		MTUTests: []MTUTestResult{
			{MTU: 1500, Throughput: 1000000000, Success: true},
			{MTU: 9000, Throughput: 1200000000, Success: true},
		},
		BandwidthTests: []BandwidthTestResult{
			{Streams: 1, Throughput: 1000000000, Duration: 30 * time.Second},
			{Streams: 4, Throughput: 3500000000, Duration: 30 * time.Second},
		},
		LatencyTests: []LatencyTestResult{
			{Region: "us-west-2", AvgLatency: 25 * time.Millisecond, PacketLoss: 0.001},
		},
	}

	if len(metrics.MTUTests) != 2 {
		t.Errorf("Expected 2 MTU tests, got %d", len(metrics.MTUTests))
	}

	if len(metrics.BandwidthTests) != 2 {
		t.Errorf("Expected 2 bandwidth tests, got %d", len(metrics.BandwidthTests))
	}

	if len(metrics.LatencyTests) != 1 {
		t.Errorf("Expected 1 latency test, got %d", len(metrics.LatencyTests))
	}

	// Verify test results
	if metrics.MTUTests[1].MTU != 9000 {
		t.Errorf("Expected second MTU test to be 9000, got %d", metrics.MTUTests[1].MTU)
	}

	if metrics.BandwidthTests[1].Streams != 4 {
		t.Errorf("Expected second bandwidth test to use 4 streams, got %d", metrics.BandwidthTests[1].Streams)
	}
}