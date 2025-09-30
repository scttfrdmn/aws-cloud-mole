package aws

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	"golang.org/x/crypto/curve25519"
)

// DeploymentConfig contains all deployment parameters
type DeploymentConfig struct {
	VPCId           string
	PublicSubnetId  string           // Public subnet for tunnel terminator
	PrivateSubnetId string           // Private subnet to provide NAT for (optional)
	VPCCidr         string           // VPC CIDR for routing configuration
	InstanceType    types.InstanceType
	TunnelCount     int
	MTUSize         int
	AllowedCIDR     string
	SSHPublicKey    string
	Profile         string
	Region          string
	EnableNAT       bool             // Enable NAT functionality for private subnet
	DeployTarget    bool             // Deploy test target instance in private subnet
	TargetInstance  types.InstanceType // Instance type for test target
	ClientPrivateKey string           // Local WireGuard private key (generated during deployment)
	ClientPublicKey  string           // Local WireGuard public key (sent to server)
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
	TargetInstanceID  string  // Test target instance ID (if deployed)
	TargetPrivateIP   string  // Test target private IP (if deployed)
	ClientPrivateKey  string  // Local WireGuard private key
	ClientPublicKey   string  // Local WireGuard public key
	ServerPublicKey   string  // Server WireGuard public key (retrieved from instance)
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

	// Step 2: Create IAM role for EC2 instance
	fmt.Println("🔒 Creating IAM role for instance permissions...")
	roleName, err := a.createIAMRole(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create IAM role: %w", err)
	}
	fmt.Printf("  ✓ IAM role created: %s\n", roleName)

	// Step 3: Create AWS-managed key pair (for emergency access only)
	fmt.Println("🔑 Setting up emergency access key...")
	keyName, err := a.createKeyPair(ctx, config.SSHPublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create key pair: %w", err)
	}
	result.KeyPairName = keyName
	fmt.Printf("  ✓ Emergency key configured: %s\n", keyName)

	// Step 4: Generate local WireGuard client keys
	fmt.Println("🔑 Generating WireGuard client keys...")
	clientPrivateKey, clientPublicKey, err := a.generateWireGuardKeys()
	if err != nil {
		return nil, fmt.Errorf("failed to generate WireGuard keys: %w", err)
	}
	config.ClientPrivateKey = clientPrivateKey
	config.ClientPublicKey = clientPublicKey
	fmt.Printf("  ✓ Client keys generated\n")

	// Step 5: Launch Instance with client public key
	fmt.Println("☁️  Launching bastion instance...")
	instanceID, err := a.launchBastion(ctx, config, sgID, keyName, roleName)
	if err != nil {
		return nil, fmt.Errorf("failed to launch bastion: %w", err)
	}
	result.BastionInstanceID = instanceID
	fmt.Printf("  ✓ Instance launched: %s\n", instanceID)

	// Step 5: Wait for instance to be running
	fmt.Println("⏳ Waiting for instance to be running...")
	if err := a.waitForInstanceRunning(ctx, instanceID); err != nil {
		return nil, fmt.Errorf("instance failed to start: %w", err)
	}

	// Step 6: Get instance details
	fmt.Println("📋 Retrieving instance details...")
	publicIP, privateIP, err := a.getInstanceIPs(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance IPs: %w", err)
	}
	result.BastionPublicIP = publicIP
	result.BastionPrivateIP = privateIP

	// Step 7: Retrieve server WireGuard public key from instance tags
	fmt.Println("🔑 Retrieving server WireGuard public key...")
	serverPublicKey, err := a.getServerPublicKey(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server public key: %w", err)
	}
	result.ServerPublicKey = serverPublicKey
	result.ClientPrivateKey = config.ClientPrivateKey
	result.ClientPublicKey = config.ClientPublicKey
	fmt.Printf("  ✓ Keys exchanged successfully\n")

	// Step 8: Deploy test target instance if requested
	if config.DeployTarget && config.PrivateSubnetId != "" {
		fmt.Println("🎯 Deploying test target in private subnet...")
		targetID, targetIP, err := a.deployTargetInstance(ctx, config, sgID, keyName)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy test target: %w", err)
		}
		result.TargetInstanceID = targetID
		result.TargetPrivateIP = targetIP
		fmt.Printf("  ✓ Target instance: %s (%s)\n", targetID, targetIP)
	}

	// Step 9: Configure routing for private subnet (if specified)
	if config.PrivateSubnetId != "" {
		fmt.Println("🗺️  Configuring routes for private subnet access...")
		err := a.configurePrivateSubnetRouting(ctx, config, instanceID)
		if err != nil {
			fmt.Printf("  ⚠️  Warning: Failed to configure private subnet routing: %v\n", err)
		} else {
			fmt.Printf("  ✅ Private subnet routing configured\n")
		}
	}

	// Step 10: Calculate cost estimate
	result.CostEstimate = a.calculateCostEstimate(config.InstanceType)
	if config.DeployTarget {
		targetCost := a.calculateCostEstimate(config.TargetInstance)
		result.CostEstimate.HourlyCost += targetCost.HourlyCost
		result.CostEstimate.DailyCost += targetCost.DailyCost
		result.CostEstimate.MonthlyCost += targetCost.MonthlyCost
	}

	fmt.Printf("🎉 Deployment complete!\n")
	fmt.Printf("  Bastion IP: %s\n", publicIP)
	fmt.Printf("  Instance: %s (%s)\n", instanceID, config.InstanceType)
	if config.DeployTarget {
		fmt.Printf("  Target IP: %s (%s)\n", result.TargetPrivateIP, result.TargetInstanceID)
	}
	fmt.Printf("  Monthly cost: $%.2f\n", result.CostEstimate.MonthlyCost)

	// Step 11: Auto-establish WireGuard tunnel
	fmt.Println("🔗 Establishing WireGuard tunnel...")
	err = a.setupLocalTunnel(result)
	if err != nil {
		fmt.Printf("  ⚠️  Warning: Failed to establish local tunnel: %v\n", err)
		fmt.Printf("  💡 You can manually establish the tunnel later using the saved config\n")
	} else {
		fmt.Printf("  ✅ WireGuard tunnel established successfully!\n")
		fmt.Printf("  🎯 Try: ping 10.100.2.8\n")
	}

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

	// ICMP (ping) - allow from VPC CIDR and tunnel network
	ingressRules = append(ingressRules, types.IpPermission{
		IpProtocol: aws.String("icmp"),
		FromPort:   aws.Int32(-1),
		ToPort:     aws.Int32(-1),
		IpRanges: []types.IpRange{
			{
				CidrIp:      &config.VPCCidr,
				Description: aws.String("ICMP from VPC"),
			},
			{
				CidrIp:      aws.String("10.100.1.0/24"),
				Description: aws.String("ICMP from WireGuard tunnel"),
			},
		},
	})

	// HTTP port 8080 for test server (from tunnel network)
	httpPort := int32(8080)
	ingressRules = append(ingressRules, types.IpPermission{
		IpProtocol: aws.String("tcp"),
		FromPort:   &httpPort,
		ToPort:     &httpPort,
		IpRanges: []types.IpRange{
			{
				CidrIp:      aws.String("10.100.1.0/24"),
				Description: aws.String("HTTP test server from tunnel"),
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

// createKeyPair creates an AWS-managed SSH key pair
func (a *AWSClient) createKeyPair(ctx context.Context, publicKey string) (string, error) {
	keyName := fmt.Sprintf("mole-key-%d", time.Now().Unix())

	// Create new AWS-managed key pair
	result, err := a.client.CreateKeyPair(ctx, &ec2.CreateKeyPairInput{
		KeyName: &keyName,
		KeyType: types.KeyTypeRsa,
		KeyFormat: types.KeyFormatPem,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create key pair: %w", err)
	}

	// Save private key to ~/.mole/keys/ directory
	keyDir := filepath.Join(os.Getenv("HOME"), ".mole", "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create key directory: %w", err)
	}

	keyFile := filepath.Join(keyDir, keyName+".pem")
	if err := os.WriteFile(keyFile, []byte(*result.KeyMaterial), 0600); err != nil {
		return "", fmt.Errorf("failed to save private key: %w", err)
	}

	fmt.Printf("   ✅ Private key saved: ~/.mole/keys/%s.pem\n", keyName)
	return keyName, nil
}

// launchBastion launches the bastion EC2 instance
func (a *AWSClient) launchBastion(ctx context.Context, config *DeploymentConfig, sgID, keyName, iamRole string) (string, error) {
	// Get Amazon Linux AMI (lightweight & optimized)
	ami, err := a.getAmazonLinuxAMI(ctx)
	if err != nil {
		return "", err
	}

	// Create user data script with NAT bridge configuration
	userData := a.generateUserData(ctx, config)

	// Launch instance
	runResult, err := a.client.RunInstances(ctx, &ec2.RunInstancesInput{
		ImageId:          &ami,
		InstanceType:     config.InstanceType,
		KeyName:          &keyName,
		MinCount:         aws.Int32(1),
		MaxCount:         aws.Int32(1),
		SecurityGroupIds: []string{sgID},
		SubnetId:         &config.PublicSubnetId,
		UserData:         &userData,
		IamInstanceProfile: &types.IamInstanceProfileSpecification{
			Name: &iamRole,
		},
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

// generateUserData creates BULLETPROOF user data - 30 second boot, zero failures
func (a *AWSClient) generateUserData(ctx context.Context, config *DeploymentConfig) string {
	// Pre-calculate private subnet CIDR to avoid API calls in user data
	privateSubnetCidr, err := a.getSubnetCIDR(ctx, config.PrivateSubnetId)
	if err != nil || privateSubnetCidr == "" {
		// Use fallback based on VPC CIDR or sensible default
		if strings.Contains(config.VPCCidr, "10.100.") {
			privateSubnetCidr = "10.100.2.0/24"
		} else {
			privateSubnetCidr = "10.200.2.0/24"
		}
	}

	script := fmt.Sprintf(`#!/bin/bash
set -euo pipefail

# BULLETPROOF: Pre-calculated values, no API calls, minimal operations
CLIENT_PUBLIC_KEY="%s"
PRIVATE_SUBNET_CIDR="%s"
REGION="%s"

# Install only essentials - skip updates for speed
dnf install -y wireguard-tools --skip-broken

# Enable IP forwarding
echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
sysctl -p

# Generate WireGuard keys (fast)
mkdir -p /etc/mole/keys
wg genkey | tee /etc/mole/keys/wg0_private.key | wg pubkey > /etc/mole/keys/wg0_public.key
chmod 600 /etc/mole/keys/wg0_private.key

# Get keys
SERVER_PRIVATE_KEY=$(cat /etc/mole/keys/wg0_private.key)
SERVER_PUBLIC_KEY=$(cat /etc/mole/keys/wg0_public.key)

# Create WireGuard config (minimal)
cat > /etc/wireguard/wg0.conf << EOF
[Interface]
PrivateKey = $SERVER_PRIVATE_KEY
Address = 10.100.1.1/24
ListenPort = 51820

[Peer]
PublicKey = $CLIENT_PUBLIC_KEY
AllowedIPs = 10.100.1.2/32
EOF

# Start WireGuard
wg-quick up wg0

# Basic iptables rules (minimal)
iptables -A INPUT -p udp --dport 51820 -j ACCEPT
iptables -A FORWARD -i wg0 -j ACCEPT
iptables -A FORWARD -o wg0 -j ACCEPT
iptables -t nat -A POSTROUTING -s $PRIVATE_SUBNET_CIDR -j MASQUERADE

# Get instance ID and tag with server public key (fast)
TOKEN=$(curl -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
INSTANCE_ID=$(curl -H "X-aws-ec2-metadata-token: $TOKEN" http://169.254.169.254/latest/meta-data/instance-id)

# Disable source/dest check and tag
aws ec2 modify-instance-attribute --instance-id $INSTANCE_ID --no-source-dest-check --region $REGION &
aws ec2 create-tags --resources $INSTANCE_ID --tags Key=WireGuardPublicKey,Value="$SERVER_PUBLIC_KEY" --region $REGION &

# Signal ready - FAST BOOT COMPLETE
echo "ready" > /etc/mole/status
`, config.ClientPublicKey, privateSubnetCidr, config.Region)

	// Base64 encode the script for AWS user data
	return base64.StdEncoding.EncodeToString([]byte(script))
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

// getAmazonLinuxAMI finds the latest Amazon Linux 2023 AMI (lightweight & optimized)
func (a *AWSClient) getAmazonLinuxAMI(ctx context.Context) (string, error) {
	// Try ARM64 first (for Graviton instances - better performance/cost)
	output, err := a.client.DescribeImages(ctx, &ec2.DescribeImagesInput{
		Owners: []string{"amazon"}, // Amazon official AMIs
		Filters: []types.Filter{
			{
				Name:   aws.String("name"),
				Values: []string{"al2023-ami-*"},
			},
			{
				Name:   aws.String("architecture"),
				Values: []string{"arm64"},
			},
			{
				Name:   aws.String("state"),
				Values: []string{"available"},
			},
			{
				Name:   aws.String("image-type"),
				Values: []string{"machine"},
			},
		},
	})
	if err != nil {
		return "", err
	}

	// If ARM64 not available, fallback to x86_64
	if len(output.Images) == 0 {
		output, err = a.client.DescribeImages(ctx, &ec2.DescribeImagesInput{
			Owners: []string{"amazon"},
			Filters: []types.Filter{
				{
					Name:   aws.String("name"),
					Values: []string{"al2023-ami-*"},
				},
				{
					Name:   aws.String("architecture"),
					Values: []string{"x86_64"},
				},
				{
					Name:   aws.String("state"),
					Values: []string{"available"},
				},
				{
					Name:   aws.String("image-type"),
					Values: []string{"machine"},
				},
			},
		})
		if err != nil {
			return "", err
		}
	}

	if len(output.Images) == 0 {
		return "", fmt.Errorf("no suitable Amazon Linux AMI found")
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

// deployTargetInstance deploys a test target instance in the private subnet
func (a *AWSClient) deployTargetInstance(ctx context.Context, config *DeploymentConfig, sgID, keyName string) (string, string, error) {
	amiID, err := a.getAmazonLinuxAMI(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get Amazon Linux AMI: %w", err)
	}

	// Create minimal, fast user data for test target - prioritize speed
	userData := `#!/bin/bash
set -euo pipefail

# Skip system updates for speed - install only essentials
dnf install -y net-tools python3 --skip-broken

# Enable ICMP (ping) responses
echo 'net.ipv4.icmp_echo_ignore_all = 0' >> /etc/sysctl.conf
sysctl -p

# Create lightweight test server - starts immediately
cat > /home/ec2-user/test-server.py << 'EOF'
#!/usr/bin/env python3
import http.server
import socketserver
import socket

class TestHandler(http.server.SimpleHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.send_header('Content-type', 'text/html')
        self.end_headers()

        hostname = socket.gethostname()
        try:
            local_ip = socket.gethostbyname(hostname)
        except:
            local_ip = "unknown"

        response = f"""<html><body>
<h1>AWS Cloud Mole Test Target</h1>
<p><strong>Hostname:</strong> {hostname}</p>
<p><strong>Private IP:</strong> {local_ip}</p>
<p><strong>Status:</strong> ✅ NAT Bridge Working!</p>
<p>Successfully reached test target through WireGuard tunnel.</p>
</body></html>"""
        self.wfile.write(response.encode())

PORT = 8080
with socketserver.TCPServer(("", PORT), TestHandler) as httpd:
    print(f"Test server running on port {PORT}")
    httpd.serve_forever()
EOF

chown ec2-user:ec2-user /home/ec2-user/test-server.py
chmod +x /home/ec2-user/test-server.py

# Start test server immediately in background (no systemd delay)
nohup python3 /home/ec2-user/test-server.py > /home/ec2-user/server.log 2>&1 &
echo $! > /home/ec2-user/server.pid

echo "test-target-ready" > /home/ec2-user/status
`

	userDataEncoded := base64.StdEncoding.EncodeToString([]byte(userData))

	// Use the same instance type or default to nano for cost efficiency
	targetInstanceType := config.TargetInstance
	if targetInstanceType == "" {
		targetInstanceType = types.InstanceTypeT4gNano
	}

	runInput := &ec2.RunInstancesInput{
		ImageId:                       &amiID,
		InstanceType:                  targetInstanceType,
		KeyName:                       &keyName, // Use same SSH key as bastion
		SecurityGroupIds:              []string{sgID}, // Use same security group
		SubnetId:                      &config.PrivateSubnetId, // Deploy in private subnet
		UserData:                      &userDataEncoded,
		MinCount:                      aws.Int32(1),
		MaxCount:                      aws.Int32(1),
		// No public IP assignment - private subnet only (handled by subnet config)
		InstanceInitiatedShutdownBehavior: types.ShutdownBehaviorTerminate,
		Monitoring: &types.RunInstancesMonitoringEnabled{
			Enabled: aws.Bool(true),
		},
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags: []types.Tag{
					{Key: aws.String("Name"), Value: aws.String("mole-test-target")},
					{Key: aws.String("Purpose"), Value: aws.String("nat-bridge-testing")},
					{Key: aws.String("CreatedBy"), Value: aws.String("aws-cloud-mole")},
				},
			},
		},
	}

	result, err := a.client.RunInstances(ctx, runInput)
	if err != nil {
		return "", "", err
	}

	instanceID := *result.Instances[0].InstanceId

	// Wait for instance to be running
	if err := a.waitForInstanceRunning(ctx, instanceID); err != nil {
		return "", "", fmt.Errorf("target instance failed to start: %w", err)
	}

	// Get private IP
	_, privateIP, err := a.getInstanceIPs(ctx, instanceID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get target instance IP: %w", err)
	}

	return instanceID, privateIP, nil
}

// InstanceInfo contains basic instance information
type InstanceInfo struct {
	InstanceID string
	PrivateIP  string
	PublicIP   string
	State      string
	Name       string
}

// FindInstancesByTag finds instances by tag key-value pair
func (a *AWSClient) FindInstancesByTag(ctx context.Context, tagKey, tagValue string) ([]InstanceInfo, error) {
	input := &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", tagKey)),
				Values: []string{tagValue},
			},
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running", "pending", "stopped"},
			},
		},
	}

	result, err := a.client.DescribeInstances(ctx, input)
	if err != nil {
		return nil, err
	}

	var instances []InstanceInfo
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			info := InstanceInfo{
				InstanceID: aws.ToString(instance.InstanceId),
				State:      string(instance.State.Name),
			}

			if instance.PrivateIpAddress != nil {
				info.PrivateIP = *instance.PrivateIpAddress
			}

			if instance.PublicIpAddress != nil {
				info.PublicIP = *instance.PublicIpAddress
			}

			// Get the Name tag
			for _, tag := range instance.Tags {
				if *tag.Key == "Name" {
					info.Name = *tag.Value
					break
				}
			}

			instances = append(instances, info)
		}
	}

	return instances, nil
}

// configurePrivateSubnetRouting adds routes to the private subnet for WireGuard tunnel access
func (a *AWSClient) configurePrivateSubnetRouting(ctx context.Context, config *DeploymentConfig, bastionInstanceID string) error {
	// Find the route table associated with the private subnet
	routeTablesResult, err := a.client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("association.subnet-id"),
				Values: []string{config.PrivateSubnetId},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to find route table for private subnet: %w", err)
	}

	if len(routeTablesResult.RouteTables) == 0 {
		return fmt.Errorf("no route table found for private subnet %s", config.PrivateSubnetId)
	}

	routeTableId := *routeTablesResult.RouteTables[0].RouteTableId

	// Add a route for WireGuard tunnel network (10.100.1.0/24) to the bastion instance
	// This allows private subnet instances to reach the tunnel network
	_, err = a.client.CreateRoute(ctx, &ec2.CreateRouteInput{
		RouteTableId:         &routeTableId,
		DestinationCidrBlock: aws.String("10.100.1.0/24"), // WireGuard tunnel network
		InstanceId:           &bastionInstanceID,
	})
	if err != nil {
		// Check if route already exists
		if ec2ErrorContains(err, "RouteAlreadyExists") {
			// Route already exists, that's fine
			return nil
		}
		return fmt.Errorf("failed to create route to tunnel network: %w", err)
	}

	return nil
}

// ec2ErrorContains checks if an AWS EC2 error contains a specific error code
func ec2ErrorContains(err error, code string) bool {
	return err != nil && (
		err.Error() == code ||
		(len(err.Error()) > len(code) && err.Error()[len(err.Error())-len(code):] == code))
}

// generateWireGuardKeys generates a WireGuard private/public key pair
func (a *AWSClient) generateWireGuardKeys() (privateKey, publicKey string, err error) {
	var private [32]byte
	_, err = rand.Read(private[:])
	if err != nil {
		return "", "", err
	}

	var public [32]byte
	curve25519.ScalarBaseMult(&public, &private)

	privateKeyB64 := base64.StdEncoding.EncodeToString(private[:])
	publicKeyB64 := base64.StdEncoding.EncodeToString(public[:])

	return privateKeyB64, publicKeyB64, nil
}

// getServerPublicKey retrieves the server's WireGuard public key from instance tags
func (a *AWSClient) getServerPublicKey(ctx context.Context, instanceID string) (string, error) {
	// Wait a bit for the instance to finish initialization and tag itself
	time.Sleep(30 * time.Second)

	for i := 0; i < 12; i++ { // Try for up to 2 minutes
		result, err := a.client.DescribeTags(ctx, &ec2.DescribeTagsInput{
			Filters: []types.Filter{
				{
					Name:   aws.String("resource-id"),
					Values: []string{instanceID},
				},
				{
					Name:   aws.String("key"),
					Values: []string{"WireGuardPublicKey"},
				},
			},
		})
		if err != nil {
			return "", err
		}

		if len(result.Tags) > 0 && result.Tags[0].Value != nil {
			return *result.Tags[0].Value, nil
		}

		fmt.Printf("  ⏳ Waiting for server key generation... (%d/12)\n", i+1)
		time.Sleep(10 * time.Second)
	}

	return "", fmt.Errorf("server public key not found in instance tags after 2 minutes")
}

// setupLocalTunnel creates and establishes the local WireGuard tunnel with platform awareness
func (a *AWSClient) setupLocalTunnel(result *DeploymentResult) error {
	fmt.Printf("  🖥️  Detected platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Check privilege level
	privLevel := a.detectPrivilegeLevel()
	fmt.Printf("  🔐 Privilege level: %s\n", privLevel)

	// Set up environment for sudo with askpass (platform-aware)
	env := a.setupSudoEnvironment()

	fmt.Printf("  🔍 Detecting existing WireGuard interfaces...\n")

	// Detect and clean up ALL existing WireGuard interfaces automatically
	if err := a.cleanupExistingInterfaces(env); err != nil {
		return fmt.Errorf("failed to cleanup existing interfaces: %w", err)
	}

	// Platform-specific tunnel creation
	switch runtime.GOOS {
	case "darwin":
		return a.setupMacOSTunnel(result, env)
	case "linux":
		return a.setupLinuxTunnel(result, env)
	case "windows":
		return a.setupWindowsTunnel(result, env)
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		return a.setupBSDTunnel(result, env)
	default:
		fmt.Printf("  ❌ Unsupported platform: %s\n", runtime.GOOS)
		return fmt.Errorf("unsupported platform: %s. AWS Cloud Mole supports macOS, Linux, Windows, and BSD variants (FreeBSD, OpenBSD, NetBSD, DragonFly)", runtime.GOOS)
	}
}

// setupSudoEnvironment configures sudo environment based on platform
func (a *AWSClient) setupSudoEnvironment() []string {
	env := os.Environ()

	switch runtime.GOOS {
	case "darwin":
		// macOS: Try common askpass locations
		askpassPaths := []string{
			"/opt/homebrew/Cellar/macos-askpass/1.0.2/bin/askpass",
			"/usr/local/bin/askpass",
			"/opt/homebrew/bin/askpass",
		}
		for _, askpassPath := range askpassPaths {
			if _, err := os.Stat(askpassPath); err == nil {
				env = append(env, fmt.Sprintf("SUDO_ASKPASS=%s", askpassPath))
				fmt.Printf("  🔑 Using askpass: %s\n", askpassPath)
				break
			}
		}
	case "linux":
		// Linux: Try common askpass programs
		askpassPaths := []string{
			"/usr/bin/ssh-askpass",
			"/usr/libexec/openssh/ssh-askpass",
			"/usr/bin/ksshaskpass",
		}
		for _, askpassPath := range askpassPaths {
			if _, err := os.Stat(askpassPath); err == nil {
				env = append(env, fmt.Sprintf("SUDO_ASKPASS=%s", askpassPath))
				fmt.Printf("  🔑 Using askpass: %s\n", askpassPath)
				break
			}
		}
	case "windows":
		// Windows: No sudo/askpass needed, WireGuard uses service elevation
		fmt.Printf("  🪟 Windows: Using service elevation for WireGuard\n")
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		// BSD: Similar to Linux but check BSD-specific locations
		fmt.Printf("  🔱 BSD (%s): Setting up sudo environment\n", runtime.GOOS)
		askpassPaths := []string{
			"/usr/local/bin/ssh-askpass",
			"/usr/bin/ssh-askpass",
			"/usr/X11R6/bin/ssh-askpass", // OpenBSD
		}
		for _, askpassPath := range askpassPaths {
			if _, err := os.Stat(askpassPath); err == nil {
				env = append(env, fmt.Sprintf("SUDO_ASKPASS=%s", askpassPath))
				fmt.Printf("  🔑 Using askpass: %s\n", askpassPath)
				break
			}
		}
	}

	return env
}

// detectPrivilegeLevel determines if the application is running with elevated privileges
func (a *AWSClient) detectPrivilegeLevel() string {
	switch runtime.GOOS {
	case "darwin", "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		// Unix-like systems: check if running as root
		if os.Getuid() == 0 {
			return "elevated (root)"
		}
		return "normal (will use sudo)"
	case "windows":
		// Windows: check if running as Administrator
		if a.isWindowsAdmin() {
			return "elevated (administrator)"
		}
		return "normal (will request elevation)"
	default:
		return "unknown"
	}
}

// isWindowsAdmin checks if the current process has administrator privileges on Windows
func (a *AWSClient) isWindowsAdmin() bool {
	// On non-Windows systems, always return false
	if runtime.GOOS != "windows" {
		return false
	}

	// Try to create a temporary file in the Windows directory (requires admin)
	testFile := `C:\Windows\temp_mole_test.tmp`
	file, err := os.Create(testFile)
	if err != nil {
		return false
	}
	file.Close()
	os.Remove(testFile)
	return true
}

// setupMacOSTunnel creates WireGuard tunnel on macOS
func (a *AWSClient) setupMacOSTunnel(result *DeploymentResult, env []string) error {
	fmt.Printf("  🍎 Setting up macOS WireGuard tunnel...\n")

	// Create tunnel config in user directory (safer on macOS)
	tunnelDir := filepath.Join(os.Getenv("HOME"), ".mole", "tunnels")
	if err := os.MkdirAll(tunnelDir, 0755); err != nil {
		return fmt.Errorf("failed to create tunnel directory: %w", err)
	}

	configPath := filepath.Join(tunnelDir, "wg0.conf")
	configContent := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = 10.100.1.2/24
ListenPort = 51821
MTU = 1500

[Peer]
PublicKey = %s
Endpoint = %s:51820
AllowedIPs = 10.100.2.0/24
PersistentKeepalive = 25
`, result.ClientPrivateKey, result.ServerPublicKey, result.BastionPublicIP)

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write tunnel config: %w", err)
	}

	fmt.Printf("  📄 Config written: %s\n", configPath)
	fmt.Printf("  🚀 Establishing WireGuard tunnel...\n")

	// On macOS, use wg-quick with full path to config
	upCmd := exec.Command("sudo", "-A", "wg-quick", "up", configPath)
	upCmd.Env = env
	if output, err := upCmd.CombinedOutput(); err != nil {
		// More detailed error handling on macOS
		if strings.Contains(string(output), "already exists") {
			fmt.Printf("  💡 Interface already exists, attempting cleanup and retry...\n")
			// Try cleanup one more time and retry
			a.cleanupMacOSInterfaces([]string{"wg0", "utun8", "utun9", "utun10"}, env)

			// Retry with a slightly different approach
			upCmd = exec.Command("sudo", "-A", "wg-quick", "up", configPath)
			upCmd.Env = env
			if output, err := upCmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to bring up tunnel on macOS: %w\nOutput: %s", err, output)
			}
		} else {
			return fmt.Errorf("failed to bring up tunnel on macOS: %w\nOutput: %s", err, output)
		}
	}

	fmt.Printf("  ✅ macOS WireGuard tunnel established\n")
	return nil
}

// setupLinuxTunnel creates WireGuard tunnel on Linux
func (a *AWSClient) setupLinuxTunnel(result *DeploymentResult, env []string) error {
	fmt.Printf("  🐧 Setting up Linux WireGuard tunnel...\n")

	// On Linux, use system-wide config directory
	configDir := "/etc/wireguard"
	configPath := filepath.Join(configDir, "wg0.conf")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		// Fallback to user directory if system directory fails
		userConfigDir := filepath.Join(os.Getenv("HOME"), ".config", "wireguard")
		if err := os.MkdirAll(userConfigDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		configPath = filepath.Join(userConfigDir, "wg0.conf")
	}

	configContent := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = 10.100.1.2/24
ListenPort = 51821
MTU = 1500

[Peer]
PublicKey = %s
Endpoint = %s:51820
AllowedIPs = 10.100.2.0/24
PersistentKeepalive = 25
`, result.ClientPrivateKey, result.ServerPublicKey, result.BastionPublicIP)

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write tunnel config: %w", err)
	}

	fmt.Printf("  📄 Config written: %s\n", configPath)
	fmt.Printf("  🚀 Establishing WireGuard tunnel...\n")

	// On Linux, can use interface name directly
	upCmd := exec.Command("sudo", "-A", "wg-quick", "up", "wg0")
	upCmd.Env = env
	if output, err := upCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up tunnel on Linux: %w\nOutput: %s", err, output)
	}

	fmt.Printf("  ✅ Linux WireGuard tunnel established\n")
	return nil
}

