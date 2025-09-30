package aws

import (
	"fmt"
	"strings"
)

// ExportTerraform generates equivalent Terraform configuration
func (a *AWSClient) ExportTerraform(config *DeploymentConfig) string {
	var tf strings.Builder

	tf.WriteString(`# AWS Cloud Mole Infrastructure (Generated)
terraform {
  required_version = ">= 1.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region  = "` + config.Region + `"
  profile = "` + config.Profile + `"
}

# Security Group
resource "aws_security_group" "mole_wireguard" {
  name        = "mole-wireguard-sg"
  description = "AWS Cloud Mole WireGuard Security Group"
  vpc_id      = "` + config.VPCId + `"

`)

	// Add WireGuard port rules
	for i := 0; i < config.TunnelCount; i++ {
		port := 51820 + i
		tf.WriteString(fmt.Sprintf(`  ingress {
    from_port   = %d
    to_port     = %d
    protocol    = "udp"
    cidr_blocks = ["%s"]
    description = "WireGuard tunnel %d"
  }

`, port, port, config.AllowedCIDR, i))
	}

	tf.WriteString(`  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["` + config.AllowedCIDR + `"]
    description = "SSH management"
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "All outbound"
  }

  tags = {
    Name      = "mole-wireguard-sg"
    Project   = "aws-cloud-mole"
    ManagedBy = "terraform"
  }
}

# Key Pair
resource "aws_key_pair" "mole" {
  key_name   = "mole-keypair"
  public_key = "` + config.SSHPublicKey + `"

  tags = {
    Name    = "mole-keypair"
    Project = "aws-cloud-mole"
  }
}

# Ubuntu AMI
data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"]

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-22.04-lts-*"]
  }

  filter {
    name   = "architecture"
    values = ["arm64"]
  }
}

# User Data
locals {
  user_data = base64encode(<<-EOF
#!/bin/bash
set -euo pipefail

TUNNEL_COUNT=` + fmt.Sprintf("%d", config.TunnelCount) + `
MTU_SIZE=` + fmt.Sprintf("%d", config.MTUSize) + `

apt-get update -y
apt-get install -y wireguard wireguard-tools iptables-persistent awscli

echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
sysctl -p

mkdir -p /etc/mole/keys

for i in $(seq 0 $((TUNNEL_COUNT - 1))); do
    wg genkey | tee /etc/mole/keys/wg$${i}_private.key | wg pubkey > /etc/mole/keys/wg$${i}_public.key
    chmod 600 /etc/mole/keys/wg$${i}_private.key
done

echo "ready" > /etc/mole/status
EOF
  )
}

# EC2 Instance
resource "aws_instance" "mole_bastion" {
  ami                     = data.aws_ami.ubuntu.id
  instance_type           = "` + string(config.InstanceType) + `"
  key_name                = aws_key_pair.mole.key_name
  vpc_security_group_ids  = [aws_security_group.mole_wireguard.id]
  subnet_id               = "` + config.PublicSubnetId + `"
  user_data               = local.user_data

  associate_public_ip_address = true
  monitoring                 = true

  root_block_device {
    volume_type = "gp3"
    volume_size = 20
    encrypted   = true
  }

  tags = {
    Name      = "mole-bastion"
    Project   = "aws-cloud-mole"
    ManagedBy = "terraform"
  }
}

# Outputs
output "bastion_public_ip" {
  description = "Public IP of the bastion"
  value       = aws_instance.mole_bastion.public_ip
}

output "bastion_instance_id" {
  description = "Instance ID of the bastion"
  value       = aws_instance.mole_bastion.id
}

output "ssh_command" {
  description = "SSH command to connect"
  value       = "ssh -i ~/.ssh/mole-key ubuntu@${aws_instance.mole_bastion.public_ip}"
}
`)

	return tf.String()
}

