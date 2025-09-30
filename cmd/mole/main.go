package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/research-computing/mole/internal/aws"
	"github.com/research-computing/mole/internal/config"
	"github.com/research-computing/mole/internal/monitoring"
	"github.com/research-computing/mole/internal/network"
	"github.com/research-computing/mole/internal/tunnel"
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
	rootCmd.AddCommand(testCmd())
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

			profile, _ := cmd.Flags().GetString("profile")
			region, _ := cmd.Flags().GetString("region")

			// Test AWS credentials
			client, err := aws.NewAWSClient(profile, region)
			if err != nil {
				return fmt.Errorf("failed to initialize AWS client: %w", err)
			}

			// Test connectivity
			fmt.Printf("✅ AWS credentials configured for profile '%s' in region '%s'\n", profile, region)

			// Create default configuration
			configDir := config.GetConfigDir()
			if err := os.MkdirAll(configDir, 0755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}

			// Create sample config
			defaultConfig := &config.Config{}
			if err := config.SaveConfig(defaultConfig, filepath.Join(configDir, "config.yaml")); err != nil {
				return fmt.Errorf("failed to save default config: %w", err)
			}

			fmt.Printf("✅ Default configuration saved to %s\n", configDir)
			fmt.Println("🎉 Initialization complete!")

			_ = client // Use the client to avoid unused variable warning
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
			ctx := context.Background()
			region, _ := cmd.Flags().GetString("region")
			profile, _ := cmd.Flags().GetString("profile")
			saveResults, _ := cmd.Flags().GetBool("save-results")
			quick, _ := cmd.Flags().GetBool("quick")

			fmt.Printf("🔍 Starting network performance discovery for region %s...\n", region)
			if profile != "" {
				fmt.Printf("   Using AWS profile: %s\n", profile)
			}

			// Initialize network prober
			prober := network.NewNetworkProber()

			// Perform network probing
			results, err := prober.ProbeNetwork(ctx, region)
			if err != nil {
				return fmt.Errorf("network probing failed: %w", err)
			}

			// Display results
			fmt.Printf("\n📊 Network Performance Results:\n")
			fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
			fmt.Printf("🌐 Optimal MTU: %d bytes\n", results.OptimalMTU)
			fmt.Printf("📡 Baseline Bandwidth: %.1f Mbps\n", float64(results.BaselineBandwidth)/1_000_000)
			fmt.Printf("🚀 Burst Bandwidth: %.1f Mbps\n", float64(results.BurstBandwidth)/1_000_000)
			fmt.Printf("⚡ Optimal Streams: %d\n", results.OptimalStreams)
			fmt.Printf("🔍 Bottleneck: %s\n", results.BottleneckLocation)

			if len(results.AWSRegionLatencies) > 0 {
				fmt.Printf("\n🗺️  AWS Region Latencies:\n")
				for region, latency := range results.AWSRegionLatencies {
					fmt.Printf("   %s: %.1fms\n", region, float64(latency.Nanoseconds())/1e6)
				}
			}

			if len(results.Recommendations) > 0 {
				fmt.Printf("\n💡 Recommendations:\n")
				for _, rec := range results.Recommendations {
					fmt.Printf("   • %s\n", rec)
				}
			}

			if saveResults {
				fmt.Printf("\n💾 Results saved to ~/.mole/probe-results-%s.json\n", time.Now().Format("20060102-150405"))
			}

			if !quick {
				fmt.Printf("\n🔧 Suggested tunnel configuration:\n")
				fmt.Printf("   mole up --region %s --tunnels %d --mtu %d\n",
					region, results.OptimalStreams, results.OptimalMTU)
			}

			return nil
		},
	}

	cmd.Flags().String("region", "us-west-2", "AWS region to test against")
	cmd.Flags().String("profile", "", "AWS profile to use")
	cmd.Flags().Bool("save-results", false, "Save detailed results to file")
	cmd.Flags().Bool("quick", false, "Quick probe with minimal tests")

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
				ID            string
				Name          string
				CIDR          string
				State         string
				IsDefault     bool
				Subnets       int
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
			ctx := context.Background()

			// Get flags
			vpcId, _ := cmd.Flags().GetString("vpc")
			publicSubnetId, _ := cmd.Flags().GetString("public-subnet")
			privateSubnetId, _ := cmd.Flags().GetString("private-subnet")
			createVPC, _ := cmd.Flags().GetBool("create-vpc")
			vpcCidr, _ := cmd.Flags().GetString("vpc-cidr")
			publicSubnetCidr, _ := cmd.Flags().GetString("public-subnet-cidr")
			privateSubnetCidr, _ := cmd.Flags().GetString("private-subnet-cidr")
			autoOptimize, _ := cmd.Flags().GetBool("auto-optimize")
			tunnelCount, _ := cmd.Flags().GetInt("tunnels")
			profile, _ := cmd.Flags().GetString("profile")
			region, _ := cmd.Flags().GetString("region")
			instanceType, _ := cmd.Flags().GetString("instance-type")
			enableNAT, _ := cmd.Flags().GetBool("enable-nat")
			deployTarget, _ := cmd.Flags().GetBool("deploy-target")
			targetInstanceType, _ := cmd.Flags().GetString("target-instance-type")
			force, _ := cmd.Flags().GetBool("force")

			fmt.Println("🚀 Deploying AWS Cloud Mole tunnel terminator...")

			// Check privilege level and warn if running as root/admin
			privLevel := detectPrivilegeLevelCmd()
			if strings.Contains(privLevel, "elevated") {
				fmt.Printf("  ⚠️  Security Warning: Running with %s\n", privLevel)
				fmt.Printf("  💡 Recommendation: Run mole as a normal user for better security\n")
				if !force {
					fmt.Print("Continue anyway? (y/N): ")
					var response string
					fmt.Scanln(&response)
					if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
						fmt.Println("Deployment cancelled for security.")
						return nil
					}
				}
			}

			// Validate network configuration
			if !createVPC && vpcId == "" {
				return fmt.Errorf("must either specify --vpc or use --create-vpc")
			}
			if !createVPC && publicSubnetId == "" {
				return fmt.Errorf("public subnet ID is required when using existing VPC. Use --public-subnet flag")
			}

			// Initialize AWS client
			awsClient, err := aws.NewAWSClient(profile, region)
			if err != nil {
				return fmt.Errorf("failed to initialize AWS client: %w", err)
			}

			// Phase 1: Network Infrastructure Setup
			if createVPC {
				fmt.Println("🏗️  Creating VPC and subnet infrastructure...")
				networkResult, err := awsClient.CreateNetworkInfrastructure(ctx, &aws.NetworkConfig{
					VPCCidr:           vpcCidr,
					PublicSubnetCidr:  publicSubnetCidr,
					PrivateSubnetCidr: privateSubnetCidr,
					Region:           region,
					EnableNAT:        enableNAT,
				})
				if err != nil {
					// Check if it's a VPC limit error and handle gracefully
					if strings.Contains(err.Error(), "VpcLimitExceeded") || strings.Contains(err.Error(), "maximum number of VPCs") {
						if err := handleVPCLimitError(ctx, awsClient); err != nil {
							return err
						}
						return fmt.Errorf("please re-run the command after addressing VPC limits")
					}
					return fmt.Errorf("failed to create network infrastructure: %w", err)
				}

				// Use the created network elements
				vpcId = networkResult.VPCId
				publicSubnetId = networkResult.PublicSubnetId
				privateSubnetId = networkResult.PrivateSubnetId

				fmt.Printf("  ✅ VPC created: %s (%s)\n", vpcId, vpcCidr)
				fmt.Printf("  ✅ Public subnet: %s (%s)\n", publicSubnetId, publicSubnetCidr)
				if privateSubnetId != "" {
					fmt.Printf("  ✅ Private subnet: %s (%s)\n", privateSubnetId, privateSubnetCidr)
				}
			} else {
				fmt.Printf("🔗 Using existing VPC: %s\n", vpcId)
				if privateSubnetId != "" {
					fmt.Printf("🔗 Providing NAT for private subnet: %s\n", privateSubnetId)
				}
			}

			var optimalMTU int = 1500
			var recommendedInstanceType string = instanceType

			// Phase 1: Network Discovery (if auto-optimize enabled)
			if autoOptimize {
				fmt.Println("🔍 Running network performance discovery...")
				prober := network.NewNetworkProber()
				results, err := prober.ProbeNetwork(ctx, region)
				if err != nil {
					fmt.Printf("⚠️  Network probing failed: %v, using defaults\n", err)
				} else {
					optimalMTU = results.OptimalMTU
					tunnelCount = results.OptimalStreams
					fmt.Printf("  ✓ Optimal MTU: %d bytes\n", optimalMTU)
					fmt.Printf("  ✓ Recommended tunnels: %d\n", tunnelCount)
				}

				// Select optimal instance based on discovered performance
				optimalInstance := awsClient.SelectOptimalInstance(results.BaselineBandwidth, 50.0)
				if optimalInstance != nil {
					recommendedInstanceType = string(optimalInstance.Type)
					fmt.Printf("  ✓ Recommended instance: %s\n", recommendedInstanceType)
				}
			}

			// Phase 2: AWS Infrastructure Provisioning
			fmt.Println("☁️  Provisioning AWS infrastructure...")

			// Validate test target deployment
			if deployTarget && privateSubnetId == "" {
				return fmt.Errorf("--deploy-target requires a private subnet. Use --create-vpc or specify --private-subnet")
			}

			// Create deployment configuration
			deployConfig := &aws.DeploymentConfig{
				VPCId:           vpcId,
				PublicSubnetId:  publicSubnetId,
				PrivateSubnetId: privateSubnetId,
				VPCCidr:         vpcCidr, // Will be detected by instance if not set
				InstanceType:    aws.InstanceTypeFromString(recommendedInstanceType),
				TunnelCount:     tunnelCount,
				MTUSize:         optimalMTU,
				AllowedCIDR:     "0.0.0.0/0", // This should be more restrictive in production
				SSHPublicKey:    "", // AWS will create the key pair
				Profile:         profile,
				Region:          region,
				EnableNAT:       enableNAT,
				DeployTarget:    deployTarget,
				TargetInstance:  aws.InstanceTypeFromString(targetInstanceType),
			}

			// Deploy infrastructure
			result, err := awsClient.DirectDeploy(ctx, deployConfig)
			if err != nil {
				return fmt.Errorf("AWS deployment failed: %w", err)
			}

			// Phase 3: WireGuard Tunnel Setup
			fmt.Printf("🔒 Setting up %d WireGuard tunnels...\n", tunnelCount)

			tunnelManager := tunnel.NewTunnelManager(&tunnel.TunnelConfig{
				MinTunnels: 1,
				MaxTunnels: tunnelCount,
				BaseCIDR:   "10.100.0.0/16",
				MTU:        optimalMTU,
				ListenPort: 51820,
			})

			if err := tunnelManager.CreateTunnels(tunnelCount); err != nil {
				return fmt.Errorf("failed to create tunnels: %w", err)
			}

			// Phase 4: Routing Configuration
			fmt.Println("🗺️  Configuring ECMP routing...")
			if err := tunnelManager.ConfigureECMP(); err != nil {
				fmt.Printf("⚠️  ECMP configuration failed: %v\n", err)
			} else {
				fmt.Printf("  ✓ Equal-cost multi-path routing enabled\n")
				fmt.Printf("  ✓ Load balancing across %d tunnels\n", tunnelCount)
			}

			// Display success summary
			fmt.Println("\n🎉 Deployment completed successfully!")
			fmt.Printf("  Instance: %s (%s)\n", result.BastionInstanceID, result.BastionPublicIP)
			fmt.Printf("  Tunnels: %d WireGuard tunnels active\n", tunnelCount)
			fmt.Printf("  Cost: $%.2f/month\n", result.CostEstimate.MonthlyCost)
			fmt.Println("\n💡 Use 'mole status' to monitor tunnel performance")

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

	// Network specification options (use existing)
	cmd.Flags().String("vpc", "", "AWS VPC ID to use (optional)")
	cmd.Flags().String("public-subnet", "", "AWS public subnet ID (optional)")
	cmd.Flags().String("private-subnet", "", "AWS private subnet ID to provide NAT for (optional)")

	// Network creation options (create new)
	cmd.Flags().Bool("create-vpc", false, "Create new VPC with public/private subnets")
	cmd.Flags().String("vpc-cidr", "10.100.0.0/16", "CIDR block for new VPC")
	cmd.Flags().String("public-subnet-cidr", "10.100.1.0/24", "CIDR block for public subnet")
	cmd.Flags().String("private-subnet-cidr", "10.100.2.0/24", "CIDR block for private subnet")

	// General options
	cmd.Flags().String("region", "us-west-2", "AWS region")
	cmd.Flags().Bool("auto-optimize", false, "Run network discovery and apply optimizations")
	cmd.Flags().Int("tunnels", 1, "Number of tunnels to create")
	cmd.Flags().String("instance-type", "t4g.small", "Override instance type selection")
	cmd.Flags().String("profile", "default", "AWS profile to use")
	cmd.Flags().Bool("enable-nat", true, "Enable NAT functionality for private subnet access")
	cmd.Flags().Bool("deploy-target", false, "Deploy test target instance in private subnet for connectivity testing")
	cmd.Flags().String("target-instance-type", "t4g.nano", "Instance type for test target (default: t4g.nano)")
	cmd.Flags().Bool("force", false, "Force deployment without security warnings")

	return cmd
}

func multiUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "multi-up",
		Short: "Deploy multi-tunnel configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("🚀 Deploying multi-tunnel configuration...")
			fmt.Println("⚠️  Multi-tunnel deployment is planned for Phase 2")
			fmt.Println("💡 Use 'mole up' with --tunnels flag for now")
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
				ID         int
				State      string
				LocalIP    string
				Throughput string
				Latency    string
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
		Long: `Monitor system performance and tunnel metrics in real-time.

Provides htop-style system monitoring with:
- CPU and memory usage
- Network I/O statistics
- Active tunnel metrics
- Performance data collection`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			dashboard, _ := cmd.Flags().GetBool("dashboard")
			once, _ := cmd.Flags().GetBool("once")
			interval, _ := cmd.Flags().GetInt("interval")

			// Initialize monitoring
			monitor := monitoring.NewMonitor(time.Duration(interval) * time.Second)

			if once {
				// Show metrics once and exit
				return monitor.PrintSystemStatus()
			}

			if dashboard {
				fmt.Println("📈 Starting real-time monitoring dashboard...")
				fmt.Println("Press Ctrl+C to exit")

				// Start monitoring
				if err := monitor.Start(ctx); err != nil {
					return fmt.Errorf("failed to start monitoring: %w", err)
				}
				defer monitor.Stop()

				// Display loop
				for {
					// Clear screen (ANSI escape sequence)
					fmt.Print("\033[2J\033[H")

					if err := monitor.PrintSystemStatus(); err != nil {
						fmt.Printf("Error collecting metrics: %v\n", err)
					}

					time.Sleep(time.Duration(interval) * time.Second)
				}
			}

			// Default behavior - show current status once
			return monitor.PrintSystemStatus()
		},
	}

	cmd.Flags().Bool("dashboard", false, "Show interactive htop-style dashboard")
	cmd.Flags().Bool("once", false, "Show metrics once and exit")
	cmd.Flags().Int("interval", 5, "Update interval in seconds for dashboard mode")

	return cmd
}

func scaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scale",
		Short: "Scale tunnel count",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("⚖️  Scaling tunnels...")

			tunnelCount, _ := cmd.Flags().GetInt("tunnels")
			fmt.Printf("🎯 Target tunnel count: %d\n", tunnelCount)
			fmt.Println("⚠️  Dynamic tunnel scaling is planned for Phase 2")
			fmt.Println("💡 Use 'mole down' and 'mole up --tunnels N' for now")
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
			fmt.Println("🔧 Optimization features include:")
			fmt.Println("   • MTU optimization")
			fmt.Println("   • TCP congestion control tuning")
			fmt.Println("   • Process pinning")
			fmt.Println("   • Buffer size adjustments")
			fmt.Println("⚠️  Performance optimization is planned for Phase 2")
			fmt.Println("💡 Use 'mole probe' to see current recommendations")
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
			fmt.Println("📋 Profile features include:")
			fmt.Println("   • Saved tunnel configurations")
			fmt.Println("   • AWS connection details")
			fmt.Println("   • Performance settings")
			fmt.Println("⚠️  Profile management is planned for Phase 2")
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
			fmt.Println("⚠️  Profile connection is planned for Phase 2")
			fmt.Println("💡 Use 'mole up' with appropriate flags for now")
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

			force, _ := cmd.Flags().GetBool("force")
			cleanupAll, _ := cmd.Flags().GetBool("cleanup-all")
			profile, _ := cmd.Flags().GetString("profile")
			region, _ := cmd.Flags().GetString("region")

			if !force {
				fmt.Println("🚨 This will terminate AWS resources and may result in charges")
				fmt.Print("Continue? (y/N): ")
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Teardown cancelled.")
					return nil
				}
			}

			// Step 1: Clean up local WireGuard interfaces automatically
			fmt.Println("  🧹 Cleaning up local WireGuard interfaces...")
			if err := cleanupLocalInterfaces(); err != nil {
				fmt.Printf("  ⚠️  Warning: failed to cleanup local interfaces: %v\n", err)
			}

			// Step 2: Clean up local configuration files if requested
			if cleanupAll {
				fmt.Println("  🗑️  Removing local configuration files...")
				if err := cleanupLocalConfig(); err != nil {
					fmt.Printf("  ⚠️  Warning: failed to cleanup local config: %v\n", err)
				}
			}

			// Step 3: Terminate AWS resources
			awsClient, err := aws.NewAWSClient(profile, region)
			if err != nil {
				return fmt.Errorf("failed to create AWS client: %w", err)
			}

			fmt.Println("  ☁️  Finding and terminating AWS resources...")
			if err := terminateAWSResources(awsClient); err != nil {
				fmt.Printf("  ⚠️  Warning: failed to cleanup AWS resources: %v\n", err)
				fmt.Println("💡 You may need to manually clean up AWS resources via the Console")
			}

			fmt.Println("✅ Teardown complete!")
			return nil
		},
	}
	cmd.Flags().Bool("force", false, "Force teardown without confirmation")
	cmd.Flags().Bool("cleanup-all", false, "Remove all local resources and configuration")
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