// setupGenericTunnel provides generic tunnel setup for unknown platforms
func (a *AWSClient) setupGenericTunnel(result *DeploymentResult, env []string) error {
	fmt.Printf("  🖥️  Setting up generic WireGuard tunnel...\n")

	// Use user directory for unknown platforms
	tunnelDir := filepath.Join(os.Getenv("HOME"), ".mole", "tunnels")
	if err := os.MkdirAll(tunnelDir, 0755); err != nil {
		return fmt.Errorf("failed to create tunnel directory: %w", err)
	}

	configPath := filepath.Join(tunnelDir, "wg0.conf")
	configContent := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = 10.100.1.2/24
ListenPort = 51821
MTU = 1500

[Peer]
PublicKey = %s
Endpoint = %s:51820
AllowedIPs = 10.100.2.0/24
PersistentKeepalive = 25
`, result.ClientPrivateKey, result.ServerPublicKey, result.BastionPublicIP)

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write tunnel config: %w", err)
	}

	fmt.Printf("  📄 Config written: %s\n", configPath)
	fmt.Printf("  🚀 Establishing WireGuard tunnel...\n")

	// Generic approach
	upCmd := exec.Command("sudo", "-A", "wg-quick", "up", configPath)
	upCmd.Env = env
	if output, err := upCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up tunnel: %w\nOutput: %s", err, output)
	}

	fmt.Printf("  ✅ Generic WireGuard tunnel established\n")
	return nil
}

// setupWindowsTunnel creates WireGuard tunnel on Windows 11+
func (a *AWSClient) setupWindowsTunnel(result *DeploymentResult, env []string) error {
	fmt.Printf("  🪟 Setting up Windows WireGuard tunnel...\n")

	// Windows WireGuard uses %USERPROFILE%\Documents or %PROGRAMFILES%\WireGuard\Data\Configurations
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		userProfile = os.Getenv("HOME") // Fallback
	}

	// Create config directory in user's Documents folder (accessible without admin)
	configDir := filepath.Join(userProfile, "Documents", "WireGuard")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		// Fallback to AppData if Documents fails
		appData := os.Getenv("APPDATA")
		if appData != "" {
			configDir = filepath.Join(appData, "WireGuard")
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create Windows config directory: %w", err)
			}
		} else {
			return fmt.Errorf("failed to create Windows config directory: %w", err)
		}
	}

	configPath := filepath.Join(configDir, "mole-tunnel.conf")
	configContent := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = 10.100.1.2/24
ListenPort = 51821
MTU = 1500
DNS = 1.1.1.1

[Peer]
PublicKey = %s
Endpoint = %s:51820
AllowedIPs = 10.100.2.0/24
PersistentKeepalive = 25
`, result.ClientPrivateKey, result.ServerPublicKey, result.BastionPublicIP)

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write Windows tunnel config: %w", err)
	}

	fmt.Printf("  📄 Config written: %s\n", configPath)
	fmt.Printf("  🚀 Establishing WireGuard tunnel on Windows...\n")

	// Windows WireGuard uses different commands
	// Try WireGuard for Windows (wireguard.exe) first, then wg-quick as fallback
	var upCmd *exec.Cmd

	// Check for WireGuard for Windows installation
	wireguardPaths := []string{
		`C:\Program Files\WireGuard\wireguard.exe`,
		`C:\Program Files (x86)\WireGuard\wireguard.exe`,
		`wireguard.exe`, // In PATH
	}

	wireguardFound := false
	for _, wgPath := range wireguardPaths {
		if _, err := os.Stat(wgPath); err == nil {
			// Use WireGuard for Windows GUI to install tunnel
			upCmd = exec.Command(wgPath, "/installtunnelservice", configPath)
			wireguardFound = true
			fmt.Printf("  🔧 Using WireGuard for Windows: %s\n", wgPath)
			break
		}
	}

	if !wireguardFound {
		// Fallback to wg-quick (if available from WSL, chocolatey, or scoop)
		fmt.Printf("  🔄 WireGuard for Windows not found, trying wg-quick...\n")
		upCmd = exec.Command("wg-quick", "up", configPath)
		upCmd.Env = env
	}

	if output, err := upCmd.CombinedOutput(); err != nil {
		// Provide helpful Windows-specific error message
		errorMsg := fmt.Sprintf("failed to bring up tunnel on Windows: %v\nOutput: %s", err, output)

		if !wireguardFound {
			errorMsg += "\n\n💡 Windows Setup Help:"
			errorMsg += "\n  1. Install WireGuard for Windows: https://www.wireguard.com/install/"
			errorMsg += "\n  2. Or install via chocolatey: choco install wireguard"
			errorMsg += "\n  3. Or install via winget: winget install WireGuard.WireGuard"
			errorMsg += fmt.Sprintf("\n  4. Manual import: Open WireGuard app and import %s", configPath)
		}

		return fmt.Errorf(errorMsg)
	}

	fmt.Printf("  ✅ Windows WireGuard tunnel established\n")
	fmt.Printf("  💡 You can manage the tunnel via the WireGuard system tray icon\n")
	return nil
}

