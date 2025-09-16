package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config represents the complete application configuration
type Config struct {
	Tunnel       TunnelConfig       `yaml:"tunnel"`
	Scaling      ScalingConfig      `yaml:"scaling"`
	Probing      ProbingConfig      `yaml:"probing"`
	Optimization OptimizationConfig `yaml:"optimization"`
	AWS          AWSConfig          `yaml:"aws"`
	MPTCP        MPTCPConfig        `yaml:"mptcp"`
	Monitoring   MonitoringConfig   `yaml:"monitoring"`
}

// TunnelConfig defines tunnel parameters
type TunnelConfig struct {
	MinTunnels int    `yaml:"min_tunnels"`
	MaxTunnels int    `yaml:"max_tunnels"`
	BaseCIDR   string `yaml:"base_cidr"`
	MTU        int    `yaml:"mtu"`
}

// ScalingConfig defines scaling behavior
type ScalingConfig struct {
	ScaleUpThreshold       float64       `yaml:"scale_up_threshold"`
	ScaleDownThreshold     float64       `yaml:"scale_down_threshold"`
	ScaleInterval          time.Duration `yaml:"scale_interval"`
	ScaleUpCooldown        time.Duration `yaml:"scale_up_cooldown"`
	ScaleDownCooldown      time.Duration `yaml:"scale_down_cooldown"`
	ElephantFlowThreshold  int64         `yaml:"elephant_flow_threshold"`
	BurstDetectionWindow   time.Duration `yaml:"burst_detection_window"`
}

// ProbingConfig defines network probing parameters
type ProbingConfig struct {
	TestDuration        time.Duration `yaml:"test_duration"`
	MTURange           []int         `yaml:"mtu_range"`
	ParallelStreams    []int         `yaml:"parallel_streams"`
	EnableJumboFrames  bool          `yaml:"enable_jumbo_frames"`
	TestRegions        []string      `yaml:"test_regions"`
	LatencyTestCount   int           `yaml:"latency_test_count"`
}

// OptimizationConfig defines performance optimization settings
type OptimizationConfig struct {
	EnableProcessPinning bool   `yaml:"enable_process_pinning"`
	ReserveCores        int    `yaml:"reserve_cores"`
	CPUGovernor         string `yaml:"cpu_governor"`
	TCPCongestion       string `yaml:"tcp_congestion"`
}

// AWSConfig defines AWS-specific settings
type AWSConfig struct {
	InstanceTypes      []string `yaml:"instance_types"`
	MaxInstances       int      `yaml:"max_instances"`
	AvailabilityZones  []string `yaml:"availability_zones"`
	BudgetLimit        float64  `yaml:"budget_limit"`
}

// MPTCPConfig defines MPTCP settings
type MPTCPConfig struct {
	Enable      bool   `yaml:"enable"`
	Scheduler   string `yaml:"scheduler"`
	PathManager string `yaml:"path_manager"`
}

// MonitoringConfig defines monitoring settings
type MonitoringConfig struct {
	EnablePrometheus      bool          `yaml:"enable_prometheus"`
	MetricsPort          int           `yaml:"metrics_port"`
	HealthCheckInterval  time.Duration `yaml:"health_check_interval"`
	LogLevel             string        `yaml:"log_level"`
}

