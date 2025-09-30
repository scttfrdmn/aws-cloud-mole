package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// NetworkConfig contains network creation parameters
type NetworkConfig struct {
	VPCCidr           string
	PublicSubnetCidr  string
	PrivateSubnetCidr string
	Region            string
	EnableNAT         bool
}

// NetworkResult contains created network infrastructure IDs
type NetworkResult struct {
	VPCId           string
	PublicSubnetId  string
	PrivateSubnetId string
	InternetGatewayId string
	PublicRouteTableId string
	PrivateRouteTableId string
}

// CreateNetworkInfrastructure creates a complete VPC with public/private subnets
func (a *AWSClient) CreateNetworkInfrastructure(ctx context.Context, config *NetworkConfig) (*NetworkResult, error) {
	result := &NetworkResult{}

	// Step 1: Create VPC
	fmt.Println("   🏗️  Creating VPC...")
	vpcResult, err := a.client.CreateVpc(ctx, &ec2.CreateVpcInput{
		CidrBlock: &config.VPCCidr,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeVpc,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("mole-vpc")},
					{Key: aws.String("Purpose"), Value: aws.String("wireguard-tunnel-terminator")},
					{Key: aws.String("CreatedBy"), Value: aws.String("aws-cloud-mole")},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC: %w", err)
	}
	result.VPCId = *vpcResult.Vpc.VpcId

	// Step 2: Create Internet Gateway
	fmt.Println("   🌐 Creating Internet Gateway...")
	igwResult, err := a.client.CreateInternetGateway(ctx, &ec2.CreateInternetGatewayInput{
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInternetGateway,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("mole-igw")},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Internet Gateway: %w", err)
	}
	result.InternetGatewayId = *igwResult.InternetGateway.InternetGatewayId

	// Step 3: Attach Internet Gateway to VPC
	_, err = a.client.AttachInternetGateway(ctx, &ec2.AttachInternetGatewayInput{
		InternetGatewayId: &result.InternetGatewayId,
		VpcId:             &result.VPCId,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to attach Internet Gateway: %w", err)
	}

	// Step 4: Create Public Subnet
	fmt.Println("   🌐 Creating public subnet...")
	publicSubnetResult, err := a.client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
		VpcId:     &result.VPCId,
		CidrBlock: &config.PublicSubnetCidr,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSubnet,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("mole-public-subnet")},
					{Key: aws.String("Type"), Value: aws.String("public")},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create public subnet: %w", err)
	}
	result.PublicSubnetId = *publicSubnetResult.Subnet.SubnetId

	// Enable auto-assign public IP for public subnet
	_, err = a.client.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
		SubnetId:            &result.PublicSubnetId,
		MapPublicIpOnLaunch: &types.AttributeBooleanValue{Value: aws.Bool(true)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to enable public IP assignment: %w", err)
	}

	// Step 5: Create Public Route Table
	fmt.Println("   🗺️  Creating public route table...")
	publicRtResult, err := a.client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
		VpcId: &result.VPCId,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeRouteTable,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("mole-public-rt")},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create public route table: %w", err)
	}
	result.PublicRouteTableId = *publicRtResult.RouteTable.RouteTableId

	// Step 6: Create route to Internet Gateway
	_, err = a.client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         &result.PublicRouteTableId,
		DestinationCidrBlock: aws.String("0.0.0.0/0"),
		GatewayId:            &result.InternetGatewayId,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create route to Internet Gateway: %w", err)
	}

	// Step 7: Associate public subnet with public route table
	_, err = a.client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
		RouteTableId: &result.PublicRouteTableId,
		SubnetId:     &result.PublicSubnetId,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to associate public subnet with route table: %w", err)
	}

	// Step 8: Create Private Subnet (if specified)
	if config.PrivateSubnetCidr != "" {
		fmt.Println("   🔒 Creating private subnet...")
		privateSubnetResult, err := a.client.CreateSubnet(ctx, &ec2.CreateSubnetInput{
			VpcId:     &result.VPCId,
			CidrBlock: &config.PrivateSubnetCidr,
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeSubnet,
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String("mole-private-subnet")},
						{Key: aws.String("Type"), Value: aws.String("private")},
					},
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create private subnet: %w", err)
		}
		result.PrivateSubnetId = *privateSubnetResult.Subnet.SubnetId

		// Step 9: Create Private Route Table
		fmt.Println("   🗺️  Creating private route table...")
		privateRtResult, err := a.client.CreateRouteTable(ctx, &ec2.CreateRouteTableInput{
			VpcId: &result.VPCId,
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeRouteTable,
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String("mole-private-rt")},
					},
				},
			},
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create private route table: %w", err)
		}
		result.PrivateRouteTableId = *privateRtResult.RouteTable.RouteTableId

		// Step 10: Associate private subnet with private route table
		_, err = a.client.AssociateRouteTable(ctx, &ec2.AssociateRouteTableInput{
			RouteTableId: &result.PrivateRouteTableId,
			SubnetId:     &result.PrivateSubnetId,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to associate private subnet with route table: %w", err)
		}
	}

	// Wait for VPC to be available
	fmt.Println("   ⏳ Waiting for VPC to be available...")
	waiter := ec2.NewVpcAvailableWaiter(a.client)
	err = waiter.Wait(ctx, &ec2.DescribeVpcsInput{
		VpcIds: []string{result.VPCId},
	}, 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("VPC did not become available: %w", err)
	}

	fmt.Printf("   ✅ Network infrastructure ready!\n")
	return result, nil
}