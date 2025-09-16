package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// TunnelScaler manages dynamic tunnel scaling
type TunnelScaler struct {
	config          *ScalingConfig
	tunnelManager   *TunnelManager
	awsClient       *AWSClient
	metrics         *TunnelMetrics
	scalingDecision chan ScalingAction
	stopCh          chan struct{}
	mu              sync.RWMutex
}

// ScalingConfig defines scaling behavior
type ScalingConfig struct {
	MinTunnels        int           `yaml:"min_tunnels"`         // Minimum tunnels (default: 1)
	MaxTunnels        int           `yaml:"max_tunnels"`         // Maximum tunnels (default: 8)
	ScaleUpThreshold  float64       `yaml:"scale_up_threshold"`  // Scale up at 80% utilization
	ScaleDownThreshold float64      `yaml:"scale_down_threshold"` // Scale down at 30% utilization
	ScaleInterval     time.Duration `yaml:"scale_interval"`      // Check every 30 seconds
	ScaleUpCooldown   time.Duration `yaml:"scale_up_cooldown"`   // Wait 2 minutes before scaling up again
	ScaleDownCooldown time.Duration `yaml:"scale_down_cooldown"` // Wait 10 minutes before scaling down
	ElephantFlowThreshold int64     `yaml:"elephant_flow_threshold"` // 100MB+ flows trigger immediate scaling
	BurstDetectionWindow  time.Duration `yaml:"burst_detection_window"`  // 5 minute window for burst detection
}

// ScalingAction represents a scaling decision
type ScalingAction struct {
	Action      string    // "scale_up", "scale_down", "no_action"
	TargetCount int       // Target number of tunnels
	Reason      string    // Human-readable reason
	Timestamp   time.Time
}

// TunnelMetrics tracks tunnel performance
type TunnelMetrics struct {
	mu                sync.RWMutex
	TotalThroughput   int64     // Bytes per second across all tunnels
	PerTunnelThroughput []int64 // Bytes per second per tunnel
	UtilizationPct    float64   // Overall utilization percentage
	ElephantFlows     int       // Number of active elephant flows
	LastScaleAction   time.Time // When we last scaled
	BurstHistory      []BurstEvent // Recent burst events
}

// BurstEvent tracks traffic bursts
type BurstEvent struct {
	Timestamp   time.Time
	PeakThroughput int64
	Duration    time.Duration
	TunnelCount int
}

// GravitonInstanceConfig optimizes instance selection for cost
type GravitonInstanceConfig struct {
	InstanceType     types.InstanceType `yaml:"instance_type"`
	BaselineBandwidth int64             `yaml:"baseline_mbps"`    // Sustainable bandwidth
	BurstBandwidth   int64             `yaml:"burst_mbps"`       // Burst bandwidth
	BurstCredits     int               `yaml:"burst_credits"`    // Minutes of burst
	MonthlyCost      float64          `yaml:"monthly_cost_usd"` // Estimated cost
	MaxTunnels       int              `yaml:"max_tunnels"`      // Recommended max tunnels
}

// NewTunnelScaler creates a new tunnel scaler
func NewTunnelScaler(config *ScalingConfig, tm *TunnelManager, aws *AWSClient) *TunnelScaler {
	if config == nil {
		config = &ScalingConfig{
			MinTunnels:            1,
			MaxTunnels:            8,
			ScaleUpThreshold:      0.80,
			ScaleDownThreshold:    0.30,
			ScaleInterval:         30 * time.Second,
			ScaleUpCooldown:       2 * time.Minute,
			ScaleDownCooldown:     10 * time.Minute,
			ElephantFlowThreshold: 100 * 1024 * 1024, // 100 MB
			BurstDetectionWindow:  5 * time.Minute,
		}
	}

	return &TunnelScaler{
		config:          config,
		tunnelManager:   tm,
		awsClient:       aws,
		metrics:         &TunnelMetrics{},
		scalingDecision: make(chan ScalingAction, 10),
		stopCh:          make(chan struct{}),
	}
}

