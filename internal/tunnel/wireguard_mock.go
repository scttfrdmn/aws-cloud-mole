// +build test

package tunnel

import (
	"fmt"
	"os"
	"strings"
)

// Mock implementations for testing that don't require sudo

// CreateWireGuardInterface creates a mock WireGuard interface for testing
func (tm *TunnelManager) CreateWireGuardInterface(config *WireGuardConfig) error {
	// In test mode, just validate the configuration without creating actual interface
	configContent := tm.generateWireGuardConfig(config)
	configFile := fmt.Sprintf("/tmp/%s.conf", config.Interface)

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write WireGuard config: %w", err)
	}

	// Mock success message
	fmt.Printf("✅ Mock WireGuard interface %s created successfully (test mode)\n", config.Interface)
	return nil
}

// DestroyWireGuardInterface tears down a mock WireGuard interface for testing
func (tm *TunnelManager) DestroyWireGuardInterface(interfaceName string) error {
	configFile := fmt.Sprintf("/tmp/%s.conf", interfaceName)

	// Clean up config file
	if err := os.Remove(configFile); err != nil {
		// Not fatal in test mode
		fmt.Printf("Warning: failed to remove config file %s: %v\n", configFile, err)
	}

	fmt.Printf("✅ Mock WireGuard interface %s destroyed successfully (test mode)\n", interfaceName)
	return nil
}

// GetWireGuardStats returns mock statistics for testing
func (tm *TunnelManager) GetWireGuardStats(interfaceName string) (*WireGuardStats, error) {
	// Return mock stats for testing
	stats := &WireGuardStats{
		Interface: interfaceName,
		Peers: []PeerStats{
			{
				PublicKey: "mock_peer_public_key",
				RxBytes:   1024000, // 1MB
				TxBytes:   2048000, // 2MB
			},
		},
	}
	return stats, nil
}

// ValidateWireGuardInterface validates mock interface for testing
func (tm *TunnelManager) ValidateWireGuardInterface(interfaceName string) error {
	// In test mode, just check if config file exists
	configFile := fmt.Sprintf("/tmp/%s.conf", interfaceName)
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("mock WireGuard interface %s not found (config file missing)", interfaceName)
	}

	// Check if config file contains expected content
	content, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	if !strings.Contains(string(content), "[Interface]") {
		return fmt.Errorf("mock WireGuard interface %s configuration is invalid", interfaceName)
	}

	return nil
}