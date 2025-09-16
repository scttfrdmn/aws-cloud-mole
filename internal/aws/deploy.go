package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// DeploymentConfig contains all deployment parameters
type DeploymentConfig struct {
	VPCId        string
	SubnetId     string
	InstanceType types.InstanceType
	TunnelCount  int
	MTUSize      int
	AllowedCIDR  string
	SSHPublicKey string
	Profile      string
	Region       string
}

// DeploymentResult contains deployment outputs
type DeploymentResult struct {
	BastionInstanceID string
	BastionPublicIP   string
	BastionPrivateIP  string
	SecurityGroupID   string
	KeyPairName       string
	TunnelPorts       []int
	CostEstimate      CostEstimate
}

// CostEstimate contains cost information
type CostEstimate struct {
	HourlyCost  float64
	DailyCost   float64
	MonthlyCost float64
}

// DirectDeploy deploys infrastructure directly using AWS SDK
func (a *AWSClient) DirectDeploy(ctx context.Context, config *DeploymentConfig) (*DeploymentResult, error) {
	fmt.Println("🚀 Starting direct AWS deployment...")

	result := &DeploymentResult{
		TunnelPorts: make([]int, config.TunnelCount),
	}

	// Generate tunnel ports
	for i := 0; i < config.TunnelCount; i++ {
		result.TunnelPorts[i] = 51820 + i
	}

	// Step 1: Create Security Group
	fmt.Println("🔒 Creating security group...")
	sgID, err := a.createSecurityGroup(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create security group: %w", err)
	}
	result.SecurityGroupID = sgID
	fmt.Printf("  ✓ Security group created: %s\n", sgID)

	// Step 2: Create/Import SSH Key
	fmt.Println("🔑 Setting up SSH key...")
	keyName, err := a.createKeyPair(ctx, config.SSHPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create key pair: %w", err)
	}
	result.KeyPairName = keyName
	fmt.Printf("  ✓ SSH key configured: %s\n", keyName)

	// Step 3: Launch Instance
	fmt.Println("☁️  Launching bastion instance...")
	instanceID, err := a.launchBastion(ctx, config, sgID, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to launch bastion: %w", err)
	}
	result.BastionInstanceID = instanceID
	fmt.Printf("  ✓ Instance launched: %s\n", instanceID)

	// Step 4: Wait for instance to be running
	fmt.Println("⏳ Waiting for instance to be running...")
	if err := a.waitForInstanceRunning(ctx, instanceID); err != nil {
		return nil, fmt.Errorf("instance failed to start: %w", err)
	}

	// Step 5: Get instance details
	fmt.Println("📋 Retrieving instance details...")
	publicIP, privateIP, err := a.getInstanceIPs(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance IPs: %w", err)
	}
	result.BastionPublicIP = publicIP
	result.BastionPrivateIP = privateIP

	// Step 6: Calculate cost estimate
	result.CostEstimate = a.calculateCostEstimate(config.InstanceType)

	fmt.Printf("🎉 Deployment complete!\n")
	fmt.Printf("  Bastion IP: %s\n", publicIP)
	fmt.Printf("  Instance: %s (%s)\n", instanceID, config.InstanceType)
	fmt.Printf("  Monthly cost: $%.2f\n", result.CostEstimate.MonthlyCost)

	return result, nil
}

// createSecurityGroup creates a security group for WireGuard
func (a *AWSClient) createSecurityGroup(ctx context.Context, config *DeploymentConfig) (string, error) {
	groupName := fmt.Sprintf("mole-wireguard-%d", time.Now().Unix())

	// Create security group
	createOutput, err := a.client.CreateSecurityGroup(ctx, &ec2.CreateSecurityGroupInput{
		GroupName:   &groupName,
		Description: aws.String("AWS Cloud Mole WireGuard Security Group"),
		VpcId:       &config.VPCId,
	})
	if err != nil {
		return "", err
	}

	sgID := *createOutput.GroupId

	// Create ingress rules
	var ingressRules []types.IpPermission

	// WireGuard ports
	for i := 0; i < config.TunnelCount; i++ {
		port := int32(51820 + i)
		ingressRules = append(ingressRules, types.IpPermission{
			IpProtocol: aws.String("udp"),
			FromPort:   &port,
			ToPort:     &port,
			IpRanges: []types.IpRange{
				{
					CidrIp:      &config.AllowedCIDR,
					Description: aws.String(fmt.Sprintf("WireGuard tunnel %d", i)),
				},
			},
		})
	}

	// SSH port
	sshPort := int32(22)
	ingressRules = append(ingressRules, types.IpPermission{
		IpProtocol: aws.String("tcp"),
		FromPort:   &sshPort,
		ToPort:     &sshPort,
		IpRanges: []types.IpRange{
			{
				CidrIp:      &config.AllowedCIDR,
				Description: aws.String("SSH management"),
			},
		},
	})

	// Add ingress rules
	_, err = a.client.AuthorizeSecurityGroupIngress(ctx, &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       &sgID,
		IpPermissions: ingressRules,
	})
	if err != nil {
		return "", err
	}

	return sgID, nil
}

// createKeyPair creates or imports an SSH key pair
func (a *AWSClient) createKeyPair(ctx context.Context, publicKey string) (string, error) {
	keyName := fmt.Sprintf("mole-key-%d", time.Now().Unix())

	_, err := a.client.ImportKeyPair(ctx, &ec2.ImportKeyPairInput{
		KeyName:           &keyName,
		PublicKeyMaterial: []byte(publicKey),
	})
	if err != nil {
		return "", err
	}

	return keyName, nil
}