// GetOptimalGravitonInstance returns the best Graviton instance for the workload
func GetOptimalGravitonInstance(targetThroughput int64, budget float64) *GravitonInstanceConfig {
	instances := []GravitonInstanceConfig{
		{
			InstanceType:      types.InstanceTypeT4gNano,
			BaselineBandwidth: 32,   // 32 Mbps baseline
			BurstBandwidth:    5000, // 5 Gbps burst
			BurstCredits:      15,   // 15 minutes
			MonthlyCost:       1.31, // $0.0042/hour
			MaxTunnels:        1,
		},
		{
			InstanceType:      types.InstanceTypeT4gMicro,
			BaselineBandwidth: 62,   // 62 Mbps baseline
			BurstBandwidth:    5000, // 5 Gbps burst
			BurstCredits:      30,   // 30 minutes
			MonthlyCost:       2.63, // $0.0084/hour
			MaxTunnels:        2,
		},
		{
			InstanceType:      types.InstanceTypeT4gSmall,
			BaselineBandwidth: 125,  // 125 Mbps baseline
			BurstBandwidth:    5000, // 5 Gbps burst
			BurstCredits:      60,   // 60 minutes
			MonthlyCost:       5.26, // $0.0168/hour
			MaxTunnels:        4,
		},
		{
			InstanceType:      types.InstanceTypeC6gnMedium,
			BaselineBandwidth: 3125,  // 3.125 Gbps sustained
			BurstBandwidth:    12500, // 12.5 Gbps burst
			BurstCredits:      0,     // No burst credits, sustained performance
			MonthlyCost:       27.00, // $0.0864/hour
			MaxTunnels:        6,
		},
		{
			InstanceType:      types.InstanceTypeC6gnLarge,
			BaselineBandwidth: 6250,  // 6.25 Gbps sustained
			BurstBandwidth:    25000, // 25 Gbps burst
			BurstCredits:      0,     // No burst credits
			MonthlyCost:       54.00, // $0.1728/hour
			MaxTunnels:        8,
		},
		{
			InstanceType:      types.InstanceTypeC6gnXlarge,
			BaselineBandwidth: 12500, // 12.5 Gbps sustained
			BurstBandwidth:    25000, // 25 Gbps burst
			BurstCredits:      0,     // No burst credits
			MonthlyCost:       108.00, // $0.3456/hour
			MaxTunnels:        12,
		},
	}

	// Find the most cost-effective instance that meets throughput requirements
	for _, instance := range instances {
		// Convert target throughput from bytes/sec to Mbps
		targetMbps := targetThroughput * 8 / 1024 / 1024

		// Check if instance can handle the sustained load
		if instance.BaselineBandwidth >= targetMbps && instance.MonthlyCost <= budget {
			return &instance
		}

		// For burst workloads, consider burst capacity
		if instance.BurstBandwidth >= targetMbps && instance.MonthlyCost <= budget {
			log.Printf("Instance %s can handle burst load but not sustained", instance.InstanceType)
			return &instance
		}
	}

	// If no instance fits budget, return the cheapest option with a warning
	log.Printf("Warning: No instance fits target throughput %d Mbps within budget $%.2f", 
		targetThroughput*8/1024/1024, budget)
	return &instances[0]
}

// Start begins the dynamic scaling loop
func (ts *TunnelScaler) Start() {
	go ts.scalingLoop()
	go ts.metricsCollection()
}

// Stop stops the dynamic scaling
func (ts *TunnelScaler) Stop() {
	close(ts.stopCh)
}

// scalingLoop continuously monitors and makes scaling decisions
func (ts *TunnelScaler) scalingLoop() {
	ticker := time.NewTicker(ts.config.ScaleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ts.stopCh:
			return
		case <-ticker.C:
			action := ts.evaluateScaling()
			if action.Action != "no_action" {
				ts.scalingDecision <- action
				ts.executeScaling(action)
			}
		}
	}
}

// metricsCollection gathers tunnel performance metrics
func (ts *TunnelScaler) metricsCollection() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ts.stopCh:
			return
		case <-ticker.C:
			ts.collectMetrics()
		}
	}
}

