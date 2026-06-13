package notify

import (
	"net"
	"os"
	"strings"
)

// FQDN returns the fully-qualified domain name of the host, mirroring Python's
// socket.getfqdn(): it resolves the hostname to addresses and reverse-looks-up
// a canonical name, falling back to the bare hostname.
func FQDN() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "localhost"
	}

	addrs, err := net.LookupHost(host)
	if err == nil {
		for _, addr := range addrs {
			if names, err := net.LookupAddr(addr); err == nil && len(names) > 0 {
				return strings.TrimSuffix(names[0], ".")
			}
		}
	}

	if cname, err := net.LookupCNAME(host); err == nil && cname != "" {
		return strings.TrimSuffix(cname, ".")
	}
	return host
}
