package aws

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestDeploymentConfig(t *testing.T) {
	config := DeploymentConfig{
		VPCId:           "vpc-12345",
		PublicSubnetId:  "subnet-67890",
		PrivateSubnetId: "subnet-54321",
		InstanceType:    types.InstanceTypeT4gSmall,
		TunnelCount:     4,
		MTUSize:         1500,
		AllowedCIDR:     "0.0.0.0/0",
		SSHPublicKey:    "ssh-rsa AAAAB3NzaC1yc2E...",
		Profile:         "default",
		Region:          "us-west-2",
		EnableNAT:       true,
		DeployTarget:    false,
		TargetInstance:  types.InstanceTypeT4gNano,
	}

	if config.VPCId != "vpc-12345" {
		t.Errorf("Expected VPCId=vpc-12345, got %s", config.VPCId)
	}

	if config.TunnelCount != 4 {
		t.Errorf("Expected TunnelCount=4, got %d", config.TunnelCount)
	}

	if config.InstanceType != types.InstanceTypeT4gSmall {
		t.Errorf("Expected InstanceType=T4gSmall, got %v", config.InstanceType)
	}

	if config.MTUSize != 1500 {
		t.Errorf("Expected MTUSize=1500, got %d", config.MTUSize)
	}

	if !config.EnableNAT {
		t.Error("Expected EnableNAT=true")
	}

	if config.DeployTarget {
		t.Error("Expected DeployTarget=false")
	}

	if config.TargetInstance != types.InstanceTypeT4gNano {
		t.Errorf("Expected TargetInstance=T4gNano, got %v", config.TargetInstance)
	}
}

func TestDeploymentResult(t *testing.T) {
	result := DeploymentResult{
		BastionInstanceID: "i-1234567890abcdef0",
		BastionPublicIP:   "54.123.45.67",
		BastionPrivateIP:  "10.0.1.100",
		SecurityGroupID:   "sg-12345",
		KeyPairName:       "mole-key-123",
		TunnelPorts:       []int{51820, 51821, 51822, 51823},
		CostEstimate: CostEstimate{
			HourlyCost:  0.0168,
			MonthlyCost: 12.26,
			DailyCost:   0.4032,
		},
		ClientPrivateKey: "test-private-key",
		ServerPublicKey:  "test-public-key",
		TargetInstanceID: "i-target123",
		TargetPrivateIP:  "10.0.2.100",
	}

	if result.BastionInstanceID != "i-1234567890abcdef0" {
		t.Errorf("Expected BastionInstanceID=i-1234567890abcdef0, got %s", result.BastionInstanceID)
	}

	if len(result.TunnelPorts) != 4 {
		t.Errorf("Expected 4 tunnel ports, got %d", len(result.TunnelPorts))
	}

	if result.CostEstimate.MonthlyCost != 12.26 {
		t.Errorf("Expected MonthlyCost=12.26, got %f", result.CostEstimate.MonthlyCost)
	}

	if result.ClientPrivateKey == "" {
		t.Error("ClientPrivateKey should not be empty")
	}

	if result.ServerPublicKey == "" {
		t.Error("ServerPublicKey should not be empty")
	}

	if result.TargetInstanceID != "i-target123" {
		t.Errorf("Expected TargetInstanceID=i-target123, got %s", result.TargetInstanceID)
	}
}

// Test WireGuard key generation
func TestGenerateWireGuardKeys(t *testing.T) {
	client := &AWSClient{
		profile: "test",
		region:  "us-west-2",
	}

	privateKey, publicKey, err := client.generateWireGuardKeys()
	if err != nil {
		t.Fatalf("generateWireGuardKeys failed: %v", err)
	}

	if privateKey == "" {
		t.Error("Private key should not be empty")
	}

	if publicKey == "" {
		t.Error("Public key should not be empty")
	}

	// Keys should be different on each call
	privateKey2, publicKey2, err := client.generateWireGuardKeys()
	if err != nil {
		t.Fatalf("generateWireGuardKeys failed on second call: %v", err)
	}

	if privateKey == privateKey2 {
		t.Error("Private keys should be different on each call")
	}

	if publicKey == publicKey2 {
		t.Error("Public keys should be different on each call")
	}
}

