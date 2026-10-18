package v1

import (
	"net"
	"net/netip"
	"strconv"
	"strings"
)

// ipv6 returns host's IPv6 address, written bare or in one pair of brackets, its zone
// included; ok is false for anything else, a name or an IPv4 address among them.
func ipv6(host string) (addr string, ok bool) {
	addr = host
	if inner, cut := strings.CutPrefix(host, "["); cut {
		if inner, cut := strings.CutSuffix(inner, "]"); cut {
			addr = inner
		}
	}
	if a, err := netip.ParseAddr(addr); err == nil && a.Is6() {
		return addr, true
	}
	return "", false
}

// hostPort is host and port as a DSN writes an address: an IPv6 address in brackets,
// given bare or already bracketed, so fd00::5 and [fd00::5] reach the driver alike. Any
// other host is joined as written, so a malformed one ([], [fd00::5, db:5432) still
// fails to parse instead of turning into another address.
func hostPort(host string, port int) string {
	if addr, ok := ipv6(host); ok {
		return net.JoinHostPort(addr, strconv.Itoa(port))
	}
	return host + ":" + strconv.Itoa(port)
}
