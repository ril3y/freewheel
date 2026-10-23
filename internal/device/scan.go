package device

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/ril3y/bowflex-tool/internal/adb"
)

// ScanNetwork probes the given /24 subnets ("a.b.c" prefixes) for ADB
// devices. With none given it scans the /24 of each local interface, which
// misses a bike on another VLAN; pass that subnet explicitly instead.
func ScanNetwork(log Logger, subnets ...string) []string {
	var results []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	if len(subnets) == 0 {
		subnets = getLocalSubnets()
	}
	if len(subnets) == 0 {
		log.Info("Could not determine local network. Trying 192.168.1.0/24")
		subnets = []string{"192.168.1"}
	}

	for _, subnet := range subnets {
		log.Info(fmt.Sprintf("Scanning %s.0/24...", subnet))
		for i := 1; i < 255; i++ {
			wg.Add(1)
			ip := fmt.Sprintf("%s.%d", subnet, i)
			go func(ip string) {
				defer wg.Done()
				if adb.IsADBPort(ip) {
					mu.Lock()
					results = append(results, ip)
					mu.Unlock()
					log.Info(fmt.Sprintf("Found device: %s", ip))
				}
			}(ip)
		}
	}
	wg.Wait()
	return results
}

// IsJailbroken checks if freewheel components are installed on the device.
func IsJailbroken(ip string, log Logger) bool {
	conn, err := adb.Connect(ip, 5555, log.Warn)
	if err != nil {
		return false
	}
	out, _ := conn.Shell("pm list packages 2>/dev/null | grep -E 'freewheel.launcher|freewheel.bridge'")
	conn.Close()
	return strings.Contains(out, "freewheel.launcher") || strings.Contains(out, "freewheel.bridge")
}

// ParseSubnet accepts a /24 network written as "192.168.10", "192.168.10.0"
// or "192.168.10.0/24" and returns its "a.b.c" prefix. A host address such as
// "192.168.10.5" is not a subnet.
func ParseSubnet(s string) (string, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "/24")
	parts := strings.Split(s, ".")
	switch {
	case len(parts) == 4 && parts[3] == "0":
		parts = parts[:3]
	case len(parts) != 3:
		return "", false
	}
	prefix := strings.Join(parts, ".")
	if ip := net.ParseIP(prefix + ".0").To4(); ip == nil {
		return "", false
	}
	return prefix, true
}

func getLocalSubnets() []string {
	var subnets []string
	seen := map[string]bool{}
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, _ := iface.Addrs()
		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok {
				continue
			}
			ip := ipNet.IP.To4()
			if ip == nil || ip.IsLoopback() {
				continue
			}
			subnet := fmt.Sprintf("%d.%d.%d", ip[0], ip[1], ip[2])
			if !seen[subnet] {
				seen[subnet] = true
				subnets = append(subnets, subnet)
			}
		}
	}
	return subnets
}
