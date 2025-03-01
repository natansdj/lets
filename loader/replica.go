package loader

import (
	"context"
	"net"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"
)

var InfoReplica *types.Replica

func Replica() {
	if lets.IsNil(InfoReplica) {
		InfoReplica = &types.Replica{}
	}

	var add bool
	for _, hostname := range InfoNetwork.Hostname {
		// Lookup ip
		ips, err := net.DefaultResolver.LookupIPAddr(context.Background(), hostname)
		if err != nil {
			lets.LogErr(err)
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
				}
			}
		}
	}
}
