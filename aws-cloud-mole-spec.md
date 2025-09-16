# AWS Cloud Mole: Complete Project Specification

## Project Overview

AWS Cloud Mole (`mole`) is a high-performance tunneling tool that creates optimized WireGuard connections between on-premises systems (particularly Data Transfer Nodes in academic research environments) and AWS private subnets. The tool automatically provisions AWS infrastructure, performs network performance discovery, and dynamically scales tunnel capacity to handle multi-gigabit elephant flows.

### Core Value Proposition
- Single command deployment and teardown
- Automatic network performance discovery and optimization
- Dynamic scaling from single-tunnel to multi-instance architectures
- Cost optimization with Graviton instance selection
- Academic research workflow integration

## Technical Architecture

### System Components

```
┌─────────────────┐         ┌─────────────────┐         ┌─────────────────┐
│   Local DTN     │         │   AWS Bastion   │         │ Private Subnet  │
│                 │         │                 │         │                 │
│ ┌─────────────┐ │  Multi  │ ┌─────────────┐ │  NAT    │ ┌─────────────┐ │
│ │    mole     │◄┼─Tunnel──┼►│ WireGuard   │◄┼─────────┼►│ Research    │ │
│ │   client    │ │ WG+ECMP │ │   Server    │ │ Forward │ │ Resources   │ │
│ └─────────────┘ │         │ └─────────────┘ │         │ └─────────────┘ │
│                 │         │                 │         │                 │
└─────────────────┘         └─────────────────┘         └─────────────────┘
```

### Scaling Phases

1. **Phase 1: Vertical Scaling** - Single instance, 1-8 WireGuard tunnels
2. **Phase 2: Horizontal Scaling** - Multiple instances across AZs

## Project Structure

```
mole/
├── cmd/
│   └── mole/
│       └── main.go                 # CLI entry point
├── internal/
│   ├── aws/
│   │   ├── client.go              # AWS SDK integration
│   │   ├── instances.go           # EC2 instance management
│   │   └── networking.go          # VPC, security groups, routing
│   ├── tunnel/
│   │   ├── manager.go             # Tunnel lifecycle management
│   │   ├── wireguard.go           # WireGuard configuration
│   │   ├── scaling.go             # Dynamic tunnel scaling
│   │   └── affinity.go            # CPU process pinning
│   ├── network/
│   │   ├── prober.go              # Network performance discovery
│   │   ├── analyzer.go            # Bottleneck identification
│   │   └── optimizer.go           # Configuration optimization
│   ├── cluster/
│   │   ├── manager.go             # Multi-instance orchestration
│   │   ├── phases.go              # Scaling phase management
│   │   └── graviton.go            # Instance type optimization
│   ├── mptcp/
│   │   ├── manager.go             # MPTCP configuration
│   │   ├── scheduler.go           # Path scheduling algorithms
│   │   └── monitoring.go          # Subflow performance tracking
│   ├── config/
│   │   ├── config.go              # Configuration management
│   │   ├── profiles.go            # Saved tunnel profiles
│   │   └── validation.go          # Config validation
│   └── monitoring/
│       ├── metrics.go             # Performance metrics collection
│       ├── health.go              # Tunnel health checking
│       └── reporting.go           # Status reporting
├── pkg/
│   ├── terraform/
│   │   ├── main.tf                # Infrastructure as Code
│   │   ├── variables.tf           # Terraform variables
│   │   └── outputs.tf             # Terraform outputs
│   └── scripts/
│       ├── bastion-init.sh        # EC2 user data script
│       ├── dtn-optimize.sh        # DTN performance tuning
│       └── tunnel-setup.sh        # WireGuard tunnel configuration
├── configs/
│   ├── default.yaml               # Default configuration
│   ├── dtn.yaml                   # DTN-optimized configuration
│   └── research.yaml              # Academic research presets
├── docs/
│   ├── README.md                  # User documentation
│   ├── ARCHITECTURE.md            # Technical architecture
│   ├── PERFORMANCE.md             # Performance guide
│   └── TROUBLESHOOTING.md         # Common issues and solutions
├── go.mod
├── go.sum
├── Makefile                       # Build and test automation
└── .goreleaser.yml               # Release automation
```

