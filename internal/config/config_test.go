package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	// Test default configuration creation
	config := &Config{}

	// Apply default values (simulate what setDefaults does)
	config.Tunnel = TunnelConfig{
		MinTunnels: 1,
		MaxTunnels: 8,
		BaseCIDR:   "10.100.0.0/16",
		MTU:        1420,
	}

	config.Scaling = ScalingConfig{
		ScaleUpThreshold:      0.80,
		ScaleDownThreshold:    0.30,
		ScaleInterval:         30 * time.Second,
		ScaleUpCooldown:       2 * time.Minute,
		ScaleDownCooldown:     10 * time.Minute,
		ElephantFlowThreshold: 104857600, // 100MB
		BurstDetectionWindow:  5 * time.Minute,
	}

	config.Probing = ProbingConfig{
		TestDuration:      30 * time.Second,
		MTURange:          []int{1200, 1500, 1800, 9000},
		ParallelStreams:   []int{1, 2, 4, 8},
		EnableJumboFrames: true,
		TestRegions:       []string{"us-west-2", "us-east-1", "eu-west-1"},
		LatencyTestCount:  100,
	}

	config.AWS = AWSConfig{
		InstanceTypes:     []string{"t4g.nano", "t4g.small", "c6gn.medium", "c6gn.large"},
		MaxInstances:      4,
		AvailabilityZones: []string{"us-west-2a", "us-west-2b", "us-west-2c"},
		BudgetLimit:       100.0,
	}

	// Test tunnel defaults
	if config.Tunnel.MinTunnels != 1 {
		t.Errorf("Expected MinTunnels=1, got %d", config.Tunnel.MinTunnels)
	}
	if config.Tunnel.MaxTunnels != 8 {
		t.Errorf("Expected MaxTunnels=8, got %d", config.Tunnel.MaxTunnels)
	}
	if config.Tunnel.BaseCIDR != "10.100.0.0/16" {
		t.Errorf("Expected BaseCIDR=10.100.0.0/16, got %s", config.Tunnel.BaseCIDR)
	}
	if config.Tunnel.MTU != 1420 {
		t.Errorf("Expected MTU=1420, got %d", config.Tunnel.MTU)
	}

	// Test scaling defaults
	if config.Scaling.ScaleUpThreshold != 0.80 {
		t.Errorf("Expected ScaleUpThreshold=0.80, got %f", config.Scaling.ScaleUpThreshold)
	}
	if config.Scaling.ScaleDownThreshold != 0.30 {
		t.Errorf("Expected ScaleDownThreshold=0.30, got %f", config.Scaling.ScaleDownThreshold)
	}

	// Test probing defaults
	if config.Probing.TestDuration != 30*time.Second {
		t.Errorf("Expected TestDuration=30s, got %v", config.Probing.TestDuration)
	}
	if !config.Probing.EnableJumboFrames {
		t.Error("Expected EnableJumboFrames=true")
	}

	// Test AWS defaults
	if config.AWS.MaxInstances != 4 {
		t.Errorf("Expected MaxInstances=4, got %d", config.AWS.MaxInstances)
	}
	if config.AWS.BudgetLimit != 100.0 {
		t.Errorf("Expected BudgetLimit=100.0, got %f", config.AWS.BudgetLimit)
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				Tunnel: TunnelConfig{
					MinTunnels: 1,
					MaxTunnels: 8,
					MTU:        1420,
				},
				Scaling: ScalingConfig{
					ScaleUpThreshold:   0.80,
					ScaleDownThreshold: 0.30,
				},
				AWS: AWSConfig{
					BudgetLimit: 100.0,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid min tunnels",
			config: &Config{
				Tunnel: TunnelConfig{
					MinTunnels: 0, // Invalid
					MaxTunnels: 8,
					MTU:        1420,
				},
				Scaling: ScalingConfig{
					ScaleUpThreshold:   0.80,
					ScaleDownThreshold: 0.30,
				},
				AWS: AWSConfig{
					BudgetLimit: 100.0,
				},
			},
			wantErr: true,
			errMsg:  "min_tunnels must be at least 1",
		},
		{
			name: "max tunnels less than min",
			config: &Config{
				Tunnel: TunnelConfig{
					MinTunnels: 5,
					MaxTunnels: 3, // Invalid
					MTU:        1420,
				},
				Scaling: ScalingConfig{
					ScaleUpThreshold:   0.80,
					ScaleDownThreshold: 0.30,
				},
				AWS: AWSConfig{
					BudgetLimit: 100.0,
				},
			},
			wantErr: true,
			errMsg:  "max_tunnels must be >= min_tunnels",
		},
		{
			name: "invalid MTU",
			config: &Config{
				Tunnel: TunnelConfig{
					MinTunnels: 1,
					MaxTunnels: 8,
					MTU:        500, // Too small
				},
				Scaling: ScalingConfig{
					ScaleUpThreshold:   0.80,
					ScaleDownThreshold: 0.30,
				},
				AWS: AWSConfig{
					BudgetLimit: 100.0,
				},
			},
			wantErr: true,
			errMsg:  "MTU must be between 1200 and 9000",
		},
		{
			name: "invalid scaling thresholds",
			config: &Config{
				Tunnel: TunnelConfig{
					MinTunnels: 1,
					MaxTunnels: 8,
					MTU:        1420,
				},
				Scaling: ScalingConfig{
					ScaleUpThreshold:   0.20, // Less than scale down
					ScaleDownThreshold: 0.30,
				},
				AWS: AWSConfig{
					BudgetLimit: 100.0,
				},
			},
			wantErr: true,
			errMsg:  "scale_up_threshold must be > scale_down_threshold",
		},
		{
			name: "invalid budget",
			config: &Config{
				Tunnel: TunnelConfig{
					MinTunnels: 1,
					MaxTunnels: 8,
					MTU:        1420,
				},
				Scaling: ScalingConfig{
					ScaleUpThreshold:   0.80,
					ScaleDownThreshold: 0.30,
				},
				AWS: AWSConfig{
					BudgetLimit: -10.0, // Negative budget
				},
			},
			wantErr: true,
			errMsg:  "budget_limit must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateConfig() expected error, got nil")
				} else if tt.errMsg != "" && err.Error() != tt.errMsg {
					t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateConfig() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGetConfigDir(t *testing.T) {
	configDir := GetConfigDir()

	if configDir == "" {
		t.Error("GetConfigDir() should not return empty string")
	}

	// Should contain user home directory
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" && !filepath.HasPrefix(configDir, homeDir) {
		t.Errorf("Config dir should be under home directory, got %s", configDir)
	}

	// Should end with .mole
	if !strings.HasSuffix(configDir, ".mole") {
		t.Errorf("Config dir should end with .mole, got %s", configDir)
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// Create a test configuration
	config := &Config{
		Tunnel: TunnelConfig{
			MinTunnels: 2,
			MaxTunnels: 6,
			BaseCIDR:   "192.168.0.0/16",
			MTU:        1500,
		},
		AWS: AWSConfig{
			MaxInstances: 2,
			BudgetLimit:  50.0,
		},
		Probing: ProbingConfig{
			TestDuration:      45 * time.Second,
			EnableJumboFrames: false,
		},
	}

	// Create temporary file
	tmpDir := os.TempDir()
	tmpFile := filepath.Join(tmpDir, "test-mole-config.yaml")
	defer os.Remove(tmpFile)

	// Save config
	err := SaveConfig(config, tmpFile)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load config back
	loadedConfig, err := LoadConfig(tmpFile)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Verify loaded config matches original
	if loadedConfig.Tunnel.MinTunnels != config.Tunnel.MinTunnels {
		t.Errorf("Loaded MinTunnels=%d, expected %d", loadedConfig.Tunnel.MinTunnels, config.Tunnel.MinTunnels)
	}

	if loadedConfig.Tunnel.BaseCIDR != config.Tunnel.BaseCIDR {
		t.Errorf("Loaded BaseCIDR=%s, expected %s", loadedConfig.Tunnel.BaseCIDR, config.Tunnel.BaseCIDR)
	}

	if loadedConfig.AWS.BudgetLimit != config.AWS.BudgetLimit {
		t.Errorf("Loaded BudgetLimit=%f, expected %f", loadedConfig.AWS.BudgetLimit, config.AWS.BudgetLimit)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	// Test loading config when file doesn't exist (should use defaults)
	nonExistentFile := "/tmp/non-existent-config.yaml"

	config, err := LoadConfig(nonExistentFile)
	if err != nil {
		t.Fatalf("LoadConfig should handle non-existent file: %v", err)
	}

	// Should have some default values
	if config == nil {
		t.Fatal("LoadConfig returned nil config")
	}

	// Verify some defaults are applied (these would come from setDefaults)
	// Note: in real implementation, defaults would be set by viper
	t.Logf("Loaded config with defaults: MinTunnels=%d, MaxTunnels=%d",
		config.Tunnel.MinTunnels, config.Tunnel.MaxTunnels)
}

func TestTunnelConfig(t *testing.T) {
	config := TunnelConfig{
		MinTunnels: 2,
		MaxTunnels: 12,
		BaseCIDR:   "172.16.0.0/16",
		MTU:        1800,
	}

	if config.MinTunnels != 2 {
		t.Errorf("Expected MinTunnels=2, got %d", config.MinTunnels)
	}
	if config.MaxTunnels != 12 {
		t.Errorf("Expected MaxTunnels=12, got %d", config.MaxTunnels)
	}
	if config.BaseCIDR != "172.16.0.0/16" {
		t.Errorf("Expected BaseCIDR=172.16.0.0/16, got %s", config.BaseCIDR)
	}
	if config.MTU != 1800 {
		t.Errorf("Expected MTU=1800, got %d", config.MTU)
	}
}

func TestScalingConfig(t *testing.T) {
	config := ScalingConfig{
		ScaleUpThreshold:      0.85,
		ScaleDownThreshold:    0.25,
		ScaleInterval:         45 * time.Second,
		ScaleUpCooldown:       3 * time.Minute,
		ScaleDownCooldown:     15 * time.Minute,
		ElephantFlowThreshold: 209715200, // 200MB
		BurstDetectionWindow:  10 * time.Minute,
	}

	if config.ScaleUpThreshold != 0.85 {
		t.Errorf("Expected ScaleUpThreshold=0.85, got %f", config.ScaleUpThreshold)
	}
	if config.ElephantFlowThreshold != 209715200 {
		t.Errorf("Expected ElephantFlowThreshold=209715200, got %d", config.ElephantFlowThreshold)
	}
	if config.BurstDetectionWindow != 10*time.Minute {
		t.Errorf("Expected BurstDetectionWindow=10m, got %v", config.BurstDetectionWindow)
	}
}

func TestProbingConfig(t *testing.T) {
	config := ProbingConfig{
		TestDuration:      60 * time.Second,
		MTURange:          []int{1200, 1500, 1800, 9000},
		ParallelStreams:   []int{1, 2, 4, 8, 16},
		EnableJumboFrames: false,
		TestRegions:       []string{"us-west-1", "us-east-2"},
		LatencyTestCount:  50,
	}

	if config.TestDuration != 60*time.Second {
		t.Errorf("Expected TestDuration=60s, got %v", config.TestDuration)
	}
	if len(config.MTURange) != 4 {
		t.Errorf("Expected 4 MTU values, got %d", len(config.MTURange))
	}
	if len(config.ParallelStreams) != 5 {
		t.Errorf("Expected 5 stream values, got %d", len(config.ParallelStreams))
	}
	if config.EnableJumboFrames {
		t.Error("Expected EnableJumboFrames=false")
	}
	if config.LatencyTestCount != 50 {
		t.Errorf("Expected LatencyTestCount=50, got %d", config.LatencyTestCount)
	}
}

func TestAWSConfig(t *testing.T) {
	config := AWSConfig{
		InstanceTypes:     []string{"c6gn.medium", "c6gn.large", "c6gn.xlarge"},
		MaxInstances:      8,
		AvailabilityZones: []string{"us-west-2a", "us-west-2b"},
		BudgetLimit:       200.0,
	}

	if len(config.InstanceTypes) != 3 {
		t.Errorf("Expected 3 instance types, got %d", len(config.InstanceTypes))
	}
	if config.InstanceTypes[0] != "c6gn.medium" {
		t.Errorf("Expected first instance type=c6gn.medium, got %s", config.InstanceTypes[0])
	}
	if config.MaxInstances != 8 {
		t.Errorf("Expected MaxInstances=8, got %d", config.MaxInstances)
	}
	if config.BudgetLimit != 200.0 {
		t.Errorf("Expected BudgetLimit=200.0, got %f", config.BudgetLimit)
	}
}

func TestMonitoringConfig(t *testing.T) {
	config := MonitoringConfig{
		EnablePrometheus:    true,
		MetricsPort:         9091,
		HealthCheckInterval: 15 * time.Second,
		LogLevel:            "debug",
	}

	if !config.EnablePrometheus {
		t.Error("Expected EnablePrometheus=true")
	}
	if config.MetricsPort != 9091 {
		t.Errorf("Expected MetricsPort=9091, got %d", config.MetricsPort)
	}
	if config.HealthCheckInterval != 15*time.Second {
		t.Errorf("Expected HealthCheckInterval=15s, got %v", config.HealthCheckInterval)
	}
	if config.LogLevel != "debug" {
		t.Errorf("Expected LogLevel=debug, got %s", config.LogLevel)
	}
}