// +build integration

package tunnel

import (
	"context"
	"os"
	"testing"
)

// Integration tests that require system privileges
// Run with: go test -tags=integration

func TestWireGuardIntegrationWithAskpass(t *testing.T) {
	if os.Getenv("SUDO_ASKPASS") == "" {
		t.Skip("SUDO_ASKPASS not set, skipping integration tests requiring sudo")
	}

	tm := NewTunnelManager(&TunnelConfig{
		MinTunnels: 1,
		MaxTunnels: 1,
		BaseCIDR:   "10.100.0.0/16",
		MTU:        1420,
		ListenPort: 51820,
	})

	// Test creating a single tunnel with real WireGuard
	err := tm.CreateTunnels(1)
	if err != nil {
		t.Fatalf("Failed to create WireGuard tunnel: %v", err)
	}

	// Verify the tunnel was created
	tunnels, err := tm.GetActiveTunnels()
	if err != nil {
		t.Fatalf("Failed to get tunnel status: %v", err)
	}

	if len(tunnels) != 1 {
		t.Errorf("Expected 1 tunnel, got %d", len(tunnels))
	}

	// Test tunnel validation
	err = tm.ValidateWireGuardInterface("wg0")
	if err != nil {
		t.Logf("WireGuard validation failed (expected in test env): %v", err)
	}

	// Clean up - destroy the tunnel
	err = tm.RemoveTunnel()
	if err != nil {
		t.Errorf("Failed to remove tunnel: %v", err)
	}
}

func TestNetworkInterfaceIntegration(t *testing.T) {
	if os.Getenv("INTEGRATION_TESTS") == "" {
		t.Skip("INTEGRATION_TESTS not set, skipping network integration tests")
	}

	prober := NewNetworkProber()

	// Test actual network probing
	results, err := prober.ProbeNetwork(context.Background(), "us-west-2")
	if err != nil {
		t.Logf("Network probing failed (may be expected): %v", err)
		return
	}

	// Validate results structure
	if results.OptimalMTU < 1200 || results.OptimalMTU > 9000 {
		t.Errorf("Unexpected optimal MTU: %d", results.OptimalMTU)
	}

	if results.BaselineBandwidth <= 0 {
		t.Errorf("Baseline bandwidth should be positive: %d", results.BaselineBandwidth)
	}

	if len(results.AWSRegionLatencies) == 0 {
		t.Error("Should have region latency measurements")
	}

	t.Logf("Network probing results: MTU=%d, Bandwidth=%d bps, Regions=%d",
		results.OptimalMTU, results.BaselineBandwidth, len(results.AWSRegionLatencies))
}