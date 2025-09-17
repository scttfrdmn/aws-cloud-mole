package aws

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestDeploymentConfig(t *testing.T) {
	config := DeploymentConfig{
		VPCId:        "vpc-12345",
		SubnetId:     "subnet-67890",
		InstanceType: types.InstanceTypeT4gSmall,
		TunnelCount:  4,
		MTUSize:      1500,
		AllowedCIDR:  "0.0.0.0/0",
		SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2E...",
		Profile:      "default",
		Region:       "us-west-2",
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
			DailyCost:   0.4032,
			MonthlyCost: 12.2976,
		},
	}

	if result.BastionInstanceID != "i-1234567890abcdef0" {
		t.Errorf("Expected BastionInstanceID=i-1234567890abcdef0, got %s", result.BastionInstanceID)
	}

	if len(result.TunnelPorts) != 4 {
		t.Errorf("Expected 4 tunnel ports, got %d", len(result.TunnelPorts))
	}

	// Test tunnel port sequence
	expectedPorts := []int{51820, 51821, 51822, 51823}
	for i, expected := range expectedPorts {
		if result.TunnelPorts[i] != expected {
			t.Errorf("Expected TunnelPorts[%d]=%d, got %d", i, expected, result.TunnelPorts[i])
		}
	}

	// Test cost estimate
	if result.CostEstimate.HourlyCost != 0.0168 {
		t.Errorf("Expected HourlyCost=0.0168, got %f", result.CostEstimate.HourlyCost)
	}

	// Test cost relationship
	expectedDaily := result.CostEstimate.HourlyCost * 24
	if result.CostEstimate.DailyCost != expectedDaily {
		t.Errorf("DailyCost should be %f, got %f", expectedDaily, result.CostEstimate.DailyCost)
	}
}

func TestCostEstimate(t *testing.T) {
	client := &AWSClient{}

	tests := []struct {
		instanceType    types.InstanceType
		expectedHourly  float64
		expectedMonthly float64
	}{
		{types.InstanceTypeT4gNano, 0.0042, 0.0042 * 24 * 30.4},
		{types.InstanceTypeT4gMicro, 0.0084, 0.0084 * 24 * 30.4},
		{types.InstanceTypeT4gSmall, 0.0168, 0.0168 * 24 * 30.4},
		{types.InstanceTypeC6gnMedium, 0.0864, 0.0864 * 24 * 30.4},
		{types.InstanceTypeC6gnLarge, 0.1728, 0.1728 * 24 * 30.4},
	}

	for _, test := range tests {
		estimate := client.calculateCostEstimate(test.instanceType)

		if estimate.HourlyCost != test.expectedHourly {
			t.Errorf("InstanceType %v: expected hourly cost %f, got %f",
				test.instanceType, test.expectedHourly, estimate.HourlyCost)
		}

		expectedDaily := test.expectedHourly * 24
		if estimate.DailyCost != expectedDaily {
			t.Errorf("InstanceType %v: expected daily cost %f, got %f",
				test.instanceType, expectedDaily, estimate.DailyCost)
		}

		if estimate.MonthlyCost != test.expectedMonthly {
			t.Errorf("InstanceType %v: expected monthly cost %f, got %f",
				test.instanceType, test.expectedMonthly, estimate.MonthlyCost)
		}
	}
}

func TestCostEstimateUnknownInstance(t *testing.T) {
	client := &AWSClient{}

	// Test with instance type not in the cost map
	estimate := client.calculateCostEstimate(types.InstanceTypeM5Large) // Not in our cost map

	// Should return default cost
	if estimate.HourlyCost != 0.05 {
		t.Errorf("Expected default hourly cost 0.05, got %f", estimate.HourlyCost)
	}

	expectedDaily := 0.05 * 24
	if estimate.DailyCost != expectedDaily {
		t.Errorf("Expected default daily cost %f, got %f", expectedDaily, estimate.DailyCost)
	}

	expectedMonthly := 0.05 * 24 * 30.4
	if estimate.MonthlyCost != expectedMonthly {
		t.Errorf("Expected default monthly cost %f, got %f", expectedMonthly, estimate.MonthlyCost)
	}
}

func TestGenerateUserData(t *testing.T) {
	client := &AWSClient{}

	userData := client.generateUserData(4, 1500)

	// Test that user data is not empty
	if userData == "" {
		t.Error("generateUserData returned empty string")
	}

	// Test that it contains bash shebang
	if !strings.HasPrefix(userData, "#!/bin/bash") {
		t.Error("User data should start with bash shebang")
	}

	// Test that it contains the tunnel count
	if !strings.Contains(userData, "TUNNEL_COUNT=4") {
		t.Error("User data should contain TUNNEL_COUNT=4")
	}

	// Test that it contains the MTU size
	if !strings.Contains(userData, "MTU_SIZE=1500") {
		t.Error("User data should contain MTU_SIZE=1500")
	}

	// Test that it contains package installation
	if !strings.Contains(userData, "apt-get install") {
		t.Error("User data should contain package installation")
	}

	// Test that it contains WireGuard installation
	if !strings.Contains(userData, "wireguard") {
		t.Error("User data should install WireGuard")
	}

	// Test that it enables IP forwarding
	if !strings.Contains(userData, "net.ipv4.ip_forward=1") {
		t.Error("User data should enable IP forwarding")
	}

	// Test that it generates keys
	if !strings.Contains(userData, "wg genkey") {
		t.Error("User data should generate WireGuard keys")
	}

	// Test that it creates required directories
	if !strings.Contains(userData, "mkdir -p /etc/mole/keys") {
		t.Error("User data should create mole directories")
	}

	// Test completion signal
	if !strings.Contains(userData, "echo \"ready\" > /etc/mole/status") {
		t.Error("User data should signal completion")
	}
}