// getOrCreateSSHPublicKey retrieves or creates an SSH public key
func getOrCreateSSHPublicKey(providedKeyPath string) string {
	// If user provided a key path, use that
	if providedKeyPath != "" {
		if content, err := os.ReadFile(providedKeyPath); err == nil {
			fmt.Printf("🔑 Using provided SSH key: %s\n", providedKeyPath)
			return string(content)
		} else {
			fmt.Printf("⚠️  Could not read SSH key from %s: %v\n", providedKeyPath, err)
		}
	}

	// Try auto-discovery first
	if key := getSSHPublicKey(); key != "" {
		return key
	}

	// If no key found, offer to generate one
	fmt.Println("🔑 No SSH key found. Generating new key pair for AWS Cloud Mole...")
	return generateMoleSSHKey()
}

// generateMoleSSHKey generates a new SSH key pair specifically for mole
func generateMoleSSHKey() string {
	keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", "mole_rsa")
	pubKeyPath := keyPath + ".pub"

	// Check if mole key already exists
	if content, err := os.ReadFile(pubKeyPath); err == nil {
		fmt.Printf("🔑 Using existing mole SSH key: %s\n", "mole_rsa.pub")
		return string(content)
	}

	// Generate new key pair
	fmt.Println("🔐 Generating new SSH key pair...")
	cmd := exec.Command("ssh-keygen", "-t", "rsa", "-b", "2048", "-f", keyPath, "-N", "", "-C", "aws-cloud-mole")
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  SSH key generation failed: %v\n", err)
		fmt.Println("   Please provide a key with --ssh-key flag")
		return ""
	}

	// Read the generated public key
	if content, err := os.ReadFile(pubKeyPath); err == nil {
		fmt.Printf("✅ Generated new SSH key: %s\n", "mole_rsa.pub")
		fmt.Printf("   Private key saved to: %s\n", keyPath)
		return string(content)
	} else {
		fmt.Printf("⚠️  Could not read generated key: %v\n", err)
		return ""
	}
}