## Core Components Specification

### 1. Network Prober (`internal/network/prober.go`)

**Purpose**: Automatically discover network performance characteristics during deployment.

**Key Functions**:
```go
type NetworkProber struct {
    config *ProbeConfig
    results *ProbeResults
}

func (np *NetworkProber) ProbeNetwork(ctx context.Context, targetRegion string) (*ProbeResults, error)
func (np *NetworkProber) discoverLocalInterface() error
func (np *NetworkProber) discoverOptimalMTU(ctx context.Context, region string) error
func (np *NetworkProber) measureRegionLatencies(ctx context.Context) error
func (np *NetworkProber) discoverBandwidthCapacity(ctx context.Context, region string) error
func (np *NetworkProber) identifyBottlenecks()
func (np *NetworkProber) generateRecommendations()
```

**Required Tests**:
- MTU discovery (1200, 1500, 1800, 9000 bytes)
- Single-stream baseline bandwidth
- Multi-stream bandwidth scaling (1, 2, 4, 8 streams)
- Latency to AWS regions
- Local interface capability detection

**Output**: Optimal MTU, recommended tunnel count, instance sizing, bottleneck identification

### 2. Tunnel Manager (`internal/tunnel/manager.go`)

**Purpose**: Manage WireGuard tunnel lifecycle with dynamic scaling.

**Key Functions**:
```go
type TunnelManager struct {
    config *TunnelConfig
    tunnels map[int]*WireGuardTunnel
    scaler *TunnelScaler
}

func (tm *TunnelManager) CreateTunnels(count int) error
func (tm *TunnelManager) AddTunnel() error
func (tm *TunnelManager) RemoveTunnel() error
func (tm *TunnelManager) ConfigureECMP() error
func (tm *TunnelManager) GetActiveTunnels() ([]TunnelStatus, error)
func (tm *TunnelManager) MonitorPerformance() <-chan TunnelMetrics
```

**Scaling Logic**:
- Scale up: Utilization > 80% OR elephant flows detected
- Scale down: Utilization < 30% for 10+ minutes
- Immediate scaling: Large flows (>100MB) detected
- Max tunnels per instance: 8
- CPU core pinning for each tunnel process

### 3. Cluster Manager (`internal/cluster/manager.go`)

**Purpose**: Orchestrate single-instance and multi-instance scaling.

**Scaling Phases**:
1. **Vertical**: Add tunnels to single instance (1-8 tunnels)
2. **Horizontal**: Add instances when single instance saturated

**Key Functions**:
```go
type ClusterManager struct {
    currentPhase ScalingPhase
    instances map[string]*InstanceManager
}

func (cm *ClusterManager) ScaleUp(utilization float64) error
func (cm *ClusterManager) scaleVertical(utilization float64) error
func (cm *ClusterManager) scaleHorizontal(utilization float64) error
func (cm *ClusterManager) addTunnelWithPinning(instance *InstanceManager, tunnelID int) error
```

### 4. AWS Client (`internal/aws/client.go`)

**Purpose**: Manage AWS resources with cost optimization.

**Key Functions**:
```go
type AWSClient struct {
    profile string
    region string
    client *ec2.Client
}

func (a *AWSClient) CreateBastion(config *BastionConfig) (*BastionInfo, error)
func (a *AWSClient) CreateSecurityGroups(vpcID string, tunnelCount int) (string, error)
func (a *AWSClient) SelectOptimalInstance(throughput int64, budget float64) *InstanceConfig
func (a *AWSClient) DeployInfrastructure(terraformConfig *TerraformConfig) error
```

**Graviton Instance Selection**:
- t4g.nano: $1.31/month, 32 Mbps baseline, 5 Gbps burst
- t4g.small: $5.26/month, 125 Mbps baseline, 5 Gbps burst  
- c6gn.medium: $27/month, 3.125 Gbps sustained
- c6gn.large: $54/month, 6.25 Gbps sustained
- c6gn.xlarge: $108/month, 12.5 Gbps sustained

