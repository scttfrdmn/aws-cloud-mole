package network

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// getDefaultInterface discovers the default network interface
func (np *NetworkProber) getDefaultInterface() (NetworkInterface, error) {
	var iface NetworkInterface

	// Get default route interface
	cmd := exec.Command("ip", "route", "show", "default")
	output, err := cmd.Output()
	if err != nil {
		return iface, err
	}

	// Parse interface name from route
	re := regexp.MustCompile(`dev (\w+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return iface, fmt.Errorf("could not determine default interface")
	}

	iface.Name = matches[1]

	// Get interface details
	if err := np.getInterfaceDetails(&iface); err != nil {
		return iface, err
	}

	return iface, nil
}

// getInterfaceDetails populates interface configuration details
func (np *NetworkProber) getInterfaceDetails(iface *NetworkInterface) error {
	// Get MTU
	cmd := exec.Command("cat", fmt.Sprintf("/sys/class/net/%s/mtu", iface.Name))
	if output, err := cmd.Output(); err == nil {
		if mtu, err := strconv.Atoi(strings.TrimSpace(string(output))); err == nil {
			iface.MTU = mtu
		}
	}

	// Get speed (link speed in Mbps)
	cmd = exec.Command("cat", fmt.Sprintf("/sys/class/net/%s/speed", iface.Name))
	if output, err := cmd.Output(); err == nil {
		if speed, err := strconv.ParseInt(strings.TrimSpace(string(output)), 10, 64); err == nil {
			iface.Speed = speed * 1024 * 1024 // Convert Mbps to bps
		}
	}

	// Get driver info
	cmd = exec.Command("ethtool", "-i", iface.Name)
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "driver:") {
				iface.Driver = strings.TrimSpace(strings.TrimPrefix(line, "driver:"))
				break
			}
		}
	}

	// Get queue count
	cmd = exec.Command("ls", fmt.Sprintf("/sys/class/net/%s/queues/", iface.Name))
	if output, err := cmd.Output(); err == nil {
		queueDirs := strings.Fields(string(output))
		rxQueues := 0
		for _, dir := range queueDirs {
			if strings.HasPrefix(dir, "rx-") {
				rxQueues++
			}
		}
		iface.QueueCount = rxQueues
	}

	// Get offloading features
	iface.Offloading = make(map[string]bool)
	cmd = exec.Command("ethtool", "-k", iface.Name)
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, ":") {
				parts := strings.Split(line, ":")
				if len(parts) == 2 {
					feature := strings.TrimSpace(parts[0])
					status := strings.TrimSpace(parts[1])
					iface.Offloading[feature] = status == "on"
				}
			}
		}
	}

	return nil
}

// testPathMTU verifies path supports given MTU size
func (np *NetworkProber) testPathMTU(endpoint string, mtu int) bool {
	// Use ping with DF flag to test path MTU
	payloadSize := mtu - 28 // Subtract IP + ICMP headers
	cmd := exec.Command("ping", "-M", "do", "-s", strconv.Itoa(payloadSize), "-c", "3", "-W", "5", endpoint)

	// Run ping and check if packets go through
	err := cmd.Run()
	return err == nil
}

// testMTUThroughput measures throughput with specific MTU
func (np *NetworkProber) testMTUThroughput(ctx context.Context, endpoint string, mtu int) (int64, error) {
	// This would use iperf3 or custom tool to test throughput with specific MTU
	// For simulation, return reasonable values based on MTU efficiency
	efficiency := float64(mtu-40) / float64(mtu) // Account for headers
	baseThroughput := int64(500 * 1024 * 1024)   // 500 Mbps base

	return int64(float64(baseThroughput) * efficiency), nil
}

// measureLatency measures round-trip latency to endpoint
func (np *NetworkProber) measureLatency(endpoint string) (time.Duration, error) {
	cmd := exec.Command("ping", "-c", strconv.Itoa(np.config.LatencyTestCount), "-i", "0.01", endpoint)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	// Parse ping output for average latency
	re := regexp.MustCompile(`rtt min/avg/max/mdev = [\d.]+/([\d.]+)/[\d.]+/[\d.]+ ms`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return 0, fmt.Errorf("could not parse ping output")
	}

	avgMs, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	return time.Duration(avgMs * float64(time.Millisecond)), nil
}

// testBandwidth measures throughput with specified number of parallel streams
func (np *NetworkProber) testBandwidth(ctx context.Context, endpoint string, streams int) (int64, error) {
	// This would use iperf3 or similar tool
	// For simulation, model diminishing returns with multiple streams
	baseRate := int64(800 * 1024 * 1024) // 800 Mbps single stream

	// Model realistic scaling: each stream adds less capacity due to overhead
	scaling := []float64{1.0, 1.8, 3.2, 4.2}
	if streams <= len(scaling) {
		return int64(float64(baseRate) * scaling[streams-1]), nil
	}

	// For more streams, use logarithmic scaling
	multiplier := 4.2 + 0.3*float64(streams-4)
	return int64(float64(baseRate) * multiplier), nil
}

// formatBandwidth formats bandwidth in human-readable form
func formatBandwidth(bps int64) string {
	const unit = 1000
	if bps < unit {
		return fmt.Sprintf("%d bps", bps)
	}

	units := []string{"Kbps", "Mbps", "Gbps", "Tbps"}
	div := int64(unit)
	exp := 0

	for n := bps / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %s", float64(bps)/float64(div), units[exp])
}

// getTestEndpoint returns a test endpoint for the given region
func (np *NetworkProber) getTestEndpoint(region string) (string, error) {
	// Use AWS region endpoint for testing
	endpoints := map[string]string{
		"us-west-2": "ec2.us-west-2.amazonaws.com",
		"us-east-1": "ec2.us-east-1.amazonaws.com",
		"eu-west-1": "ec2.eu-west-1.amazonaws.com",
	}

	if endpoint, ok := endpoints[region]; ok {
		return endpoint, nil
	}

	return "", fmt.Errorf("no test endpoint for region %s", region)
}