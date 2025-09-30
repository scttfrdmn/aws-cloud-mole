package monitoring

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/research-computing/mole/internal/logger"
)

// SystemMetrics holds system performance metrics
type SystemMetrics struct {
	Timestamp      time.Time     `json:"timestamp"`
	CPUUsage       float64       `json:"cpu_usage"`        // Percentage
	MemoryUsage    MemoryMetrics `json:"memory_usage"`     // Memory stats
	NetworkStats   NetworkStats  `json:"network_stats"`    // Network I/O
	TunnelCount    int           `json:"tunnel_count"`     // Active tunnel count
	ProcessMetrics ProcessInfo   `json:"process_metrics"`  // Process-specific metrics
}

// MemoryMetrics holds memory usage information
type MemoryMetrics struct {
	TotalMB    uint64  `json:"total_mb"`
	UsedMB     uint64  `json:"used_mb"`
	FreeMB     uint64  `json:"free_mb"`
	UsagePercent float64 `json:"usage_percent"`
}

// NetworkStats holds network interface statistics
type NetworkStats struct {
	BytesIn     uint64 `json:"bytes_in"`
	BytesOut    uint64 `json:"bytes_out"`
	PacketsIn   uint64 `json:"packets_in"`
	PacketsOut  uint64 `json:"packets_out"`
	ErrorsIn    uint64 `json:"errors_in"`
	ErrorsOut   uint64 `json:"errors_out"`
}