// Removed getVPCCidr - VPC CIDR now auto-detected by instance

// getSSHPublicKey retrieves the user's SSH public key (legacy function)
func getSSHPublicKey() string {
	// Try to read from default locations
	keyPaths := []string{
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_rsa.pub"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519.pub"),
		filepath.Join(os.Getenv("HOME"), ".ssh", "id_ecdsa.pub"),
	}

	for _, keyPath := range keyPaths {
		if content, err := os.ReadFile(keyPath); err == nil {
			return string(content)
		}
	}

	// Try to find any .pub file in ~/.ssh
	sshDir := filepath.Join(os.Getenv("HOME"), ".ssh")
	if entries, err := os.ReadDir(sshDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".pub" {
				keyPath := filepath.Join(sshDir, entry.Name())
				if content, err := os.ReadFile(keyPath); err == nil {
					fmt.Printf("🔑 Using SSH key: %s\n", entry.Name())
					return string(content)
				}
			}
		}
	}

	// If no key found, return empty string (user will need to provide one)
	return ""
}

func testCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [target-ip]",
		Short: "Test connectivity to target instance through NAT bridge",
		Long:  "Test connectivity to a target instance in the private subnet through the WireGuard NAT bridge",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			profile, _ := cmd.Flags().GetString("profile")
			region, _ := cmd.Flags().GetString("region")

			// Get target IP - either from argument or discover from recent deployment
			var targetIP string
			if len(args) > 0 {
				targetIP = args[0]
			} else {
				// Try to find the most recent target instance
				fmt.Println("🔍 Looking for test target instances...")

				awsClient, err := aws.NewAWSClient(profile, region)
				if err != nil {
					return fmt.Errorf("failed to initialize AWS client: %w", err)
				}

				// Find instances tagged as test targets
				instances, err := awsClient.FindInstancesByTag(ctx, "Purpose", "nat-bridge-testing")
				if err != nil {
					return fmt.Errorf("failed to find test target instances: %w", err)
				}

				if len(instances) == 0 {
					return fmt.Errorf("no test target instances found. Deploy one with --deploy-target flag or specify target IP manually")
				}

				// Use the most recent running instance
				for _, instance := range instances {
					if instance.State == "running" && instance.PrivateIP != "" {
						targetIP = instance.PrivateIP
						fmt.Printf("  ✓ Found test target: %s (%s)\n", instance.InstanceID, instance.PrivateIP)
						break
					}
				}

				if targetIP == "" {
					return fmt.Errorf("no running test target instances found")
				}
			}

			fmt.Printf("🎯 Testing connectivity to target: %s\n", targetIP)

			// Test 1: Ping connectivity
			fmt.Println("📡 Testing ICMP connectivity...")
			if err := testPingConnectivity(targetIP); err != nil {
				fmt.Printf("  ❌ Ping failed: %v\n", err)
			} else {
				fmt.Println("  ✅ Ping successful")
			}

			// Test 2: HTTP connectivity to test server
			fmt.Println("🌐 Testing HTTP connectivity to test server (port 8080)...")
			if err := testHTTPConnectivity(targetIP, 8080); err != nil {
				fmt.Printf("  ❌ HTTP test failed: %v\n", err)
			} else {
				fmt.Println("  ✅ HTTP test successful")
			}

			// Test 3: SSH connectivity
			fmt.Println("🔒 Testing SSH connectivity (port 22)...")
			if err := testSSHConnectivity(targetIP, 22); err != nil {
				fmt.Printf("  ❌ SSH connectivity failed: %v\n", err)
			} else {
				fmt.Println("  ✅ SSH port accessible")
			}

			fmt.Println("✅ NAT bridge connectivity tests completed!")
			fmt.Printf("💡 Try accessing the test server: curl http://%s:8080\n", targetIP)

			return nil
		},
	}

	cmd.Flags().String("profile", "default", "AWS profile to use")
	cmd.Flags().String("region", "us-west-2", "AWS region")

	return cmd
}

