package aws

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// Test CreateBastion with mocked AWS client
func TestCreateBastionWithMock(t *testing.T) {
	mock := NewMockAWSClient()

	config := &BastionConfig{
		InstanceType:     types.InstanceTypeT4gSmall,
		VPCId:            "vpc-12345",
		SubnetId:         "subnet-67890",
		SecurityGroupIds: []string{"sg-11111"},
		KeyPairName:      "test-key",
		UserData:         "#!/bin/bash\necho test",
	}

	// Test successful creation
	info, err := mock.CreateBastion(context.Background(), config)
	if err != nil {
		t.Fatalf("CreateBastion should succeed with mock: %v", err)
	}

	if info == nil {
		t.Fatal("CreateBastion should return bastion info")
	}

	if info.InstanceId != mock.MockInstanceID {
		t.Errorf("Expected instance ID %s, got %s", mock.MockInstanceID, info.InstanceId)
	}

	if info.PublicIP != mock.MockPublicIP {
		t.Errorf("Expected public IP %s, got %s", mock.MockPublicIP, info.PublicIP)
	}

	// Verify the call was tracked
	if len(mock.CreateBastionCalls) != 1 {
		t.Errorf("Expected 1 CreateBastion call, got %d", len(mock.CreateBastionCalls))
	}

	if mock.CreateBastionCalls[0].VPCId != "vpc-12345" {
		t.Errorf("Expected VPC ID vpc-12345, got %s", mock.CreateBastionCalls[0].VPCId)
	}
}

// Test CreateBastion failure scenario
func TestCreateBastionFailure(t *testing.T) {
	mock := NewMockAWSClient()
	mock.ShouldFailCreateBastion = true

	config := &BastionConfig{
		InstanceType: types.InstanceTypeT4gSmall,
		VPCId:        "vpc-12345",
	}

	_, err := mock.CreateBastion(context.Background(), config)
	if err == nil {
		t.Error("CreateBastion should fail when mock is configured to fail")
	}

	expectedError := "mock error: failed to create bastion"
	if err.Error() != expectedError {
		t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
	}
}

// Test CreateSecurityGroups with mock
func TestCreateSecurityGroupsWithMock(t *testing.T) {
	mock := NewMockAWSClient()

	sgID, err := mock.CreateSecurityGroups(context.Background(), "vpc-test", 4)
	if err != nil {
		t.Fatalf("CreateSecurityGroups should succeed with mock: %v", err)
	}

	if sgID != mock.MockSecurityGroupID {
		t.Errorf("Expected SG ID %s, got %s", mock.MockSecurityGroupID, sgID)
	}

	// Verify call tracking
	if len(mock.CreateSGCalls) != 1 {
		t.Errorf("Expected 1 CreateSecurityGroups call, got %d", len(mock.CreateSGCalls))
	}

	call := mock.CreateSGCalls[0]
	if call.VpcID != "vpc-test" {
		t.Errorf("Expected VPC ID vpc-test, got %s", call.VpcID)
	}

	if call.TunnelCount != 4 {
		t.Errorf("Expected tunnel count 4, got %d", call.TunnelCount)
	}
}

// Test CreateSecurityGroups failure
func TestCreateSecurityGroupsFailure(t *testing.T) {
	mock := NewMockAWSClient()
	mock.ShouldFailCreateSG = true

	_, err := mock.CreateSecurityGroups(context.Background(), "vpc-test", 4)
	if err == nil {
		t.Error("CreateSecurityGroups should fail when mock is configured to fail")
	}
}

// Test GetInstanceStatus with mock
func TestGetInstanceStatusWithMock(t *testing.T) {
	mock := NewMockAWSClient()

	status, err := mock.GetInstanceStatus(context.Background(), "i-test123")
	if err != nil {
		t.Fatalf("GetInstanceStatus should succeed with mock: %v", err)
	}

	if status != mock.MockInstanceStatus {
		t.Errorf("Expected status %s, got %s", mock.MockInstanceStatus, status)
	}

	// Verify call tracking
	if len(mock.GetInstanceStatusCalls) != 1 {
		t.Errorf("Expected 1 GetInstanceStatus call, got %d", len(mock.GetInstanceStatusCalls))
	}

	if mock.GetInstanceStatusCalls[0] != "i-test123" {
		t.Errorf("Expected instance ID i-test123, got %s", mock.GetInstanceStatusCalls[0])
	}
}

