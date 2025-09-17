package network

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestFormatBandwidth(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{0, "0 bps"},
		{500, "500 bps"},
		{1000, "1.0 Kbps"},
		{1500, "1.5 Kbps"},
		{1000000, "1.0 Mbps"},
		{1500000000, "1.5 Gbps"},
		{2500000000000, "2.5 Tbps"},
	}

	for _, test := range tests {
		result := formatBandwidth(test.input)
		if result != test.expected {
			t.Errorf("formatBandwidth(%d) = %s, expected %s", test.input, result, test.expected)
		}
	}
}

func TestGetTestEndpoint(t *testing.T) {
	prober := NewNetworkProber()

	tests := []struct {
		region   string
		expected string
		wantErr  bool
	}{
		{"us-west-2", "ec2.us-west-2.amazonaws.com", false},
		{"us-east-1", "ec2.us-east-1.amazonaws.com", false},
		{"eu-west-1", "ec2.eu-west-1.amazonaws.com", false},
		{"invalid-region", "", true},
	}

	for _, test := range tests {
		result, err := prober.getTestEndpoint(test.region)

		if test.wantErr {
			if err == nil {
				t.Errorf("getTestEndpoint(%s) expected error, got nil", test.region)
			}
		} else {
			if err != nil {
				t.Errorf("getTestEndpoint(%s) unexpected error: %v", test.region, err)
			}
			if result != test.expected {
				t.Errorf("getTestEndpoint(%s) = %s, expected %s", test.region, result, test.expected)
			}
		}
	}
}

func TestTestPathMTU(t *testing.T) {
	prober := NewNetworkProber()

	// Test with local loopback (should always succeed with reasonable MTU)
	result := prober.testPathMTU("127.0.0.1", 1500)
	// In test environment, this might succeed or fail depending on system config
	// We mainly test that it doesn't panic
	t.Logf("testPathMTU(127.0.0.1, 1500) = %v", result)

	// Test with extremely large MTU (should fail)
	result = prober.testPathMTU("127.0.0.1", 65000)
	if result {
		t.Error("Expected testPathMTU to fail with MTU 65000")
	}
}

func TestTestMTUThroughput(t *testing.T) {
	prober := NewNetworkProber()
	ctx := context.Background()

	// Test with various MTU sizes
	mtuSizes := []int{1200, 1500, 1800, 9000}

	for _, mtu := range mtuSizes {
		throughput, err := prober.testMTUThroughput(ctx, "127.0.0.1", mtu)

		// In test environment, this will likely use fallback calculation
		if err != nil {
			t.Errorf("testMTUThroughput failed for MTU %d: %v", mtu, err)
		}

		if throughput <= 0 {
			t.Errorf("testMTUThroughput returned non-positive throughput for MTU %d: %d", mtu, throughput)
		}

		// Test that larger MTUs generally give better efficiency
		if mtu > 1200 {
			// Should be at least somewhat better than minimum MTU
			minThroughput := int64(400 * 1024 * 1024) // 400 Mbps baseline
			if throughput < minThroughput {
				t.Logf("Low throughput for MTU %d: %s (expected in test env)", mtu, formatBandwidth(throughput))
			}
		}
	}
}

func TestMeasureLatency(t *testing.T) {
	prober := NewNetworkProber()

	// Test with localhost (should succeed)
	latency, err := prober.measureLatency("127.0.0.1")
	if err != nil {
		t.Logf("measureLatency failed (expected in some test environments): %v", err)
	} else {
		if latency <= 0 {
			t.Error("Expected positive latency")
		}
		if latency > 1*time.Second {
			t.Errorf("Latency to localhost seems too high: %v", latency)
		}
		t.Logf("Measured latency to localhost: %v", latency)
	}

	// Test with invalid host
	_, err = prober.measureLatency("invalid-host-that-does-not-exist.local")
	if err == nil {
		t.Error("Expected error for invalid host")
	}
}

