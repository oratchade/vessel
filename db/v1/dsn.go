package v1

import (
	"net"
	"net/netip"
	"strconv"
)

// ipv6 returns host's IPv6 address, written bare or in one pair of brackets, its zone
// included, as text and parsed; ok is false for anything else, a name or an IPv4
// address among them.
func ipv6(host string) (addr string, ip netip.Addr, ok bool) {
	addr = host
	if len(addr) >= 2 && addr[0] == '[' && addr[len(addr)-1] == ']' {
		addr = addr[1 : len(addr)-1]
	}
	if a, err := netip.ParseAddr(addr); err == nil && a.Is6() {
		return addr, a, true
	}
	return "", netip.Addr{}, false
}

// needsZone reports whether ip is zoned and reachable only through its zone: a
// link-local or interface-local address. The loopback and a global address route
// without one.
func needsZone(ip netip.Addr) bool {
	return ip.Zone() != "" && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast())
}

// hostPort is host and port as a DSN writes an address: an IPv6 address in brackets,
// given bare or already bracketed, so fd00::5 and [fd00::5] reach the driver alike. Any
// other host is joined as written. PostgreSQL and MySQL always joined it so, and
// whatever they made of a malformed one ([], [fd00::5, db:5432) they make of it still;
// SQL Server bracketed any host holding a colon, so a malformed [::1 connected;
// MSSQLConfig.Validate now refuses a host its driver would not read back as written.
func hostPort(host string, port int) string {
	if addr, _, ok := ipv6(host); ok {
		return net.JoinHostPort(addr, strconv.Itoa(port))
	}
	return host + ":" + strconv.Itoa(port)
}
