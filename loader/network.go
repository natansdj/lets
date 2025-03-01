package loader

import (
	"fmt"
	"net"
	"slices"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/types"
)

var InfoNetwork *types.IdentityNetwork

func Network() {
	if lets.IsNil(InfoNetwork) {
		InfoNetwork = &types.IdentityNetwork{}
	}

	ifaces, err := net.Interfaces()
	if err != nil {
		lets.LogErr(err)
		return
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			lets.LogErr(err)
			return
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip.IsGlobalUnicast() {
				kind := "public"
				if ip.IsPrivate() {
					kind = "private"
				}

				// asdasdasd, _ := net.DefaultResolver.LookupHost(context.Background(), ip.String())
				// lets.LogJI(asdasdasd)

				// asd, _ := net.DefaultResolver.LookupNS(context.Background(), ip.String())
				// lets.LogJI(asd)

				info := fmt.Sprintf("%s (%s %s)", i.Name, ip.String(), kind)
				InfoNetwork.NetInterface = append(InfoNetwork.NetInterface, info)

				if ipv4 := ip.To4(); ipv4 != nil {
					InfoNetwork.IPV4 = append(InfoNetwork.IPV4, ip)
				} else {
					InfoNetwork.IPV6 = append(InfoNetwork.IPV6, ip)
				}

				// Lookup Hostname
				addrs, err := net.LookupAddr(ip.String())
				if err != nil {
					lets.LogErr(err)
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
}
