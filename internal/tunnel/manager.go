package tunnel

import (
	"fmt"
	"sync"
	"time"
)

// TunnelManager manages WireGuard tunnel lifecycle with dynamic scaling
type TunnelManager struct {
	config  *TunnelConfig
	tunnels map[int]*WireGuardTunnel
	scaler  *TunnelScaler
	mu      sync.RWMutex
}

// TunnelConfig defines tunnel configuration
type TunnelConfig struct {
	MinTunnels int    `yaml:"min_tunnels"` // Minimum tunnels (default: 1)
	MaxTunnels int    `yaml:"max_tunnels"` // Maximum tunnels (default: 8)
	BaseCIDR   string `yaml:"base_cidr"`   // Base CIDR for tunnel IPs
	MTU        int    `yaml:"mtu"`         // Tunnel MTU
	ListenPort int    `yaml:"listen_port"` // Starting port for tunnels
}

// WireGuardTunnel represents a single WireGuard tunnel
type WireGuardTunnel struct {
	ID         int
	Interface  string
	PrivateKey string
	PublicKey  string
	EndpointIP string
	Port       int
	Status     TunnelStatus
	Metrics    TunnelMetrics
	mu         sync.RWMutex
}

// TunnelStatus represents tunnel state
type TunnelStatus struct {
	State      string // "active", "inactive", "error"
	LastSeen   time.Time
	TxBytes    string
	RxBytes    string
	Handshakes int64
	Latency    time.Duration
}

// TunnelMetrics tracks tunnel performance
type TunnelMetrics struct {
	Throughput int64         // Current throughput in bps
	PacketLoss float64       // Packet loss percentage
	Jitter     time.Duration // Network jitter
	LastUpdate time.Time
}

// TunnelScaler manages dynamic tunnel scaling (placeholder for now)
type TunnelScaler struct {
	// Implementation will be moved from dynamic-scaling.go
}

// NewTunnelManager creates a new tunnel manager
func NewTunnelManager(config *TunnelConfig) *TunnelManager {
	if config == nil {
		config = &TunnelConfig{
			MinTunnels: 1,
			MaxTunnels: 8,
			BaseCIDR:   "10.100.0.0/16",
			MTU:        1420,
			ListenPort: 51820,
		}
	}

	return &TunnelManager{
		config:  config,
		tunnels: make(map[int]*WireGuardTunnel),
	}
}

// CreateTunnels creates the specified number of tunnels
func (tm *TunnelManager) CreateTunnels(count int) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if count > tm.config.MaxTunnels {
		return fmt.Errorf("requested tunnel count (%d) exceeds maximum (%d)",
			count, tm.config.MaxTunnels)
	}

	for i := 0; i < count; i++ {
		if err := tm.createTunnel(i); err != nil {
			return fmt.Errorf("failed to create tunnel %d: %w", i, err)
		}
	}

	return nil
}

// AddTunnel adds a new tunnel
func (tm *TunnelManager) AddTunnel() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if len(tm.tunnels) >= tm.config.MaxTunnels {
		return fmt.Errorf("maximum tunnel count reached (%d)", tm.config.MaxTunnels)
	}

	nextID := len(tm.tunnels)
	return tm.createTunnel(nextID)
}

// RemoveTunnel removes a tunnel
func (tm *TunnelManager) RemoveTunnel() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if len(tm.tunnels) <= tm.config.MinTunnels {
		return fmt.Errorf("minimum tunnel count reached (%d)", tm.config.MinTunnels)
	}

	// Remove the highest numbered tunnel
	var highestID int
	for id := range tm.tunnels {
		if id > highestID {
			highestID = id
		}
	}

	return tm.destroyTunnel(highestID)
}

// ConfigureECMP configures Equal Cost Multi-Path routing for tunnels
func (tm *TunnelManager) ConfigureECMP() error {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	// Implementation placeholder
	fmt.Printf("Configuring ECMP for %d tunnels\n", len(tm.tunnels))
	return nil
}

// GetActiveTunnels returns status of all active tunnels
func (tm *TunnelManager) GetActiveTunnels() ([]TunnelStatus, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var statuses []TunnelStatus
	for _, tunnel := range tm.tunnels {
		tunnel.mu.RLock()
		statuses = append(statuses, tunnel.Status)
		tunnel.mu.RUnlock()
	}

	return statuses, nil
}

