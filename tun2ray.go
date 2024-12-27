package libbox

import (
	"bytes"
	"context"
	"net"
	"net/netip"
	"syscall"
	"time"
	_ "unsafe"

	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing-vmess/packetaddr"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"
	udpnat "github.com/sagernet/sing/common/udpnat2"
	"github.com/v2fly/v2ray-core/v5"
	v2rayCommon "github.com/v2fly/v2ray-core/v5/common"
	v2rayBuf "github.com/v2fly/v2ray-core/v5/common/buf"
	"github.com/v2fly/v2ray-core/v5/common/log"
	v2rayNet "github.com/v2fly/v2ray-core/v5/common/net"
	"github.com/v2fly/v2ray-core/v5/common/session"
	"github.com/v2fly/v2ray-core/v5/features/routing"
)

type tun2ray struct {
	ctx        context.Context
	instance   *core.Instance
	dispatcher routing.Dispatcher
	iif        PlatformInterface
	network    *networkManager
	tunOptions tun.Options
	dnsServer  netip.Addr
	tun        tun.Tun
	stack      tun.Stack
}

func newTun2ray(ctx context.Context, instance *core.Instance, iif PlatformInterface) *tun2ray {
	return &tun2ray{
		ctx:        ctx,
		instance:   instance,
		dispatcher: instance.GetFeature(routing.DispatcherType()).(routing.Dispatcher),
		iif:        iif,
		network:    newNetworkManager(iif),
		tunOptions: tun.Options{
			Inet4Address: []netip.Prefix{
				netip.MustParsePrefix("172.19.0.1/30"),
			},
			Inet6Address: []netip.Prefix{
				netip.MustParsePrefix("fdfe:dcba:9876::1/126"),
			},
			DNSServers: []netip.Addr{
				netip.MustParseAddr("1.1.1.1"),
			},
			MTU:       9000,
			AutoRoute: true,
			Logger:    (*v2rayLogger)(nil),
		},
	}
}

func (t *tun2ray) Start() error {
	err := t.network.Start()
	if err != nil {
		return err
	}
	routeRanges, err := t.tunOptions.BuildAutoRouteRanges(true)
	if err != nil {
		return err
	}
	tunFd, err := t.iif.OpenTun(&tunOptions{
		Options:     &t.tunOptions,
		routeRanges: routeRanges,
	})
	if err != nil {
		return err
	}
	t.tunOptions.Name, err = getTunnelName(tunFd)
	if err != nil {
		return E.Cause(err, "query tun name")
	}
	dupFd, err := dup(int(tunFd))
	if err != nil {
		return E.Cause(err, "dup tun file descriptor")
	}
	t.tunOptions.FileDescriptor = dupFd
	sTun, err := tun.New(t.tunOptions)
	if err != nil {
		syscall.Close(dupFd)
		return E.Cause(err, "create tun instance")
	}
	sStack, err := tun.NewStack("", tun.StackOptions{
		Context:                t.ctx,
		Tun:                    sTun,
		TunOptions:             t.tunOptions,
		UDPTimeout:             time.Minute,
		Handler:                t,
		Logger:                 (*v2rayLogger)(nil),
		ForwarderBindInterface: true,
		IncludeAllNetworks:     t.iif.IncludeAllNetworks(),
		InterfaceFinder:        t.network,
	})
	if err != nil {
		sTun.Close()
		return E.Cause(err, "create tun stack")
	}
	err = sStack.Start()
	if err != nil {
		sTun.Close()
		return E.Cause(err, "start tun stack")
	}
	err = sTun.Start()
	if err != nil {
		sTun.Close()
		sStack.Close()
		return E.Cause(err, "start tun")
	}
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Info,
		Content:  F.ToString("started at ", t.tunOptions.Name),
	})
	t.tun = sTun
	t.stack = sStack
	return nil
}

func (t *tun2ray) PrepareConnection(network string, source M.Socksaddr, destination M.Socksaddr) error {
	return nil
}

func (t *tun2ray) NewConnectionEx(ctx context.Context, conn net.Conn, source M.Socksaddr, destination M.Socksaddr, _ N.CloseHandlerFunc) {
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Info,
		Content:  F.ToString("inbound connection from ", source, " to ", destination),
	})
	inbound := &session.Inbound{
		Source: tcpDestination(source.AddrPort()),
		Tag:    "injectedTun",
	}
	ctx = toContext(ctx, t.instance)
	ctx = session.ContextWithInbound(ctx, inbound)
	isDNS := destination.Port == 53
	if isDNS {
		ctx = session.ContextWithContent(ctx, &session.Content{
			Protocol: "dns",
		})
	} else {
		ctx = session.ContextWithContent(ctx, &session.Content{
			SniffingRequest: session.SniffingRequest{
				Enabled: true,
			},
		})
	}
	link, err := t.dispatcher.Dispatch(ctx, tcpDestination(destination.AddrPort()))
	if err != nil {
		log.Record(&log.GeneralMessage{
			Severity: log.Severity_Error,
			Content:  F.ToString("process connection from ", source, " to ", destination, ": ", err),
		})
		return
	}
	var group task.Group
	group.Append("upload", func(ctx context.Context) error {
		return v2rayBuf.Copy(v2rayBuf.NewReader(conn), link.Writer)
	})
	group.Append("download", func(ctx context.Context) error {
		return v2rayBuf.Copy(link.Reader, v2rayBuf.NewWriter(conn))
	})
	group.FastFail()
	group.Cleanup(func() {
		conn.Close()
		v2rayCommon.Interrupt(link.Reader)
		v2rayCommon.Interrupt(link.Writer)
	})
	_ = group.Run(ctx)
}