// ProcessInfo holds process-specific metrics
type ProcessInfo struct {
	PID         int     `json:"pid"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryMB    uint64  `json:"memory_mb"`
	OpenFiles   int     `json:"open_files"`
	Goroutines  int     `json:"goroutines"`
}

// TunnelMetrics holds tunnel-specific performance data
type TunnelMetrics struct {
	Interface    string    `json:"interface"`
	Timestamp    time.Time `json:"timestamp"`
	BytesIn      uint64    `json:"bytes_in"`
	BytesOut     uint64    `json:"bytes_out"`
	PacketsIn    uint64    `json:"packets_in"`
	PacketsOut   uint64    `json:"packets_out"`
	Latency      float64   `json:"latency_ms"`    // Round-trip latency in ms
	Throughput   float64   `json:"throughput_mbps"` // Throughput in Mbps
	PeerConnected bool     `json:"peer_connected"`
}

// Monitor provides system and application monitoring
type Monitor struct {
	logger         *logger.Logger
	updateInterval time.Duration
	running        bool
	stopChan       chan struct{}
	metricsChan    chan SystemMetrics
	tunnelMetrics  map[string]*TunnelMetrics
	mutex          sync.RWMutex

	// Previous metrics for delta calculation
	prevNetworkStats NetworkStats
	prevTime         time.Time
}

// NewMonitor creates a new monitoring instance
func NewMonitor(updateInterval time.Duration) *Monitor {
	monitor := &Monitor{
		updateInterval: updateInterval,
		stopChan:       make(chan struct{}),
		metricsChan:    make(chan SystemMetrics, 100),
		tunnelMetrics:  make(map[string]*TunnelMetrics),
		prevTime:       time.Now(),
	}

	// Initialize logger (ignore errors for now)
	if l, err := logger.New(logger.Config{Component: "monitor", Level: logger.LevelInfo}); err == nil {
		monitor.logger = l
	}

	return monitor
}

// Start begins monitoring system metrics
func (m *Monitor) Start(ctx context.Context) error {
	if m.running {
		return fmt.Errorf("monitor is already running")
	}

	m.running = true
	m.logger.Info("Starting system monitor", "interval", m.updateInterval)

	go m.monitorLoop(ctx)
	return nil
}

// Stop stops the monitoring system
func (m *Monitor) Stop() {
	if !m.running {
		return
	}

	m.logger.Info("Stopping system monitor")
	close(m.stopChan)
	m.running = false
}

// GetMetrics returns the latest system metrics
func (m *Monitor) GetMetrics() (SystemMetrics, error) {
	return m.collectSystemMetrics()
}

// GetTunnelMetrics returns metrics for a specific tunnel
func (m *Monitor) GetTunnelMetrics(interfaceName string) (*TunnelMetrics, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if metrics, exists := m.tunnelMetrics[interfaceName]; exists {
		return metrics, nil
	}
	return nil, fmt.Errorf("tunnel %s not found", interfaceName)
}

// GetAllTunnelMetrics returns metrics for all tunnels
func (m *Monitor) GetAllTunnelMetrics() map[string]*TunnelMetrics {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	result := make(map[string]*TunnelMetrics)
	for k, v := range m.tunnelMetrics {
		result[k] = v
	}
	return result
}

// AddTunnel starts monitoring a tunnel interface
func (m *Monitor) AddTunnel(interfaceName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.tunnelMetrics[interfaceName] = &TunnelMetrics{
		Interface: interfaceName,
		Timestamp: time.Now(),
	}
	m.logger.Info("Added tunnel to monitoring", "interface", interfaceName)
}

// RemoveTunnel stops monitoring a tunnel interface
func (m *Monitor) RemoveTunnel(interfaceName string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.tunnelMetrics, interfaceName)
	m.logger.Info("Removed tunnel from monitoring", "interface", interfaceName)
}

// monitorLoop is the main monitoring loop
func (m *Monitor) monitorLoop(ctx context.Context) {
	ticker := time.NewTicker(m.updateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopChan:
			return
		case <-ticker.C:
			if metrics, err := m.collectSystemMetrics(); err != nil {
				m.logger.Error("Failed to collect system metrics", "error", err)
			} else {
				m.updateTunnelMetrics()
				select {
				case m.metricsChan <- metrics:
				default:
					// Channel full, skip this update
				}
			}
		}
	}
}

// collectSystemMetrics gathers current system performance data
func (m *Monitor) collectSystemMetrics() (SystemMetrics, error) {
	metrics := SystemMetrics{
		Timestamp: time.Now(),
	}

	// Collect CPU usage
	if cpu, err := m.getCPUUsage(); err == nil {
		metrics.CPUUsage = cpu
	}

	// Collect memory usage
	if mem, err := m.getMemoryUsage(); err == nil {
		metrics.MemoryUsage = mem
	}

	// Collect network stats
	if net, err := m.getNetworkStats(); err == nil {
		metrics.NetworkStats = net
	}

	// Collect process metrics
	if proc, err := m.getProcessMetrics(); err == nil {
		metrics.ProcessMetrics = proc
	}

	// Get tunnel count
	m.mutex.RLock()
	metrics.TunnelCount = len(m.tunnelMetrics)
	m.mutex.RUnlock()

	return metrics, nil
}

// getCPUUsage returns current CPU usage percentage
func (m *Monitor) getCPUUsage() (float64, error) {
	// On macOS, use iostat or top to get CPU usage
	cmd := exec.Command("sh", "-c", "top -l 1 -n 0 | grep 'CPU usage' | awk '{print $3}' | sed 's/%//'")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	cpuStr := strings.TrimSpace(string(output))
	return strconv.ParseFloat(cpuStr, 64)
}

// getMemoryUsage returns current memory usage information
func (m *Monitor) getMemoryUsage() (MemoryMetrics, error) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	// Convert bytes to MB
	totalMB := mem.Sys / 1024 / 1024
	usedMB := mem.Alloc / 1024 / 1024
	freeMB := totalMB - usedMB
	usagePercent := float64(usedMB) / float64(totalMB) * 100

	return MemoryMetrics{
		TotalMB:      totalMB,
		UsedMB:       usedMB,
		FreeMB:       freeMB,
		UsagePercent: usagePercent,
	}, nil
}

// getNetworkStats returns network interface statistics
func (m *Monitor) getNetworkStats() (NetworkStats, error) {
	// On macOS, use netstat to get network statistics
	cmd := exec.Command("netstat", "-ib")
	output, err := cmd.Output()
	if err != nil {
		return NetworkStats{}, err
	}

	var stats NetworkStats
	lines := strings.Split(string(output), "\n")

	// Parse network statistics (simplified)
	for _, line := range lines {
		if strings.Contains(line, "en0") || strings.Contains(line, "utun") {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				if bytesIn, err := strconv.ParseUint(fields[6], 10, 64); err == nil {
					stats.BytesIn += bytesIn
				}
				if bytesOut, err := strconv.ParseUint(fields[9], 10, 64); err == nil {
					stats.BytesOut += bytesOut
				}
			}
		}
	}

	return stats, nil
}

// getProcessMetrics returns process-specific performance metrics
func (m *Monitor) getProcessMetrics() (ProcessInfo, error) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	return ProcessInfo{
		PID:        0, // Will need to get actual PID
		MemoryMB:   mem.Alloc / 1024 / 1024,
		Goroutines: runtime.NumGoroutine(),
	}, nil
}

// updateTunnelMetrics updates metrics for all monitored tunnels
func (m *Monitor) updateTunnelMetrics() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for interfaceName, metrics := range m.tunnelMetrics {
		if updated, err := m.collectTunnelMetrics(interfaceName); err == nil {
			*metrics = *updated
		} else {
			m.logger.Debug("Failed to update tunnel metrics", "interface", interfaceName, "error", err)
		}
	}
}

// collectTunnelMetrics gathers metrics for a specific tunnel
func (m *Monitor) collectTunnelMetrics(interfaceName string) (*TunnelMetrics, error) {
	metrics := &TunnelMetrics{
		Interface: interfaceName,
		Timestamp: time.Now(),
	}

	// Get WireGuard stats
	cmd := exec.Command("sudo", "-A", "wg", "show", interfaceName, "dump")
	cmd.Env = append(cmd.Env, "SUDO_ASKPASS=/opt/homebrew/bin/askpass")

	output, err := cmd.Output()
	if err != nil {
		return metrics, fmt.Errorf("failed to get wg stats: %w", err)
	}

	// Parse WireGuard dump output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) >= 6 {
			// Fields: interface, private-key, public-key, preshared-key, endpoint, allowed-ips, latest-handshake, transfer-rx, transfer-tx, persistent-keepalive
			if len(fields) >= 9 {
				if rxBytes, err := strconv.ParseUint(fields[7], 10, 64); err == nil {
					metrics.BytesIn = rxBytes
				}
				if txBytes, err := strconv.ParseUint(fields[8], 10, 64); err == nil {
					metrics.BytesOut = txBytes
				}
			}
		}
	}

	return metrics, nil
}

// PrintSystemStatus prints a htop-style system status display
func (m *Monitor) PrintSystemStatus() error {
	metrics, err := m.GetMetrics()
	if err != nil {
		return err
	}

	fmt.Printf("\n=== AWS Cloud Mole System Status ===\n")
	fmt.Printf("Time: %s\n", metrics.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("CPU Usage: %.1f%%\n", metrics.CPUUsage)
	fmt.Printf("Memory: %.1fMB / %.1fMB (%.1f%%)\n",
		float64(metrics.MemoryUsage.UsedMB),
		float64(metrics.MemoryUsage.TotalMB),
		metrics.MemoryUsage.UsagePercent)
	fmt.Printf("Tunnels: %d active\n", metrics.TunnelCount)
	fmt.Printf("Network: %.1f MB in, %.1f MB out\n",
		float64(metrics.NetworkStats.BytesIn)/1024/1024,
		float64(metrics.NetworkStats.BytesOut)/1024/1024)
	fmt.Printf("Goroutines: %d\n", metrics.ProcessMetrics.Goroutines)

	// Print tunnel details
	if metrics.TunnelCount > 0 {
		fmt.Printf("\n=== Active Tunnels ===\n")
		tunnelMetrics := m.GetAllTunnelMetrics()
		for interfaceName, tMetrics := range tunnelMetrics {
			fmt.Printf("%-8s | In: %8.1f MB | Out: %8.1f MB | Updated: %s\n",
				interfaceName,
				float64(tMetrics.BytesIn)/1024/1024,
				float64(tMetrics.BytesOut)/1024/1024,
				tMetrics.Timestamp.Format("15:04:05"))
		}
	}

	fmt.Printf("\n")
	return nil
}