### 5. MPTCP Integration (`internal/mptcp/manager.go`)

**Purpose**: Enable bandwidth aggregation across multiple tunnels for single connections.

**Scheduling Strategies**:
- `roundrobin`: Even distribution for bandwidth aggregation
- `redundant`: Send on all paths for minimum latency
- `adaptive`: AI-driven path selection based on performance

**Key Functions**:
```go
type MPTCPManager struct {
    scheduler string
    subflows map[int]*SubflowMetrics
}

func (mm *MPTCPManager) EnableMPTCP() error
func (mm *MPTCPManager) ConfigureScheduler(strategy string) error
func (mm *MPTCPManager) MonitorSubflows() <-chan SubflowMetrics
```

## Configuration Schema

### Default Configuration (`configs/default.yaml`)

```yaml
# Tunnel Configuration
tunnel:
  min_tunnels: 1
  max_tunnels: 8
  base_cidr: "10.100.0.0/16"
  mtu: 1420

# Scaling Configuration  
scaling:
  scale_up_threshold: 0.80
  scale_down_threshold: 0.30
  scale_interval: "30s"
  scale_up_cooldown: "2m"
  scale_down_cooldown: "10m"
  elephant_flow_threshold: 104857600  # 100MB

# Network Probing
probing:
  test_duration: "30s"
  mtu_range: [1200, 1500, 1800, 9000]
  parallel_streams: [1, 2, 4, 8]
  enable_jumbo_frames: true

# Process Optimization
optimization:
  enable_process_pinning: true
  reserve_cores: 2
  cpu_governor: "performance"
  tcp_congestion: "bbr"

# AWS Configuration
aws:
  instance_types: ["t4g.nano", "t4g.small", "c6gn.medium", "c6gn.large"]
  max_instances: 4
  availability_zones: ["us-west-2a", "us-west-2b", "us-west-2c"]

# MPTCP Configuration
mptcp:
  enable: false
  scheduler: "roundrobin"  # roundrobin, redundant, adaptive
  path_manager: "fullmesh"
```

### DTN Configuration (`configs/dtn.yaml`)

```yaml
# Data Transfer Node optimizations
extends: default

optimization:
  enable_sriov: true
  large_mtu: 9000
  multi_queue_nic: true
  tcp_window_scaling: true
  buffer_sizes:
    rmem_max: 134217728  # 128MB
    wmem_max: 134217728  # 128MB

tunnel:
  max_tunnels: 12  # More tunnels for DTN environments

mptcp:
  enable: true
  scheduler: "roundrobin"  # Bandwidth aggregation for large transfers
```

## CLI Interface Specification

### Primary Commands

```bash
# Initialize with AWS credentials
mole init --profile research-aws --region us-west-2

# Network performance discovery
mole probe --region us-west-2 --save-results

# Deploy with automatic optimization
mole up --vpc vpc-12345 --auto-optimize --profile research-cluster

# Deploy with specific configuration
mole up --vpc vpc-12345 --tunnels 4 --instance-type c6gn.medium

# Multi-tunnel deployment with MPTCP
mole multi-up --tunnels 6 --strategy adaptive --mptcp

# Monitor active tunnels
mole status
mole monitor  # Real-time dashboard

# Scaling operations
mole scale --tunnels 8
mole optimize  # Apply performance recommendations

# Profile management
mole create-profile research-vpc --vpc vpc-12345 --tunnels 4
mole connect research-vpc

# Teardown
mole down
mole down --force --cleanup-all
```

### Command Implementations

```go
// cmd/mole/main.go
func main() {
    rootCmd := &cobra.Command{
        Use:   "mole",
        Short: "High-performance AWS tunneling for research workloads",
    }
    
    rootCmd.AddCommand(
        initCmd(),
        probeCmd(),
        upCmd(),
        multiUpCmd(),
        statusCmd(),
        monitorCmd(),
        scaleCmd(),
        optimizeCmd(),
        createProfileCmd(),
        connectCmd(),
        downCmd(),
    )
    
    if err := rootCmd.Execute(); err != nil {
        log.Fatal(err)
    }
}
```

