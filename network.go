package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	mathrand "math/rand"
	"net"
	"net/netip"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type socketManager struct {
	opts       *options
	ipv4       *icmp.PacketConn
	ipv6       *icmp.PacketConn
	ipv4Net    string
	ipv6Net    string
	hostByIDv4 map[int]*host
	hostByIDv6 map[int]*host
}

func newSocketManager(opts *options) (*socketManager, error) {
	sm := &socketManager{
		opts:       opts,
		hostByIDv4: make(map[int]*host),
		hostByIDv6: make(map[int]*host),
	}
	var err error

	local4 := opts.localAddr(4)
	local6 := opts.localAddr(6)

	if !opts.ipv6Only {
		sm.ipv4, sm.ipv4Net, err = listenICMP("ip4:icmp", local4)
		if err != nil {
			return nil, err
		}
		sm.configureIPv4()
	}
	if !opts.ipv4Only {
		sm.ipv6, sm.ipv6Net, err = listenICMP("ip6:ipv6-icmp", local6)
		if err != nil && opts.ipv6Only {
			return nil, err
		}
		sm.configureIPv6()
	}
	if sm.ipv4 == nil && sm.ipv6 == nil {
		return nil, errors.New("unable to open ICMP sockets")
	}
	return sm, nil
}

func (sm *socketManager) configureIPv4() {
	if sm.ipv4 == nil {
		return
	}
	if pc := sm.ipv4.IPv4PacketConn(); pc != nil {
		if sm.opts.ttl > 0 {
			pc.SetTTL(sm.opts.ttl)
		}
		if sm.opts.tos > 0 {
			pc.SetTOS(sm.opts.tos)
		}
	}
}

func (sm *socketManager) configureIPv6() {
	if sm.ipv6 == nil {
		return
	}
	if pc := sm.ipv6.IPv6PacketConn(); pc != nil {
		if sm.opts.ttl > 0 {
			pc.SetHopLimit(sm.opts.ttl)
		}
		if sm.opts.tos > 0 {
			pc.SetTrafficClass(sm.opts.tos)
		}
	}
}

func listenICMP(network, source string) (*icmp.PacketConn, string, error) {
	conn, err := icmp.ListenPacket(network, source)
	if err == nil {
		return conn, network, nil
	}
	altNetwork := ""
	switch network {
	case "ip4:icmp":
		altNetwork = "udp4"
	case "ip6:ipv6-icmp":
		altNetwork = "udp6"
	}
	if altNetwork == "" {
		return nil, "", err
	}
	conn, err = icmp.ListenPacket(altNetwork, source)
	return conn, altNetwork, err
}

func (sm *socketManager) assignIDs(hosts []*host) {
	seed := make([]byte, 8)
	if _, err := rand.Read(seed); err == nil {
		mathrand.Seed(int64(binary.LittleEndian.Uint64(seed)))
	} else {
		mathrand.Seed(time.Now().UnixNano())
	}

	const maxICMPIDs = 0x10000
	v4count := 0
	v6count := 0
	for _, h := range hosts {
		if h.ipVersion == 4 {
			v4count++
		} else {
			v6count++
		}
	}

	base4 := 0
	base6 := 0
	if v4count > 0 {
		span := maxICMPIDs - v4count
		if span <= 0 {
			base4 = 0
		} else {
			base4 = mathrand.Intn(span)
		}
	}
	if v6count > 0 {
		span := maxICMPIDs - v6count
		if span <= 0 {
			base6 = 0
		} else {
			base6 = mathrand.Intn(span)
		}
	}

	for _, h := range hosts {
		if h.ipVersion == 4 {
			h.icmpID = base4
			base4++
			sm.hostByIDv4[h.icmpID] = h
		} else {
			h.icmpID = base6
			base6++
			sm.hostByIDv6[h.icmpID] = h
		}
		h.replyCh = make(chan reply, 4)
	}
}

