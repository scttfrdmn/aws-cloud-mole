# AWS Cloud Mole Infrastructure
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
  region  = var.aws_region
  profile = var.aws_profile

  default_tags {
    tags = {
      Project     = "aws-cloud-mole"
      Environment = var.environment
      ManagedBy   = "terraform"
    }
  }
}

# Data sources
data "aws_vpc" "target" {
  id = var.vpc_id
}

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-22.04-lts-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "architecture"
    values = [var.instance_architecture]
  }
}

# Security Group for WireGuard
resource "aws_security_group" "wireguard" {
  name_prefix = "mole-wireguard-"
  vpc_id      = var.vpc_id

  description = "Security group for AWS Cloud Mole WireGuard bastion"

  # WireGuard ports (51820-51827 for up to 8 tunnels)
  dynamic "ingress" {
    for_each = range(var.tunnel_count)
    content {
      from_port   = 51820 + ingress.value
      to_port     = 51820 + ingress.value
      protocol    = "udp"
      cidr_blocks = [var.allowed_cidr]
      description = "WireGuard tunnel ${ingress.value}"
    }
  }

  # SSH for management
  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.allowed_cidr]
    description = "SSH management"
  }

  # All outbound traffic
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
    description = "All outbound traffic"
  }

  tags = {
    Name = "mole-wireguard-sg"
  }

  lifecycle {
    create_before_destroy = true
  }
}

# IAM role for the bastion instance
resource "aws_iam_role" "bastion" {
  name_prefix = "mole-bastion-"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "ec2.amazonaws.com"
        }
      }
    ]
  })

  tags = {
    Name = "mole-bastion-role"
  }
}

# IAM instance profile for the bastion
resource "aws_iam_instance_profile" "bastion" {
  name_prefix = "mole-bastion-"
  role        = aws_iam_role.bastion.name

  tags = {
    Name = "mole-bastion-profile"
  }
}

# Policy for CloudWatch and Systems Manager access
resource "aws_iam_role_policy" "bastion" {
  name_prefix = "mole-bastion-"
  role        = aws_iam_role.bastion.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "cloudwatch:PutMetricData",
          "ec2:DescribeVolumes",
          "ec2:DescribeTags",
          "logs:PutLogEvents",
          "logs:CreateLogGroup",
          "logs:CreateLogStream"
        ]
        Resource = "*"
      },
      {
        Effect = "Allow"
        Action = [
          "ssm:GetParameter",
          "ssm:PutParameter",
          "ssm:GetParameters"
        ]
        Resource = "arn:aws:ssm:*:*:parameter/mole/*"
      }
    ]
  })
}

# Key pair for SSH access
resource "aws_key_pair" "mole" {
  key_name   = "mole-${random_id.suffix.hex}"
  public_key = var.ssh_public_key

  tags = {
    Name = "mole-keypair"
  }
}

# Random suffix for unique naming
resource "random_id" "suffix" {
  byte_length = 4
}

# User data script for bastion setup
locals {
  user_data = base64encode(templatefile("${path.module}/../scripts/bastion-init.sh", {
    tunnel_count = var.tunnel_count
    mtu_size     = var.mtu_size
  }))
}

# Bastion EC2 instance
resource "aws_instance" "bastion" {
  ami                         = data.aws_ami.ubuntu.id
  instance_type               = var.instance_type
  key_name                    = aws_key_pair.mole.key_name
  vpc_security_group_ids      = [aws_security_group.wireguard.id]
  subnet_id                   = var.subnet_id
  iam_instance_profile        = aws_iam_instance_profile.bastion.name
  user_data                   = local.user_data
  associate_public_ip_address = true

  # Enhanced networking for better performance
  ena_support    = true
  sriov_net_support = "simple"

  # Root volume configuration
  root_block_device {
    volume_type = "gp3"
    volume_size = 20
    encrypted   = true

    tags = {
      Name = "mole-bastion-root"
    }
  }

  # Detailed monitoring for CloudWatch
  monitoring = true

  tags = {
    Name = "mole-bastion"
    Role = "wireguard-server"
  }

  lifecycle {
    create_before_destroy = true
  }
}

# Elastic IP for consistent connectivity
resource "aws_eip" "bastion" {
  domain   = "vpc"
  instance = aws_instance.bastion.id

  tags = {
    Name = "mole-bastion-eip"
  }

  depends_on = [aws_instance.bastion]
}

# Route table for tunnel traffic (if needed)
resource "aws_route_table" "tunnel" {
  count  = var.create_route_table ? 1 : 0
  vpc_id = var.vpc_id

  tags = {
    Name = "mole-tunnel-routes"
  }
}

# Route table association (if route table created)
resource "aws_route_table_association" "tunnel" {
  count          = var.create_route_table ? 1 : 0
  subnet_id      = var.subnet_id
  route_table_id = aws_route_table.tunnel[0].id
}