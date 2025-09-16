package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
	"time"
)

// ScalingPhase defines the current scaling phase
type ScalingPhase int

const (
	PhaseVertical   ScalingPhase = iota // Single instance, multiple tunnels
	PhaseHorizontal                     // Multiple instances
)

// ClusterManager handles both single and multi-instance scaling
type ClusterManager struct {
	currentPhase     ScalingPhase
	instances        map[string]*InstanceManager
	tunnelManager    *TunnelManager
	scalingConfig    *PhaseConfig
	mu               sync.RWMutex
}

// PhaseConfig defines scaling behavior for each phase
type PhaseConfig struct {
	// Phase 1: Vertical scaling (single instance)
	MaxTunnelsPerInstance int     `yaml:"max_tunnels_per_instance"` // 8 tunnels max
	VerticalThreshold     float64 `yaml:"vertical_threshold"`       // 85% util triggers horizontal
	
	// Phase 2: Horizontal scaling (multiple instances)
	MaxInstances          int     `yaml:"max_instances"`            // 4 instances max
	MaxAZs                int     `yaml:"max_azs"`                  // Spread across 2-3 AZs
	HorizontalThreshold   float64 `yaml:"horizontal_threshold"`     // 90% util triggers new instance
	
	// Process pinning configuration
	EnableProcessPinning  bool    `yaml:"enable_process_pinning"`   // Pin WireGuard to cores
	ReserveCores          int     `yaml:"reserve_cores"`            // Reserve cores for system
}

// InstanceManager manages a single EC2 instance and its tunnels
type InstanceManager struct {
	InstanceID       string
	PrivateIP        string
	PublicIP         string
	AvailabilityZone string
	TunnelCount      int
	ProcessAffinity  map[int]int    // tunnel_id -> cpu_core mapping
	Performance      InstancePerf
	mu               sync.RWMutex
}

// InstancePerf tracks instance performance metrics
type InstancePerf struct {
	CPUUtilization    float64   // Overall CPU usage
	NetworkUtilization float64  // Network bandwidth usage
	TunnelUtilization []float64 // Per-tunnel utilization
	LastUpdate        time.Time
}

// DTNClientConfig optimizes for Data Transfer Node environments
type DTNClientConfig struct {
	EnableSRIOV          bool `yaml:"enable_sriov"`           // SR-IOV for DTN performance
	LargeMTU             int  `yaml:"large_mtu"`              // Jumbo frames if supported
	MultiQueueNIC        bool `yaml:"multi_queue_nic"`        // Multiple TX/RX queues
	CPUGovernor          string `yaml:"cpu_governor"`         // performance, powersave
	TCPCongestionControl string `yaml:"tcp_congestion"`       // bbr, cubic
}

// NewClusterManager creates a new cluster manager
func NewClusterManager(config *PhaseConfig) *ClusterManager {
	if config == nil {
		config = &PhaseConfig{
			MaxTunnelsPerInstance: 8,
			VerticalThreshold:     0.85,
			MaxInstances:          4,
			MaxAZs:                3,
			HorizontalThreshold:   0.90,
			EnableProcessPinning:  true,
			ReserveCores:          2, // Reserve 2 cores for system
		}
	}

	return &ClusterManager{
		currentPhase:  PhaseVertical,
		instances:     make(map[string]*InstanceManager),
		scalingConfig: config,
	}
}

// ScaleUp determines whether to scale vertically or horizontally
func (cm *ClusterManager) ScaleUp(totalUtilization float64) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	switch cm.currentPhase {
	case PhaseVertical:
		return cm.scaleVertical(totalUtilization)
	case PhaseHorizontal:
		return cm.scaleHorizontal(totalUtilization)
	default:
		return fmt.Errorf("unknown scaling phase")
	}
}

// scaleVertical adds tunnels to the single instance
func (cm *ClusterManager) scaleVertical(utilization float64) error {
	if len(cm.instances) == 0 {
		return fmt.Errorf("no instances available for vertical scaling")
	}

	// Get the primary instance
	var primaryInstance *InstanceManager
	for _, instance := range cm.instances {
		primaryInstance = instance
		break
	}

	// Check if we can add more tunnels
	if primaryInstance.TunnelCount >= cm.scalingConfig.MaxTunnelsPerInstance {
		if utilization > cm.scalingConfig.VerticalThreshold {
			// Transition to horizontal scaling
			cm.currentPhase = PhaseHorizontal
			fmt.Printf("Transitioning to horizontal scaling phase (utilization: %.1f%%)\n", 
				utilization*100)
			return cm.scaleHorizontal(utilization)
		}
		return fmt.Errorf("max tunnels reached on single instance")
	}

	// Add new tunnel with process pinning
	newTunnelID := primaryInstance.TunnelCount
	if err := cm.addTunnelWithPinning(primaryInstance, newTunnelID); err != nil {
		return fmt.Errorf("failed to add tunnel: %w", err)
	}

	primaryInstance.TunnelCount++
	fmt.Printf("Added tunnel %d to instance %s (total: %d tunnels)\n", 
		newTunnelID, primaryInstance.InstanceID, primaryInstance.TunnelCount)

	return nil
}

