# AWS Cloud Mole

[![CI](https://github.com/research-computing/mole/actions/workflows/ci.yml/badge.svg)](https://github.com/research-computing/mole/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/research-computing/mole)](https://goreportcard.com/report/github.com/research-computing/mole)
[![Release](https://img.shields.io/github/release/research-computing/mole.svg)](https://github.com/research-computing/mole/releases/latest)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

AWS Cloud Mole (`mole`) is a high-performance tunneling tool that creates optimized WireGuard connections between on-premises systems (particularly Data Transfer Nodes in academic research environments) and AWS private subnets. The tool automatically provisions AWS infrastructure, performs network performance discovery, and dynamically scales tunnel capacity to handle multi-gigabit elephant flows.

## Features

- 🚀 **Single command deployment and teardown**
- 🔍 **Automatic network performance discovery and optimization**
- ⚖️ **Dynamic scaling from single-tunnel to multi-instance architectures**
- 💰 **Cost optimization with Graviton instance selection**
- 🎓 **Academic research workflow integration**
- 📊 **Real-time monitoring and performance dashboards**
- 🔒 **Secure WireGuard tunnels with automatic key management**

## Quick Start

### Installation

#### Homebrew (macOS/Linux)
```bash
brew tap research-computing/aws-cloud-mole
brew install aws-cloud-mole
```

#### Linux Package Managers
```bash
# Debian/Ubuntu
wget https://github.com/research-computing/mole/releases/latest/download/aws-cloud-mole_amd64.deb
sudo dpkg -i aws-cloud-mole_amd64.deb

# Red Hat/CentOS/Fedora
wget https://github.com/research-computing/mole/releases/latest/download/aws-cloud-mole_amd64.rpm
sudo rpm -i aws-cloud-mole_amd64.rpm
```

#### Go Install
```bash
go install github.com/research-computing/mole/cmd/mole@latest
```

### Basic Usage

```bash
# Initialize with AWS credentials
mole init --profile research-aws --region us-west-2

# Discover network performance characteristics
mole probe --region us-west-2 --save-results

# Deploy optimized tunnel
mole up --vpc vpc-12345 --auto-optimize

# Monitor performance
mole monitor --dashboard

# Scale tunnel count
mole scale --tunnels 8

# Tear down
mole down
```

## Architecture

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

## Performance

- **Single tunnel**: 1.5 Gbps (WireGuard measured limit)
- **4 tunnels**: 5-6 Gbps aggregate throughput
- **8 tunnels**: 8-10 Gbps aggregate throughput
- **Scaling efficiency**: >75% with proper CPU pinning

## Configuration

### DTN (Data Transfer Node) Configuration

For high-performance academic environments:

```bash
mole up --config dtn --vpc vpc-12345 --tunnels 8 --mptcp
```

Includes optimizations for:
- Jumbo frames (9000 MTU)
- SR-IOV support
- BBR congestion control
- Large TCP buffers
- Multi-queue networking

### Custom Configuration

Create custom profiles:

```bash
mole create-profile research-cluster --vpc vpc-12345 --tunnels 6
mole connect research-cluster
```

## Commands

| Command | Description |
|---------|-------------|
| `mole init` | Initialize AWS credentials and configuration |
| `mole probe` | Perform network performance discovery |
| `mole up` | Deploy tunnel with automatic optimization |
| `mole multi-up` | Deploy multi-tunnel configuration with MPTCP |
| `mole status` | Show current tunnel status |
| `mole monitor` | Real-time monitoring dashboard |
| `mole scale` | Scale tunnel count |
| `mole optimize` | Apply performance recommendations |
| `mole create-profile` | Create saved tunnel profile |
| `mole connect` | Connect using saved profile |
| `mole down` | Tear down tunnel and infrastructure |

## Monitoring

Real-time monitoring capabilities:

```bash
# htop-style dashboard
mole monitor --dashboard

# Network traffic monitoring
mole monitor --traffic

# Performance metrics
mole monitor --performance

# Latency tracking
mole monitor --latency
```

## Requirements

### System Requirements
- Linux or macOS
- WireGuard kernel module or userspace implementation
- Go 1.22+ (for building from source)

### AWS Requirements
- AWS CLI configured with appropriate permissions
- VPC with private subnets
- EC2 permissions for instance management
- VPC permissions for security group and route management

### Optional Dependencies
- `iperf3` for bandwidth testing
- `ethtool` for interface optimization
- `htop`/`iotop` for system monitoring

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.

## Security

For security issues, please see [SECURITY.md](SECURITY.md).

## Support

- 📖 [Documentation](docs/)
- 🐛 [Issue Tracker](https://github.com/research-computing/mole/issues)
- 💬 [Discussions](https://github.com/research-computing/mole/discussions)