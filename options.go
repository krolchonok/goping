package main

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const (
	defaultInterval              = 10 * time.Millisecond
	defaultPerHost               = time.Second
	defaultTimeout               = 500 * time.Millisecond
	defaultRetry                 = 0
	defaultPacketSize            = 56
	defaultSeqmapTimeout         = 10 * time.Second
	maxGeneratedTargets          = 131072
	defaultBackoff       float64 = 1.5
	defaultTCPPort               = 80
)

type options struct {
	ipv4Only        bool
	ipv6Only        bool
	showAlive       bool
	showUnreach     bool
	showAddr        bool
	reverseLookup   bool
	nameLookup      bool
	packetSize      int
	backoff         float64
	count           int
	reportAllRtts   bool
	timestamp       bool
	timestampFormat string
	elapsed         bool
	targetFile      string
	generateSpec    *generateSpec
	ttl             int
	printTTL        bool
	printTOS        bool
	interval        time.Duration
	perHostInterval time.Duration
	iface           string
	json            bool
	progress        bool
	randomPayload   bool
	loop            bool
	multiAddr       bool
	dontFragment    bool
	netdata         bool
	outage          bool
	tos             int
	quiet           bool
	trafficStats    bool
	reportInterval  time.Duration
	cumulativeStats bool
	retry           int
	stats           bool
	sourceAddr4     string
	sourceAddr6     string
	seqmapTimeout   time.Duration
	timeout         time.Duration
	showHelp        bool
	showVersion     bool
	minReachable    int
	fastReachable   int
	checkSource     bool
	icmpTimestamp   bool
	generateCIDR    string
	generateStart   string
	generateEnd     string
	useGenerator    bool
	targetFileIsStd bool
	outFile         string
	locale          string
	tcpProbe        bool
	tcpPort         int
}

type generateSpec struct {
	start netip.Addr
	end   netip.Addr
	cidr  *netip.Prefix
}

func defaultOptions() *options {
	return &options{
		showAlive:       true,
		packetSize:      defaultPacketSize,
		backoff:         defaultBackoff,
		count:           0,
		interval:        defaultInterval,
		perHostInterval: defaultPerHost,
		retry:           defaultRetry,
		timeout:         defaultTimeout,
		seqmapTimeout:   defaultSeqmapTimeout,
		timestampFormat: "ctime",
		locale:          detectSystemLocale(),
		tcpPort:         defaultTCPPort,
	}
}

