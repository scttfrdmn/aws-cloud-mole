# AWS Cloud Mole Terraform Variables

variable "aws_region" {
  description = "AWS region for deployment"
  type        = string
  default     = "us-west-2"
}

variable "aws_profile" {
  description = "AWS CLI profile to use"
  type        = string
  default     = "default"
}

variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "dev"
}

variable "vpc_id" {
  description = "VPC ID where bastion will be deployed"
  type        = string
  validation {
    condition     = can(regex("^vpc-[a-f0-9]{8,17}$", var.vpc_id))
    error_message = "VPC ID must be a valid format (vpc-xxxxxxxx)."
  }
}

variable "subnet_id" {
  description = "Subnet ID for bastion instance (must be public)"
  type        = string
  validation {
    condition     = can(regex("^subnet-[a-f0-9]{8,17}$", var.subnet_id))
    error_message = "Subnet ID must be a valid format (subnet-xxxxxxxx)."
  }
}

variable "instance_type" {
  description = "EC2 instance type for bastion"
  type        = string
  default     = "t4g.small"

  validation {
    condition = contains([
      "t4g.nano", "t4g.micro", "t4g.small", "t4g.medium",
      "c6gn.medium", "c6gn.large", "c6gn.xlarge", "c6gn.2xlarge"
    ], var.instance_type)
    error_message = "Instance type must be a supported Graviton instance."
  }
}

variable "instance_architecture" {
  description = "EC2 instance architecture"
  type        = string
  default     = "arm64"

  validation {
    condition     = contains(["arm64", "x86_64"], var.instance_architecture)
    error_message = "Architecture must be either arm64 or x86_64."
  }
}

variable "tunnel_count" {
  description = "Number of WireGuard tunnels to configure"
  type        = number
  default     = 1

  validation {
    condition     = var.tunnel_count >= 1 && var.tunnel_count <= 8
    error_message = "Tunnel count must be between 1 and 8."
  }
}

variable "mtu_size" {
  description = "MTU size for WireGuard interfaces"
  type        = number
  default     = 1420

  validation {
    condition     = var.mtu_size >= 1200 && var.mtu_size <= 9000
    error_message = "MTU size must be between 1200 and 9000."
  }
}

variable "allowed_cidr" {
  description = "CIDR block allowed to connect to WireGuard"
  type        = string
  default     = "0.0.0.0/0"

  validation {
    condition     = can(cidrhost(var.allowed_cidr, 0))
    error_message = "Must be a valid CIDR block."
  }
}

variable "ssh_public_key" {
  description = "SSH public key for bastion access"
  type        = string
}

variable "create_route_table" {
  description = "Whether to create a custom route table"
  type        = bool
  default     = false
}

# Graviton instance configurations
locals {
  graviton_instances = {
    "t4g.nano" = {
      baseline_bandwidth = 32    # Mbps
      burst_bandwidth   = 5000   # Mbps
      monthly_cost     = 1.31    # USD
      max_tunnels      = 1
    }
    "t4g.micro" = {
      baseline_bandwidth = 62
      burst_bandwidth   = 5000
      monthly_cost     = 2.63
      max_tunnels      = 2
    }
    "t4g.small" = {
      baseline_bandwidth = 125
      burst_bandwidth   = 5000
      monthly_cost     = 5.26
      max_tunnels      = 4
    }
    "t4g.medium" = {
      baseline_bandwidth = 250
      burst_bandwidth   = 5000
      monthly_cost     = 10.52
      max_tunnels      = 6
    }
    "c6gn.medium" = {
      baseline_bandwidth = 3125
      burst_bandwidth   = 12500
      monthly_cost     = 27.00
      max_tunnels      = 6
    }
    "c6gn.large" = {
      baseline_bandwidth = 6250
      burst_bandwidth   = 25000
      monthly_cost     = 54.00
      max_tunnels      = 8
    }
    "c6gn.xlarge" = {
      baseline_bandwidth = 12500
      burst_bandwidth   = 25000
      monthly_cost     = 108.00
      max_tunnels      = 12
    }
  }

  selected_instance = local.graviton_instances[var.instance_type]
}