func (sm *socketManager) start(ctx context.Context) error {
	if sm.ipv4 != nil {
		go sm.readLoop(ctx, sm.ipv4, 4)
	}
	if sm.ipv6 != nil {
		go sm.readLoop(ctx, sm.ipv6, 6)
	}
	go func() {
		<-ctx.Done()
		sm.close()
	}()
	return nil
}

func (sm *socketManager) close() {
	if sm.ipv4 != nil {
		sm.ipv4.Close()
	}
	if sm.ipv6 != nil {
		sm.ipv6.Close()
	}
}

func (sm *socketManager) send(h *host, seq int, opts *options) (int, error) {
	var conn *icmp.PacketConn
	var msg icmp.Message
	payload := buildPayload(opts.packetSize, opts.randomPayload)

	if h.ipVersion == 4 {
		conn = sm.ipv4
		if opts.icmpTimestamp {
			msg = icmp.Message{
				Type: ipv4.ICMPTypeTimestamp,
				Code: 0,
				Body: &timestampBody{
					id:        h.icmpID,
					seq:       seq,
					originate: msSinceMidnight(time.Now()),
					receive:   0,
					transmit:  0,
				},
			}
		} else {
			msg = icmp.Message{
				Type: ipv4.ICMPTypeEcho,
				Code: 0,
				Body: &icmp.Echo{
					ID:   h.icmpID,
					Seq:  seq,
					Data: payload,
				},
			}
		}
	} else {
		conn = sm.ipv6
		msg = icmp.Message{
			Type: ipv6.ICMPTypeEchoRequest,
			Code: 0,
			Body: &icmp.Echo{
				ID:   h.icmpID,
				Seq:  seq,
				Data: payload,
			},
		}
	}
	if conn == nil {
		return 0, errors.New("no socket for address family")
	}
	b, err := msg.Marshal(nil)
	if err != nil {
		return 0, err
	}
	ip := h.addr.AsSlice()
	var addr net.Addr
	if h.ipVersion == 4 {
		if sm.ipv4Net == "udp4" {
			addr = &net.UDPAddr{IP: ip}
		} else {
			ipAddr := &net.IPAddr{IP: ip}
			addr = ipAddr
		}
	} else {
		if sm.ipv6Net == "udp6" {
			udpAddr := &net.UDPAddr{IP: ip}
			if h.zone != "" {
				udpAddr.Zone = h.zone
			}
			addr = udpAddr
		} else {
			ipAddr := &net.IPAddr{IP: ip}
			if h.zone != "" {
				ipAddr.Zone = h.zone
			}
			addr = ipAddr
		}
	}
	if h.zone != "" {
		switch a := addr.(type) {
		case *net.IPAddr:
			a.Zone = h.zone
		case *net.UDPAddr:
			a.Zone = h.zone
		}
	}
	n, err := conn.WriteTo(b, addr)
	return n, err
}