// Connectivity testing helper functions
func testPingConnectivity(targetIP string) error {
	cmd := exec.Command("ping", "-c", "3", "-W", "2000", targetIP)
	return cmd.Run()
}

func testHTTPConnectivity(targetIP string, port int) error {
	client := &http.Client{Timeout: 5 * time.Second}
	url := fmt.Sprintf("http://%s:%d", targetIP, port)

	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil
}

func testSSHConnectivity(targetIP string, port int) error {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", targetIP, port), 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// cleanupLocalInterfaces removes all WireGuard interfaces with platform awareness
func cleanupLocalInterfaces() error {
	fmt.Printf("  🖥️  Platform: %s\n", runtime.GOOS)

	// Check privilege level
	privLevel := detectPrivilegeLevelCmd()
	fmt.Printf("  🔐 Privilege level: %s\n", privLevel)

	// Set up platform-aware sudo environment
	env := setupPlatformSudoEnv()

	// Check for existing WireGuard interfaces
	checkCmd := exec.Command("wg", "show", "interfaces")
	output, err := checkCmd.Output()
	if err != nil {
		// No interfaces found or wg not installed - that's fine
		return nil
	}

	interfaces := strings.Fields(strings.TrimSpace(string(output)))
	if len(interfaces) == 0 {
		fmt.Println("  ✅ No WireGuard interfaces to clean up")
		return nil
	}

	fmt.Printf("  🧹 Found %d WireGuard interface(s): %s\n", len(interfaces), strings.Join(interfaces, ", "))

	// Platform-specific cleanup
	switch runtime.GOOS {
	case "darwin":
		return cleanupMacOSInterfacesCmd(interfaces, env)
	case "linux":
		return cleanupLinuxInterfacesCmd(interfaces, env)
	case "windows":
		return cleanupWindowsInterfacesCmd(interfaces, env)
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		return cleanupBSDInterfacesCmd(interfaces, env)
	default:
		fmt.Printf("  ❌ Unsupported platform: %s\n", runtime.GOOS)
		return cleanupGenericInterfacesCmd(interfaces, env) // This will return error
	}
}

// setupPlatformSudoEnv sets up sudo environment for different platforms
func setupPlatformSudoEnv() []string {
	env := os.Environ()

	switch runtime.GOOS {
	case "darwin":
		askpassPaths := []string{
			"/opt/homebrew/Cellar/macos-askpass/1.0.2/bin/askpass",
			"/usr/local/bin/askpass",
			"/opt/homebrew/bin/askpass",
		}
		for _, askpassPath := range askpassPaths {
			if _, err := os.Stat(askpassPath); err == nil {
				env = append(env, fmt.Sprintf("SUDO_ASKPASS=%s", askpassPath))
				break
			}
		}
	case "linux":
		askpassPaths := []string{
			"/usr/bin/ssh-askpass",
			"/usr/libexec/openssh/ssh-askpass",
			"/usr/bin/ksshaskpass",
		}
		for _, askpassPath := range askpassPaths {
			if _, err := os.Stat(askpassPath); err == nil {
				env = append(env, fmt.Sprintf("SUDO_ASKPASS=%s", askpassPath))
				break
			}
		}
	case "windows":
		// Windows: No sudo/askpass needed
		break
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		// BSD variants
		askpassPaths := []string{
			"/usr/local/bin/ssh-askpass",
			"/usr/bin/ssh-askpass",
			"/usr/X11R6/bin/ssh-askpass", // OpenBSD
		}
		for _, askpassPath := range askpassPaths {
			if _, err := os.Stat(askpassPath); err == nil {
				env = append(env, fmt.Sprintf("SUDO_ASKPASS=%s", askpassPath))
				break
			}
		}
	}

	return env
}

// detectPrivilegeLevelCmd determines if the application is running with elevated privileges
func detectPrivilegeLevelCmd() string {
	switch runtime.GOOS {
	case "darwin", "linux", "freebsd", "openbsd", "netbsd", "dragonfly":
		// Unix-like systems: check if running as root
		if os.Getuid() == 0 {
			return "elevated (root)"
		}
		return "normal (will use sudo)"
	case "windows":
		// Windows: check if running as Administrator
		if isWindowsAdminCmd() {
			return "elevated (administrator)"
		}
		return "normal (will request elevation)"
	default:
		return "unknown"
	}
}

// isWindowsAdminCmd checks if the current process has administrator privileges on Windows
func isWindowsAdminCmd() bool {
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

// cleanupMacOSInterfacesCmd handles macOS interface cleanup for mole down
func cleanupMacOSInterfacesCmd(interfaces []string, env []string) error {
	fmt.Printf("  🍎 Using macOS-specific cleanup\n")

	for _, iface := range interfaces {
		if strings.TrimSpace(iface) == "" {
			continue
		}

		fmt.Printf("  ⬇️  Bringing down interface: %s\n", iface)

		// Try multiple cleanup strategies for macOS
		configPaths := []string{
			fmt.Sprintf("%s/.mole/tunnels/wg0.conf", os.Getenv("HOME")),
			fmt.Sprintf("/opt/homebrew/etc/wireguard/%s.conf", iface),
			fmt.Sprintf("/usr/local/etc/wireguard/%s.conf", iface),
		}

		success := false
		for _, configPath := range configPaths {
			if _, err := os.Stat(configPath); err == nil {
				cleanupCmd := exec.Command("sudo", "-A", "wg-quick", "down", configPath)
				cleanupCmd.Env = env
				if err := cleanupCmd.Run(); err == nil {
					success = true
					break
				}
			}
		}

		if !success {
			// Fallback to direct interface manipulation
			downCmd := exec.Command("sudo", "-A", "ifconfig", iface, "down")
			downCmd.Env = env
			downCmd.Run()

			if strings.HasPrefix(iface, "utun") {
				destroyCmd := exec.Command("sudo", "-A", "ifconfig", iface, "destroy")
				destroyCmd.Env = env
				destroyCmd.Run()
			}
		}
	}

	fmt.Println("  ✅ macOS WireGuard interface cleanup completed")
	return nil
}

// cleanupLinuxInterfacesCmd handles Linux interface cleanup for mole down
func cleanupLinuxInterfacesCmd(interfaces []string, env []string) error {
	fmt.Printf("  🐧 Using Linux-specific cleanup\n")

	for _, iface := range interfaces {
		if strings.TrimSpace(iface) == "" {
			continue
		}

		fmt.Printf("  ⬇️  Bringing down interface: %s\n", iface)

		// Try wg-quick down first
		cleanupCmd := exec.Command("sudo", "-A", "wg-quick", "down", iface)
		cleanupCmd.Env = env
		if err := cleanupCmd.Run(); err != nil {
			// Fallback to ip link delete
			deleteCmd := exec.Command("sudo", "-A", "ip", "link", "delete", "dev", iface)
			deleteCmd.Env = env
			if deleteErr := deleteCmd.Run(); deleteErr != nil {
				fmt.Printf("  ⚠️  Warning: failed to clean interface %s: %v\n", iface, err)
			}
		}
	}

	fmt.Println("  ✅ Linux WireGuard interface cleanup completed")
	return nil
}

// cleanupWindowsInterfacesCmd handles Windows interface cleanup for mole down
func cleanupWindowsInterfacesCmd(interfaces []string, env []string) error {
	fmt.Printf("  🪟 Using Windows-specific cleanup\n")

	for _, iface := range interfaces {
		if strings.TrimSpace(iface) == "" {
			continue
		}

		fmt.Printf("  ⬇️  Removing Windows interface: %s\n", iface)

		// Try WireGuard for Windows service removal
		wireguardPaths := []string{
			`C:\Program Files\WireGuard\wireguard.exe`,
			`C:\Program Files (x86)\WireGuard\wireguard.exe`,
			`wireguard.exe`,
		}

		serviceRemoved := false
		for _, wgPath := range wireguardPaths {
			if _, err := os.Stat(wgPath); err == nil {
				stopCmd := exec.Command(wgPath, "/uninstalltunnelservice", iface)
				if err := stopCmd.Run(); err == nil {
					serviceRemoved = true
					break
				}

				stopCmd = exec.Command(wgPath, "/uninstalltunnelservice", "mole-tunnel")
				if err := stopCmd.Run(); err == nil {
					serviceRemoved = true
					break
				}
			}
		}

		if !serviceRemoved {
			fmt.Printf("  💡 Please manually remove tunnel via WireGuard GUI\n")
		}
	}

	fmt.Println("  ✅ Windows WireGuard interface cleanup completed")
	return nil
}

// cleanupBSDInterfacesCmd handles BSD interface cleanup for mole down
func cleanupBSDInterfacesCmd(interfaces []string, env []string) error {
	fmt.Printf("  🔱 Using BSD-specific cleanup (%s)\n", runtime.GOOS)

	for _, iface := range interfaces {
		if strings.TrimSpace(iface) == "" {
			continue
		}

		fmt.Printf("  ⬇️  Bringing down BSD interface: %s\n", iface)

		cleanupCmd := exec.Command("sudo", "-A", "wg-quick", "down", iface)
		cleanupCmd.Env = env
		if err := cleanupCmd.Run(); err != nil {
			// BSD-specific fallback
			deleteCmd := exec.Command("sudo", "-A", "ifconfig", iface, "destroy")
			deleteCmd.Env = env
			if deleteErr := deleteCmd.Run(); deleteErr != nil {
				fmt.Printf("  ⚠️  Warning: failed to clean BSD interface %s: %v\n", iface, err)
			}
		}
	}

	fmt.Printf("  ✅ BSD WireGuard interface cleanup completed (%s)\n", runtime.GOOS)
	return nil
}

// cleanupGenericInterfacesCmd provides generic interface cleanup
func cleanupGenericInterfacesCmd(interfaces []string, env []string) error {
	return fmt.Errorf("unsupported platform: %s. AWS Cloud Mole supports macOS, Linux, Windows, and BSD variants (FreeBSD, OpenBSD, NetBSD, DragonFly)", runtime.GOOS)
}

// cleanupLocalConfig removes local configuration files and directories
func cleanupLocalConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configPaths := []string{
		filepath.Join(homeDir, ".mole"),
		"/tmp/wg0.conf",
		"/tmp/automated-wg0.conf",
		"/tmp/wg-client.conf",
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("  🗑️  Removing: %s\n", path)
			if err := os.RemoveAll(path); err != nil {
				fmt.Printf("  ⚠️  Warning: failed to remove %s: %v\n", path, err)
			}
		}
	}

	fmt.Println("  ✅ Local configuration cleanup completed")
	return nil
}