// collectMetrics gathers current tunnel performance data
func (ts *TunnelScaler) collectMetrics() {
	ts.metrics.mu.Lock()
	defer ts.metrics.mu.Unlock()

	// Get tunnel stats from WireGuard
	tunnels, err := ts.tunnelManager.GetActiveTunnels()
	if err != nil {
		log.Printf("Error collecting tunnel metrics: %v", err)
		return
	}

	var totalThroughput int64
	perTunnelThroughput := make([]int64, len(tunnels))
	elephantFlows := 0

	for i, tunnel := range tunnels {
		// Parse throughput from tunnel stats
		throughput := ts.parseThroughput(tunnel.TxBytes, tunnel.RxBytes)
		perTunnelThroughput[i] = throughput
		totalThroughput += throughput

		// Detect elephant flows (flows > threshold)
		if throughput > ts.config.ElephantFlowThreshold {
			elephantFlows++
		}
	}

	// Calculate utilization based on theoretical maximum
	// Assume 1.5 Gbps max per tunnel (user's measured limit)
	maxThroughput := int64(len(tunnels)) * 1500 * 1024 * 1024 // 1.5 Gbps per tunnel in bytes/sec
	utilization := float64(totalThroughput) / float64(maxThroughput)

	ts.metrics.TotalThroughput = totalThroughput
	ts.metrics.PerTunnelThroughput = perTunnelThroughput
	ts.metrics.UtilizationPct = utilization
	ts.metrics.ElephantFlows = elephantFlows

	// Detect burst events
	if ts.detectBurstEvent(totalThroughput, len(tunnels)) {
		burst := BurstEvent{
			Timestamp:      time.Now(),
			PeakThroughput: totalThroughput,
			TunnelCount:    len(tunnels),
		}
		ts.metrics.BurstHistory = append(ts.metrics.BurstHistory, burst)
		
		// Keep only recent burst history
		cutoff := time.Now().Add(-ts.config.BurstDetectionWindow)
		filtered := ts.metrics.BurstHistory[:0]
		for _, b := range ts.metrics.BurstHistory {
			if b.Timestamp.After(cutoff) {
				filtered = append(filtered, b)
			}
		}
		ts.metrics.BurstHistory = filtered
	}
}

// evaluateScaling determines if scaling action is needed
func (ts *TunnelScaler) evaluateScaling() ScalingAction {
	ts.metrics.mu.RLock()
	defer ts.metrics.mu.RUnlock()

	currentTunnels := len(ts.metrics.PerTunnelThroughput)
	now := time.Now()

	// Check cooldown periods
	scaleUpCooldown := now.Sub(ts.metrics.LastScaleAction) < ts.config.ScaleUpCooldown
	scaleDownCooldown := now.Sub(ts.metrics.LastScaleAction) < ts.config.ScaleDownCooldown

	// Immediate scaling for elephant flows
	if ts.metrics.ElephantFlows > currentTunnels && currentTunnels < ts.config.MaxTunnels && !scaleUpCooldown {
		return ScalingAction{
			Action:      "scale_up",
			TargetCount: int(math.Min(float64(ts.metrics.ElephantFlows), float64(ts.config.MaxTunnels))),
			Reason:      fmt.Sprintf("Elephant flow scaling: %d flows detected", ts.metrics.ElephantFlows),
			Timestamp:   now,
		}
	}

	// Scale up based on utilization
	if ts.metrics.UtilizationPct > ts.config.ScaleUpThreshold && 
	   currentTunnels < ts.config.MaxTunnels && !scaleUpCooldown {
		return ScalingAction{
			Action:      "scale_up",
			TargetCount: currentTunnels + 1,
			Reason:      fmt.Sprintf("High utilization: %.1f%%", ts.metrics.UtilizationPct*100),
			Timestamp:   now,
		}
	}

	// Scale down based on utilization
	if ts.metrics.UtilizationPct < ts.config.ScaleDownThreshold && 
	   currentTunnels > ts.config.MinTunnels && !scaleDownCooldown {
		return ScalingAction{
			Action:      "scale_down",
			TargetCount: currentTunnels - 1,
			Reason:      fmt.Sprintf("Low utilization: %.1f%%", ts.metrics.UtilizationPct*100),
			Timestamp:   now,
		}
	}

	// Burst-based scaling
	if len(ts.metrics.BurstHistory) >= 3 && currentTunnels < ts.config.MaxTunnels && !scaleUpCooldown {
		return ScalingAction{
			Action:      "scale_up",
			TargetCount: currentTunnels + 2, // More aggressive scaling for bursts
			Reason:      fmt.Sprintf("Burst pattern detected: %d recent bursts", len(ts.metrics.BurstHistory)),
			Timestamp:   now,
		}
	}

	return ScalingAction{
		Action:    "no_action",
		Reason:    "No scaling needed",
		Timestamp: now,
	}
}

