package aws

import (
	"context"
	"fmt"
)

// MockAWSClient implements AWSClientInterface for testing
type MockAWSClient struct {
	// Control test behavior
	ShouldFailCreateBastion    bool
	ShouldFailCreateSG         bool
	ShouldFailGetInstanceStatus bool
	ShouldFailListVPCs         bool
	ShouldFailDeploy           bool
	ShouldFailTerminate        bool

	// Mock data
	MockInstanceID       string
	MockSecurityGroupID  string
	MockPublicIP         string
	MockPrivateIP        string
	MockInstanceStatus   string
	MockVPCs            []VPCInfo

	// Track calls for verification
	CreateBastionCalls    []BastionConfig
	CreateSGCalls         []CreateSGCall
	GetInstanceStatusCalls []string
	ListVPCsCalls         int
	DeployCalls           []interface{}
	TerminateCalls        []string
	OptimalInstanceCalls  []OptimalInstanceCall
}

type CreateSGCall struct {
	VpcID       string
	TunnelCount int
}

type OptimalInstanceCall struct {
	Throughput int64
	Budget     float64
}

// NewMockAWSClient creates a new mock AWS client
func NewMockAWSClient() *MockAWSClient {
	return &MockAWSClient{
		MockInstanceID:      "i-mockinstance123",
		MockSecurityGroupID: "sg-mocksg456",
		MockPublicIP:        "203.0.113.1",
		MockPrivateIP:       "10.0.1.10",
		MockInstanceStatus:  "running",
		MockVPCs: []VPCInfo{
			{
				VpcId:     "vpc-mock123",
				CidrBlock: "10.0.0.0/16",
				State:     "available",
				IsDefault: true,
				Name:      "Mock VPC",
			},
		},
	}
}

func (m *MockAWSClient) CreateBastion(ctx context.Context, config *BastionConfig) (*BastionInfo, error) {
	m.CreateBastionCalls = append(m.CreateBastionCalls, *config)

	if m.ShouldFailCreateBastion {
		return nil, fmt.Errorf("mock error: failed to create bastion")
	}

	return &BastionInfo{
		InstanceId: m.MockInstanceID,
		PublicIP:   m.MockPublicIP,
		PrivateIP:  m.MockPrivateIP,
		Region:     "us-west-2",
	}, nil
}

func (m *MockAWSClient) CreateSecurityGroups(ctx context.Context, vpcID string, tunnelCount int) (string, error) {
	m.CreateSGCalls = append(m.CreateSGCalls, CreateSGCall{
		VpcID:       vpcID,
		TunnelCount: tunnelCount,
	})

	if m.ShouldFailCreateSG {
		return "", fmt.Errorf("mock error: failed to create security groups")
	}

	return m.MockSecurityGroupID, nil
}

func (m *MockAWSClient) SelectOptimalInstance(throughput int64, budget float64) *InstanceConfig {
	m.OptimalInstanceCalls = append(m.OptimalInstanceCalls, OptimalInstanceCall{
		Throughput: throughput,
		Budget:     budget,
	})

	// Use the real logic but with controlled data
	client := &AWSClient{}
	return client.SelectOptimalInstance(throughput, budget)
}

func (m *MockAWSClient) GetInstanceStatus(ctx context.Context, instanceID string) (string, error) {
	m.GetInstanceStatusCalls = append(m.GetInstanceStatusCalls, instanceID)

	if m.ShouldFailGetInstanceStatus {
		return "", fmt.Errorf("mock error: failed to get instance status")
	}

	return m.MockInstanceStatus, nil
}

func (m *MockAWSClient) ListVPCs(ctx context.Context) ([]VPCInfo, error) {
	m.ListVPCsCalls++

	if m.ShouldFailListVPCs {
		return nil, fmt.Errorf("mock error: failed to list VPCs")
	}

	return m.MockVPCs, nil
}

func (m *MockAWSClient) DeployInfrastructure(ctx context.Context, terraformConfig interface{}) error {
	m.DeployCalls = append(m.DeployCalls, terraformConfig)

	if m.ShouldFailDeploy {
		return fmt.Errorf("mock error: failed to deploy infrastructure")
	}

	return nil
}