// scaleHorizontal adds new instances
func (cm *ClusterManager) scaleHorizontal(utilization float64) error {
	if len(cm.instances) >= cm.scalingConfig.MaxInstances {
		return fmt.Errorf("max instances reached (%d)", cm.scalingConfig.MaxInstances)
	}

	if utilization <= cm.scalingConfig.HorizontalThreshold {
		return fmt.Errorf("utilization too low for horizontal scaling")
	}

	// Select AZ for new instance (round-robin across AZs)
	selectedAZ := cm.selectOptimalAZ()
	
	// Create new instance
	newInstance, err := cm.createNewInstance(selectedAZ)
	if err != nil {
		return fmt.Errorf("failed to create new instance: %w", err)
	}

	cm.instances[newInstance.InstanceID] = newInstance
	fmt.Printf("Created new instance %s in AZ %s (%d total instances)\n", 
		newInstance.InstanceID, selectedAZ, len(cm.instances))

	// Add initial tunnel to new instance
	if err := cm.addTunnelWithPinning(newInstance, 0); err != nil {
		return fmt.Errorf("failed to add initial tunnel to new instance: %w", err)
	}

	newInstance.TunnelCount = 1
	return nil
}

// addTunnelWithPinning adds a tunnel and pins the WireGuard process to a specific CPU core
func (cm *ClusterManager) addTunnelWithPinning(instance *InstanceManager, tunnelID int) error {
	if !cm.scalingConfig.EnableProcessPinning {
		// Add tunnel without pinning
		return cm.tunnelManager.AddTunnel()
	}

	// Calculate optimal CPU core for this tunnel
	coreID := cm.calculateOptimalCore(instance, tunnelID)
	
	// Add tunnel
	if err := cm.tunnelManager.AddTunnel(); err != nil {
		return err
	}

	// Pin WireGuard process to specific core
	if err := cm.pinWireGuardProcess(tunnelID, coreID); err != nil {
		fmt.Printf("Warning: failed to pin tunnel %d to core %d: %v\n", tunnelID, coreID, err)
		// Continue anyway - pinning is optimization, not requirement
	}

	// Track the affinity mapping
	instance.mu.Lock()
	if instance.ProcessAffinity == nil {
		instance.ProcessAffinity = make(map[int]int)
	}
	instance.ProcessAffinity[tunnelID] = coreID
	instance.mu.Unlock()

	fmt.Printf("Tunnel %d pinned to CPU core %d\n", tunnelID, coreID)
	return nil
}

// calculateOptimalCore determines the best CPU core for a new tunnel
func (cm *ClusterManager) calculateOptimalCore(instance *InstanceManager, tunnelID int) int {
	numCPUs := runtime.NumCPU()
	reservedCores := cm.scalingConfig.ReserveCores
	availableCores := numCPUs - reservedCores

	if availableCores <= 0 {
		// Fallback if misconfigured
		return tunnelID % numCPUs
	}

	// Round-robin assignment starting after reserved cores
	coreOffset := reservedCores + (tunnelID % availableCores)
	return coreOffset
}

// pinWireGuardProcess pins a WireGuard process to a specific CPU core
func (cm *ClusterManager) pinWireGuardProcess(tunnelID, coreID int) error {
	// Find WireGuard process for this tunnel
	wgInterface := fmt.Sprintf("wg%d", tunnelID)
	
	// Get WireGuard process PID
	cmd := exec.Command("pgrep", "-f", wgInterface)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to find WireGuard process for %s: %w", wgInterface, err)
	}

	pid, err := strconv.Atoi(string(output[:len(output)-1])) // Remove newline
	if err != nil {
		return fmt.Errorf("failed to parse PID: %w", err)
	}

	// Pin process to CPU core using taskset
	cmd = exec.Command("sudo", "taskset", "-cp", strconv.Itoa(coreID), strconv.Itoa(pid))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to pin process %d to core %d: %w", pid, coreID, err)
	}

	return nil
}

// selectOptimalAZ selects the best availability zone for a new instance
func (cm *ClusterManager) selectOptimalAZ() string {
	// Count instances per AZ
	azCounts := make(map[string]int)
	for _, instance := range cm.instances {
		azCounts[instance.AvailabilityZone]++
	}

	// Find AZ with fewest instances
	minCount := cm.scalingConfig.MaxInstances
	selectedAZ := "us-west-2a" // Default fallback

	for az, count := range azCounts {
		if count < minCount {
			minCount = count
			selectedAZ = az
		}
	}

	// If all AZs have equal instances, cycle through them
	if minCount > 0 {
		azOptions := []string{"us-west-2a", "us-west-2b", "us-west-2c"}
		return azOptions[len(cm.instances)%len(azOptions)]
	}

	return selectedAZ
}