// launchBastion launches the bastion EC2 instance
func (a *AWSClient) launchBastion(ctx context.Context, config *DeploymentConfig, sgID, keyName string) (string, error) {
	// Get Ubuntu AMI
	ami, err := a.getUbuntuAMI(ctx)
	if err != nil {
		return "", err
	}

	// Create user data script
	userData := a.generateUserData(config.TunnelCount, config.MTUSize)

	// Launch instance
	runResult, err := a.client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:          &ami,
		InstanceType:     config.InstanceType,
		KeyName:          &keyName,
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		SecurityGroupIds: []string{sgID},
		SubnetId:         &config.SubnetId,
		UserData:         &userData,
		Monitoring: &types.RunInstancesMonitoringEnabled{
			Enabled: aws.Bool(true),
		},
		BlockDeviceMappings: []types.BlockDeviceMapping{
			{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &types.EbsBlockDevice{
					VolumeSize:          aws.Int32(20),
					VolumeType:          types.VolumeTypeGp3,
					DeleteOnTermination: aws.Bool(true),
					Encrypted:           aws.Bool(true),
				},
			},
		},
	})
	if err != nil {
		return "", err
	}

	instanceID := *runResult.Instances[0].InstanceId

	// Tag the instance
	_, err = a.client.CreateTags(ctx, &ec2.CreateTagsInput{
		Resources: []string{instanceID},
		Tags: []types.Tag{
			{Key: aws.String("Name"), Value: aws.String("mole-bastion")},
			{Key: aws.String("Project"), Value: aws.String("aws-cloud-mole")},
			{Key: aws.String("ManagedBy"), Value: aws.String("mole-cli")},
		},
	})
	if err != nil {
		// Non-fatal error
		fmt.Printf("Warning: failed to tag instance: %v\n", err)
	}

	return instanceID, nil
}

// generateUserData creates the user data script
func (a *AWSClient) generateUserData(tunnelCount, mtuSize int) string {
	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

# Variables
TUNNEL_COUNT=%d
MTU_SIZE=%d

# Update and install packages
apt-get update -y
apt-get install -y wireguard wireguard-tools iptables-persistent awscli

# Enable IP forwarding
echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
sysctl -p

# Create directories
mkdir -p /etc/mole/keys

# Generate keys and configurations
for i in $(seq 0 $((TUNNEL_COUNT - 1))); do
    wg genkey | tee /etc/mole/keys/wg${i}_private.key | wg pubkey > /etc/mole/keys/wg${i}_public.key
    chmod 600 /etc/mole/keys/wg${i}_private.key
done

# Signal completion
echo "ready" > /etc/mole/status
`, tunnelCount, mtuSize)

	// Base64 encode the script
	return script
}

// waitForInstanceRunning waits for instance to reach running state
func (a *AWSClient) waitForInstanceRunning(ctx context.Context, instanceID string) error {
	waiter := ec2.NewInstanceRunningWaiter(a.client)
	return waiter.Wait(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	}, 5*time.Minute)
}

// getInstanceIPs retrieves public and private IP addresses
func (a *AWSClient) getInstanceIPs(ctx context.Context, instanceID string) (publicIP, privateIP string, err error) {
	output, err := a.client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return "", "", err
	}

	if len(output.Reservations) == 0 || len(output.Reservations[0].Instances) == 0 {
		return "", "", fmt.Errorf("instance not found")
	}

	instance := output.Reservations[0].Instances[0]

	if instance.PublicIpAddress != nil {
		publicIP = *instance.PublicIpAddress
	}

	if instance.PrivateIpAddress != nil {
		privateIP = *instance.PrivateIpAddress
	}

	return publicIP, privateIP, nil
}

// calculateCostEstimate calculates cost for the instance
func (a *AWSClient) calculateCostEstimate(instanceType types.InstanceType) CostEstimate {
	// Cost per hour for Graviton instances (approximate)
	costs := map[types.InstanceType]float64{
		types.InstanceTypeT4gNano:    0.0042,
		types.InstanceTypeT4gMicro:   0.0084,
		types.InstanceTypeT4gSmall:   0.0168,
		types.InstanceTypeT4gMedium:  0.0336,
		types.InstanceTypeC6gnMedium: 0.0864,
		types.InstanceTypeC6gnLarge:  0.1728,
		types.InstanceTypeC6gnXlarge: 0.3456,
	}

	hourly := costs[instanceType]
	if hourly == 0 {
		hourly = 0.05 // Default estimate
	}

	return CostEstimate{
		HourlyCost:  hourly,
		DailyCost:   hourly * 24,
		MonthlyCost: hourly * 24 * 30.4,
	}
}

// getUbuntuAMI finds the latest Ubuntu 22.04 LTS AMI
func (a *AWSClient) getUbuntuAMI(ctx context.Context) (string, error) {
	output, err := a.client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"099720109477"}, // Canonical
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"ubuntu/images/hvm-ssd/ubuntu-22.04-lts-*"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"arm64"}, // For Graviton instances
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
		},
	})
	if err != nil {
		return "", err
	}

	if len(output.Images) == 0 {
		return "", fmt.Errorf("no suitable Ubuntu AMI found")
	}

	// Return the most recent AMI
	latestAMI := output.Images[0]
	for _, image := range output.Images {
		if image.CreationDate != nil && latestAMI.CreationDate != nil &&
			*image.CreationDate > *latestAMI.CreationDate {
			latestAMI = image
		}
	}

	return *latestAMI.ImageId, nil
}