func (sm *socketManager) readLoop(ctx context.Context, conn *icmp.PacketConn, family int) {
	buf := make([]byte, 2048)
	proto := 1
	var ipv4Conn *ipv4.PacketConn
	var ipv6Conn *ipv6.PacketConn
	if family == 6 {
		proto = 58
	}
	if family == 4 {
		ipv4Conn = conn.IPv4PacketConn()
		if ipv4Conn != nil && sm.opts.printTTL {
			var flags ipv4.ControlFlags
			if sm.opts.printTTL {
				flags |= ipv4.FlagTTL
			}
			ipv4Conn.SetControlMessage(flags, true)
		}
	} else if family == 6 {
		ipv6Conn = conn.IPv6PacketConn()
		if ipv6Conn != nil && sm.opts.printTTL {
			var flags ipv6.ControlFlags
			if sm.opts.printTTL {
				flags |= ipv6.FlagHopLimit
			}
			ipv6Conn.SetControlMessage(flags, true)
		}
	}
	for {
		deadline := time.Now().Add(500 * time.Millisecond)
		var (
			n    int
			addr net.Addr
			err  error
			ttl  = -1
			tos  = -1
			cm4  *ipv4.ControlMessage
			cm6  *ipv6.ControlMessage
		)
		if family == 4 && ipv4Conn != nil && sm.ipv4Net == "ip4:icmp" {
			ipv4Conn.SetReadDeadline(deadline)
			n, cm4, addr, err = ipv4Conn.ReadFrom(buf)
			if cm4 != nil {
				ttl = cm4.TTL
			}
		} else if family == 6 && ipv6Conn != nil && sm.ipv6Net == "ip6:ipv6-icmp" {
			ipv6Conn.SetReadDeadline(deadline)
			n, cm6, addr, err = ipv6Conn.ReadFrom(buf)
			if cm6 != nil {
				ttl = cm6.HopLimit
			}
		} else {
			conn.SetReadDeadline(deadline)
			n, addr, err = conn.ReadFrom(buf)
		}
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				select {
				case <-ctx.Done():
					return
				default:
				}
				continue
			}
			return
		}
		msg, err := icmp.ParseMessage(proto, buf[:n])
		if err != nil {
			continue
		}
		var (
			h      *host
			seq    int
			tsInfo *timestampInfo
			hostID int
		)
		switch body := msg.Body.(type) {
		case *icmp.Echo:
			seq = body.Seq
			hostID = body.ID
		case *icmp.RawBody:
			if family == 4 && msg.Type == ipv4.ICMPTypeTimestampReply && len(body.Data) >= 16 {
				hostID = int(binary.BigEndian.Uint16(body.Data[0:2]))
				seq = int(binary.BigEndian.Uint16(body.Data[2:4]))
				tsInfo = &timestampInfo{
					originate: binary.BigEndian.Uint32(body.Data[4:8]),
					receive:   binary.BigEndian.Uint32(body.Data[8:12]),
					transmit:  binary.BigEndian.Uint32(body.Data[12:16]),
				}
			} else {
				continue
			}
		default:
			continue
		}
		h = sm.lookupHost(family, hostID)
		if h == nil {
			continue
		}
		if sm.opts.checkSource && !addrMatches(addr, h.addr) {
			continue
		}
		reply := reply{
			recvTime:   time.Now(),
			size:       n,
			src:        addr,
			seq:        seq,
			ttl:        ttl,
			tos:        tos,
			timestamps: tsInfo,
		}
		select {
		case h.replyCh <- reply:
		default:
		}
	}
}

func (sm *socketManager) lookupHost(family int, id int) *host {
	if family == 4 {
		return sm.hostByIDv4[id]
	}
	return sm.hostByIDv6[id]
}

func buildPayload(size int, randomize bool) []byte {
	if size < 0 {
		size = 0
	}
	payload := make([]byte, size)
	if randomize {
		rand.Read(payload)
	}
	return payload
}

func addrMatches(src net.Addr, target netip.Addr) bool {
	var ip net.IP
	switch v := src.(type) {
	case *net.IPAddr:
		ip = v.IP
	case *net.UDPAddr:
		ip = v.IP
	default:
		return false
	}
	ipAddr, err := netip.ParseAddr(ip.String())
	if err != nil {
		return false
	}
	return ipAddr == target
}

type timestampBody struct {
	id        int
	seq       int
	originate uint32
	receive   uint32
	transmit  uint32
}

func (t *timestampBody) Len(proto int) int {
	return 16
}

func (t *timestampBody) Marshal(proto int) ([]byte, error) {
	b := make([]byte, 16)
	binary.BigEndian.PutUint16(b[0:2], uint16(t.id))
	binary.BigEndian.PutUint16(b[2:4], uint16(t.seq))
	binary.BigEndian.PutUint32(b[4:8], t.originate)
	binary.BigEndian.PutUint32(b[8:12], t.receive)
	binary.BigEndian.PutUint32(b[12:16], t.transmit)
	return b, nil
}