func parseArgs(args []string) (*options, []string, error) {
	opts := defaultOptions()
	var targets []string

	i := 0
	for i < len(args) {
		arg := args[i]
		if arg == "--" {
			targets = append(targets, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			targets = append(targets, arg)
			i++
			continue
		}

		if strings.HasPrefix(arg, "--") {
			name, val, hasVal := parseLongArg(arg)
			if !hasVal && i+1 < len(args) && needsValue(name) {
				val = args[i+1]
				i++
			}
			if err := applyLongOption(opts, name, val); err != nil {
				return nil, nil, err
			}
			i++
			continue
		}

		if err := parseShortBundle(opts, args, &i); err != nil {
			return nil, nil, err
		}
	}

	if opts.ipv4Only && opts.ipv6Only {
		return nil, nil, errors.New("cannot mix -4 and -6")
	}

	if opts.useGenerator {
		if opts.generateCIDR == "" {
			switch len(targets) {
			case 1:
				opts.generateCIDR = targets[0]
			case 2:
				opts.generateStart = targets[0]
				opts.generateEnd = targets[1]
			default:
				return nil, nil, errors.New("-g requires a CIDR or start/end pair")
			}
			targets = nil
		}
		spec, err := buildGenerateSpec(opts.generateCIDR, opts.generateStart, opts.generateEnd)
		if err != nil {
			return nil, nil, err
		}
		opts.generateSpec = spec
	}

	return opts, targets, nil
}

func parseLongArg(arg string) (string, string, bool) {
	if idx := strings.IndexRune(arg, '='); idx >= 0 {
		return arg[2:idx], arg[idx+1:], true
	}
	return arg[2:], "", false
}

func needsValue(name string) bool {
	switch name {
	case "size", "backoff", "count", "vcount", "timestamp-format", "file", "ttl", "interval",
		"iface", "fwmark", "period", "squiet", "retry", "src", "timeout",
		"reachable", "fast-reachable", "seqmap-timeout", "tcp-port":
		return true
	}
	return false
}

func applyLongOption(opts *options, name, val string) error {
	switch name {
	case "help":
		opts.showHelp = true
	case "version":
		opts.showVersion = true
	case "ipv4":
		opts.ipv4Only = true
	case "ipv6":
		opts.ipv6Only = true
	case "alive":
		opts.showAlive = true
	case "addr":
		opts.showAddr = true
	case "size":
		return setInt(val, &opts.packetSize)
	case "backoff":
		return setFloat(val, &opts.backoff)
	case "count":
		if err := setInt(val, &opts.count); err != nil {
			return err
		}
	case "vcount":
		if err := setInt(val, &opts.count); err != nil {
			return err
		}
		opts.reportAllRtts = true
	case "rdns":
		opts.reverseLookup = true
	case "timestamp":
		opts.timestamp = true
	case "timestamp-format":
		opts.timestampFormat = val
	case "elapsed":
		opts.elapsed = true
	case "file":
		opts.targetFile = val
	case "generate":
		opts.useGenerator = true
		opts.generateCIDR = val
	case "ttl":
		return setInt(val, &opts.ttl)
	case "interval":
		return setDuration(val, &opts.interval)
	case "iface":
		opts.iface = val
	case "json":
		opts.json = true
	case "progress":
		opts.progress = true
	case "icmp-timestamp":
		opts.icmpTimestamp = true
	case "loop":
		opts.loop = true
	case "all":
		opts.multiAddr = true
	case "dontfrag":
		opts.dontFragment = true
	case "name":
		opts.nameLookup = true
	case "netdata":
		opts.netdata = true
	case "outage":
		opts.outage = true
	case "tos":
		return setInt(val, &opts.tos)
	case "period":
		return setDuration(val, &opts.perHostInterval)
	case "quiet":
		opts.quiet = true
	case "traffic":
		opts.trafficStats = true
	case "squiet":
		dur, cumulative, err := parseReportIntervalSpec(val)
		if err != nil {
			return err
		}
		opts.reportInterval = dur
		opts.cumulativeStats = cumulative
	case "retry":
		return setInt(val, &opts.retry)
	case "random":
		opts.randomPayload = true
	case "stats":
		opts.stats = true
	case "src":
		return opts.setSourceAddr(val)
	case "seqmap-timeout":
		return setDuration(val, &opts.seqmapTimeout)
	case "timeout":
		return setDuration(val, &opts.timeout)
	case "unreach":
		opts.showUnreach = true
	case "reachable":
		return setInt(val, &opts.minReachable)
	case "fast-reachable":
		return setInt(val, &opts.fastReachable)
	case "check-source":
		opts.checkSource = true
	case "print-ttl":
		opts.printTTL = true
	case "print-tos":
		opts.printTOS = true
	case "tcp-probe":
		opts.tcpProbe = true
	case "tcp-port":
		return setInt(val, &opts.tcpPort)
	default:
		return fmt.Errorf("unknown option --%s", name)
	}
	return nil
}

func parseShortBundle(opts *options, args []string, idx *int) error {
	arg := args[*idx]
	for pos := 1; pos < len(arg); pos++ {
		ch := arg[pos]
		switch ch {
		case '4':
			opts.ipv4Only = true
		case '6':
			opts.ipv6Only = true
		case 'a':
			opts.showAlive = true
		case 'A':
			opts.showAddr = true
		case 'b':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setInt(val, &opts.packetSize); err != nil {
				return err
			}
		case 'B':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setFloat(val, &opts.backoff); err != nil {
				return err
			}
		case 'c':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setInt(val, &opts.count); err != nil {
				return err
			}
		case 'C':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setInt(val, &opts.count); err != nil {
				return err
			}
			opts.reportAllRtts = true
		case 'd':
			opts.reverseLookup = true
		case 'D':
			opts.timestamp = true
		case 'e':
			opts.elapsed = true
		case 'f':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			opts.targetFile = val
		case 'g':
			opts.useGenerator = true
		case 'h':
			opts.showHelp = true
		case 'H':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setInt(val, &opts.ttl); err != nil {
				return err
			}
		case 'i':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setDuration(val, &opts.interval); err != nil {
				return err
			}
		case 'I':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			opts.iface = val
		case 'J':
			opts.json = true
		case 'l':
			opts.loop = true
		case 'L':
			opts.trafficStats = true
		case 'm':
			opts.multiAddr = true
		case 'M':
			opts.dontFragment = true
		case 'n':
			opts.nameLookup = true
		case 'N':
			opts.netdata = true
		case 'o':
			opts.outage = true
		case 'O':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setInt(val, &opts.tos); err != nil {
				return err
			}
		case 'p':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setDuration(val, &opts.perHostInterval); err != nil {
				return err
			}
		case 'q':
			opts.quiet = true
		case 'P':
			opts.progress = true
		case 'Q':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			dur, cumulative, err := parseReportIntervalSpec(val)
			if err != nil {
				return err
			}
			opts.reportInterval = dur
			opts.cumulativeStats = cumulative
		case 'r':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setInt(val, &opts.retry); err != nil {
				return err
			}
		case 'R':
			opts.randomPayload = true
		case 's':
			opts.stats = true
		case 'S':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := opts.setSourceAddr(val); err != nil {
				return err
			}
		case 't':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setDuration(val, &opts.timeout); err != nil {
				return err
			}
		case 'u':
			opts.showUnreach = true
		case 'v':
			opts.showVersion = true
		case 'x':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setInt(val, &opts.minReachable); err != nil {
				return err
			}
		case 'X':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setInt(val, &opts.fastReachable); err != nil {
				return err
			}
		case 'y':
			opts.tcpProbe = true
		case 'Y':
			val, newPos, err := takeValue(arg, args, idx, pos)
			if err != nil {
				return err
			}
			pos = newPos
			if err := setInt(val, &opts.tcpPort); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown option -%c", ch)
		}
	}
	*idx = *idx + 1
	return nil
}

func takeValue(current string, args []string, idx *int, pos int) (string, int, error) {
	if pos+1 < len(current) {
		return current[pos+1:], len(current) - 1, nil
	}
	if *idx+1 < len(args) {
		*idx++
		return args[*idx], len(args[*idx]), nil
	}
	return "", pos, errors.New("missing option argument")
}

func setInt(val string, dst *int) error {
	if val == "" {
		return errors.New("missing value")
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return err
	}
	*dst = n
	return nil
}

func setFloat(val string, dst *float64) error {
	if val == "" {
		return errors.New("missing value")
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return err
	}
	*dst = f
	return nil
}

func setDuration(val string, dst *time.Duration) error {
	if val == "" {
		return errors.New("missing value")
	}
	if strings.Contains(val, "ms") || strings.Contains(val, "s") {
		d, err := time.ParseDuration(val)
		if err != nil {
			return err
		}
		*dst = d
		return nil
	}
	ms, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return err
	}
	*dst = time.Duration(ms * float64(time.Millisecond))
	return nil
}

