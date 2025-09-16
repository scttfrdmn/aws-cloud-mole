package tunnel

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/crypto/curve25519"
)

// WireGuardConfig represents WireGuard configuration
type WireGuardConfig struct {
	Interface     string
	PrivateKey    string
	PublicKey     string
	ListenPort    int
	Address       string
	PeerPublicKey string
	PeerEndpoint  string
	AllowedIPs    string
	MTU           int
}

// GenerateWireGuardKeys generates a new WireGuard key pair
func GenerateWireGuardKeys() (privateKey, publicKey string, err error) {
	// Generate private key
	var private [32]byte
	if _, err := rand.Read(private[:]); err != nil {
		return "", "", fmt.Errorf("failed to generate private key: %w", err)
	}

	// Generate public key from private key
	var public [32]byte
	curve25519.ScalarBaseMult(&public, &private)

	// Encode to base64
	privateKey = base64.StdEncoding.EncodeToString(private[:])
	publicKey = base64.StdEncoding.EncodeToString(public[:])

	return privateKey, publicKey, nil
}

// CreateWireGuardInterface creates and configures a WireGuard interface
func (tm *TunnelManager) CreateWireGuardInterface(config *WireGuardConfig) error {
	// Create WireGuard configuration file
	configContent := tm.generateWireGuardConfig(config)
	configFile := fmt.Sprintf("/tmp/%s.conf", config.Interface)

	if err := os.WriteFile(configFile, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write WireGuard config: %w", err)
	}

	// Bring up the interface
	cmd := exec.Command("sudo", "wg-quick", "up", configFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up WireGuard interface: %w, output: %s", err, string(output))
	}

	fmt.Printf("✅ WireGuard interface %s created successfully\n", config.Interface)
	return nil
}

// DestroyWireGuardInterface tears down a WireGuard interface
func (tm *TunnelManager) DestroyWireGuardInterface(interfaceName string) error {
	configFile := fmt.Sprintf("/tmp/%s.conf", interfaceName)

	// Bring down the interface
	cmd := exec.Command("sudo", "wg-quick", "down", configFile)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring down WireGuard interface: %w, output: %s", err, string(output))
	}

	// Clean up config file
	if err := os.Remove(configFile); err != nil {
		// Not fatal, just log
		fmt.Printf("Warning: failed to remove config file %s: %v\n", configFile, err)
	}

	fmt.Printf("✅ WireGuard interface %s destroyed successfully\n", interfaceName)
	return nil
}

// generateWireGuardConfig generates WireGuard configuration file content
func (tm *TunnelManager) generateWireGuardConfig(config *WireGuardConfig) string {
	var builder strings.Builder

	// Interface section
	builder.WriteString("[Interface]\n")
	builder.WriteString(fmt.Sprintf("PrivateKey = %s\n", config.PrivateKey))
	builder.WriteString(fmt.Sprintf("Address = %s\n", config.Address))
	builder.WriteString(fmt.Sprintf("ListenPort = %d\n", config.ListenPort))
	if config.MTU > 0 {
		builder.WriteString(fmt.Sprintf("MTU = %d\n", config.MTU))
	}

	// Add post-up and post-down rules for routing
	builder.WriteString("PostUp = ip route add 0.0.0.0/0 dev %i table 200\n")
	builder.WriteString("PostUp = ip rule add from %s table 200\n")
	builder.WriteString("PostDown = ip route del 0.0.0.0/0 dev %i table 200\n")
	builder.WriteString("PostDown = ip rule del from %s table 200\n")

	// Peer section
	if config.PeerPublicKey != "" {
		builder.WriteString("\n[Peer]\n")
		builder.WriteString(fmt.Sprintf("PublicKey = %s\n", config.PeerPublicKey))
		if config.PeerEndpoint != "" {
			builder.WriteString(fmt.Sprintf("Endpoint = %s\n", config.PeerEndpoint))
		}
		if config.AllowedIPs != "" {
			builder.WriteString(fmt.Sprintf("AllowedIPs = %s\n", config.AllowedIPs))
		}
		builder.WriteString("PersistentKeepalive = 25\n")
	}

	return builder.String()
}

// GetWireGuardStats retrieves statistics for a WireGuard interface
func (tm *TunnelManager) GetWireGuardStats(interfaceName string) (*WireGuardStats, error) {
	cmd := exec.Command("sudo", "wg", "show", interfaceName, "dump")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get WireGuard stats: %w", err)
	}

	// Parse the dump output
	stats := &WireGuardStats{
		Interface: interfaceName,
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) >= 3 {
			// Parse peer statistics
			peerStats := PeerStats{
				PublicKey: fields[0],
			}

			if len(fields) >= 6 {
				// fields[4] = rx bytes, fields[5] = tx bytes
				fmt.Sscanf(fields[4], "%d", &peerStats.RxBytes)
				fmt.Sscanf(fields[5], "%d", &peerStats.TxBytes)
			}

			stats.Peers = append(stats.Peers, peerStats)
		}
	}

	return stats, nil
}

// WireGuardStats represents WireGuard interface statistics
type WireGuardStats struct {
	Interface string
	Peers     []PeerStats
}

// PeerStats represents statistics for a WireGuard peer
type PeerStats struct {
	PublicKey string
	RxBytes   int64
	TxBytes   int64
}

// ValidateWireGuardInterface checks if WireGuard interface is operational
func (tm *TunnelManager) ValidateWireGuardInterface(interfaceName string) error {
	// Check if interface exists
	cmd := exec.Command("ip", "link", "show", interfaceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("WireGuard interface %s not found: %w", interfaceName, err)
	}

	// Check if interface is up
	cmd = exec.Command("ip", "link", "show", interfaceName, "up")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("WireGuard interface %s is not up: %w", interfaceName, err)
	}

	// Check WireGuard configuration
	cmd = exec.Command("sudo", "wg", "show", interfaceName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("WireGuard interface %s configuration error: %w", interfaceName, err)
	}

	return nil
}