// Test privilege detection
func TestDetectPrivilegeLevel(t *testing.T) {
	client := &AWSClient{}

	privLevel := client.detectPrivilegeLevel()

	// Should contain platform-appropriate text
	switch runtime.GOOS {
	case "darwin", "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		if !strings.Contains(privLevel, "root") && !strings.Contains(privLevel, "sudo") {
			t.Errorf("Expected Unix privilege level to contain 'root' or 'sudo', got: %s", privLevel)
		}
	case "windows":
		if !strings.Contains(privLevel, "administrator") && !strings.Contains(privLevel, "elevation") {
			t.Errorf("Expected Windows privilege level to contain 'administrator' or 'elevation', got: %s", privLevel)
		}
	default:
		if privLevel != "unknown" {
			t.Errorf("Expected 'unknown' for unsupported platform, got: %s", privLevel)
		}
	}
}

// Test Windows admin detection
func TestIsWindowsAdmin(t *testing.T) {
	client := &AWSClient{}

	isAdmin := client.isWindowsAdmin()

	if runtime.GOOS != "windows" {
		// On non-Windows systems, should always return false
		if isAdmin {
			t.Error("isWindowsAdmin should return false on non-Windows systems")
		}
	}
	// On Windows, the result depends on actual elevation status
	// We don't test the Windows-specific behavior as it requires admin privileges
}

// Note: TestInstanceTypeFromString and TestSelectOptimalInstance are in client_test.go

// Test sudo environment setup
func TestSetupSudoEnvironment(t *testing.T) {
	client := &AWSClient{}

	env := client.setupSudoEnvironment()

	// Should return environment variables
	if env == nil {
		t.Fatal("setupSudoEnvironment returned nil")
	}

	// Should contain current environment
	if len(env) == 0 {
		t.Error("setupSudoEnvironment should contain environment variables")
	}

	// On supported platforms, should try to add SUDO_ASKPASS
	// (We can't test the file existence part reliably in CI)
}

// Test VPC info struct
func TestVPCInfo(t *testing.T) {
	info := VPCInfo{
		VpcId:     "vpc-12345",
		CidrBlock: "10.0.0.0/16",
		State:     "available",
		IsDefault: true,
		Name:      "test-vpc",
	}

	if info.VpcId != "vpc-12345" {
		t.Errorf("Expected VpcId=vpc-12345, got %s", info.VpcId)
	}

	if !info.IsDefault {
		t.Error("Expected IsDefault=true")
	}

	if info.State != "available" {
		t.Errorf("Expected State=available, got %s", info.State)
	}
}

// Benchmark WireGuard key generation
func BenchmarkGenerateWireGuardKeys(b *testing.B) {
	client := &AWSClient{}

	for i := 0; i < b.N; i++ {
		_, _, err := client.generateWireGuardKeys()
		if err != nil {
			b.Fatalf("generateWireGuardKeys failed: %v", err)
		}
	}
}

// Test environment variable handling
func TestEnvironmentHandling(t *testing.T) {
	// Save original values
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")

	// Test with HOME set (Unix-like)
	os.Setenv("HOME", "/tmp/test-home")

	// This would normally be tested in platform-specific setup functions
	// but we can verify basic environment handling

	home := os.Getenv("HOME")
	if home != "/tmp/test-home" {
		t.Errorf("Expected HOME=/tmp/test-home, got %s", home)
	}

	// Restore original values
	if origHome != "" {
		os.Setenv("HOME", origHome)
	}
	if origUserProfile != "" {
		os.Setenv("USERPROFILE", origUserProfile)
	}
}

// Test deployment config validation
func TestDeploymentConfigValidation(t *testing.T) {
	tests := []struct {
		name   string
		config DeploymentConfig
		valid  bool
	}{
		{
			name: "valid config",
			config: DeploymentConfig{
				VPCId:           "vpc-12345",
				PublicSubnetId:  "subnet-67890",
				InstanceType:    types.InstanceTypeT4gSmall,
				TunnelCount:     1,
				MTUSize:         1500,
				Profile:         "default",
				Region:          "us-west-2",
			},
			valid: true,
		},
		{
			name: "missing VPC ID",
			config: DeploymentConfig{
				PublicSubnetId: "subnet-67890",
				InstanceType:   types.InstanceTypeT4gSmall,
			},
			valid: false,
		},
		{
			name: "zero tunnel count",
			config: DeploymentConfig{
				VPCId:        "vpc-12345",
				TunnelCount:  0,
				InstanceType: types.InstanceTypeT4gSmall,
			},
			valid: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			isValid := test.config.VPCId != "" && test.config.TunnelCount > 0
			if isValid != test.valid {
				t.Errorf("Config validation mismatch for %s: expected %v, got %v", test.name, test.valid, isValid)
			}
		})
	}
}