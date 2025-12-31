package main

import (
	"bufio"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
)

func readTargetFile(path string) ([]string, error) {
	var scanner *bufio.Scanner
	if path == "-" {
		scanner = bufio.NewScanner(os.Stdin)
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		scanner = bufio.NewScanner(f)
	}
	var targets []string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		targets = append(targets, fields...)
	}
	return targets, scanner.Err()
}

func generateFromSpec(spec *generateSpec, ipv4Only, ipv6Only bool) ([]string, error) {
	var res []string
	if spec.cidr != nil {
		if count, ok := prefixCount(*spec.cidr); ok && count > maxGeneratedTargets {
			return nil, fmt.Errorf("generate limit exceeded (%d)", maxGeneratedTargets)
		}
		start := spec.cidr.Masked().Addr()
		broadcast, hasBroadcast := broadcastAddr(*spec.cidr)
		for ip := start; spec.cidr.Contains(ip); ip = incrementAddr(ip) {
			if hasBroadcast && ip == broadcast {
				continue
			}
			if ipv4Only && !ip.Is4() {
				continue
			}
			if ipv6Only && !ip.Is6() {
				continue
			}
			res = append(res, ip.String())
			if len(res) > maxGeneratedTargets {
				return nil, fmt.Errorf("generate limit exceeded (%d)", maxGeneratedTargets)
			}
		}
		return res, nil
	}
	for ip := spec.start; ; ip = incrementAddr(ip) {
		if ipv4Only && !ip.Is4() {
			continue
		}
		if ipv6Only && !ip.Is6() {
			continue
		}
		res = append(res, ip.String())
		if len(res) > maxGeneratedTargets {
			return nil, fmt.Errorf("generate limit exceeded (%d)", maxGeneratedTargets)
		}
		if ip == spec.end {
			break
		}
	}
	return res, nil
}

func prefixCount(prefix netip.Prefix) (int, bool) {
	bits := 32
	if prefix.Addr().Is6() {
		bits = 128
	}
	ones := prefix.Bits()
	if ones < 0 {
		return 0, false
	}
	remaining := bits - ones
	if remaining > 24 {
		return maxGeneratedTargets + 1, true
	}
	count := 1 << remaining
	if count > maxGeneratedTargets {
		return count, true
	}
	return count, true
}

func incrementAddr(addr netip.Addr) netip.Addr {
	if addr.Is4() {
		bytes := addr.As4()
		for i := len(bytes) - 1; i >= 0; i-- {
			bytes[i]++
			if bytes[i] != 0 {
				break
			}
		}
		return netip.AddrFrom4(bytes)
	}
	bytes := addr.As16()
	for i := len(bytes) - 1; i >= 0; i-- {
		bytes[i]++
		if bytes[i] != 0 {
			break
		}
	}
	return netip.AddrFrom16(bytes)
}

func broadcastAddr(prefix netip.Prefix) (netip.Addr, bool) {
	if !prefix.Addr().Is4() {
		return netip.Addr{}, false
	}
	ones := prefix.Bits()
	if ones <= 0 || ones >= 32 {
		return netip.Addr{}, false
	}
	network := prefix.Masked().Addr()
	base := addr4ToUint32(network)
	hostBits := 32 - ones
	mask := uint32(1<<hostBits) - 1
	last := base | mask
	return addrFromUint32(last), true
}

func addr4ToUint32(addr netip.Addr) uint32 {
	b := addr.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
}

func addrFromUint32(v uint32) netip.Addr {
	return netip.AddrFrom4([4]byte{
		byte(v >> 24),
		byte(v >> 16),
		byte(v >> 8),
		byte(v),
	})
}

func resolveTargets(targets []string, opts *options) ([]*host, error) {
	var hosts []*host
	for _, target := range targets {
		entryHosts, err := resolveOne(target, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", target, err)
			continue
		}
		hosts = append(hosts, entryHosts...)
	}
	for i, h := range hosts {
		h.index = i
	}
	return hosts, nil
}

func resolveOne(target string, opts *options) ([]*host, error) {
	var addrs []netip.Addr
	if ip, err := netip.ParseAddr(target); err == nil {
		addrs = append(addrs, ip)
	} else {
		ips, err := net.LookupIP(target)
		if err != nil {
			return nil, err
		}
		for _, ip := range ips {
			if ip4 := ip.To4(); ip4 != nil {
				if opts.ipv6Only {
					continue
				}
				addrs = append(addrs, netip.AddrFrom4([4]byte(ip4)))
			} else {
				if opts.ipv4Only {
					continue
				}
				var arr [16]byte
				copy(arr[:], ip.To16())
				addrs = append(addrs, netip.AddrFrom16(arr))
			}
		}
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses for %s", target)
	}
	if !opts.multiAddr {
		addrs = addrs[:1]
	}
	var hosts []*host
	for _, addr := range addrs {
		h := &host{
			inputName: target,
			addr:      addr,
			ipVersion: 4,
			opts:      opts,
		}
		if addr.Is6() {
			h.ipVersion = 6
			if opts.iface != "" {
				h.zone = opts.iface
			}
		}
		h.display = formatDisplay(target, addr, opts)
		if opts.netdata {
			h.netdataName = sanitizeMetricName(target)
		}
		hosts = append(hosts, h)
	}
	return hosts, nil
}

func formatDisplay(name string, addr netip.Addr, opts *options) string {
	switch {
	case opts.showAddr:
		return addr.String()
	case opts.reverseLookup:
		if host, err := net.LookupAddr(addr.String()); err == nil && len(host) > 0 {
			return host[0]
		}
		return addr.String()
	case opts.nameLookup && net.ParseIP(name) != nil:
		if host, err := net.LookupAddr(addr.String()); err == nil && len(host) > 0 {
			return host[0]
		}
		return name
	default:
		return name
	}
}

func sanitizeMetricName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