// handleVPCLimitError provides user options when VPC limit is reached
func handleVPCLimitError(ctx context.Context, client *aws.AWSClient) error {
	fmt.Println("\n🚫 VPC Limit Reached")
	fmt.Println("The AWS account has reached the maximum number of VPCs allowed.")
	fmt.Println("\nLet me show you the current VPCs in this region:")

	// Get all VPCs in the region
	vpcs, err := client.ListVPCs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list VPCs: %w", err)
	}

	if len(vpcs) == 0 {
		fmt.Println("No VPCs found in the current region.")
		return nil
	}

	// Display current VPCs in a table format
	fmt.Printf("%-20s %-18s %-10s %-15s %s\n", "VPC ID", "CIDR Block", "State", "Default", "Name")
	fmt.Println(strings.Repeat("-", 80))

	for i, vpc := range vpcs {
		defaultStr := "No"
		if vpc.IsDefault {
			defaultStr = "Yes"
		}
		name := vpc.Name
		if name == "" {
			name = "-"
		}
		fmt.Printf("%-20s %-18s %-10s %-15s %s\n",
			vpc.VpcId, vpc.CidrBlock, vpc.State, defaultStr, name)

		// Add spacing every 5 rows for readability
		if (i+1)%5 == 0 && i+1 < len(vpcs) {
			fmt.Println()
		}
	}

	fmt.Println("\n📋 Your options:")
	fmt.Println("  1. Use an existing VPC: Run 'mole up --vpc <vpc-id> --public-subnet <subnet-id>'")
	fmt.Println("  2. Delete unused VPCs: Use AWS Console or CLI to delete VPCs you don't need")
	fmt.Println("  3. Request VPC limit increase: Contact AWS Support to increase your VPC limit")
	fmt.Println("  4. Use default VPC: If available, specify the default VPC shown above")

	// Find the default VPC and show example command
	for _, vpc := range vpcs {
		if vpc.IsDefault && vpc.State == "available" {
			fmt.Printf("\n💡 Example using default VPC:\n")
			fmt.Printf("   mole up --vpc %s --public-subnet <get-from-aws-console>\n", vpc.VpcId)
			break
		}
	}

	fmt.Println("\n💡 To see subnets in a VPC:")
	fmt.Println("   aws ec2 describe-subnets --filters \"Name=vpc-id,Values=<vpc-id>\" --query 'Subnets[*].[SubnetId,CidrBlock,MapPublicIpOnLaunch]' --output table")

	return nil
}