// Test ListVPCs with mock
func TestListVPCsWithMock(t *testing.T) {
	mock := NewMockAWSClient()

	vpcs, err := mock.ListVPCs(context.Background())
	if err != nil {
		t.Fatalf("ListVPCs should succeed with mock: %v", err)
	}

	if len(vpcs) != len(mock.MockVPCs) {
		t.Errorf("Expected %d VPCs, got %d", len(mock.MockVPCs), len(vpcs))
	}

	if len(vpcs) > 0 {
		vpc := vpcs[0]
		mockVpc := mock.MockVPCs[0]

		if vpc.VpcId != mockVpc.VpcId {
			t.Errorf("Expected VPC ID %s, got %s", mockVpc.VpcId, vpc.VpcId)
		}

		if vpc.CidrBlock != mockVpc.CidrBlock {
			t.Errorf("Expected CIDR %s, got %s", mockVpc.CidrBlock, vpc.CidrBlock)
		}

		if vpc.IsDefault != mockVpc.IsDefault {
			t.Errorf("Expected IsDefault %v, got %v", mockVpc.IsDefault, vpc.IsDefault)
		}
	}

	// Verify call tracking
	if mock.ListVPCsCalls != 1 {
		t.Errorf("Expected 1 ListVPCs call, got %d", mock.ListVPCsCalls)
	}
}

// Test DeployInfrastructure with mock
func TestDeployInfrastructureWithMock(t *testing.T) {
	mock := NewMockAWSClient()

	terraformConfig := map[string]interface{}{
		"region":        "us-west-2",
		"instance_type": "t4g.small",
	}

	err := mock.DeployInfrastructure(context.Background(), terraformConfig)
	if err != nil {
		t.Fatalf("DeployInfrastructure should succeed with mock: %v", err)
	}

	// Verify call tracking
	if len(mock.DeployCalls) != 1 {
		t.Errorf("Expected 1 DeployInfrastructure call, got %d", len(mock.DeployCalls))
	}
}

// Test TerminateBastion with mock
func TestTerminateBastionWithMock(t *testing.T) {
	mock := NewMockAWSClient()

	err := mock.TerminateBastion(context.Background(), "i-test123")
	if err != nil {
		t.Fatalf("TerminateBastion should succeed with mock: %v", err)
	}

	// Verify call tracking
	if len(mock.TerminateCalls) != 1 {
		t.Errorf("Expected 1 TerminateBastion call, got %d", len(mock.TerminateCalls))
	}

	if mock.TerminateCalls[0] != "i-test123" {
		t.Errorf("Expected instance ID i-test123, got %s", mock.TerminateCalls[0])
	}
}

// Test multiple operations in sequence
func TestMultipleOperationsSequence(t *testing.T) {
	mock := NewMockAWSClient()

	// Create bastion
	bastionConfig := &BastionConfig{
		InstanceType: types.InstanceTypeT4gSmall,
		VPCId:        "vpc-sequence",
		SubnetId:     "subnet-sequence",
	}

	info, err := mock.CreateBastion(context.Background(), bastionConfig)
	if err != nil {
		t.Fatalf("CreateBastion failed: %v", err)
	}

	// Create security groups
	sgID, err := mock.CreateSecurityGroups(context.Background(), "vpc-sequence", 2)
	if err != nil {
		t.Fatalf("CreateSecurityGroups failed: %v", err)
	}

	// Check instance status
	status, err := mock.GetInstanceStatus(context.Background(), info.InstanceId)
	if err != nil {
		t.Fatalf("GetInstanceStatus failed: %v", err)
	}

	if status != "running" {
		t.Errorf("Expected status 'running', got '%s'", status)
	}

	// Terminate instance
	err = mock.TerminateBastion(context.Background(), info.InstanceId)
	if err != nil {
		t.Fatalf("TerminateBastion failed: %v", err)
	}

	// Verify all calls were tracked
	if len(mock.CreateBastionCalls) != 1 {
		t.Errorf("Expected 1 CreateBastion call, got %d", len(mock.CreateBastionCalls))
	}

	if len(mock.CreateSGCalls) != 1 {
		t.Errorf("Expected 1 CreateSecurityGroups call, got %d", len(mock.CreateSGCalls))
	}

	if len(mock.GetInstanceStatusCalls) != 1 {
		t.Errorf("Expected 1 GetInstanceStatus call, got %d", len(mock.GetInstanceStatusCalls))
	}

	if len(mock.TerminateCalls) != 1 {
		t.Errorf("Expected 1 TerminateBastion call, got %d", len(mock.TerminateCalls))
	}

	// Verify security group was created for correct VPC
	sgCall := mock.CreateSGCalls[0]
	if sgCall.VpcID != "vpc-sequence" {
		t.Errorf("Security group created for wrong VPC: expected vpc-sequence, got %s", sgCall.VpcID)
	}

	if sgCall.TunnelCount != 2 {
		t.Errorf("Security group created for wrong tunnel count: expected 2, got %d", sgCall.TunnelCount)
	}

	t.Logf("Sequence test completed: created bastion %s, SG %s, checked status %s, terminated instance",
		info.InstanceId, sgID, status)
}