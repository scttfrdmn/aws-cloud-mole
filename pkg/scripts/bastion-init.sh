#!/bin/bash
# AWS Cloud Mole Bastion Initialization Script
set -euo pipefail

# Variables from Terraform
TUNNEL_COUNT=${tunnel_count}
MTU_SIZE=${mtu_size}

# Logging
exec > >(tee /var/log/mole-init.log)
exec 2>&1

echo "🚀 Starting AWS Cloud Mole bastion initialization..."
echo "   Tunnel Count: $TUNNEL_COUNT"
echo "   MTU Size: $MTU_SIZE"

# Update system
echo "📦 Updating system packages..."
apt-get update -y
apt-get upgrade -y

# Install required packages
echo "📦 Installing required packages..."
apt-get install -y \
    wireguard \
    wireguard-tools \
    iptables \
    iptables-persistent \
    htop \
    iotop \
    nethogs \
    tcpdump \
    iperf3 \
    jq \
    curl \
    wget \
    awscli

# Enable IP forwarding
echo "🔄 Enabling IP forwarding..."
echo 'net.ipv4.ip_forward=1' >> /etc/sysctl.conf
echo 'net.ipv6.conf.all.forwarding=1' >> /etc/sysctl.conf
sysctl -p

# Optimize network stack for high performance
echo "⚡ Optimizing network stack..."
cat >> /etc/sysctl.conf << EOF

# AWS Cloud Mole Network Optimizations
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.ipv4.tcp_rmem = 4096 87380 134217728
net.ipv4.tcp_wmem = 4096 65536 134217728
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_window_scaling = 1
net.ipv4.tcp_congestion_control = bbr

# Reduce swappiness for better performance
vm.swappiness = 10
EOF

sysctl -p

# Create WireGuard directory structure
echo "📁 Creating WireGuard directories..."
mkdir -p /etc/wireguard
mkdir -p /etc/mole/{keys,configs,logs}

# Generate WireGuard keys for each tunnel
echo "🔑 Generating WireGuard keys..."
for i in $(seq 0 $((TUNNEL_COUNT - 1))); do
    wg genkey | tee /etc/mole/keys/wg${i}_private.key | wg pubkey > /etc/mole/keys/wg${i}_public.key
    chmod 600 /etc/mole/keys/wg${i}_private.key
    chmod 644 /etc/mole/keys/wg${i}_public.key
    echo "Generated keys for wg$i"
done

# Create WireGuard configuration templates
echo "📝 Creating WireGuard configuration templates..."
for i in $(seq 0 $((TUNNEL_COUNT - 1))); do
    PORT=$((51820 + i))
    PRIVATE_KEY=$(cat /etc/mole/keys/wg${i}_private.key)

    cat > /etc/wireguard/wg${i}.conf << EOF
[Interface]
PrivateKey = $PRIVATE_KEY
Address = 10.100.$((i + 1)).2/24
ListenPort = $PORT
MTU = $MTU_SIZE

# PostUp commands for routing and iptables
PostUp = iptables -A FORWARD -i wg${i} -j ACCEPT
PostUp = iptables -A FORWARD -o wg${i} -j ACCEPT
PostUp = iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE

# PostDown commands to clean up
PostDown = iptables -D FORWARD -i wg${i} -j ACCEPT
PostDown = iptables -D FORWARD -o wg${i} -j ACCEPT
PostDown = iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

# Peer section will be added when client connects
EOF

    echo "Created configuration for wg$i (port $PORT)"
done

# Set up monitoring script
echo "📊 Setting up monitoring..."
cat > /usr/local/bin/mole-monitor << 'EOF'
#!/bin/bash
# AWS Cloud Mole monitoring script

echo "AWS Cloud Mole Bastion Status"
echo "============================="
echo "Uptime: $(uptime -p)"
echo "Load: $(uptime | awk -F'load average:' '{ print $2 }')"
echo

