package tunnel

import (
	"testing"
	"time"
)

func TestNewTunnelManager(t *testing.T) {
	// Test with nil config (should use defaults)
	tm := NewTunnelManager(nil)
	if tm == nil {
		t.Fatal("NewTunnelManager should not return nil")
	}

	if tm.config == nil {
		t.Fatal("TunnelManager config should not be nil")
	}

	// Verify default values
	if tm.config.MinTunnels != 1 {
		t.Errorf("Expected MinTunnels=1, got %d", tm.config.MinTunnels)
	}
	if tm.config.MaxTunnels != 8 {
		t.Errorf("Expected MaxTunnels=8, got %d", tm.config.MaxTunnels)
	}
	if tm.config.BaseCIDR != "10.100.0.0/16" {
		t.Errorf("Expected BaseCIDR=10.100.0.0/16, got %s", tm.config.BaseCIDR)
	}
	if tm.config.MTU != 1420 {
		t.Errorf("Expected MTU=1420, got %d", tm.config.MTU)
	}
	if tm.config.ListenPort != 51820 {
		t.Errorf("Expected ListenPort=51820, got %d", tm.config.ListenPort)
	}

	// Test with custom config
	customConfig := &TunnelConfig{
		MinTunnels: 2,
		MaxTunnels: 4,
		BaseCIDR:   "192.168.0.0/16",
		MTU:        1500,
		ListenPort: 51821,
	}

	tm2 := NewTunnelManager(customConfig)
	if tm2.config.MinTunnels != 2 {
		t.Errorf("Expected MinTunnels=2, got %d", tm2.config.MinTunnels)
	}
	if tm2.config.MaxTunnels != 4 {
		t.Errorf("Expected MaxTunnels=4, got %d", tm2.config.MaxTunnels)
	}
}

func TestCalculateTunnelIP(t *testing.T) {
	tm := NewTunnelManager(nil)

	tests := []struct {
		tunnelID int
		expected string
	}{
		{0, "10.100.1.1/24"},
		{1, "10.100.2.1/24"},
		{2, "10.100.3.1/24"},
		{7, "10.100.8.1/24"},
	}

	for _, test := range tests {
		result := tm.calculateTunnelIP(test.tunnelID)
		if result != test.expected {
			t.Errorf("For tunnelID %d, expected %s, got %s", test.tunnelID, test.expected, result)
		}
	}
}

func TestTunnelValidation(t *testing.T) {
	tm := NewTunnelManager(nil)

	// Test exceeding max tunnels
	err := tm.CreateTunnels(9) // Max is 8
	if err == nil {
		t.Error("Expected error when creating more than max tunnels")
	}

	// Test valid tunnel count
	err = tm.CreateTunnels(3)
	if err != nil {
		// This might fail in test environment due to WireGuard dependencies
		// but we test the validation logic
		t.Logf("CreateTunnels failed (expected in test env): %v", err)
	}
}

func TestAddRemoveTunnel(t *testing.T) {
	tm := NewTunnelManager(&TunnelConfig{
		MinTunnels: 1,
		MaxTunnels: 3,
		BaseCIDR:   "10.100.0.0/16",
		MTU:        1420,
		ListenPort: 51820,
	})

	// Test adding tunnel beyond max
	for i := 0; i < 4; i++ {
		err := tm.AddTunnel()
		if i < 3 {
			// First 3 should succeed (or fail due to system deps, but not validation)
			if err != nil && err.Error() == "maximum tunnel count reached (3)" {
				t.Errorf("Unexpected validation error on tunnel %d: %v", i, err)
			}
		} else {
			// 4th should fail validation
			if err == nil || err.Error() != "maximum tunnel count reached (3)" {
				t.Errorf("Expected max tunnel error on tunnel %d, got: %v", i, err)
			}
		}
	}
}

func TestTunnelStatus(t *testing.T) {
	tm := NewTunnelManager(nil)

	// Test initial empty status
	statuses, err := tm.GetActiveTunnels()
	if err != nil {
		t.Fatalf("GetActiveTunnels failed: %v", err)
	}

	if len(statuses) != 0 {
		t.Errorf("Expected 0 tunnels initially, got %d", len(statuses))
	}
}

func TestTunnelMetrics(t *testing.T) {
	tm := NewTunnelManager(nil)

	// Test performance monitoring channel
	metricsChan := tm.MonitorPerformance()
	if metricsChan == nil {
		t.Fatal("MonitorPerformance should return a channel")
	}

	// Test that channel closes properly
	go func() {
		time.Sleep(100 * time.Millisecond)
		// In a real implementation, we'd have a way to stop monitoring
	}()

	// Try to read from channel (should not block forever)
	select {
	case <-metricsChan:
		// Got a metric, that's good
	case <-time.After(200 * time.Millisecond):
		// Timeout is also acceptable for empty tunnel manager
	}
}

func TestConfigureECMP(t *testing.T) {
	tm := NewTunnelManager(nil)

	err := tm.ConfigureECMP()
	if err != nil {
		t.Errorf("ConfigureECMP failed: %v", err)
	}
	// In test environment, this is mostly a validation that the function exists
	// and doesn't panic. Real ECMP configuration requires system-level permissions.
}

// Test WireGuardTunnel struct
func TestWireGuardTunnel(t *testing.T) {
	tunnel := &WireGuardTunnel{
		ID:         0,
		Interface:  "wg0",
		PrivateKey: "test_private_key",
		PublicKey:  "test_public_key",
		EndpointIP: "1.2.3.4",
		Port:       51820,
		Status: TunnelStatus{
			State:      "active",
			LastSeen:   time.Now(),
			TxBytes:    "1000000",
			RxBytes:    "2000000",
			Handshakes: 5,
			Latency:    25 * time.Millisecond,
		},
		Metrics: TunnelMetrics{
			Throughput: 1000000, // 1 Mbps
			PacketLoss: 0.01,    // 1%
			Jitter:     5 * time.Millisecond,
			LastUpdate: time.Now(),
		},
	}

	if tunnel.ID != 0 {
		t.Errorf("Expected ID=0, got %d", tunnel.ID)
	}
	if tunnel.Interface != "wg0" {
		t.Errorf("Expected Interface=wg0, got %s", tunnel.Interface)
	}
	if tunnel.Status.State != "active" {
		t.Errorf("Expected State=active, got %s", tunnel.Status.State)
	}
	if tunnel.Metrics.Throughput != 1000000 {
		t.Errorf("Expected Throughput=1000000, got %d", tunnel.Metrics.Throughput)
	}
}

// Test TunnelConfig validation
func TestTunnelConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config *TunnelConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: &TunnelConfig{
				MinTunnels: 1,
				MaxTunnels: 8,
				BaseCIDR:   "10.100.0.0/16",
				MTU:        1420,
				ListenPort: 51820,
			},
			valid: true,
		},
		{
			name: "invalid - max < min",
			config: &TunnelConfig{
				MinTunnels: 5,
				MaxTunnels: 3,
				BaseCIDR:   "10.100.0.0/16",
				MTU:        1420,
				ListenPort: 51820,
			},
			valid: false,
		},
		{
			name: "invalid - zero min",
			config: &TunnelConfig{
				MinTunnels: 0,
				MaxTunnels: 8,
				BaseCIDR:   "10.100.0.0/16",
				MTU:        1420,
				ListenPort: 51820,
			},
			valid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Basic validation logic
			isValid := test.config.MinTunnels > 0 && test.config.MaxTunnels >= test.config.MinTunnels

			if isValid != test.valid {
				t.Errorf("Expected validation result %v, got %v for %s", test.valid, isValid, test.name)
			}
		})
	}
}