func (m *MockAWSClient) TerminateBastion(ctx context.Context, instanceID string) error {
	m.TerminateCalls = append(m.TerminateCalls, instanceID)

	if m.ShouldFailTerminate {
		return fmt.Errorf("mock error: failed to terminate bastion")
	}

	return nil
}

// MockEC2Client implements EC2ClientInterface for testing
type MockEC2Client struct {
	ShouldFailRunInstances        bool
	ShouldFailDescribeInstances   bool
	ShouldFailCreateSecurityGroup bool
	ShouldFailAuthorizeIngress    bool
	ShouldFailDescribeVpcs        bool
	ShouldFailTerminateInstances  bool

	MockInstances []Instance
	MockVpcs      []Vpc
}

func NewMockEC2Client() *MockEC2Client {
	return &MockEC2Client{
		MockInstances: []Instance{
			{
				InstanceId: "i-mockinstance123",
				State: InstanceState{
					Name: "running",
				},
				PublicIpAddress:  stringPtr("203.0.113.1"),
				PrivateIpAddress: stringPtr("10.0.1.10"),
			},
		},
		MockVpcs: []Vpc{
			{
				VpcId:     "vpc-mock123",
				CidrBlock: "10.0.0.0/16",
				State:     "available",
				IsDefault: true,
				Tags: []Tag{
					{Key: "Name", Value: "Mock VPC"},
				},
			},
		},
	}
}

func (m *MockEC2Client) RunInstances(ctx context.Context, params *RunInstancesInput) (*RunInstancesOutput, error) {
	if m.ShouldFailRunInstances {
		return nil, fmt.Errorf("mock error: failed to run instances")
	}

	return &RunInstancesOutput{
		Instances: m.MockInstances,
	}, nil
}

func (m *MockEC2Client) DescribeInstances(ctx context.Context, params *DescribeInstancesInput) (*DescribeInstancesOutput, error) {
	if m.ShouldFailDescribeInstances {
		return nil, fmt.Errorf("mock error: failed to describe instances")
	}

	return &DescribeInstancesOutput{
		Reservations: []Reservation{
			{Instances: m.MockInstances},
		},
	}, nil
}

func (m *MockEC2Client) CreateSecurityGroup(ctx context.Context, params *CreateSecurityGroupInput) (*CreateSecurityGroupOutput, error) {
	if m.ShouldFailCreateSecurityGroup {
		return nil, fmt.Errorf("mock error: failed to create security group")
	}

	return &CreateSecurityGroupOutput{
		GroupId: "sg-mocksg456",
	}, nil
}

func (m *MockEC2Client) AuthorizeSecurityGroupIngress(ctx context.Context, params *AuthorizeSecurityGroupIngressInput) (*AuthorizeSecurityGroupIngressOutput, error) {
	if m.ShouldFailAuthorizeIngress {
		return nil, fmt.Errorf("mock error: failed to authorize security group ingress")
	}

	return &AuthorizeSecurityGroupIngressOutput{}, nil
}

func (m *MockEC2Client) DescribeVpcs(ctx context.Context, params *DescribeVpcsInput) (*DescribeVpcsOutput, error) {
	if m.ShouldFailDescribeVpcs {
		return nil, fmt.Errorf("mock error: failed to describe VPCs")
	}

	return &DescribeVpcsOutput{
		Vpcs: m.MockVpcs,
	}, nil
}

func (m *MockEC2Client) TerminateInstances(ctx context.Context, params *TerminateInstancesInput) (*TerminateInstancesOutput, error) {
	if m.ShouldFailTerminateInstances {
		return nil, fmt.Errorf("mock error: failed to terminate instances")
	}

	return &TerminateInstancesOutput{
		TerminatingInstances: []InstanceStateChange{
			{
				InstanceId: params.InstanceIds[0],
				CurrentState: InstanceState{Name: "shutting-down"},
				PreviousState: InstanceState{Name: "running"},
			},
		},
	}, nil
}

func stringPtr(s string) *string {
	return &s
}