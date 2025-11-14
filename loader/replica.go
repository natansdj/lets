package loader

import (
	"context"
	"net"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"
)

var InfoReplica *types.Replica

// Replica collects replica IP addresses from network hostnames.
// Requires Network() to be called first to populate InfoNetwork.
// Can be disabled via environment variable: LETS_ENABLE_REPLICA_INFO=false
func Replica() {
	// Check if replica info collection is enabled
	if SystemInfoConfig != nil && !SystemInfoConfig.EnableReplicaInfo() {
		lets.LogD("Replica information collection disabled via configuration")
		return
	}

	// Check if network info is available (Network() must be called first)
	if InfoNetwork == nil {
		lets.LogD("Replica information skipped - Network information not available")
		return
	}

	// Check if there are hostnames to resolve
	if len(InfoNetwork.Hostname) == 0 {
		lets.LogD("Replica information skipped - No hostnames available")
		return
	}

	if lets.IsNil(InfoReplica) {
		InfoReplica = &types.Replica{}
	}

	var add bool
	resolvedCount := 0
	for _, hostname := range InfoNetwork.Hostname {
		// Lookup ip
		ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), hostname)
		if err != nil {
			lets.LogERL("replica-dns-lookup", "Failed to resolve hostname %s: %v", hostname, err)
			continue
		}

		for _, ip := range ips {
			add = true

			// IPV4
			if ipv4 := ip.IP.To4(); ipv4 != nil {
				for _, replicaIp := range InfoReplica.IPV4 {
					if replicaIp.String() == ip.IP.String() {
						add = false
						break
					}
				}

				if add {
					InfoReplica.IPV4 = append(InfoReplica.IPV4, ip.IP)
					resolvedCount++
				}
			} else { // IPV6
				for _, replicaIp := range InfoReplica.IPV6 {
					if replicaIp.String() == ip.IP.String() {
						add = false
						break
					}
				}

				if add {
					InfoReplica.IPV6 = append(InfoReplica.IPV6, ip.IP)
					resolvedCount++
				}
			}
		}
	}

	// Log collection summary in verbose mode
	verboseMode := SystemInfoConfig != nil && SystemInfoConfig.EnableVerboseNetworkInfo()
	if verboseMode {
		lets.LogD("Replica info collected: %d IPs (%d IPv4, %d IPv6) from %d hostnames",
			resolvedCount, len(InfoReplica.IPV4), len(InfoReplica.IPV6), len(InfoNetwork.Hostname))
	}
}