// LoadConfig loads configuration from file and environment
func LoadConfig(configFile string) (*Config, error) {
	// Set config defaults
	setDefaults()

	// Set config file path
	if configFile != \"\" {
		viper.SetConfigFile(configFile)
	} else {
		// Look for config in multiple locations
		viper.SetConfigName(\"config\")
		viper.SetConfigType(\"yaml\")
		viper.AddConfigPath(\".\")
		viper.AddConfigPath(\"./configs\")
		viper.AddConfigPath(\"$HOME/.mole\")
		viper.AddConfigPath(\"/etc/mole\")
	}

	// Enable environment variable support
	viper.SetEnvPrefix(\"MOLE\")
	viper.AutomaticEnv()

	// Read configuration
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf(\"failed to read config file: %w\", err)
		}
		// Config file not found is OK, we'll use defaults
	}

	// Unmarshal into struct
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf(\"failed to unmarshal config: %w\", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf(\"invalid configuration: %w\", err)
	}

	return &config, nil
}

// setDefaults sets default configuration values
func setDefaults() {
	// Tunnel defaults
	viper.SetDefault(\"tunnel.min_tunnels\", 1)
	viper.SetDefault(\"tunnel.max_tunnels\", 8)
	viper.SetDefault(\"tunnel.base_cidr\", \"10.100.0.0/16\")
	viper.SetDefault(\"tunnel.mtu\", 1420)

	// Scaling defaults
	viper.SetDefault(\"scaling.scale_up_threshold\", 0.80)
	viper.SetDefault(\"scaling.scale_down_threshold\", 0.30)
	viper.SetDefault(\"scaling.scale_interval\", \"30s\")
	viper.SetDefault(\"scaling.scale_up_cooldown\", \"2m\")
	viper.SetDefault(\"scaling.scale_down_cooldown\", \"10m\")
	viper.SetDefault(\"scaling.elephant_flow_threshold\", 104857600) // 100MB
	viper.SetDefault(\"scaling.burst_detection_window\", \"5m\")

	// Probing defaults
	viper.SetDefault(\"probing.test_duration\", \"30s\")
	viper.SetDefault(\"probing.mtu_range\", []int{1200, 1500, 1800, 9000})
	viper.SetDefault(\"probing.parallel_streams\", []int{1, 2, 4, 8})
	viper.SetDefault(\"probing.enable_jumbo_frames\", true)
	viper.SetDefault(\"probing.test_regions\", []string{\"us-west-2\", \"us-east-1\", \"eu-west-1\"})
	viper.SetDefault(\"probing.latency_test_count\", 100)

	// Optimization defaults
	viper.SetDefault(\"optimization.enable_process_pinning\", true)
	viper.SetDefault(\"optimization.reserve_cores\", 2)
	viper.SetDefault(\"optimization.cpu_governor\", \"performance\")
	viper.SetDefault(\"optimization.tcp_congestion\", \"bbr\")

	// AWS defaults
	viper.SetDefault(\"aws.instance_types\", []string{\"t4g.nano\", \"t4g.small\", \"c6gn.medium\", \"c6gn.large\"})
	viper.SetDefault(\"aws.max_instances\", 4)
	viper.SetDefault(\"aws.availability_zones\", []string{\"us-west-2a\", \"us-west-2b\", \"us-west-2c\"})
	viper.SetDefault(\"aws.budget_limit\", 100.0)

	// MPTCP defaults
	viper.SetDefault(\"mptcp.enable\", false)
	viper.SetDefault(\"mptcp.scheduler\", \"roundrobin\")
	viper.SetDefault(\"mptcp.path_manager\", \"fullmesh\")

	// Monitoring defaults
	viper.SetDefault(\"monitoring.enable_prometheus\", false)
	viper.SetDefault(\"monitoring.metrics_port\", 9090)
	viper.SetDefault(\"monitoring.health_check_interval\", \"30s\")
	viper.SetDefault(\"monitoring.log_level\", \"info\")
}

// validateConfig validates the loaded configuration
func validateConfig(config *Config) error {
	// Validate tunnel config
	if config.Tunnel.MinTunnels < 1 {
		return fmt.Errorf(\"min_tunnels must be at least 1\")
	}
	if config.Tunnel.MaxTunnels < config.Tunnel.MinTunnels {
		return fmt.Errorf(\"max_tunnels must be >= min_tunnels\")
	}
	if config.Tunnel.MTU < 1200 || config.Tunnel.MTU > 9000 {
		return fmt.Errorf(\"MTU must be between 1200 and 9000\")
	}

	// Validate scaling config
	if config.Scaling.ScaleUpThreshold <= config.Scaling.ScaleDownThreshold {
		return fmt.Errorf(\"scale_up_threshold must be > scale_down_threshold\")
	}

	// Validate AWS config
	if config.AWS.BudgetLimit <= 0 {
		return fmt.Errorf(\"budget_limit must be positive\")
	}

	return nil
}

// SaveConfig saves the current configuration to file
func SaveConfig(config *Config, filename string) error {
	// Create config directory if it doesn't exist
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf(\"failed to create config directory: %w\", err)
	}

	// Set viper values from struct
	viper.Set(\"tunnel\", config.Tunnel)
	viper.Set(\"scaling\", config.Scaling)
	viper.Set(\"probing\", config.Probing)
	viper.Set(\"optimization\", config.Optimization)
	viper.Set(\"aws\", config.AWS)
	viper.Set(\"mptcp\", config.MPTCP)
	viper.Set(\"monitoring\", config.Monitoring)

	// Write config file
	return viper.WriteConfigAs(filename)
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() string {
	homeDir, _ := os.UserHomeDir()
	return filepath.Join(homeDir, \".mole\")
}"