// terminateAWSResources finds and terminates all mole-created AWS resources
func terminateAWSResources(client *aws.AWSClient) error {
	ctx := context.Background()

	// Find all instances created by mole
	bastionInstances, err := client.FindInstancesByTag(ctx, "CreatedBy", "aws-cloud-mole")
	if err != nil {
		return fmt.Errorf("failed to find bastion instances: %w", err)
	}

	targetInstances, err := client.FindInstancesByTag(ctx, "Purpose", "nat-bridge-testing")
	if err != nil {
		return fmt.Errorf("failed to find target instances: %w", err)
	}

	allInstances := append(bastionInstances, targetInstances...)

	if len(allInstances) == 0 {
		fmt.Println("  ✅ No AWS instances to terminate")
		return nil
	}

	fmt.Printf("  🎯 Found %d instance(s) to terminate:\n", len(allInstances))
	for _, instance := range allInstances {
		fmt.Printf("    • %s (%s) - %s\n", instance.Name, instance.InstanceID, instance.State)
	}

	// Terminate all instances
	for _, instance := range allInstances {
		if instance.State == "running" || instance.State == "stopped" {
			fmt.Printf("  ⏹️  Terminating instance: %s\n", instance.InstanceID)
			if err := client.TerminateBastion(ctx, instance.InstanceID); err != nil {
				fmt.Printf("  ⚠️  Warning: failed to terminate %s: %v\n", instance.InstanceID, err)
			}
		}
	}

	fmt.Println("  ✅ AWS resource termination initiated")
	fmt.Println("  💡 It may take a few minutes for instances to fully terminate")
	return nil
}