// MonitorPerformance returns a channel for tunnel metrics
func (tm *TunnelManager) MonitorPerformance() <-chan TunnelMetrics {
	metrics := make(chan TunnelMetrics, 10)

	go func() {
		defer close(metrics)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			tm.mu.RLock()
			for _, tunnel := range tm.tunnels {
				tunnel.mu.RLock()
				if tunnel.Status.State == "active" {
					metrics <- tunnel.Metrics
				}
				tunnel.mu.RUnlock()
			}
			tm.mu.RUnlock()
		}
	}()

	return metrics
}

// createTunnel creates a single WireGuard tunnel
func (tm *TunnelManager) createTunnel(id int) error {
	tunnel := &WireGuardTunnel{
		ID:        id,
		Interface: fmt.Sprintf("wg%d", id),
		Port:      tm.config.ListenPort + id,
		Status: TunnelStatus{
			State: "inactive",
		},
	}

	// Generate WireGuard keys
	if err := tm.generateWireGuardKeys(tunnel); err != nil {
		return err
	}

	// Configure interface
	if err := tm.configureInterface(tunnel); err != nil {
		return err
	}

	tm.tunnels[id] = tunnel
	fmt.Printf("Created tunnel %d on interface %s\n", id, tunnel.Interface)

	return nil
}

// destroyTunnel removes a WireGuard tunnel
func (tm *TunnelManager) destroyTunnel(id int) error {
	tunnel, exists := tm.tunnels[id]
	if !exists {
		return fmt.Errorf("tunnel %d does not exist", id)
	}

	// Bring down interface (placeholder)
	if err := tm.teardownInterface(tunnel); err != nil {
		return err
	}

	delete(tm.tunnels, id)
	fmt.Printf("Destroyed tunnel %d\n", id)

	return nil
}

// generateWireGuardKeys generates private/public key pair
func (tm *TunnelManager) generateWireGuardKeys(tunnel *WireGuardTunnel) error {
	privateKey, publicKey, err := GenerateWireGuardKeys()
	if err != nil {
		return fmt.Errorf("failed to generate WireGuard keys: %w", err)
	}

	tunnel.PrivateKey = privateKey
	tunnel.PublicKey = publicKey

	fmt.Printf("Generated keys for tunnel %d (public: %s...)\n", tunnel.ID, publicKey[:20])
	return nil
}

// configureInterface configures the WireGuard interface
func (tm *TunnelManager) configureInterface(tunnel *WireGuardTunnel) error {
	// Calculate tunnel IP address
	tunnelIP := tm.calculateTunnelIP(tunnel.ID)

	// Create WireGuard configuration
	wgConfig := &WireGuardConfig{
		Interface:  tunnel.Interface,
		PrivateKey: tunnel.PrivateKey,
		PublicKey:  tunnel.PublicKey,
		ListenPort: tunnel.Port,
		Address:    tunnelIP,
		MTU:        tm.config.MTU,
		// Peer configuration will be added when connecting to AWS bastion
		AllowedIPs: "0.0.0.0/0",
	}

	// Create the WireGuard interface
	if err := tm.CreateWireGuardInterface(wgConfig); err != nil {
		return fmt.Errorf("failed to create WireGuard interface: %w", err)
	}

	tunnel.Status.State = "active"
	tunnel.Status.LastSeen = time.Now()
	tunnel.EndpointIP = tunnelIP

	return nil
}

// teardownInterface tears down the WireGuard interface
func (tm *TunnelManager) teardownInterface(tunnel *WireGuardTunnel) error {
	// Destroy the actual WireGuard interface
	if err := tm.DestroyWireGuardInterface(tunnel.Interface); err != nil {
		return fmt.Errorf("failed to destroy WireGuard interface: %w", err)
	}

	tunnel.Status.State = "inactive"
	return nil
}

// calculateTunnelIP calculates the IP address for a tunnel
func (tm *TunnelManager) calculateTunnelIP(tunnelID int) string {
	// Parse base CIDR to get network address
	// For simplicity, assume 10.100.0.0/16 and assign 10.100.X.1 to each tunnel
	return fmt.Sprintf("10.100.%d.1/24", tunnelID+1)
}