// ExportCloudFormation generates equivalent CloudFormation template
func (a *AWSClient) ExportCloudFormation(config *DeploymentConfig) string {
	var cf strings.Builder

	cf.WriteString(`AWSTemplateFormatVersion: '2010-09-09'
Description: 'AWS Cloud Mole Infrastructure (Generated)'

Parameters:
  VPCId:
    Type: AWS::EC2::VPC::Id
    Default: ` + config.VPCId + `
    Description: VPC ID for deployment

  SubnetId:
    Type: AWS::EC2::Subnet::Id
    Default: ` + config.PublicSubnetId + `
    Description: Public subnet ID for bastion

  AllowedCIDR:
    Type: String
    Default: ` + config.AllowedCIDR + `
    Description: CIDR block allowed to connect

  SSHPublicKey:
    Type: String
    Default: '` + config.SSHPublicKey + `'
    Description: SSH public key for access

Resources:
  # Security Group
  WireGuardSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: AWS Cloud Mole WireGuard Security Group
      VpcId: !Ref VPCId
      SecurityGroupIngress:
`)

	// Add WireGuard port rules
	for i := 0; i < config.TunnelCount; i++ {
		port := 51820 + i
		cf.WriteString(fmt.Sprintf(`        - IpProtocol: udp
          FromPort: %d
          ToPort: %d
          CidrIp: !Ref AllowedCIDR
          Description: WireGuard tunnel %d
`, port, port, i))
	}

	cf.WriteString(`        - IpProtocol: tcp
          FromPort: 22
          ToPort: 22
          CidrIp: !Ref AllowedCIDR
          Description: SSH management
      SecurityGroupEgress:
        - IpProtocol: -1
          CidrIp: 0.0.0.0/0
          Description: All outbound traffic
      Tags:
        - Key: Name
          Value: mole-wireguard-sg
        - Key: Project
          Value: aws-cloud-mole

  # Key Pair
  MoleKeyPair:
    Type: AWS::EC2::KeyPair
    Properties:
      KeyName: mole-keypair
      PublicKeyMaterial: !Ref SSHPublicKey
      Tags:
        - Key: Name
          Value: mole-keypair
        - Key: Project
          Value: aws-cloud-mole

  # Launch Template
  BastionLaunchTemplate:
    Type: AWS::EC2::LaunchTemplate
    Properties:
      LaunchTemplateName: mole-bastion-template
      LaunchTemplateData:
        ImageId: ami-0c2d3e23f757b5d84  # Ubuntu 22.04 LTS ARM64
        InstanceType: ` + string(config.InstanceType) + `
        KeyName: !Ref MoleKeyPair
        SecurityGroupIds:
          - !Ref WireGuardSecurityGroup
        Monitoring:
          Enabled: true
        BlockDeviceMappings:
          - DeviceName: /dev/sda1
            Ebs:
              VolumeType: gp3
              VolumeSize: 20
              Encrypted: true
              DeleteOnTermination: true
        UserData:
          Fn::Base64: !Sub |
            #!/bin/bash
            set -euo pipefail

            TUNNEL_COUNT=` + fmt.Sprintf("%d", config.TunnelCount) + `
            MTU_SIZE=` + fmt.Sprintf("%d", config.MTUSize) + `

            apt-get update -y
            apt-get install -y wireguard wireguard-tools iptables-persistent awscli

            echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
            sysctl -p

            mkdir -p /etc/mole/keys

            for i in $(seq 0 $((TUNNEL_COUNT - 1))); do
                wg genkey | tee /etc/mole/keys/wg${i}_private.key | wg pubkey > /etc/mole/keys/wg${i}_public.key
                chmod 600 /etc/mole/keys/wg${i}_private.key
            done

            echo "ready" > /etc/mole/status
        TagSpecifications:
          - ResourceType: instance
            Tags:
              - Key: Name
                Value: mole-bastion
              - Key: Project
                Value: aws-cloud-mole
              - Key: ManagedBy
                Value: cloudformation

  # EC2 Instance
  BastionInstance:
    Type: AWS::EC2::Instance
    Properties:
      LaunchTemplate:
        LaunchTemplateId: !Ref BastionLaunchTemplate
        Version: !GetAtt BastionLaunchTemplate.LatestVersionNumber
      SubnetId: !Ref SubnetId
      Tags:
        - Key: Name
          Value: mole-bastion

Outputs:
  BastionPublicIP:
    Description: Public IP address of the bastion
    Value: !GetAtt BastionInstance.PublicIp
    Export:
      Name: !Sub "${AWS::StackName}-BastionPublicIP"

  BastionInstanceId:
    Description: Instance ID of the bastion
    Value: !Ref BastionInstance
    Export:
      Name: !Sub "${AWS::StackName}-BastionInstanceId"

  SSHCommand:
    Description: SSH command to connect to bastion
    Value: !Sub "ssh -i ~/.ssh/mole-key ubuntu@${BastionInstance.PublicIp}"

  SecurityGroupId:
    Description: Security Group ID
    Value: !Ref WireGuardSecurityGroup
    Export:
      Name: !Sub "${AWS::StackName}-SecurityGroupId"
`)

	return cf.String()
}