## Performance Requirements

### Throughput Targets
- Single tunnel: 1.5 Gbps (measured WireGuard limit)
- 4 tunnels: 5-6 Gbps aggregate
- 8 tunnels: 8-10 Gbps aggregate
- Scaling efficiency: >75% with proper CPU pinning

### Latency Requirements
- Tunnel establishment: <60 seconds including AWS provisioning
- Scaling operations: <30 seconds per tunnel
- Failover time: <1 second with MPTCP

### Resource Utilization
- CPU overhead: <15% per tunnel on modern cores
- Memory usage: <100MB base + 10MB per tunnel
- Network overhead: <5% (WireGuard efficiency)

## Implementation Priorities

### Phase 1: Core Functionality
1. Network probing system
2. Single tunnel deployment
3. AWS infrastructure automation
4. Basic CLI interface

### Phase 2: Multi-Tunnel Support  
1. ECMP routing configuration
2. Dynamic tunnel scaling
3. Process pinning and CPU optimization
4. Performance monitoring

### Phase 3: Advanced Features
1. Multi-instance scaling
2. MPTCP integration
3. DTN optimizations
4. Advanced monitoring and alerting

### Phase 4: Production Features
1. High availability configurations
2. Automated backup/recovery
3. Cost optimization algorithms
4. Integration with research workflow tools

## Testing Strategy

### Unit Tests
- Network probing algorithms
- Tunnel configuration generation
- AWS resource management
- ECMP routing setup

### Integration Tests
- End-to-end tunnel establishment
- Multi-tunnel scaling scenarios
- AWS infrastructure provisioning
- Performance under load

### Performance Tests
- Single tunnel throughput benchmarks
- Multi-tunnel scaling efficiency
- Latency and jitter measurements
- Long-duration stability tests

## Dependencies

### Go Modules
```go
module github.com/research-computing/mole

require (
    github.com/aws/aws-sdk-go-v2 v1.25.0
    github.com/aws/aws-sdk-go-v2/config v1.27.0
    github.com/aws/aws-sdk-go-v2/service/ec2 v1.147.0
    github.com/spf13/cobra v1.8.0
    github.com/spf13/viper v1.18.2
    github.com/vishvananda/netlink v1.1.0
    golang.org/x/crypto v0.21.0
    golang.zx2c4.com/wireguard/wgctrl v0.0.0-20230429144221-925a1e7659e6
    github.com/prometheus/client_golang v1.19.0
)
```

### System Dependencies
- WireGuard kernel module or userspace implementation
- iproute2 tools for routing configuration
- iptables for traffic shaping
- ethtool for interface optimization
- Terraform for infrastructure provisioning

### Optional Dependencies
- MPTCP kernel support (Linux 5.6+)
- iperf3 for bandwidth testing
- tcpdump for traffic analysis
- htop/iotop for resource monitoring

## Security Considerations

### Key Management
- Automatic WireGuard key generation and rotation
- Secure key exchange via AWS Systems Manager
- No hardcoded credentials in configuration

### Network Security
- Minimal security group rules (only required WireGuard ports)
- Source IP restrictions where possible
- Regular security group auditing

### Access Control
- AWS IAM integration for authentication
- Least-privilege EC2 instance roles
- VPC-only deployment by default

## Error Handling and Recovery

### Graceful Degradation
- Continue operation with reduced tunnel count on failures
- Automatic retry with exponential backoff
- Fallback to single tunnel if multi-tunnel fails

### Recovery Scenarios
- Tunnel failure: Automatic recreation and traffic redistribution
- Instance failure: Rapid instance replacement (horizontal scaling)
- Network partition: Graceful reconnection when connectivity restored

### Monitoring and Alerting
- Health checks for all tunnels
- Performance threshold alerts
- Automatic scaling based on utilization
- Cost monitoring and budget alerts

This specification provides the complete technical foundation for implementing AWS Cloud Mole using Claude Code. The project emphasizes performance, automation, and user experience while maintaining focus on the academic research computing use case.