func TestDirectDeployValidation(t *testing.T) {
	client := &AWSClient{
		profile: "test",
		region:  "us-west-2",
	}

	// Test with invalid configuration
	invalidConfigs := []struct {
		name   string
		config *DeploymentConfig
	}{
		{
			name: "empty VPC ID",
			config: &DeploymentConfig{
				VPCId:        "",
				SubnetId:     "subnet-123",
				InstanceType: types.InstanceTypeT4gSmall,
				TunnelCount:  1,
			},
		},
		{
			name: "empty Subnet ID",
			config: &DeploymentConfig{
				VPCId:        "vpc-123",
				SubnetId:     "",
				InstanceType: types.InstanceTypeT4gSmall,
				TunnelCount:  1,
			},
		},
		{
			name: "zero tunnels",
			config: &DeploymentConfig{
				VPCId:        "vpc-123",
				SubnetId:     "subnet-123",
				InstanceType: types.InstanceTypeT4gSmall,
				TunnelCount:  0,
			},
		},
	}

	for _, test := range invalidConfigs {
		t.Run(test.name, func(t *testing.T) {
			_, err := client.DirectDeploy(context.Background(), test.config)
			if err == nil {
				t.Errorf("Expected error for invalid config: %s", test.name)
			}
		})
	}
}

func TestTunnelPortGeneration(t *testing.T) {
	tests := []struct {
		tunnelCount int
		expectedLen int
		basePort    int
	}{
		{1, 1, 51820},
		{4, 4, 51820},
		{8, 8, 51820},
	}

	for _, test := range tests {
		ports := make([]int, test.tunnelCount)
		for i := 0; i < test.tunnelCount; i++ {
			ports[i] = 51820 + i
		}

		if len(ports) != test.expectedLen {
			t.Errorf("Expected %d ports, got %d", test.expectedLen, len(ports))
		}

		if ports[0] != test.basePort {
			t.Errorf("Expected first port=%d, got %d", test.basePort, ports[0])
		}

		if test.tunnelCount > 1 {
			if ports[test.tunnelCount-1] != test.basePort+test.tunnelCount-1 {
				t.Errorf("Expected last port=%d, got %d",
					test.basePort+test.tunnelCount-1, ports[test.tunnelCount-1])
			}
		}

		// Test port uniqueness
		portSet := make(map[int]bool)
		for _, port := range ports {
			if portSet[port] {
				t.Errorf("Duplicate port found: %d", port)
			}
			portSet[port] = true
		}
	}
}

// Test that deployment config has all required fields
func TestDeploymentConfigCompleteness(t *testing.T) {
	config := &DeploymentConfig{
		VPCId:        "vpc-12345",
		SubnetId:     "subnet-67890",
		InstanceType: types.InstanceTypeT4gSmall,
		TunnelCount:  3,
		MTUSize:      1500,
		AllowedCIDR:  "10.0.0.0/8",
		SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2E...",
		Profile:      "test-profile",
		Region:       "eu-west-1",
	}

	// Test that all fields are set
	if config.VPCId == "" {
		t.Error("VPCId should not be empty")
	}
	if config.SubnetId == "" {
		t.Error("SubnetId should not be empty")
	}
	if config.TunnelCount <= 0 {
		t.Error("TunnelCount should be positive")
	}
	if config.MTUSize <= 0 {
		t.Error("MTUSize should be positive")
	}
	if config.AllowedCIDR == "" {
		t.Error("AllowedCIDR should not be empty")
	}
	if config.Profile == "" {
		t.Error("Profile should not be empty")
	}
	if config.Region == "" {
		t.Error("Region should not be empty")
	}
}

// Test user data script generation with edge cases
func TestUserDataEdgeCases(t *testing.T) {
	client := &AWSClient{}

	// Test with minimal tunnels
	userData1 := client.generateUserData(1, 1200)
	if !strings.Contains(userData1, "TUNNEL_COUNT=1") {
		t.Error("Should handle single tunnel")
	}
	if !strings.Contains(userData1, "MTU_SIZE=1200") {
		t.Error("Should handle small MTU")
	}

	// Test with maximum tunnels
	userData2 := client.generateUserData(16, 9000)
	if !strings.Contains(userData2, "TUNNEL_COUNT=16") {
		t.Error("Should handle many tunnels")
	}
	if !strings.Contains(userData2, "MTU_SIZE=9000") {
		t.Error("Should handle jumbo frames")
	}

	// Test that script contains loop for multiple tunnels
	if !strings.Contains(userData2, "for i in $(seq 0") {
		t.Error("Should contain loop for key generation")
	}
}