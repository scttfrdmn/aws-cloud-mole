package tunnel

import (
	"os"
	"strings"
	"testing"
)

func TestGenerateWireGuardKeys(t *testing.T) {
	privateKey, publicKey, err := GenerateWireGuardKeys()
	if err != nil {
		t.Fatalf("Failed to generate WireGuard keys: %v", err)
	}

	// Test that keys are not empty
	if privateKey == "" {
		t.Error("Private key should not be empty")
	}
	if publicKey == "" {
		t.Error("Public key should not be empty")
	}

	// Test that keys are different
	if privateKey == publicKey {
		t.Error("Private and public keys should be different")
	}

	// Test that keys are properly base64 encoded (44 characters for curve25519)
	if len(privateKey) != 44 {
		t.Errorf("Private key should be 44 characters, got %d", len(privateKey))
	}
	if len(publicKey) != 44 {
		t.Errorf("Public key should be 44 characters, got %d", len(publicKey))
	}

	// Test that keys end with '=' (base64 padding)
	if !strings.HasSuffix(privateKey, "=") {
		t.Error("Private key should end with '=' (base64 padding)")
	}
	if !strings.HasSuffix(publicKey, "=") {
		t.Error("Public key should end with '=' (base64 padding)")
	}
}

func TestGenerateWireGuardConfig(t *testing.T) {
	tm := &TunnelManager{}

	config := &WireGuardConfig{
		Interface:     "wg0",
		PrivateKey:    "oK56DE9Ue9zK76rAc8pBl6opph+1v36mIqk9JuFNNT0=",
		PublicKey:     "XIuGKzMXouGh1FtJ+qm6q3w6j2V+jHzqwQMVpP8h0Ak=",
		ListenPort:    51820,
		Address:       "10.100.1.1/24",
		MTU:           1420,
		PeerPublicKey: "peer_public_key_here",
		PeerEndpoint:  "1.2.3.4:51820",
		AllowedIPs:    "0.0.0.0/0",
	}

	configStr := tm.generateWireGuardConfig(config)

	// Test that config contains required sections
	if !strings.Contains(configStr, "[Interface]") {
		t.Error("Config should contain [Interface] section")
	}
	if !strings.Contains(configStr, "[Peer]") {
		t.Error("Config should contain [Peer] section")
	}

	// Test that config contains required fields
	expectedFields := []string{
		"PrivateKey = " + config.PrivateKey,
		"Address = " + config.Address,
		"ListenPort = 51820",
		"MTU = 1420",
		"PublicKey = " + config.PeerPublicKey,
		"Endpoint = " + config.PeerEndpoint,
		"AllowedIPs = " + config.AllowedIPs,
		"PersistentKeepalive = 25",
	}

	for _, field := range expectedFields {
		if !strings.Contains(configStr, field) {
			t.Errorf("Config should contain field: %s", field)
		}
	}

	// Test routing rules
	if !strings.Contains(configStr, "PostUp") {
		t.Error("Config should contain PostUp rules")
	}
	if !strings.Contains(configStr, "PostDown") {
		t.Error("Config should contain PostDown rules")
	}
}

func TestWireGuardConfigValidation(t *testing.T) {
	tm := &TunnelManager{}

	tests := []struct {
		name     string
		config   *WireGuardConfig
		wantErr  bool
		checkFn  func(string) bool
	}{
		{
			name: "valid config",
			config: &WireGuardConfig{
				Interface:     "wg0",
				PrivateKey:    "oK56DE9Ue9zK76rAc8pBl6opph+1v36mIqk9JuFNNT0=",
				ListenPort:    51820,
				Address:       "10.100.1.1/24",
				MTU:           1420,
			},
			wantErr: false,
			checkFn: func(config string) bool {
				return strings.Contains(config, "[Interface]") &&
				       strings.Contains(config, "ListenPort = 51820") &&
				       strings.Contains(config, "MTU = 1420")
			},
		},
		{
			name: "config without peer",
			config: &WireGuardConfig{
				Interface:  "wg1",
				PrivateKey: "test_private_key",
				ListenPort: 51821,
				Address:    "10.100.2.1/24",
			},
			wantErr: false,
			checkFn: func(config string) bool {
				// Should not contain peer section if no peer configured
				return !strings.Contains(config, "[Peer]")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tm.generateWireGuardConfig(tt.config)

			if !tt.checkFn(result) {
				t.Errorf("Config validation failed for %s\nConfig:\n%s", tt.name, result)
			}
		})
	}
}

// Test that we can create configuration files
func TestWireGuardConfigFileCreation(t *testing.T) {
	tm := &TunnelManager{}

	config := &WireGuardConfig{
		Interface:  "wgtest",
		PrivateKey: "test_private_key",
		ListenPort: 51822,
		Address:    "10.100.99.1/24",
		MTU:        1420,
	}

	// Generate config content
	configContent := tm.generateWireGuardConfig(config)

	// Write to temporary file
	tmpFile := "/tmp/wgtest_unittest.conf"
	err := os.WriteFile(tmpFile, []byte(configContent), 0600)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}
	defer os.Remove(tmpFile)

	// Read back and verify
	readContent, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if string(readContent) != configContent {
		t.Error("Read content does not match written content")
	}

	// Verify file permissions
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("Failed to stat config file: %v", err)
	}

	expectedPerm := os.FileMode(0600)
	if info.Mode() != expectedPerm {
		t.Errorf("Expected file permissions %v, got %v", expectedPerm, info.Mode())
	}
}

func TestKeyGeneration(t *testing.T) {
	// Generate multiple key pairs and ensure they're all unique
	keys := make(map[string]bool)

	for i := 0; i < 10; i++ {
		privateKey, publicKey, err := GenerateWireGuardKeys()
		if err != nil {
			t.Fatalf("Failed to generate keys on iteration %d: %v", i, err)
		}

		// Check for uniqueness
		if keys[privateKey] {
			t.Error("Generated duplicate private key")
		}
		if keys[publicKey] {
			t.Error("Generated duplicate public key")
		}

		keys[privateKey] = true
		keys[publicKey] = true
	}
}