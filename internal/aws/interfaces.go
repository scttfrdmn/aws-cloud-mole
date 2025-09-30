package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// AWSClientInterface defines the interface for AWS operations
type AWSClientInterface interface {
	CreateBastion(ctx context.Context, config *BastionConfig) (*BastionInfo, error)
	CreateSecurityGroups(ctx context.Context, vpcID string, tunnelCount int) (string, error)
	SelectOptimalInstance(throughput int64, budget float64) *InstanceConfig
	GetInstanceStatus(ctx context.Context, instanceID string) (string, error)
	ListVPCs(ctx context.Context) ([]VPCInfo, error)
	DeployInfrastructure(ctx context.Context, terraformConfig interface{}) error
	TerminateBastion(ctx context.Context, instanceID string) error
}

// EC2ClientInterface defines the interface for EC2 operations
type EC2ClientInterface interface {
	RunInstances(ctx context.Context, params *RunInstancesInput) (*RunInstancesOutput, error)
	DescribeInstances(ctx context.Context, params *DescribeInstancesInput) (*DescribeInstancesOutput, error)
	CreateSecurityGroup(ctx context.Context, params *CreateSecurityGroupInput) (*CreateSecurityGroupOutput, error)
	AuthorizeSecurityGroupIngress(ctx context.Context, params *AuthorizeSecurityGroupIngressInput) (*AuthorizeSecurityGroupIngressOutput, error)
	DescribeVpcs(ctx context.Context, params *DescribeVpcsInput) (*DescribeVpcsOutput, error)
	TerminateInstances(ctx context.Context, params *TerminateInstancesInput) (*TerminateInstancesOutput, error)
}

// Mock input/output types (simplified for testing)
type RunInstancesInput struct {
	ImageId          string
	InstanceType     types.InstanceType
	MinCount         int32
	MaxCount         int32
	SubnetId         string
	SecurityGroupIds []string
	KeyName          string
	UserData         string
}

type RunInstancesOutput struct {
	Instances []Instance
}

type Instance struct {
	InstanceId   string
	State        InstanceState
	PublicIpAddress  *string
	PrivateIpAddress *string
}

type InstanceState struct {
	Name string
}

type DescribeInstancesInput struct {
	InstanceIds []string
}

type DescribeInstancesOutput struct {
	Reservations []Reservation
}

type Reservation struct {
	Instances []Instance
}

type CreateSecurityGroupInput struct {
	GroupName   string
	Description string
	VpcId       string
}

type CreateSecurityGroupOutput struct {
	GroupId string
}

type AuthorizeSecurityGroupIngressInput struct {
	GroupId     string
	IpProtocol  string
	FromPort    int32
	ToPort      int32
	CidrBlocks  []string
}

type AuthorizeSecurityGroupIngressOutput struct{}

type DescribeVpcsInput struct{}

type DescribeVpcsOutput struct {
	Vpcs []Vpc
}

type Vpc struct {
	VpcId     string
	CidrBlock string
	State     string
	IsDefault bool
	Tags      []Tag
}

type Tag struct {
	Key   string
	Value string
}

type TerminateInstancesInput struct {
	InstanceIds []string
}

type TerminateInstancesOutput struct {
	TerminatingInstances []InstanceStateChange
}

type InstanceStateChange struct {
	InstanceId    string
	CurrentState  InstanceState
	PreviousState InstanceState
}