package libbox

import (
	"net/netip"

	v2rayNet "github.com/v2fly/v2ray-core/v5/common/net"
)

func tcpDestination(port netip.AddrPort) v2rayNet.Destination {
	return v2rayNet.TCPDestination(v2rayNet.IPAddress(port.Addr().AsSlice()), v2rayNet.Port(port.Port()))
}

func udpDestination(port netip.AddrPort) v2rayNet.Destination {
	return v2rayNet.UDPDestination(v2rayNet.IPAddress(port.Addr().AsSlice()), v2rayNet.Port(port.Port()))
}