// createNewInstance provisions a new EC2 instance
func (cm *ClusterManager) createNewInstance(az string) (*InstanceManager, error) {
	// This would call AWS APIs to create instance
	// For now, return a mock instance
	instance := &InstanceManager{
		InstanceID:       fmt.Sprintf("i-%d", time.Now().Unix()),
		PrivateIP:        fmt.Sprintf("10.0.%d.10", len(cm.instances)+1),
		PublicIP:         fmt.Sprintf("54.%d.%d.%d", len(cm.instances)+1, 100, 100),
		AvailabilityZone: az,
		TunnelCount:      0,
		ProcessAffinity:  make(map[int]int),
	}

	return instance, nil
}

// OptimizeForDTN configures the client for Data Transfer Node environments
func OptimizeForDTN() *DTNClientConfig {
	return &DTNClientConfig{
		EnableSRIOV:          true,
		LargeMTU:             9000, // Jumbo frames for DTN networks
		MultiQueueNIC:        true,
		CPUGovernor:          "performance",
		TCPCongestionControl: "bbr",
	}
}

// ApplyDTNOptimizations configures the system for DTN performance
func (config *DTNClientConfig) ApplyDTNOptimizations() error {
	var errors []error

	// Set CPU governor to performance
	if err := exec.Command("sudo", "cpupower", "frequency-set", 
		"-g", config.CPUGovernor).Run(); err != nil {
		errors = append(errors, fmt.Errorf("failed to set CPU governor: %w", err))
	}

	// Configure TCP congestion control
	if err := exec.Command("sudo", "sysctl", "-w", 
		fmt.Sprintf("net.ipv4.tcp_congestion_control=%s", config.TCPCongestionControl)).Run(); err != nil {
		errors = append(errors, fmt.Errorf("failed to set congestion control: %w", err))
	}

	// Enable large MTU if supported
	if config.LargeMTU > 1500 {
		fmt.Printf("Note: Large MTU (%d) configured - ensure DTN network supports jumbo frames\n", 
			config.LargeMTU)
	}

	// Configure network queue optimizations for DTN
	optimizations := []string{
		"net.core.rmem_max=134217728",     // 128MB receive buffer
		"net.core.wmem_max=134217728",     // 128MB send buffer
		"net.ipv4.tcp_rmem=4096 87380 134217728",
		"net.ipv4.tcp_wmem=4096 65536 134217728",
		"net.core.netdev_max_backlog=5000",
		"net.ipv4.tcp_window_scaling=1",
	}

	for _, opt := range optimizations {
		if err := exec.Command("sudo", "sysctl", "-w", opt).Run(); err != nil {
			errors = append(errors, fmt.Errorf("failed to apply optimization %s: %w", opt, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("DTN optimization errors: %v", errors)
	}

	fmt.Println("DTN optimizations applied successfully")
	return nil
}

// GetClusterStatus returns current cluster state
func (cm *ClusterManager) GetClusterStatus() ClusterStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	status := ClusterStatus{
		Phase:         cm.currentPhase,
		InstanceCount: len(cm.instances),
		TotalTunnels:  0,
		Instances:     make([]InstanceStatus, 0, len(cm.instances)),
	}

	for _, instance := range cm.instances {
		instance.mu.RLock()
		instanceStatus := InstanceStatus{
			InstanceID:     instance.InstanceID,
			AZ:             instance.AvailabilityZone,
			TunnelCount:    instance.TunnelCount,
			CPUUtil:        instance.Performance.CPUUtilization,
			NetworkUtil:    instance.Performance.NetworkUtilization,
			ProcessAffinity: make(map[int]int),
		}
		
		// Copy process affinity map
		for tunnelID, coreID := range instance.ProcessAffinity {
			instanceStatus.ProcessAffinity[tunnelID] = coreID
		}
		
		status.TotalTunnels += instance.TunnelCount
		status.Instances = append(status.Instances, instanceStatus)
		instance.mu.RUnlock()
	}

	return status
}

// ClusterStatus represents current cluster state
type ClusterStatus struct {
	Phase         ScalingPhase
	InstanceCount int
	TotalTunnels  int
	Instances     []InstanceStatus
}

// InstanceStatus represents single instance state
type InstanceStatus struct {
	InstanceID      string
	AZ              string
	TunnelCount     int
	CPUUtil         float64
	NetworkUtil     float64
	ProcessAffinity map[int]int // tunnel_id -> cpu_core
}