// setupBSDTunnel creates WireGuard tunnel on BSD variants (FreeBSD, OpenBSD, NetBSD, DragonFly)
func (a *AWSClient) setupBSDTunnel(result *DeploymentResult, env []string) error {
	fmt.Printf("  🔱 Setting up BSD WireGuard tunnel (%s)...\n", runtime.GOOS)

	// BSD uses different paths than Linux
	var configDir, configPath string

	switch runtime.GOOS {
	case "freebsd":
		// FreeBSD: /usr/local/etc/wireguard
		configDir = "/usr/local/etc/wireguard"
	case "openbsd":
		// OpenBSD: /etc/wireguard
		configDir = "/etc/wireguard"
	case "netbsd":
		// NetBSD: /usr/pkg/etc/wireguard
		configDir = "/usr/pkg/etc/wireguard"
	case "dragonfly":
		// DragonFly BSD: /usr/local/etc/wireguard
		configDir = "/usr/local/etc/wireguard"
	default:
		// Fallback
		configDir = "/etc/wireguard"
	}

	configPath = filepath.Join(configDir, "wg0.conf")

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(configDir, 0755); err != nil {
		// Fallback to user directory if system directory fails
		userConfigDir := filepath.Join(os.Getenv("HOME"), ".config", "wireguard")
		if err := os.MkdirAll(userConfigDir, 0755); err != nil {
			return fmt.Errorf("failed to create BSD config directory: %w", err)
		}
		configPath = filepath.Join(userConfigDir, "wg0.conf")
	}

	configContent := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = 10.100.1.2/24
ListenPort = 51821
MTU = 1500

[Peer]
PublicKey = %s
Endpoint = %s:51820
AllowedIPs = 10.100.2.0/24
PersistentKeepalive = 25
`, result.ClientPrivateKey, result.ServerPublicKey, result.BastionPublicIP)

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		return fmt.Errorf("failed to write BSD tunnel config: %w", err)
	}

	fmt.Printf("  📄 Config written: %s\n", configPath)
	fmt.Printf("  🚀 Establishing WireGuard tunnel on %s...\n", runtime.GOOS)

	// On BSD, use wg-quick similar to Linux
	upCmd := exec.Command("sudo", "-A", "wg-quick", "up", "wg0")
	upCmd.Env = env
	if output, err := upCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring up tunnel on %s: %w\nOutput: %s", runtime.GOOS, err, output)
	}

	fmt.Printf("  ✅ BSD WireGuard tunnel established on %s\n", runtime.GOOS)
	return nil
}

// cleanupExistingInterfaces automatically detects and cleans up existing WireGuard interfaces
func (a *AWSClient) cleanupExistingInterfaces(env []string) error {
	// Check for existing WireGuard interfaces
	checkCmd := exec.Command("wg", "show", "interfaces")
	output, err := checkCmd.Output()
	if err != nil {
		// No interfaces found or wg not installed - that's fine
		return nil
	}

	interfaces := strings.Fields(strings.TrimSpace(string(output)))
	if len(interfaces) == 0 {
		return nil
	}

	fmt.Printf("  🧹 Found %d existing WireGuard interface(s): %s\n", len(interfaces), strings.Join(interfaces, ", "))

	// Platform-specific cleanup
	switch runtime.GOOS {
	case "darwin":
		return a.cleanupMacOSInterfaces(interfaces, env)
	case "linux":
		return a.cleanupLinuxInterfaces(interfaces, env)
	case "windows":
		return a.cleanupWindowsInterfaces(interfaces, env)
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		// BSD variants use similar approach to Linux but with different paths
		return a.cleanupBSDInterfaces(interfaces, env)
	default:
		fmt.Printf("  ❌ Unsupported platform: %s\n", runtime.GOOS)
		return a.cleanupGenericInterfaces(interfaces, env) // This will return error
	}
}

// cleanupMacOSInterfaces handles macOS-specific WireGuard interface cleanup
func (a *AWSClient) cleanupMacOSInterfaces(interfaces []string, env []string) error {
	fmt.Printf("  🍎 Using macOS-specific interface cleanup\n")

	for _, iface := range interfaces {
		if strings.TrimSpace(iface) == "" {
			continue
		}

		fmt.Printf("  ⬇️  Bringing down macOS interface: %s\n", iface)

		// On macOS, WireGuard interfaces are often utun devices
		// First try to find and remove the config file
		configPaths := []string{
			fmt.Sprintf("/opt/homebrew/etc/wireguard/%s.conf", iface),
			fmt.Sprintf("/usr/local/etc/wireguard/%s.conf", iface),
			fmt.Sprintf("%s/.mole/tunnels/wg0.conf", os.Getenv("HOME")),
		}

		configFound := false
		for _, configPath := range configPaths {
			if _, err := os.Stat(configPath); err == nil {
				fmt.Printf("  📄 Found config: %s\n", configPath)
				cleanupCmd := exec.Command("sudo", "-A", "wg-quick", "down", configPath)
				cleanupCmd.Env = env
				if err := cleanupCmd.Run(); err == nil {
					configFound = true
					break
				}
			}
		}

		if !configFound {
			// If no config file found, try to remove the interface directly using macOS-specific methods
			fmt.Printf("  🔧 No config found, using direct interface removal\n")

			// First, try to remove WireGuard configuration
			removeCmd := exec.Command("sudo", "-A", "wg", "set", iface, "private-key", "/dev/null", "peer", "", "remove")
			removeCmd.Env = env
			removeCmd.Run() // Ignore errors

			// Then try to bring the interface down using ifconfig (macOS-specific)
			downCmd := exec.Command("sudo", "-A", "ifconfig", iface, "down")
			downCmd.Env = env
			if err := downCmd.Run(); err != nil {
				fmt.Printf("  ⚠️  Warning: failed to bring down interface %s: %v\n", iface, err)
			}

			// Finally, try to destroy the utun interface (if it's a utun device)
			if strings.HasPrefix(iface, "utun") {
				destroyCmd := exec.Command("sudo", "-A", "ifconfig", iface, "destroy")
				destroyCmd.Env = env
				if err := destroyCmd.Run(); err != nil {
					fmt.Printf("  ⚠️  Warning: failed to destroy interface %s: %v (may require reboot)\n", iface, err)
				}
			}
		}
	}

	// Verify cleanup
	return a.verifyInterfaceCleanup()
}

// cleanupLinuxInterfaces handles Linux-specific WireGuard interface cleanup
func (a *AWSClient) cleanupLinuxInterfaces(interfaces []string, env []string) error {
	fmt.Printf("  🐧 Using Linux-specific interface cleanup\n")

	for _, iface := range interfaces {
		if strings.TrimSpace(iface) == "" {
			continue
		}

		fmt.Printf("  ⬇️  Bringing down Linux interface: %s\n", iface)

		// Try wg-quick down first with interface name
		cleanupCmd := exec.Command("sudo", "-A", "wg-quick", "down", iface)
		cleanupCmd.Env = env
		if err := cleanupCmd.Run(); err != nil {
			// If that fails, try removing with ip link (Linux-specific)
			deleteCmd := exec.Command("sudo", "-A", "ip", "link", "delete", "dev", iface)
			deleteCmd.Env = env
			if deleteErr := deleteCmd.Run(); deleteErr != nil {
				fmt.Printf("  ⚠️  Warning: failed to clean interface %s: %v\n", iface, err)
			}
		}
	}

	return a.verifyInterfaceCleanup()
}

// cleanupWindowsInterfaces handles Windows-specific WireGuard interface cleanup
func (a *AWSClient) cleanupWindowsInterfaces(interfaces []string, env []string) error {
	fmt.Printf("  🪟 Using Windows-specific interface cleanup\n")

	for _, iface := range interfaces {
		if strings.TrimSpace(iface) == "" {
			continue
		}

		fmt.Printf("  ⬇️  Removing Windows interface: %s\n", iface)

		// Try WireGuard for Windows service removal first
		wireguardPaths := []string{
			`C:\Program Files\WireGuard\wireguard.exe`,
			`C:\Program Files (x86)\WireGuard\wireguard.exe`,
			`wireguard.exe`, // In PATH
		}

		serviceRemoved := false
		for _, wgPath := range wireguardPaths {
			if _, err := os.Stat(wgPath); err == nil {
				// Stop and remove tunnel service
				stopCmd := exec.Command(wgPath, "/uninstalltunnelservice", iface)
				if err := stopCmd.Run(); err == nil {
					serviceRemoved = true
					break
				}

				// Alternative: try removing by name (mole-tunnel)
				stopCmd = exec.Command(wgPath, "/uninstalltunnelservice", "mole-tunnel")
				if err := stopCmd.Run(); err == nil {
					serviceRemoved = true
					break
				}
			}
		}

		if !serviceRemoved {
			// Fallback to wg-quick if available
			cleanupCmd := exec.Command("wg-quick", "down", iface)
			cleanupCmd.Env = env
			if err := cleanupCmd.Run(); err != nil {
				fmt.Printf("  ⚠️  Warning: failed to clean Windows interface %s: %v\n", iface, err)
				fmt.Printf("  💡 You may need to manually remove the tunnel via WireGuard GUI\n")
			}
		}
	}

	return a.verifyInterfaceCleanup()
}

// cleanupBSDInterfaces handles BSD-specific WireGuard interface cleanup
func (a *AWSClient) cleanupBSDInterfaces(interfaces []string, env []string) error {
	fmt.Printf("  🔱 Using BSD-specific interface cleanup (%s)\n", runtime.GOOS)

	for _, iface := range interfaces {
		if strings.TrimSpace(iface) == "" {
			continue
		}

		fmt.Printf("  ⬇️  Bringing down BSD interface: %s\n", iface)

		// Try wg-quick down first (similar to Linux)
		cleanupCmd := exec.Command("sudo", "-A", "wg-quick", "down", iface)
		cleanupCmd.Env = env
		if err := cleanupCmd.Run(); err != nil {
			// BSD-specific interface removal using ifconfig
			deleteCmd := exec.Command("sudo", "-A", "ifconfig", iface, "destroy")
			deleteCmd.Env = env
			if deleteErr := deleteCmd.Run(); deleteErr != nil {
				fmt.Printf("  ⚠️  Warning: failed to clean BSD interface %s: %v\n", iface, err)
			}
		}
	}

	return a.verifyInterfaceCleanup()
}

// cleanupGenericInterfaces provides generic interface cleanup for unknown platforms
func (a *AWSClient) cleanupGenericInterfaces(interfaces []string, env []string) error {
	return fmt.Errorf("unsupported platform: %s. AWS Cloud Mole supports macOS, Linux, Windows, and BSD variants (FreeBSD, OpenBSD, NetBSD, DragonFly)", runtime.GOOS)
}

// verifyInterfaceCleanup checks if interface cleanup was successful
func (a *AWSClient) verifyInterfaceCleanup() error {
	checkAgainCmd := exec.Command("wg", "show", "interfaces")
	if output, err := checkAgainCmd.Output(); err == nil {
		remaining := strings.Fields(strings.TrimSpace(string(output)))
		if len(remaining) > 0 {
			fmt.Printf("  ⚠️  Note: %d interface(s) still present: %s\n", len(remaining), strings.Join(remaining, ", "))
			if runtime.GOOS == "darwin" {
				fmt.Printf("  💡 On macOS, some utun interfaces may persist until reboot\n")
			}
		} else {
			fmt.Printf("  ✅ All WireGuard interfaces cleaned up successfully\n")
		}
	}

	return nil
}

// createIAMRole creates IAM role and instance profile for EC2 instance permissions
func (a *AWSClient) createIAMRole(ctx context.Context) (string, error) {
	roleName := fmt.Sprintf("mole-instance-role-%d", time.Now().Unix())

	// Define trust policy for EC2
	trustPolicy := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Principal": {
					"Service": "ec2.amazonaws.com"
				},
				"Action": "sts:AssumeRole"
			}
		]
	}`

	// Define instance policy for tagging and attribute modification
	instancePolicy := `{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": [
					"ec2:CreateTags",
					"ec2:ModifyInstanceAttribute",
					"ec2:DescribeInstances",
					"ec2:DescribeTags",
					"ec2:DescribeVpcs",
					"ec2:DescribeSubnets"
				],
				"Resource": "*"
			}
		]
	}`

	// Create IAM role
	_, err := a.iamClient.CreateRole(ctx, &iam.CreateRoleInput{
		RoleName:                 aws.String(roleName),
		AssumeRolePolicyDocument: aws.String(trustPolicy),
		Path:                     aws.String("/mole/"),
		Tags: []iamtypes.Tag{
			{Key: aws.String("Project"), Value: aws.String("aws-cloud-mole")},
			{Key: aws.String("Purpose"), Value: aws.String("instance-permissions")},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create IAM role: %w", err)
	}

	// Create inline policy for the role
	policyName := "MoleInstancePolicy"
	_, err = a.iamClient.PutRolePolicy(ctx, &iam.PutRolePolicyInput{
		RoleName:       aws.String(roleName),
		PolicyName:     aws.String(policyName),
		PolicyDocument: aws.String(instancePolicy),
	})
	if err != nil {
		return "", fmt.Errorf("failed to attach policy to role: %w", err)
	}

	// Create instance profile
	_, err = a.iamClient.CreateInstanceProfile(ctx, &iam.CreateInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		Path:                aws.String("/mole/"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to create instance profile: %w", err)
	}

	// Add role to instance profile
	_, err = a.iamClient.AddRoleToInstanceProfile(ctx, &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: aws.String(roleName),
		RoleName:           aws.String(roleName),
	})
	if err != nil {
		return "", fmt.Errorf("failed to add role to instance profile: %w", err)
	}

	// Wait a moment for IAM propagation
	time.Sleep(10 * time.Second)

	return roleName, nil
}

// getSubnetCIDR retrieves CIDR for a subnet ID
func (a *AWSClient) getSubnetCIDR(ctx context.Context, subnetId string) (string, error) {
	if subnetId == "" {
		return "", nil
	}

	result, err := a.client.DescribeSubnets(ctx, &ec2.DescribeSubnetsInput{
		SubnetIds: []string{subnetId},
	})
	if err != nil {
		return "", err
	}

	if len(result.Subnets) > 0 && result.Subnets[0].CidrBlock != nil {
		return *result.Subnets[0].CidrBlock, nil
	}

	return "", nil
}
