# AWS Cloud Mole Terraform Outputs

output "bastion_instance_id" {
  description = "ID of the bastion EC2 instance"
  value       = aws_instance.bastion.id
}

output "bastion_public_ip" {
  description = "Public IP address of the bastion"
  value       = aws_eip.bastion.public_ip
}

output "bastion_private_ip" {
  description = "Private IP address of the bastion"
  value       = aws_instance.bastion.private_ip
}

output "bastion_instance_type" {
  description = "Instance type of the bastion"
  value       = aws_instance.bastion.instance_type
}

output "security_group_id" {
  description = "ID of the WireGuard security group"
  value       = aws_security_group.wireguard.id
}

output "ssh_key_name" {
  description = "Name of the SSH key pair"
  value       = aws_key_pair.mole.key_name
}

output "tunnel_ports" {
  description = "WireGuard tunnel ports"
  value       = [for i in range(var.tunnel_count) : 51820 + i]
}

output "connection_info" {
  description = "Connection information for the tunnels"
  value = {
    endpoint    = aws_eip.bastion.public_ip
    ports      = [for i in range(var.tunnel_count) : 51820 + i]
    tunnel_ips = [for i in range(var.tunnel_count) : "10.100.${i + 1}.1/24"]
  }
}

output "instance_performance" {
  description = "Performance characteristics of selected instance"
  value = {
    baseline_bandwidth_mbps = local.selected_instance.baseline_bandwidth
    burst_bandwidth_mbps   = local.selected_instance.burst_bandwidth
    max_tunnels           = local.selected_instance.max_tunnels
    monthly_cost_usd      = local.selected_instance.monthly_cost
  }
}

output "cost_estimate" {
  description = "Cost estimates for the deployment"
  value = {
    hourly_cost  = local.selected_instance.monthly_cost / 730
    daily_cost   = local.selected_instance.monthly_cost / 30.4
    monthly_cost = local.selected_instance.monthly_cost
  }
}

output "ssh_command" {
  description = "SSH command to connect to the bastion"
  value       = "ssh -i ~/.ssh/mole-key ubuntu@${aws_eip.bastion.public_ip}"
}

output "infrastructure_ready" {
  description = "Indicates if infrastructure is ready for tunnel setup"
  value       = aws_instance.bastion.instance_state == "running"
}