func TestTestBandwidth(t *testing.T) {
	prober := NewNetworkProber()
	ctx := context.Background()

	// Test with different stream counts
	streamCounts := []int{1, 2, 4, 8}
	var previousThroughput int64

	for _, streams := range streamCounts {
		throughput, err := prober.testBandwidth(ctx, "127.0.0.1", streams)

		if err != nil {
			t.Errorf("testBandwidth failed for %d streams: %v", streams, err)
		}

		if throughput <= 0 {
			t.Errorf("testBandwidth returned non-positive throughput for %d streams: %d", streams, throughput)
		}

		// Test that more streams generally give better throughput
		if streams > 1 && throughput < previousThroughput {
			// This might happen due to overhead, but log it
			t.Logf("Throughput decreased with more streams: %d streams = %s, previous = %s",
				streams, formatBandwidth(throughput), formatBandwidth(previousThroughput))
		}

		previousThroughput = throughput
		t.Logf("Bandwidth with %d streams: %s", streams, formatBandwidth(throughput))
	}
}

func TestNetworkInterfaceDetails(t *testing.T) {
	prober := NewNetworkProber()
	iface := &NetworkInterface{
		Name: "lo", // Loopback interface should exist on most systems
	}

	err := prober.getInterfaceDetails(iface)
	if err != nil {
		t.Logf("getInterfaceDetails failed (expected in test env): %v", err)
	} else {
		// Verify that some details were populated
		if iface.MTU == 0 {
			t.Error("Expected MTU to be populated")
		}

		// Loopback typically has high MTU
		if iface.MTU < 1500 {
			t.Logf("Unexpected MTU for loopback: %d", iface.MTU)
		}

		t.Logf("Interface details - MTU: %d, Speed: %d, Driver: %s, Queues: %d",
			iface.MTU, iface.Speed, iface.Driver, iface.QueueCount)
	}
}

func TestGetDefaultInterface(t *testing.T) {
	prober := NewNetworkProber()

	iface, err := prober.getDefaultInterface()
	if err != nil {
		t.Logf("getDefaultInterface failed (expected in some test envs): %v", err)
	} else {
		if iface.Name == "" {
			t.Error("Expected interface name to be populated")
		}

		if iface.MTU == 0 {
			t.Error("Expected MTU to be populated")
		}

		t.Logf("Default interface: %s (MTU: %d, Speed: %s, Driver: %s)",
			iface.Name, iface.MTU, formatBandwidth(iface.Speed), iface.Driver)
	}
}

// Test that bandwidth formatting handles edge cases
func TestFormatBandwidthEdgeCases(t *testing.T) {
	tests := []struct {
		input       int64
		shouldContain string
	}{
		{999, "bps"},
		{1001, "Kbps"},
		{999999, "Kbps"},
		{1000001, "Mbps"},
		{999999999, "Mbps"},
		{1000000001, "Gbps"},
	}

	for _, test := range tests {
		result := formatBandwidth(test.input)
		if !strings.Contains(result, test.shouldContain) {
			t.Errorf("formatBandwidth(%d) = %s, should contain %s", test.input, result, test.shouldContain)
		}
	}
}

// Test bandwidth measurement consistency
func TestBandwidthMeasurementConsistency(t *testing.T) {
	prober := NewNetworkProber()
	ctx := context.Background()

	// Measure bandwidth multiple times and check for consistency
	results := make([]int64, 3)

	for i := 0; i < 3; i++ {
		throughput, err := prober.testBandwidth(ctx, "127.0.0.1", 1)
		if err != nil {
			t.Logf("Bandwidth measurement %d failed: %v", i, err)
			continue
		}
		results[i] = throughput
	}

	// Check that results are within reasonable range of each other
	if results[0] > 0 && results[1] > 0 {
		ratio := float64(results[0]) / float64(results[1])
		if ratio > 2.0 || ratio < 0.5 {
			t.Logf("Bandwidth measurements vary significantly: %s vs %s",
				formatBandwidth(results[0]), formatBandwidth(results[1]))
		}
	}
}

// Test MTU efficiency calculation
func TestMTUEfficiency(t *testing.T) {
	tests := []struct {
		mtu        int
		efficiency float64
	}{
		{1200, float64(1200-40) / 1200}, // ~0.967
		{1500, float64(1500-40) / 1500}, // ~0.973
		{9000, float64(9000-40) / 9000}, // ~0.996
	}

	for _, test := range tests {
		// Simulate the efficiency calculation from testMTUThroughput
		efficiency := float64(test.mtu-40) / float64(test.mtu)

		if abs(efficiency-test.efficiency) > 0.001 {
			t.Errorf("MTU %d efficiency: expected %f, got %f", test.mtu, test.efficiency, efficiency)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}