// executeScaling performs the actual scaling action
func (ts *TunnelScaler) executeScaling(action ScalingAction) {
	ts.metrics.mu.Lock()
	ts.metrics.LastScaleAction = action.Timestamp
	ts.metrics.mu.Unlock()

	currentTunnels := len(ts.metrics.PerTunnelThroughput)
	
	log.Printf("Scaling action: %s from %d to %d tunnels. Reason: %s", 
		action.Action, currentTunnels, action.TargetCount, action.Reason)

	switch action.Action {
	case "scale_up":
		for i := currentTunnels; i < action.TargetCount; i++ {
			if err := ts.tunnelManager.AddTunnel(); err != nil {
				log.Printf("Failed to add tunnel %d: %v", i, err)
				break
			}
			log.Printf("Added tunnel %d", i)
		}
		
	case "scale_down":
		for i := currentTunnels; i > action.TargetCount; i-- {
			if err := ts.tunnelManager.RemoveTunnel(); err != nil {
				log.Printf("Failed to remove tunnel %d: %v", i-1, err)
				break
			}
			log.Printf("Removed tunnel %d", i-1)
		}
	}
}

// detectBurstEvent identifies sudden traffic increases
func (ts *TunnelScaler) detectBurstEvent(currentThroughput int64, tunnelCount int) bool {
	// Calculate expected throughput per tunnel
	avgPerTunnel := currentThroughput / int64(tunnelCount)
	
	// Burst threshold: 70% of single tunnel capacity (1.5 Gbps * 0.7 = 1.05 Gbps)
	burstThreshold := int64(1050 * 1024 * 1024) // 1.05 Gbps in bytes/sec
	
	return avgPerTunnel > burstThreshold
}

// parseThroughput extracts throughput from WireGuard stats
func (ts *TunnelScaler) parseThroughput(txBytes, rxBytes string) int64 {
	// This would parse the actual throughput from WireGuard stats
	// For now, return a placeholder that would be calculated from
	// the rate of change in tx/rx bytes over time
	return 0 // Implementation would track byte counters over time
}

// GetScalingRecommendation provides instance and tunnel recommendations
func (ts *TunnelScaler) GetScalingRecommendation() *ScalingRecommendation {
	ts.metrics.mu.RLock()
	defer ts.metrics.mu.RUnlock()

	recommendation := &ScalingRecommendation{
		CurrentUtilization: ts.metrics.UtilizationPct,
		CurrentTunnels:     len(ts.metrics.PerTunnelThroughput),
		RecommendedTunnels: ts.calculateOptimalTunnels(),
		RecommendedInstance: GetOptimalGravitonInstance(ts.metrics.TotalThroughput, 100.0), // $100 budget
		Reasoning:          []string{},
	}

	// Add reasoning
	if ts.metrics.UtilizationPct > 0.8 {
		recommendation.Reasoning = append(recommendation.Reasoning, 
			"High utilization detected - consider scaling up")
	}
	
	if ts.metrics.ElephantFlows > 0 {
		recommendation.Reasoning = append(recommendation.Reasoning, 
			fmt.Sprintf("%d elephant flows detected - additional tunnels recommended", ts.metrics.ElephantFlows))
	}

	return recommendation
}

// ScalingRecommendation provides scaling guidance
type ScalingRecommendation struct {
	CurrentUtilization  float64
	CurrentTunnels      int
	RecommendedTunnels  int
	RecommendedInstance *GravitonInstanceConfig
	Reasoning          []string
}

// calculateOptimalTunnels determines ideal tunnel count
func (ts *TunnelScaler) calculateOptimalTunnels() int {
	// Base calculation on current throughput and target utilization of 70%
	targetUtilization := 0.70
	currentThroughput := ts.metrics.TotalThroughput
	
	// 1.5 Gbps per tunnel max throughput
	throughputPerTunnel := int64(1500 * 1024 * 1024) // bytes/sec
	
	optimalTunnels := int(math.Ceil(float64(currentThroughput) / float64(throughputPerTunnel) / targetUtilization))
	
	// Apply constraints
	if optimalTunnels < ts.config.MinTunnels {
		optimalTunnels = ts.config.MinTunnels
	}
	if optimalTunnels > ts.config.MaxTunnels {
		optimalTunnels = ts.config.MaxTunnels
	}
	
	return optimalTunnels
}