// ExportPulumi generates Pulumi configuration in Go
func (a *AWSClient) ExportPulumi(config *DeploymentConfig) string {
	var pulumi strings.Builder

	pulumi.WriteString(`package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Security Group
		sg, err := ec2.NewSecurityGroup(ctx, "mole-wireguard", &ec2.SecurityGroupArgs{
			Description: pulumi.String("AWS Cloud Mole WireGuard Security Group"),
			VpcId:       pulumi.String("` + config.VPCId + `"),
			Ingress: ec2.SecurityGroupIngressArray{
`)

	// Add WireGuard port rules
	for i := 0; i < config.TunnelCount; i++ {
		port := 51820 + i
		pulumi.WriteString(fmt.Sprintf(`				&ec2.SecurityGroupIngressArgs{
					Protocol:   pulumi.String("udp"),
					FromPort:   pulumi.Int(%d),
					ToPort:     pulumi.Int(%d),
					CidrBlocks: pulumi.StringArray{pulumi.String("%s")},
				},
`, port, port, config.AllowedCIDR))
	}

	pulumi.WriteString(`				&ec2.SecurityGroupIngressArgs{
					Protocol:   pulumi.String("tcp"),
					FromPort:   pulumi.Int(22),
					ToPort:     pulumi.Int(22),
					CidrBlocks: pulumi.StringArray{pulumi.String("` + config.AllowedCIDR + `")},
				},
			},
			Egress: ec2.SecurityGroupEgressArray{
				&ec2.SecurityGroupEgressArgs{
					Protocol:   pulumi.String("-1"),
					FromPort:   pulumi.Int(0),
					ToPort:     pulumi.Int(0),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
			Tags: pulumi.StringMap{
				"Name":    pulumi.String("mole-wireguard-sg"),
				"Project": pulumi.String("aws-cloud-mole"),
			},
		})
		if err != nil {
			return err
		}

		// Key Pair
		keyPair, err := ec2.NewKeyPair(ctx, "mole-keypair", &ec2.KeyPairArgs{
			PublicKey: pulumi.String("` + config.SSHPublicKey + `"),
		})
		if err != nil {
			return err
		}

		// Ubuntu AMI
		ubuntu, err := ec2.LookupAmi(ctx, &ec2.LookupAmiArgs{
			MostRecent: pulumi.BoolRef(true),
			Owners:     []string{"099720109477"},
			Filters: []ec2.GetAmiFilter{
				{
					Name:   "name",
					Values: []string{"ubuntu/images/hvm-ssd/ubuntu-22.04-lts-*"},
				},
				{
					Name:   "architecture",
					Values: []string{"arm64"},
				},
			},
		}, nil)
		if err != nil {
			return err
		}

		// EC2 Instance
		instance, err := ec2.NewInstance(ctx, "mole-bastion", &ec2.InstanceArgs{
			Ami:                        pulumi.String(ubuntu.Id),
			InstanceType:               pulumi.String("` + string(config.InstanceType) + `"),
			KeyName:                    keyPair.KeyName,
			VpcSecurityGroupIds:        pulumi.StringArray{sg.ID()},
			SubnetId:                   pulumi.String("` + config.PublicSubnetId + `"),
			AssociatePublicIpAddress:   pulumi.Bool(true),
			Monitoring:                 pulumi.Bool(true),
			UserData: pulumi.String(` + "`" + `#!/bin/bash
set -euo pipefail

TUNNEL_COUNT=` + fmt.Sprintf("%d", config.TunnelCount) + `
MTU_SIZE=` + fmt.Sprintf("%d", config.MTUSize) + `

apt-get update -y
apt-get install -y wireguard wireguard-tools iptables-persistent awscli

echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
sysctl -p

mkdir -p /etc/mole/keys

for i in $(seq 0 $((TUNNEL_COUNT - 1))); do
    wg genkey | tee /etc/mole/keys/wg${i}_private.key | wg pubkey > /etc/mole/keys/wg${i}_public.key
    chmod 600 /etc/mole/keys/wg${i}_private.key
done

echo "ready" > /etc/mole/status
` + "`" + `),
			Tags: pulumi.StringMap{
				"Name":    pulumi.String("mole-bastion"),
				"Project": pulumi.String("aws-cloud-mole"),
			},
		})
		if err != nil {
			return err
		}

		// Outputs
		ctx.Export("bastionPublicIP", instance.PublicIp)
		ctx.Export("bastionInstanceId", instance.ID())
		ctx.Export("sshCommand", pulumi.Sprintf("ssh -i ~/.ssh/mole-key ubuntu@%s", instance.PublicIp))

		return nil
	})
}
`)

	return pulumi.String()
}
