package loader

import (
	"fmt"
	"net"
	"slices"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"
)

var InfoNetwork *types.IdentityNetwork

// Network collects network interface information for the system.
// This includes IP addresses (IPv4/IPv6), hostnames, and network interfaces.
// Can be disabled via environment variable: LETS_ENABLE_NETWORK_INFO=false
func Network() {
	// Check if network info collection is enabled
	if SystemInfoConfig != nil && !SystemInfoConfig.EnableNetworkInfo() {
		lets.LogD("Network information collection disabled via configuration")
		return
	}

	if lets.IsNil(InfoNetwork) {
		InfoNetwork = &types.IdentityNetwork{}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		lets.LogERL("network-interfaces-error", "Failed to get network interfaces: %v", err)
		return
	}

	verboseMode := SystemInfoConfig != nil && SystemInfoConfig.EnableVerboseNetworkInfo()

	for _, i := range ifaces {
		// Skip loopback and down interfaces unless in verbose mode
		if !verboseMode {
			if i.Flags&net.FlagLoopback != 0 || i.Flags&net.FlagUp == 0 {
				continue
			}
		}

		addrs, err := i.Addrs()
		if err != nil {
			lets.LogERL("network-addrs-error", "Failed to get addresses for interface %s: %v", i.Name, err)
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Only collect global unicast addresses (or all if verbose)
			if !verboseMode && !ip.IsGlobalUnicast() {
				continue
			}

			kind := "public"
			if ip.IsPrivate() {
				kind = "private"
			} else if ip.IsLoopback() {
				kind = "loopback"
			}

			info := fmt.Sprintf("%s (%s %s)", i.Name, ip.String(), kind)
			InfoNetwork.NetInterface = append(InfoNetwork.NetInterface, info)

			if ipv4 := ip.To4(); ipv4 != nil {
				InfoNetwork.IPV4 = append(InfoNetwork.IPV4, ip)
			} else {
				InfoNetwork.IPV6 = append(InfoNetwork.IPV6, ip)
			}

			// Lookup Hostname (only for non-loopback addresses)
			if !ip.IsLoopback() {
				addrs, err := net.LookupAddr(ip.String())
				if err != nil {
					// Only log in verbose mode, hostname lookup failures are common
					if verboseMode {
						lets.LogD("Hostname lookup failed for %s: %v", ip.String(), err)
					}
					continue
				}

				for _, addr := range addrs {
					if !slices.Contains(InfoNetwork.Hostname, addr) {
						InfoNetwork.Hostname = append(InfoNetwork.Hostname, addr)
					}
				}
			}
		}
	}

	if verboseMode {
		lets.LogD("Network info collected: %d interfaces, %d IPv4, %d IPv6, %d hostnames",
			len(InfoNetwork.NetInterface), len(InfoNetwork.IPV4), len(InfoNetwork.IPV6), len(InfoNetwork.Hostname))
	}
}