echo "WireGuard Interfaces:"
for interface in $(ls /etc/wireguard/*.conf 2>/dev/null | xargs -n1 basename -s .conf); do
    if ip link show $interface >/dev/null 2>&1; then
        echo "  ✅ $interface: UP"
        wg show $interface transfer 2>/dev/null | head -1
    else
        echo "  ❌ $interface: DOWN"
    fi
done

echo
echo "Network Statistics:"
ss -tuln | grep :518 | wc -l | xargs printf "  Active WireGuard listeners: %s\n"

echo
echo "System Resources:"
free -h | grep Mem | awk '{print "  Memory: " $3 "/" $2 " (" int($3/$2*100) "%)"}'
df -h / | tail -1 | awk '{print "  Disk: " $3 "/" $2 " (" $5 ")"}'
EOF

chmod +x /usr/local/bin/mole-monitor

# Set up log rotation
echo "📝 Setting up log rotation..."
cat > /etc/logrotate.d/mole << EOF
/var/log/mole-*.log {
    daily
    missingok
    rotate 7
    compress
    delaycompress
    notifempty
    copytruncate
}
EOF

# Create systemd service for mole monitoring
echo "🔧 Creating monitoring service..."
cat > /etc/systemd/system/mole-monitor.service << EOF
[Unit]
Description=AWS Cloud Mole Monitor
After=network.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/mole-monitor
User=root

[Install]
WantedBy=multi-user.target
EOF

# Create systemd timer for regular monitoring
cat > /etc/systemd/system/mole-monitor.timer << EOF
[Unit]
Description=Run mole-monitor every 5 minutes
Requires=mole-monitor.service

[Timer]
OnCalendar=*:0/5

[Install]
WantedBy=timers.target
EOF

systemctl daemon-reload
systemctl enable mole-monitor.timer
systemctl start mole-monitor.timer

# Store configuration in SSM Parameter Store
echo "📡 Storing configuration in SSM..."
AWS_REGION=$(curl -s http://169.254.169.254/latest/meta-data/placement/region)
INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id)

# Store public keys in SSM for client retrieval
for i in $(seq 0 $((TUNNEL_COUNT - 1))); do
    PUBLIC_KEY=$(cat /etc/mole/keys/wg${i}_public.key)
    aws ssm put-parameter \
        --region "$AWS_REGION" \
        --name "/mole/bastion/$INSTANCE_ID/wg${i}/public_key" \
        --value "$PUBLIC_KEY" \
        --type "String" \
        --overwrite || echo "Warning: Could not store public key in SSM"
done

# Store bastion info
aws ssm put-parameter \
    --region "$AWS_REGION" \
    --name "/mole/bastion/$INSTANCE_ID/info" \
    --value "{\"tunnel_count\":$TUNNEL_COUNT,\"mtu\":$MTU_SIZE,\"status\":\"ready\"}" \
    --type "String" \
    --overwrite || echo "Warning: Could not store bastion info in SSM"

# Final status
echo "✅ AWS Cloud Mole bastion initialization complete!"
echo "   Instance ID: $INSTANCE_ID"
echo "   Tunnel Count: $TUNNEL_COUNT"
echo "   MTU Size: $MTU_SIZE"
echo "   Status: Ready for client connections"

# Create status file
cat > /etc/mole/status << EOF
{
    "status": "ready",
    "initialized_at": "$(date -Iseconds)",
    "instance_id": "$INSTANCE_ID",
    "tunnel_count": $TUNNEL_COUNT,
    "mtu_size": $MTU_SIZE,
    "public_keys": {
EOF

for i in $(seq 0 $((TUNNEL_COUNT - 1))); do
    PUBLIC_KEY=$(cat /etc/mole/keys/wg${i}_public.key)
    echo "        \"wg$i\": \"$PUBLIC_KEY\"" >> /etc/mole/status
    if [ $i -lt $((TUNNEL_COUNT - 1)) ]; then
        echo "," >> /etc/mole/status
    fi
done

cat >> /etc/mole/status << EOF
    }
}
EOF

echo "💾 Configuration saved to /etc/mole/status"
echo "🎉 Bastion is ready for mole client connections!"