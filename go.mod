module github.com/research-computing/mole

go 1.22.0

toolchain go1.22.5

require (
	github.com/aws/aws-sdk-go-v2 v1.30.0
	github.com/aws/aws-sdk-go-v2/config v1.27.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.147.0
	github.com/prometheus/client_golang v1.19.0
	github.com/spf13/cobra v1.8.1
	github.com/spf13/viper v1.18.2
	github.com/vishvananda/netlink v1.1.0
	golang.org/x/crypto v0.21.0
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20230429144221-925a1e7659e6
)