func (t *tun2ray) NewPacketConnectionEx(ctx context.Context, conn N.PacketConn, source M.Socksaddr, destination M.Socksaddr, _ N.CloseHandlerFunc) {
	log.Record(&log.GeneralMessage{
		Severity: log.Severity_Info,
		Content:  F.ToString("inbound packet connection from ", source, " to ", destination),
	})
	inbound := &session.Inbound{
		Source: udpDestination(source.AddrPort()),
		Tag:    "injectedTun",
	}
	ctx = toContext(ctx, t.instance)
	ctx = session.ContextWithInbound(ctx, inbound)
	isDNS := destination.Port == 53
	if isDNS {
		ctx = session.ContextWithContent(ctx, &session.Content{
			Protocol: "dns",
		})
	} else {
		ctx = session.ContextWithContent(ctx, &session.Content{
			SniffingRequest: session.SniffingRequest{
				Enabled: true,
			},
		})
	}
	var vDest v2rayNet.Destination
	//if !isDNS {
	//	vDest = v2rayNet.Destination{
	//		Address: v2rayNet.DomainAddress(packetaddr.SeqPacketMagicAddress),
	//		Network: v2rayNet.Network_UDP,
	//	}
	//} else {
	vDest = udpDestination(destination.AddrPort())
	//}
	link, err := t.dispatcher.Dispatch(ctx, vDest)
	if err != nil {
		log.Record(&log.GeneralMessage{
			Severity: log.Severity_Error,
			Content:  F.ToString("process packet connection from ", source, " to ", destination, ": ", err),
		})
		return
	}
	packetConn := &v2rayPacketConn{
		conn:        conn.(udpnat.Conn),
		destination: destination,
		// packetAddr:  !isDNS && destination.Port != 443,
	}
	var group task.Group
	group.Append("upload", func(ctx context.Context) error {
		return v2rayBuf.Copy(packetConn, link.Writer)
	})
	group.Append("download", func(ctx context.Context) error {
		return v2rayBuf.Copy(link.Reader, packetConn)
	})
	group.FastFail()
	group.Cleanup(func() {
		conn.Close()
		v2rayCommon.Interrupt(link.Reader)
		v2rayCommon.Interrupt(link.Writer)
	})
	_ = group.Run(ctx)
}

func (t *tun2ray) Close() {
	common.Close(t.stack, t.tun)
}

type v2rayPacketConn struct {
	conn        udpnat.Conn
	destination M.Socksaddr
	packetAddr  bool
}

var packetAddrMaxLen = packetaddr.AddressSerializer.AddrPortLen(M.Socksaddr{
	Addr: netip.IPv6Unspecified(),
})

func (r *v2rayPacketConn) ReadMultiBuffer() (v2rayBuf.MultiBuffer, error) {
	vBuf := v2rayBuf.New()
	buffer := buf.With(vBuf.Extend(v2rayBuf.Size))
	if r.packetAddr {
		buffer.Advance(packetAddrMaxLen)
	}
	destination, err := r.conn.ReadPacket(buffer)
	if err != nil {
		vBuf.Release()
		return nil, err
	}
	if r.packetAddr {
		err = packetaddr.AddressSerializer.WriteAddrPort(bytes.NewBuffer(buffer.Extend(packetaddr.AddressSerializer.AddrPortLen(destination))), destination)
		if err != nil {
			vBuf.Release()
			return nil, err
		}
	}
	vBuf.Clear()
	vBuf.Resize(int32(buffer.Start()), int32(buffer.Start()+buffer.Len()))
	return v2rayBuf.MultiBuffer{vBuf}, nil
}

func (r *v2rayPacketConn) WriteMultiBuffer(mb v2rayBuf.MultiBuffer) error {
	buffer := buf.NewSize(int(mb.Len()))
	buffer.Truncate(mb.Copy(buffer.FreeBytes()))
	v2rayBuf.ReleaseMulti(mb)
	if r.packetAddr {
		destination, err := packetaddr.AddressSerializer.ReadAddrPort(buffer)
		if err != nil {
			buffer.Release()
			return err
		}
		return r.conn.WritePacket(buffer, destination)
	} else {
		return r.conn.WritePacket(buffer, r.destination)
	}
}

//go:linkname toContext github.com/v2fly/v2ray-core/v5.toContext
func toContext(ctx context.Context, v *core.Instance) context.Context