func buildGenerateSpec(cidr, start, end string) (*generateSpec, error) {
	spec := &generateSpec{}
	if cidr != "" {
		pfx, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, err
		}
		spec.cidr = &pfx
		return spec, nil
	}
	if start == "" || end == "" {
		return nil, errors.New("-g requires start and end addresses")
	}
	s, err := netip.ParseAddr(start)
	if err != nil {
		return nil, err
	}
	e, err := netip.ParseAddr(end)
	if err != nil {
		return nil, err
	}
	if s.Is4() != e.Is4() {
		return nil, errors.New("start and end addresses must be the same family")
	}
	if s.Compare(e) > 0 {
		return nil, errors.New("start address is after end")
	}
	spec.start = s
	spec.end = e
	return spec, nil
}

func parseReportIntervalSpec(val string) (time.Duration, bool, error) {
	if val == "" {
		return 0, false, errors.New("missing value")
	}
	parts := strings.SplitN(val, ",", 2)
	dur, err := parseSecondsOrDuration(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0, false, err
	}
	cumulative := false
	if len(parts) == 2 {
		cumulative = strings.EqualFold(strings.TrimSpace(parts[1]), "cumulative")
	}
	return dur, cumulative, nil
}

func parseSecondsOrDuration(part string) (time.Duration, error) {
	if part == "" {
		return 0, errors.New("missing value")
	}
	if strings.ContainsAny(part, "hms") {
		return time.ParseDuration(part)
	}
	sec, err := strconv.ParseFloat(part, 64)
	if err != nil {
		return 0, err
	}
	if sec < 0 {
		return 0, errors.New("interval must be non-negative")
	}
	return time.Duration(sec * float64(time.Second)), nil
}

func (o *options) setSourceAddr(val string) error {
	if val == "" {
		return errors.New("missing value")
	}
	if addr, err := netip.ParseAddr(val); err == nil {
		o.assignSourceAddr(addr)
		return nil
	}
	ips, err := net.LookupIP(val)
	if err != nil {
		return err
	}
	assigned := false
	for _, ip := range ips {
		if ip4 := ip.To4(); ip4 != nil {
			o.sourceAddr4 = net.IP(ip4).String()
			assigned = true
		} else if ip.To16() != nil {
			o.sourceAddr6 = ip.String()
			assigned = true
		}
	}
	if !assigned {
		return fmt.Errorf("no usable addresses found for %s", val)
	}
	return nil
}

func (o *options) assignSourceAddr(addr netip.Addr) {
	if addr.Is4() {
		o.sourceAddr4 = addr.String()
	} else {
		o.sourceAddr6 = addr.String()
	}
}

func (o *options) localAddr(family int) string {
	if family == 4 {
		return o.sourceAddr4
	}
	return o.sourceAddr6
}

func (o *options) timestampPrefix(ts time.Time) string {
	if !o.timestamp {
		return ""
	}
	switch o.timestampFormat {
	case "iso":
		return ts.Format(time.RFC3339)
	case "rfc3339":
		return ts.Format("2006-01-02 15:04:05")
	default:
		return ts.Format(time.ANSIC)
	}
}
