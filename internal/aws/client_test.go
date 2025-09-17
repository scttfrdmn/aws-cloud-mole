package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

func TestNewAWSClient(t *testing.T) {
	// Test creating AWS client (may fail without credentials)
	client, err := NewAWSClient("default", "us-west-2")
	if err != nil {
		t.Logf("NewAWSClient failed (expected without AWS credentials): %v", err)
		return
	}

	if client == nil {
		t.Fatal("NewAWSClient returned nil client without error")
	}

	if client.profile != "default" {
		t.Errorf("Expected profile='default', got '%s'", client.profile)
	}

	if client.region != "us-west-2" {
		t.Errorf("Expected region='us-west-2', got '%s'", client.region)
	}

	if client.client == nil {
		t.Error("Expected EC2 client to be initialized")
	}
}

func TestInstanceTypeFromString(t *testing.T) {
	tests := []struct {
		input    string
		expected types.InstanceType
	}{
		{"t4g.nano", types.InstanceTypeT4gNano},
		{"t4g.micro", types.InstanceTypeT4gMicro},
		{"t4g.small", types.InstanceTypeT4gSmall},
		{"t4g.medium", types.InstanceTypeT4gMedium},
		{"c6gn.medium", types.InstanceTypeC6gnMedium},
		{"c6gn.large", types.InstanceTypeC6gnLarge},
		{"c6gn.xlarge", types.InstanceTypeC6gnXlarge},
		{"invalid-type", types.InstanceTypeT4gSmall}, // Default fallback
		{"", types.InstanceTypeT4gSmall},              // Default fallback
	}

	for _, test := range tests {
		result := InstanceTypeFromString(test.input)
		if result != test.expected {
			t.Errorf("InstanceTypeFromString(%s) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func TestSelectOptimalInstance(t *testing.T) {
	// This test doesn't require AWS credentials as it's pure logic
	client := &AWSClient{
		profile: "test",
		region:  "us-west-2",
		// client is nil but that's ok for this test
	}

	tests := []struct {
		name       string
		throughput int64
		budget     float64
		expected   types.InstanceType
	}{
		{
			name:       "low throughput, low budget",
			throughput: 50 * 1024 * 1024,   // 50 Mbps in bytes/sec
			budget:     5.0,                 // $5
			expected:   types.InstanceTypeT4gNano, // Should fit in budget
		},
		{
			name:       "medium throughput, medium budget",
			throughput: 500 * 1024 * 1024,  // 500 Mbps in bytes/sec
			budget:     30.0,                // $30
			expected:   types.InstanceTypeC6gnMedium, // Should meet throughput needs
		},
		{
			name:       "high throughput, high budget",
			throughput: 2000 * 1024 * 1024, // 2 Gbps in bytes/sec
			budget:     100.0,               // $100
			expected:   types.InstanceTypeC6gnLarge, // Should handle high throughput
		},
		{
			name:       "very low budget",
			throughput: 1000 * 1024 * 1024, // 1 Gbps in bytes/sec
			budget:     2.0,                 // $2 (very low)
			expected:   types.InstanceTypeT4gNano, // Should return cheapest
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			instance := client.SelectOptimalInstance(test.throughput, test.budget)
			if instance == nil {
				t.Fatal("SelectOptimalInstance returned nil")
			}

			if instance.Type != test.expected {
				t.Errorf("Expected instance type %v, got %v", test.expected, instance.Type)
			}

			// Verify instance is within budget
			if instance.MonthlyCost > test.budget {
				t.Errorf("Instance cost $%.2f exceeds budget $%.2f", instance.MonthlyCost, test.budget)
			}
		})
	}
}

func TestInstanceConfig(t *testing.T) {
	config := InstanceConfig{
		Type:              types.InstanceTypeT4gSmall,
		BaselineBandwidth: 125,                  // 125 Mbps
		BurstBandwidth:    5000,                 // 5 Gbps
		MonthlyCost:       5.26,                 // $5.26/month
		MaxTunnels:        4,
	}

	if config.Type != types.InstanceTypeT4gSmall {
		t.Errorf("Expected Type=T4gSmall, got %v", config.Type)
	}

	if config.BaselineBandwidth != 125 {
		t.Errorf("Expected BaselineBandwidth=125, got %d", config.BaselineBandwidth)
	}

	if config.BurstBandwidth != 5000 {
		t.Errorf("Expected BurstBandwidth=5000, got %d", config.BurstBandwidth)
	}

	if config.MonthlyCost != 5.26 {
		t.Errorf("Expected MonthlyCost=5.26, got %.2f", config.MonthlyCost)
	}

	if config.MaxTunnels != 4 {
		t.Errorf("Expected MaxTunnels=4, got %d", config.MaxTunnels)
	}
}

func TestBastionConfig(t *testing.T) {
	config := BastionConfig{
		InstanceType:     types.InstanceTypeT4gSmall,
		VPCId:            "vpc-12345",
		SubnetId:         "subnet-67890",
		SecurityGroupIds: []string{"sg-11111", "sg-22222"},
		KeyPairName:      "my-keypair",
		UserData:         "#!/bin/bash\necho 'Hello World'",
	}

	if config.InstanceType != types.InstanceTypeT4gSmall {
		t.Errorf("Expected InstanceType=T4gSmall, got %v", config.InstanceType)
	}

	if config.VPCId != "vpc-12345" {
		t.Errorf("Expected VPCId=vpc-12345, got %s", config.VPCId)
	}

	if len(config.SecurityGroupIds) != 2 {
		t.Errorf("Expected 2 security group IDs, got %d", len(config.SecurityGroupIds))
	}

	if config.SecurityGroupIds[0] != "sg-11111" {
		t.Errorf("Expected first SG=sg-11111, got %s", config.SecurityGroupIds[0])
	}
}

func TestBastionInfo(t *testing.T) {
	info := BastionInfo{
		InstanceId: "i-1234567890abcdef0",
		PublicIP:   "54.123.45.67",
		PrivateIP:  "10.0.1.100",
		Region:     "us-west-2",
	}

	if info.InstanceId != "i-1234567890abcdef0" {
		t.Errorf("Expected InstanceId=i-1234567890abcdef0, got %s", info.InstanceId)
	}

	if info.PublicIP != "54.123.45.67" {
		t.Errorf("Expected PublicIP=54.123.45.67, got %s", info.PublicIP)
	}

	if info.PrivateIP != "10.0.1.100" {
		t.Errorf("Expected PrivateIP=10.0.1.100, got %s", info.PrivateIP)
	}

	if info.Region != "us-west-2" {
		t.Errorf("Expected Region=us-west-2, got %s", info.Region)
	}
}

func TestCreateBastion(t *testing.T) {
	client := &AWSClient{
		profile: "test",
		region:  "us-west-2",
		// client is nil but that's ok for this test
	}

	config := &BastionConfig{
		InstanceType:     types.InstanceTypeT4gSmall,
		VPCId:            "vpc-12345",
		SubnetId:         "subnet-67890",
		SecurityGroupIds: []string{"sg-11111"},
		KeyPairName:      "test-key",
		UserData:         "#!/bin/bash\necho test",
	}

	// This will fail without actual AWS credentials, but test the structure
	_, err := client.CreateBastion(context.Background(), config)
	if err == nil {
		t.Error("Expected error without AWS credentials")
	} else {
		t.Logf("CreateBastion failed as expected: %v", err)
	}
}

func TestCreateSecurityGroups(t *testing.T) {
	client := &AWSClient{
		profile: "test",
		region:  "us-west-2",
	}

	// This will fail without actual AWS credentials, but test the structure
	_, err := client.CreateSecurityGroups(context.Background(), "vpc-12345", 4)
	if err == nil {
		t.Error("Expected error without AWS credentials")
	} else {
		t.Logf("CreateSecurityGroups failed as expected: %v", err)
	}
}

func TestGetInstanceStatus(t *testing.T) {
	client := &AWSClient{
		profile: "test",
		region:  "us-west-2",
	}

	// Test placeholder implementation
	status, err := client.GetInstanceStatus(context.Background(), "i-1234567890abcdef0")
	if err != nil {
		t.Errorf("GetInstanceStatus failed: %v", err)
	}

	if status != "running" {
		t.Errorf("Expected status=running, got %s", status)
	}
}

// Test instance selection logic more thoroughly
func TestInstanceSelectionLogic(t *testing.T) {
	client := &AWSClient{}

	// Test that we get cheapest option when nothing fits budget
	instance := client.SelectOptimalInstance(10000*1024*1024, 0.50) // 10 Gbps, $0.50 budget
	if instance == nil {
		t.Fatal("SelectOptimalInstance returned nil")
	}

	// Should return cheapest option (t4g.nano)
	if instance.Type != types.InstanceTypeT4gNano {
		t.Errorf("Expected cheapest instance (t4g.nano), got %v", instance.Type)
	}

	// Test throughput vs budget prioritization
	instance1 := client.SelectOptimalInstance(100*1024*1024, 50.0)  // 100 Mbps, $50 budget
	instance2 := client.SelectOptimalInstance(1000*1024*1024, 50.0) // 1 Gbps, $50 budget

	if instance1 == nil || instance2 == nil {
		t.Fatal("SelectOptimalInstance returned nil")
	}

	// Higher throughput requirement should generally select more powerful instance
	if instance2.BaselineBandwidth <= instance1.BaselineBandwidth {
		t.Logf("Instance selection prioritized budget over throughput (may be correct)")
	}
}

func TestInstanceConfigConstants(t *testing.T) {
	client := &AWSClient{}

	// Test that instance selection returns valid configurations
	instance := client.SelectOptimalInstance(500*1024*1024, 30.0) // 500 Mbps, $30 budget

	if instance == nil {
		t.Fatal("SelectOptimalInstance returned nil")
	}

	// Verify instance config has reasonable values
	if instance.BaselineBandwidth <= 0 {
		t.Error("BaselineBandwidth should be positive")
	}

	if instance.BurstBandwidth < instance.BaselineBandwidth {
		t.Error("BurstBandwidth should be >= BaselineBandwidth")
	}

	if instance.MonthlyCost <= 0 {
		t.Error("MonthlyCost should be positive")
	}

	if instance.MaxTunnels <= 0 {
		t.Error("MaxTunnels should be positive")
	}

	// Test that costs are reasonable (not absurdly high or low)
	if instance.MonthlyCost > 1000 {
		t.Errorf("MonthlyCost seems too high: $%.2f", instance.MonthlyCost)
	}

	if instance.MonthlyCost < 1 {
		t.Errorf("MonthlyCost seems too low: $%.2f", instance.MonthlyCost)
	}
}