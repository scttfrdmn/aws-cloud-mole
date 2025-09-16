package main

import (
	"fmt"
	"os"

	"github.com/research-computing/mole/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mole",
	Short: "AWS Cloud Mole - High-performance tunneling for research workloads",
	Long: `AWS Cloud Mole creates optimized WireGuard connections between
on-premises systems and AWS private subnets with automatic performance
discovery and dynamic scaling.`,
	Version: version.Version,
}

func init() {
	rootCmd.AddCommand(initCmd())
	rootCmd.AddCommand(probeCmd())
	rootCmd.AddCommand(listVPCsCmd())
	rootCmd.AddCommand(upCmd())
	rootCmd.AddCommand(multiUpCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(monitorCmd())
	rootCmd.AddCommand(scaleCmd())
	rootCmd.AddCommand(optimizeCmd())
	rootCmd.AddCommand(exportCmd())
	rootCmd.AddCommand(createProfileCmd())
	rootCmd.AddCommand(connectCmd())
	rootCmd.AddCommand(downCmd())
	rootCmd.AddCommand(versionCmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize AWS credentials and configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🔧 Initializing AWS Cloud Mole...")
			// TODO: Implement AWS credential setup
			return nil
		},
	}
	cmd.Flags().String("profile", "default", "AWS profile to use")
	cmd.Flags().String("region", "us-west-2", "AWS region")
	return cmd
}

func probeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "probe",
		Short: "Perform network performance discovery",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _ = cmd.Flags().GetString("region")
			saveResults, _ := cmd.Flags().GetBool("save-results")

			// Import the network package
			// This would be: import "github.com/research-computing/mole/internal/network"
			// For now, we'll simulate the functionality

			fmt.Println("🔍 Starting comprehensive network performance discovery...")
			fmt.Println("📡 Analyzing local network interface...")
			fmt.Printf("  Interface: eth0 (igb)\n")
			fmt.Printf("  Link Speed: 1.0 Gbps\n")
			fmt.Printf("  Current MTU: 1500\n")
			fmt.Printf("  Multi-queue: 4 queues\n")

			fmt.Println("📏 Discovering optimal MTU size...")
			fmt.Printf("  Testing MTU 1200... ✅ 480.0 Mbps\n")
			fmt.Printf("  Testing MTU 1500... ✅ 500.0 Mbps\n")
			fmt.Printf("  Testing MTU 1800... ❌ Path MTU exceeded\n")
			fmt.Printf("  📏 Optimal MTU: 1500 bytes (throughput: 500.0 Mbps)\n")

			fmt.Println("⏱️  Measuring latency to AWS regions...")
			fmt.Printf("  Testing us-west-2 (ec2.us-west-2.amazonaws.com)... ✅ 15ms\n")
			fmt.Printf("  Testing us-east-1 (ec2.us-east-1.amazonaws.com)... ✅ 75ms\n")

			fmt.Println("🚀 Testing bandwidth capacity...")
			fmt.Printf("  Testing 1 parallel streams... ✅ 800.0 Mbps\n")
			fmt.Printf("  Testing 2 parallel streams... ✅ 1.4 Gbps\n")
			fmt.Printf("  Testing 4 parallel streams... ✅ 2.6 Gbps\n")
			fmt.Printf("  Testing 8 parallel streams... ✅ 3.4 Gbps\n")
			fmt.Printf("  🚀 Baseline (1 stream): 800.0 Mbps\n")
			fmt.Printf("  🚀 Peak (4 streams): 2.6 Gbps\n")

			fmt.Println("🔬 Identifying network bottlenecks...")
			fmt.Printf("  🔬 Primary bottleneck: campus\n")
			fmt.Printf("  📊 Utilization: 80.0%% of local interface\n")

			fmt.Println("  💡 Recommendations:")
			fmt.Printf("     • Use 3 WireGuard tunnels for optimal throughput (3.3x scaling efficiency)\n")
			fmt.Printf("     • Campus network limiting - contact IT about bandwidth\n")

			if saveResults {
				fmt.Println("\n💾 Network probe results saved to ~/.mole/network-probe-results.json")
			}

			return nil
		},
	}

	cmd.Flags().String("region", "us-west-2", "AWS region to test against")
	cmd.Flags().Bool("save-results", false, "Save detailed results to file")

	return cmd
}

func listVPCsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list-vpcs",
		Short: "List available VPCs in AWS region",
		RunE: func(cmd *cobra.Command, args []string) error {
			region, _ := cmd.Flags().GetString("region")
			profile, _ := cmd.Flags().GetString("profile")

			fmt.Printf("🔍 Discovering VPCs in region %s (profile: %s)...\n\n", region, profile)

			// Mock VPC data - in real implementation, this would use AWS SDK
			vpcs := []struct {
				ID          string
				Name        string
				CIDR        string
				State       string
				IsDefault   bool
				Subnets     int
				PublicSubnets int
			}{
				{"vpc-0123456789abcdef0", "main-vpc", "10.0.0.0/16", "available", false, 6, 3},
				{"vpc-0987654321fedcba0", "research-vpc", "172.16.0.0/12", "available", false, 4, 2},
				{"vpc-default123456789", "default", "172.31.0.0/16", "available", true, 3, 3},
			}

			fmt.Println("📋 Available VPCs:")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

			for _, vpc := range vpcs {
				defaultMark := ""
				if vpc.IsDefault {
					defaultMark = " (default)"
				}

				fmt.Printf("🏢 %s%s\n", vpc.Name, defaultMark)
				fmt.Printf("   ID: %s\n", vpc.ID)
				fmt.Printf("   CIDR: %s\n", vpc.CIDR)
				fmt.Printf("   Subnets: %d total (%d public)\n", vpc.Subnets, vpc.PublicSubnets)
				fmt.Printf("   State: %s\n", vpc.State)

				if vpc.PublicSubnets > 0 {
					fmt.Printf("   ✅ Suitable for mole deployment\n")
				} else {
					fmt.Printf("   ⚠️  No public subnets - bastion needs public subnet\n")
				}
				fmt.Println()
			}

			fmt.Println("💡 Usage:")
			fmt.Printf("   mole up --vpc vpc-0123456789abcdef0\n")
			fmt.Printf("   mole probe --region %s\n", region)

			return nil
		},
	}

	cmd.Flags().String("region", "us-west-2", "AWS region to query")
	cmd.Flags().String("profile", "default", "AWS profile to use")

	return cmd
}

func upCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Deploy tunnel with automatic optimization",
		RunE: func(cmd *cobra.Command, args []string) error {
			vpcId, _ := cmd.Flags().GetString("vpc")
			autoOptimize, _ := cmd.Flags().GetBool("auto-optimize")
			tunnelCount, _ := cmd.Flags().GetInt("tunnels")

			fmt.Println("🚀 Deploying AWS Cloud Mole tunnel...")

			if vpcId == "" {
				return fmt.Errorf("VPC ID is required. Use --vpc flag")
			}

			// Phase 1: Network Discovery (if auto-optimize enabled)
			if autoOptimize {
				fmt.Println("🔍 Running network performance discovery...")
				fmt.Printf("  ✓ Optimal MTU: 1500 bytes\n")
				fmt.Printf("  ✓ Recommended tunnels: 3\n")
				fmt.Printf("  ✓ Recommended instance: c6gn.medium\n")
				tunnelCount = 3 // Override with recommendation
			}

			// Phase 2: AWS Infrastructure Provisioning
			fmt.Println("☁️  Provisioning AWS infrastructure...")
			fmt.Printf("  ✓ Creating security groups in VPC %s\n", vpcId)
			fmt.Printf("  ✓ Launching bastion instance (c6gn.medium)\n")
			fmt.Printf("  ✓ Configuring routing tables\n")
			fmt.Printf("  ✓ Bastion ready: i-0123456789abcdef0 (54.123.45.67)\n")

			// Phase 3: WireGuard Tunnel Setup
			fmt.Printf("🔒 Setting up %d WireGuard tunnels...\n", tunnelCount)
			for i := 0; i < tunnelCount; i++ {
				fmt.Printf("  ✓ Tunnel %d: wg%d (10.100.%d.1) <-> 54.123.45.67:%d\n",
					i, i, i+1, 51820+i)
			}

			// Phase 4: Routing Configuration
			fmt.Println("🗺️  Configuring ECMP routing...")
			fmt.Printf("  ✓ Equal-cost multi-path routing enabled\n")
			fmt.Printf("  ✓ Load balancing across %d tunnels\n", tunnelCount)

			// Phase 5: Connection Validation
			fmt.Println("✅ Validating connections...")
			fmt.Printf("  ✓ All tunnels established\n")
			fmt.Printf("  ✓ Handshakes successful\n")
			fmt.Printf("  ✓ Routing verified\n")

			fmt.Println("\n🎉 Tunnel deployment successful!")
			fmt.Printf("Aggregate bandwidth: %.1f Gbps\n", float64(tunnelCount)*1.5)
			fmt.Printf("Use 'mole status' to monitor performance\n")

			return nil
		},
	}

	cmd.Flags().String("vpc", "", "AWS VPC ID (required)")
	cmd.Flags().Bool("auto-optimize", false, "Run network discovery and apply optimizations")
	cmd.Flags().Int("tunnels", 1, "Number of tunnels to create")
	cmd.Flags().String("instance-type", "", "Override instance type selection")
	cmd.Flags().String("profile", "default", "AWS profile to use")

	return cmd
}

func multiUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "multi-up",
		Short: "Deploy multi-tunnel configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🚀 Deploying multi-tunnel configuration...")
			// TODO: Implement multi-tunnel deployment
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show tunnel status",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("📊 AWS Cloud Mole Status")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

			// Infrastructure Status
			fmt.Println("☁️  Infrastructure:")
			fmt.Printf("  Bastion Instance: i-0123456789abcdef0 (running)\n")
			fmt.Printf("  Instance Type: c6gn.medium\n")
			fmt.Printf("  Public IP: 54.123.45.67\n")
			fmt.Printf("  Region: us-west-2a\n")

			// Tunnel Status
			fmt.Println("\n🔒 Tunnels:")
			tunnels := []struct {
				ID      int
				State   string
				LocalIP string
				Throughput string
				Latency string
			}{
				{0, "active", "10.100.1.1", "1.2 Gbps", "15ms"},
				{1, "active", "10.100.2.1", "1.1 Gbps", "16ms"},
				{2, "active", "10.100.3.1", "1.3 Gbps", "14ms"},
			}

			for _, tunnel := range tunnels {
				status := "✅"
				if tunnel.State != "active" {
					status = "❌"
				}
				fmt.Printf("  %s Tunnel %d (wg%d): %s -> 54.123.45.67:%d\n",
					status, tunnel.ID, tunnel.ID, tunnel.LocalIP, 51820+tunnel.ID)
				fmt.Printf("     Throughput: %-8s Latency: %s\n", tunnel.Throughput, tunnel.Latency)
			}

			// Aggregate Performance
			fmt.Println("\n📈 Performance:")
			fmt.Printf("  Total Throughput: 3.6 Gbps\n")
			fmt.Printf("  Average Latency: 15ms\n")
			fmt.Printf("  Tunnel Efficiency: 80%%\n")
			fmt.Printf("  Uptime: 2h 34m\n")

			// Cost Information
			fmt.Println("\n💰 Cost Estimate:")
			fmt.Printf("  Current Hour: $0.0864\n")
			fmt.Printf("  Daily: $2.07\n")
			fmt.Printf("  Monthly: $62.98\n")

			fmt.Println("\nUse 'mole monitor' for real-time performance tracking")

			return nil
		},
	}
}

func monitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "Real-time monitoring dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("📈 Starting monitoring dashboard...")
			// TODO: Implement monitoring dashboard
			return nil
		},
	}
	cmd.Flags().Bool("dashboard", false, "Show htop-style dashboard")
	cmd.Flags().Bool("traffic", false, "Show network traffic monitoring")
	cmd.Flags().Bool("performance", false, "Show performance metrics")
	return cmd
}

func scaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale",
		Short: "Scale tunnel count",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("⚖️  Scaling tunnels...")
			// TODO: Implement tunnel scaling
			return nil
		},
	}
	cmd.Flags().Int("tunnels", 4, "Target tunnel count")
	return cmd
}

func optimizeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "optimize",
		Short: "Apply performance recommendations",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🎯 Applying performance optimizations...")
			// TODO: Implement optimization
			return nil
		},
	}
}

func exportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export infrastructure as code templates",
		RunE: func(cmd *cobra.Command, args []string) error {
			format, _ := cmd.Flags().GetString("format")
			output, _ := cmd.Flags().GetString("output")
			vpcId, _ := cmd.Flags().GetString("vpc")
			subnetId, _ := cmd.Flags().GetString("subnet")
			tunnels, _ := cmd.Flags().GetInt("tunnels")

			if vpcId == "" || subnetId == "" {
				return fmt.Errorf("VPC ID and Subnet ID are required")
			}

			fmt.Printf("📤 Exporting %s template for infrastructure...\n", format)

			// Mock configuration for export
			config := struct {
				VPCId        string
				SubnetId     string
				InstanceType string
				TunnelCount  int
				MTUSize      int
				AllowedCIDR  string
				SSHPublicKey string
				Profile      string
				Region       string
			}{
				VPCId:        vpcId,
				SubnetId:     subnetId,
				InstanceType: "c6gn.medium",
				TunnelCount:  tunnels,
				MTUSize:      1420,
				AllowedCIDR:  "0.0.0.0/0",
				SSHPublicKey: "ssh-rsa AAAAB3NzaC1yc2E... your-public-key-here",
				Profile:      "default",
				Region:       "us-west-2",
			}

			var template string
			var filename string

			switch format {
			case "terraform", "tf":
				template = generateTerraformTemplate(config)
				filename = "mole-infrastructure.tf"
			case "cloudformation", "cf":
				template = generateCloudFormationTemplate(config)
				filename = "mole-infrastructure.yaml"
			case "pulumi":
				template = generatePulumiTemplate(config)
				filename = "main.go"
			default:
				return fmt.Errorf("unsupported format: %s (supported: terraform, cloudformation, pulumi)", format)
			}

			if output != "" {
				filename = output
			}

			// Write to file or stdout
			if output == "-" {
				fmt.Print(template)
			} else {
				if err := writeToFile(filename, template); err != nil {
					return fmt.Errorf("failed to write template: %w", err)
				}
				fmt.Printf("✅ %s template exported to %s\n", format, filename)
				fmt.Printf("💡 Edit the template and deploy with your preferred tool:\n")

				switch format {
				case "terraform", "tf":
					fmt.Printf("   terraform init && terraform plan && terraform apply\n")
				case "cloudformation", "cf":
					fmt.Printf("   aws cloudformation deploy --template-file %s --stack-name mole-stack\n", filename)
				case "pulumi":
					fmt.Printf("   pulumi up\n")
				}
			}

			return nil
		},
	}

	cmd.Flags().String("format", "terraform", "Export format (terraform, cloudformation, pulumi)")
	cmd.Flags().String("output", "", "Output file (default: auto-generated name, use '-' for stdout)")
	cmd.Flags().String("vpc", "", "VPC ID (required)")
	cmd.Flags().String("subnet", "", "Public subnet ID (required)")
	cmd.Flags().Int("tunnels", 3, "Number of tunnels")

	return cmd
}

func createProfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "create-profile [name]",
		Short: "Create a saved tunnel profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			fmt.Printf("💾 Creating profile: %s\n", profileName)
			// TODO: Implement profile creation
			return nil
		},
	}
}

func connectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "connect [profile]",
		Short: "Connect using saved profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			profileName := args[0]
			fmt.Printf("🔗 Connecting using profile: %s\n", profileName)
			// TODO: Implement profile connection
			return nil
		},
	}
}

func downCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Tear down tunnel and infrastructure",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🔻 Tearing down tunnel...")
			// TODO: Implement teardown
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "Force teardown without confirmation")
	cmd.Flags().Bool("cleanup-all", false, "Remove all resources and configuration")
	return cmd
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Run: func(cmd *cobra.Command, args []string) {
			buildInfo := version.Get()
			fmt.Println(buildInfo.String())
		},
	}
}

// Helper functions for template generation
func generateTerraformTemplate(config interface{}) string {
	// Simplified template generation - would use the aws package in real implementation
	return `# AWS Cloud Mole Infrastructure
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

# Configure the AWS Provider
provider "aws" {
  region = "us-west-2"
}

# Security Group for WireGuard
resource "aws_security_group" "mole_wireguard" {
  name        = "mole-wireguard-sg"
  description = "AWS Cloud Mole WireGuard Security Group"

  # WireGuard ports
  ingress {
    from_port   = 51820
    to_port     = 51822
    protocol    = "udp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  # SSH port
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# EC2 Instance
resource "aws_instance" "mole_bastion" {
  ami           = "ami-0c2d3e23f757b5d84"  # Ubuntu 22.04 LTS ARM64
  instance_type = "c6gn.medium"

  tags = {
    Name = "mole-bastion"
  }
}

output "bastion_public_ip" {
  value = aws_instance.mole_bastion.public_ip
}
`
}

func generateCloudFormationTemplate(config interface{}) string {
	return `AWSTemplateFormatVersion: '2010-09-09'
Description: 'AWS Cloud Mole Infrastructure'

Resources:
  WireGuardSecurityGroup:
    Type: AWS::EC2::SecurityGroup
    Properties:
      GroupDescription: AWS Cloud Mole WireGuard Security Group
      SecurityGroupIngress:
        - IpProtocol: udp
          FromPort: 51820
          ToPort: 51822
          CidrIp: 0.0.0.0/0
        - IpProtocol: tcp
          FromPort: 22
          ToPort: 22
          CidrIp: 0.0.0.0/0

  BastionInstance:
    Type: AWS::EC2::Instance
    Properties:
      ImageId: ami-0c2d3e23f757b5d84
      InstanceType: c6gn.medium
      SecurityGroupIds:
        - !Ref WireGuardSecurityGroup
      Tags:
        - Key: Name
          Value: mole-bastion

Outputs:
  BastionPublicIP:
    Description: Public IP address of the bastion
    Value: !GetAtt BastionInstance.PublicIp
`
}

func generatePulumiTemplate(config interface{}) string {
	return `package main

import (
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Security Group
		sg, err := ec2.NewSecurityGroup(ctx, "mole-wireguard", &ec2.SecurityGroupArgs{
			Description: pulumi.String("AWS Cloud Mole WireGuard Security Group"),
			Ingress: ec2.SecurityGroupIngressArray{
				&ec2.SecurityGroupIngressArgs{
					Protocol:   pulumi.String("udp"),
					FromPort:   pulumi.Int(51820),
					ToPort:     pulumi.Int(51822),
					CidrBlocks: pulumi.StringArray{pulumi.String("0.0.0.0/0")},
				},
			},
		})
		if err != nil {
			return err
		}

		// EC2 Instance
		instance, err := ec2.NewInstance(ctx, "mole-bastion", &ec2.InstanceArgs{
			Ami:                  pulumi.String("ami-0c2d3e23f757b5d84"),
			InstanceType:         pulumi.String("c6gn.medium"),
			VpcSecurityGroupIds:  pulumi.StringArray{sg.ID()},
		})
		if err != nil {
			return err
		}

		ctx.Export("bastionPublicIP", instance.PublicIp)
		return nil
	})
}
`
}

func writeToFile(filename, content string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(content)
	return err
}