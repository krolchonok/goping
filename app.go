package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type app struct {
	opts            *options
	hosts           []*host
	socket          *socketManager
	rateLimiter     *rate.Limiter
	startTime       time.Time
	stats           globalStats
	reportMu        sync.Mutex
	statsMu         sync.Mutex
	cancel          context.CancelFunc
	printMu         sync.Mutex
	progressMu      sync.Mutex
	ppsMu           sync.Mutex
	netdataSent     bool
	jsonEncoder     *json.Encoder
	progressActive  bool
	progressTotal   int
	progressDone    int
	progressLastLen int
	ppsSampleSent   int
	ppsSampleTime   time.Time
	currentPps      float64
	maxPps          float64
}

type host struct {
	index       int
	inputName   string
	display     string
	addr        netip.Addr
	ipVersion   int
	icmpID      int
	replyCh     chan reply
	stats       hostStats
	opts        *options
	done        bool
	reverseDNS  string
	mu          sync.RWMutex
	zone        string
	netdataName string
}

const (
	progressBarWidth     = 30
	packetOverheadBytes  = 28
	ipv4HeaderBytes      = 20
	ipv6HeaderBytes      = 40
	ppsMinSampleInterval = 200 * time.Millisecond
)

type reply struct {
	recvTime   time.Time
	size       int
	src        net.Addr
	seq        int
	ttl        int
	tos        int
	timestamps *timestampInfo
}

type timestampInfo struct {
	originate uint32
	receive   uint32
	transmit  uint32
}

type globalStats struct {
	sent        int
	recv        int
	sentBytes   int
	recvBytes   int
	timeouts    int
	noAddress   int
	alive       int
	unreachable int
}

type hostStats struct {
	sent          int
	recv          int
	timeouts      int
	totalRTT      time.Duration
	minRTT        time.Duration
	maxRTT        time.Duration
	perPing       []time.Duration
	intervalSent  int
	intervalRecv  int
	intervalTotal time.Duration
	intervalMin   time.Duration
	intervalMax   time.Duration
	alive         bool
}

func newApp(opts *options) (*app, error) {
	if err := applyInterfaceSource(opts); err != nil {
		return nil, err
	}
	if opts.icmpTimestamp && opts.ipv6Only {
		return nil, errors.New("ICMP timestamp mode is available for IPv4 only")
	}
	a := &app{
		opts:        opts,
		startTime:   time.Now(),
		rateLimiter: rate.NewLimiter(rate.Every(opts.interval), 1),
	}
	if opts.json {
		a.jsonEncoder = json.NewEncoder(os.Stdout)
	}
	socket, err := newSocketManager(opts)
	if err != nil {
		return nil, err
	}
	a.socket = socket
	return a, nil
}

func (a *app) loadTargets(targets []string) error {
	if a.opts.targetFile != "" {
		entries, err := readTargetFile(a.opts.targetFile)
		if err != nil {
			return err
		}
		targets = append(targets, entries...)
	}
	if a.opts.generateSpec != nil {
		genTargets, err := generateFromSpec(a.opts.generateSpec, a.opts.ipv4Only, a.opts.ipv6Only)
		if err != nil {
			return err
		}
		targets = append(targets, genTargets...)
	}
	if len(targets) == 0 {
		return errors.New("no targets provided")
	}
	hostEntries, err := resolveTargets(targets, a.opts)
	if err != nil {
		return err
	}
	if len(hostEntries) == 0 {
		return errors.New("no usable targets")
	}
	if len(hostEntries) > 0xffff {
		return fmt.Errorf("too many targets (%d), limit 65535", len(hostEntries))
	}
	a.hosts = hostEntries
	a.socket.assignIDs(a.hosts)
	return nil
}

func (a *app) initProgressBar() {
	if !a.opts.progress {
		return
	}
	if a.opts.loop {
		fmt.Fprintln(os.Stderr, "--progress ignored when --loop is set")
		return
	}
	if len(a.hosts) == 0 {
		return
	}
	a.progressMu.Lock()
	a.progressActive = true
	a.progressTotal = len(a.hosts)
	a.progressDone = 0
	a.progressLastLen = 0
	a.renderProgressLocked()
	a.progressMu.Unlock()
}

func (a *app) hostCompleted(h *host) {
	if h == nil {
		return
	}
	h.mu.Lock()
	h.done = true
	h.mu.Unlock()
	a.progressMu.Lock()
	defer a.progressMu.Unlock()
	if !a.progressActive {
		return
	}
	a.progressDone++
	a.renderProgressLocked()
}

func (a *app) renderProgressLocked() {
	if !a.progressActive || a.progressTotal <= 0 {
		return
	}
	pps := a.currentPpsValue()
	if pps == 0 {
		pps = a.averagePacketsPerSecond()
	}
	ratio := float64(a.progressDone) / float64(a.progressTotal)
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	width := progressBarWidth
	filled := int(ratio * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("=", filled) + strings.Repeat(" ", width-filled)
	line := fmt.Sprintf("Progress: [%s] %d/%d (%.0f%%) %.1f pps", bar, a.progressDone, a.progressTotal, ratio*100, pps)
	if len(line) < a.progressLastLen {
		line += strings.Repeat(" ", a.progressLastLen-len(line))
	}
	a.progressLastLen = len(line)
	fmt.Fprintf(os.Stderr, "\r%s", line)
	if a.progressDone >= a.progressTotal {
		fmt.Fprintln(os.Stderr)
		a.progressActive = false
		a.progressLastLen = 0
	}
}

func (a *app) withStdout(fn func()) {
	if fn == nil {
		return
	}
	a.prepareProgressForPrint()
	a.printMu.Lock()
	fn()
	a.printMu.Unlock()
	a.resumeProgressAfterPrint()
}

func (a *app) prepareProgressForPrint() {
	if !a.opts.progress {
		return
	}
	a.progressMu.Lock()
	a.clearProgressLocked()
	a.progressMu.Unlock()
}

func (a *app) resumeProgressAfterPrint() {
	if !a.opts.progress {
		return
	}
	a.progressMu.Lock()
	if a.progressActive {
		a.renderProgressLocked()
	}
	a.progressMu.Unlock()
}

func (a *app) clearProgressLocked() {
	if !a.progressActive && a.progressLastLen == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "\r%s\r", strings.Repeat(" ", a.progressLastLen))
}

func (a *app) currentPpsValue() float64 {
	a.ppsMu.Lock()
	defer a.ppsMu.Unlock()
	return a.currentPps
}

func (a *app) averagePacketsPerSecond() float64 {
	elapsed := time.Since(a.startTime)
	if elapsed <= 0 {
		return 0
	}
	a.statsMu.Lock()
	sent := a.stats.sent
	a.statsMu.Unlock()
	return float64(sent) / elapsed.Seconds()
}

func (a *app) stderrf(format string, args ...interface{}) {
	if !a.opts.progress {
		fmt.Fprintf(os.Stderr, format, args...)
		return
	}
	a.progressMu.Lock()
	a.clearProgressLocked()
	a.progressMu.Unlock()
	fmt.Fprintf(os.Stderr, format, args...)
	a.progressMu.Lock()
	if a.progressActive {
		a.renderProgressLocked()
	}
	a.progressMu.Unlock()
}

func (a *app) run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel
	defer cancel()

	if err := a.socket.start(ctx); err != nil {
		return err
	}

	var wg sync.WaitGroup
	for _, h := range a.hosts {
		wg.Add(1)
		go func(h *host) {
			defer wg.Done()
			h.run(ctx, a)
		}(h)
	}

	if a.opts.reportInterval > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.reportLoop(ctx)
		}()
	}

	wg.Wait()
	return nil
}

func (a *app) waitInterval(ctx context.Context) error {
	if a.opts.interval <= 0 {
		return nil
	}
	return a.rateLimiter.Wait(ctx)
}

func (a *app) printFinalStats() {
	if a.opts.json {
		if a.opts.count > 0 || a.opts.loop || !a.opts.quiet {
			for _, h := range a.hosts {
				h.printSummaryJSON(a)
			}
		}
		if a.opts.stats {
			a.printGlobalStatsJSON()
		}
		if a.opts.showUnreach {
			for _, h := range a.hosts {
				if !h.stats.alive {
					a.emitJSON("unreach", map[string]interface{}{
						"host": h.display,
					})
				}
			}
		}
		return
	}

	if a.opts.stats {
		fmt.Printf("\n%7d targets\n", len(a.hosts))
		fmt.Printf(" %7d alive\n", a.stats.alive)
		fmt.Printf(" %7d unreachable\n", a.stats.unreachable)
		fmt.Printf(" %7d timeouts\n", a.stats.timeouts)
		fmt.Printf(" %7d ICMP echo sent\n", a.stats.sent)
		fmt.Printf(" %7d ICMP echo received\n", a.stats.recv)
	}
	if a.opts.count > 0 || a.opts.loop || !a.opts.quiet {
		for _, h := range a.hosts {
			h.printSummary(a.opts)
		}
	}
	if a.opts.showUnreach {
		for _, h := range a.hosts {
			if !h.stats.alive {
				fmt.Printf("%s is unreachable\n", h.display)
			}
		}
	}
	a.printPpsSummary()
}

func (a *app) reportLoop(ctx context.Context) {
	ticker := time.NewTicker(a.opts.reportInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.printIntervalStats()
		}
	}
}

func (a *app) maxPpsValue() float64 {
	a.ppsMu.Lock()
	peak := a.maxPps
	a.ppsMu.Unlock()
	if peak > 0 {
		return peak
	}
	return a.averagePacketsPerSecond()
}

func (a *app) printPpsSummary() {
	if !a.opts.trafficStats {
		peak := a.maxPpsValue()
		if peak <= 0 {
			return
		}
		mbps := a.mbpsForRate(peak)
		a.withStdout(func() {
			fmt.Printf("Peak throughput: %.1f pps (%.2f Mbps)\n", peak, mbps)
		})
		return
	}

	runtime := time.Since(a.startTime)
	if runtime <= 0 {
		return
	}
	sentPps := a.averagePacketsPerSecond()
	recvPps := a.averageRecvPacketsPerSecond()
	totalPps := sentPps + recvPps

	a.statsMu.Lock()
	sentPackets := a.stats.sent
	recvPackets := a.stats.recv
	sentBytes := a.stats.sentBytes
	recvBytes := a.stats.recvBytes
	a.statsMu.Unlock()

	sentMbps := rateFromBytes(sentBytes, runtime)
	recvMbps := rateFromBytes(recvBytes, runtime)
	totalMbps := rateFromBytes(sentBytes+recvBytes, runtime)
	peak := a.maxPpsValue()
	peakMbps := a.mbpsForRate(peak)

	if sentPackets == 0 && recvPackets == 0 && peak <= 0 {
		return
	}

	a.withStdout(func() {
		if sentPackets > 0 || recvPackets > 0 {
			fmt.Printf("Packets: sent %d, recv %d (runtime %.2fs)\n", sentPackets, recvPackets, runtime.Seconds())
			fmt.Printf("Average throughput: sent %.1f pps (%.2f Mbps), recv %.1f pps (%.2f Mbps)\n", sentPps, sentMbps, recvPps, recvMbps)
			fmt.Printf("Average network load: %.2f Mbps combined (%.1f total pps)\n", totalMbps, totalPps)
		}
		if peak > 0 {
			fmt.Printf("Peak throughput: %.1f pps (%.2f Mbps)\n", peak, peakMbps)
		}
	})
}

func (a *app) averageRecvPacketsPerSecond() float64 {
	elapsed := time.Since(a.startTime)
	if elapsed <= 0 {
		return 0
	}
	a.statsMu.Lock()
	recv := a.stats.recv
	a.statsMu.Unlock()
	return float64(recv) / elapsed.Seconds()
}

func (a *app) mbpsForRate(pps float64) float64 {
	if pps <= 0 {
		return 0
	}
	payload := a.opts.packetSize + packetOverheadBytes
	if payload <= 0 {
		payload = a.opts.packetSize
	}
	if payload < 0 {
		payload = 0
	}
	return pps * float64(payload) * 8 / 1_000_000
}

func rateFromBytes(bytes int, duration time.Duration) float64 {
	if bytes <= 0 || duration <= 0 {
		return 0
	}
	return (float64(bytes) * 8) / duration.Seconds() / 1_000_000
}

func ipHeaderBytesFor(version int) int {
	if version == 6 {
		return ipv6HeaderBytes
	}
	return ipv4HeaderBytes
}

func (a *app) printIntervalStats() {
	a.reportMu.Lock()
	defer a.reportMu.Unlock()
	now := time.Now()
	if a.opts.netdata {
		a.printNetdata(now)
		return
	}
	if a.opts.json {
		for _, h := range a.hosts {
			h.printIntervalJSON(a, now)
			if !a.opts.cumulativeStats {
				h.resetIntervalStats()
			}
		}
		return
	}
	a.withStdout(func() {
		fmt.Printf("\n[%s]\n", now.Format("15:04:05"))
		for _, h := range a.hosts {
			h.printInterval(a.opts)
			if !a.opts.cumulativeStats {
				h.resetIntervalStats()
			}
		}
	})
}

func (h *host) run(ctx context.Context, a *app) {
	defer a.hostCompleted(h)
	timeout := a.opts.timeout
	attempt := 0

	for {
		if err := a.waitInterval(ctx); err != nil {
			return
		}

		sendTime := time.Now()
		sentBytes, err := a.socket.send(h, attempt, a.opts)
		if err != nil {
			a.stderrf("%s: %v\n", h.display, err)
			return
		}
		h.recordSend(attempt, a.opts)
		if sentBytes > 0 {
			sentBytes += ipHeaderBytesFor(h.ipVersion)
		}
		a.incSent(sentBytes)

		timer := time.NewTimer(timeout)
		handled := false
		for !handled {
			select {
			case rep := <-h.replyCh:
				if rep.seq != attempt {
					continue
				}
				if a.opts.seqmapTimeout > 0 && rep.recvTime.Sub(sendTime) > a.opts.seqmapTimeout {
					continue
				}
				timer.Stop()
				rtt := rep.recvTime.Sub(sendTime)
				h.recordReply(a, attempt, rep, rtt)
				recvBytes := rep.size
				if recvBytes > 0 {
					recvBytes += ipHeaderBytesFor(h.ipVersion)
				}
				a.incRecv(recvBytes)
				if !h.isAlive() {
					a.markAlive()
				}
				if !a.opts.loop && a.opts.count == 0 {
					return
				}
				handled = true
			case <-timer.C:
				h.recordTimeout(a, attempt)
				a.incTimeout()
				handled = true
				if a.opts.count == 0 && attempt >= a.opts.retry {
					a.markUnreachable()
					return
				}
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}

		attempt++
		if h.shouldStop(attempt, a.opts) {
			if a.opts.loop {
				attempt = 0
				if !a.opts.cumulativeStats {
					h.resetIntervalStats()
				}
				continue
			}
			if !h.isAlive() {
				a.markUnreachable()
			}
			return
		}

		if a.opts.perHostInterval > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(a.opts.perHostInterval):
			}
		}

		if a.opts.count == 0 {
			timeout = time.Duration(float64(timeout) * a.opts.backoff)
			if timeout > 2*time.Second {
				timeout = 2 * time.Second
			}
		} else {
			timeout = a.opts.timeout
		}
	}
}

func (h *host) shouldStop(attempt int, opts *options) bool {
	if opts.count > 0 {
		return attempt >= opts.count
	}
	if h.stats.alive && !opts.loop {
		return true
	}
	return false
}

func (h *host) recordSend(idx int, opts *options) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.sent++
	h.stats.intervalSent++
	if opts.reportAllRtts && opts.count > 0 {
		if h.stats.perPing == nil {
			h.stats.perPing = make([]time.Duration, opts.count)
			for i := range h.stats.perPing {
				h.stats.perPing[i] = -1
			}
		}
		h.stats.perPing[idx%len(h.stats.perPing)] = -1
	}
}

func (h *host) recordReply(a *app, idx int, rep reply, rtt time.Duration) {
	h.mu.Lock()
	h.stats.recv++
	h.stats.intervalRecv++
	h.stats.totalRTT += rtt
	h.stats.intervalTotal += rtt
	if h.stats.minRTT == 0 || rtt < h.stats.minRTT {
		h.stats.minRTT = rtt
	}
	if rtt > h.stats.maxRTT {
		h.stats.maxRTT = rtt
	}
	if h.stats.intervalMin == 0 || rtt < h.stats.intervalMin {
		h.stats.intervalMin = rtt
	}
	if rtt > h.stats.intervalMax {
		h.stats.intervalMax = rtt
	}
	if h.stats.perPing != nil {
		h.stats.perPing[idx%len(h.stats.perPing)] = rtt
	}
	if !h.stats.alive {
		h.stats.alive = true
	}
	h.mu.Unlock()

	if h.opts.json {
		h.printReplyJSON(a, idx, rep, rtt)
		return
	}

	if h.opts.quiet {
		if h.opts.showAlive {
			a.withStdout(func() {
				fmt.Println(h.display)
			})
		}
		return
	}

	a.withStdout(func() {
		prefix := h.opts.timestampPrefix(rep.recvTime)
		if prefix != "" {
			fmt.Printf("%s ", prefix)
		}
		fmt.Printf("%s : [%d], %d bytes, %.2f ms", h.display, idx, rep.size, float64(rtt.Microseconds())/1000.0)
		if h.opts.elapsed {
			fmt.Printf(" (elapsed %.2f ms)", float64(rtt.Microseconds())/1000.0)
		}
		if h.opts.printTTL {
			if rep.ttl >= 0 {
				fmt.Printf(" (TTL %d)", rep.ttl)
			} else {
				fmt.Printf(" (TTL unknown)")
			}
		}
		if h.opts.printTOS {
			if rep.tos >= 0 {
				fmt.Printf(" (TOS %d)", rep.tos)
			} else {
				fmt.Printf(" (TOS unknown)")
			}
		}
		if rep.timestamps != nil {
			fmt.Printf(" timestamps: Originate=%d Receive=%d Transmit=%d Localreceive=%d",
				rep.timestamps.originate, rep.timestamps.receive, rep.timestamps.transmit, msSinceMidnight(rep.recvTime))
		}
		fmt.Println()
	})
}

func (h *host) recordTimeout(a *app, idx int) {
	h.mu.Lock()
	h.stats.timeouts++
	h.mu.Unlock()

	if h.opts.json {
		h.printTimeoutJSON(a, idx)
		return
	}
	if h.opts.quiet {
		return
	}
	a.withStdout(func() {
		prefix := h.opts.timestampPrefix(time.Now())
		if prefix != "" {
			fmt.Printf("%s ", prefix)
		}
		fmt.Printf("%s : [%d], timed out\n", h.display, idx)
	})
}

func (h *host) resetIntervalStats() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stats.intervalSent = 0
	h.stats.intervalRecv = 0
	h.stats.intervalTotal = 0
	h.stats.intervalMin = 0
	h.stats.intervalMax = 0
}

func (h *host) printSummary(opts *options) {
	if opts.json {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if opts.reportAllRtts && len(h.stats.perPing) > 0 {
		fmt.Printf("%s :", h.display)
		for _, v := range h.stats.perPing {
			if v <= 0 {
				fmt.Print(" -")
			} else {
				fmt.Printf(" %.2f", float64(v.Microseconds())/1000.0)
			}
		}
		fmt.Println()
		return
	}
	if opts.quiet && h.stats.recv == 0 && !opts.showUnreach {
		return
	}
	loss := 0
	if h.stats.sent > 0 {
		loss = (h.stats.sent - h.stats.recv) * 100 / h.stats.sent
	}
	fmt.Printf("%s : xmt/rcv/%%loss = %d/%d/%d%%", h.display, h.stats.sent, h.stats.recv, loss)
	if opts.outage && opts.perHostInterval > 0 {
		fmt.Printf(", outage(ms) = %d", outageMillis(h.stats.sent, h.stats.recv, opts.perHostInterval))
	}
	if h.stats.recv > 0 {
		avg := h.stats.totalRTT / time.Duration(h.stats.recv)
		fmt.Printf(", min/avg/max = %.2f/%.2f/%.2f\n",
			float64(h.stats.minRTT.Microseconds())/1000.0,
			float64(avg.Microseconds())/1000.0,
			float64(h.stats.maxRTT.Microseconds())/1000.0,
		)
	} else {
		fmt.Println()
	}
}

func (h *host) printInterval(opts *options) {
	if opts.json {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.stats.intervalSent == 0 {
		fmt.Printf("%s : no data\n", h.display)
		return
	}
	loss := 0
	if h.stats.intervalSent > 0 {
		loss = (h.stats.intervalSent - h.stats.intervalRecv) * 100 / h.stats.intervalSent
	}
	fmt.Printf("%s : xmt/rcv/%%loss = %d/%d/%d%%", h.display, h.stats.intervalSent, h.stats.intervalRecv, loss)
	if opts.outage && opts.perHostInterval > 0 {
		fmt.Printf(", outage(ms) = %d", outageMillis(h.stats.intervalSent, h.stats.intervalRecv, opts.perHostInterval))
	}
	if h.stats.intervalRecv > 0 {
		avg := h.stats.intervalTotal / time.Duration(h.stats.intervalRecv)
		fmt.Printf(", min/avg/max = %.2f/%.2f/%.2f\n",
			float64(h.stats.intervalMin.Microseconds())/1000.0,
			float64(avg.Microseconds())/1000.0,
			float64(h.stats.intervalMax.Microseconds())/1000.0,
		)
	} else {
		fmt.Println()
	}
}

func (h *host) isAlive() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.stats.alive
}

func (a *app) incSent(bytes int) {
	a.statsMu.Lock()
	a.stats.sent++
	a.stats.sentBytes += bytes
	sent := a.stats.sent
	a.statsMu.Unlock()
	a.updatePpsEstimate(sent)
}

func (a *app) updatePpsEstimate(sent int) {
	a.ppsMu.Lock()
	defer a.ppsMu.Unlock()
	now := time.Now()
	if a.ppsSampleTime.IsZero() {
		a.ppsSampleTime = now
		a.ppsSampleSent = sent
		return
	}
	elapsed := now.Sub(a.ppsSampleTime)
	if elapsed < ppsMinSampleInterval {
		return
	}
	delta := sent - a.ppsSampleSent
	if delta <= 0 {
		a.ppsSampleTime = now
		a.ppsSampleSent = sent
		return
	}
	pps := float64(delta) / elapsed.Seconds()
	a.currentPps = pps
	if pps > a.maxPps {
		a.maxPps = pps
	}
	a.ppsSampleTime = now
	a.ppsSampleSent = sent
}

func (a *app) incRecv(bytes int) {
	a.statsMu.Lock()
	a.stats.recv++
	a.stats.recvBytes += bytes
	a.statsMu.Unlock()
}

func (a *app) incTimeout() {
	a.statsMu.Lock()
	a.stats.timeouts++
	a.statsMu.Unlock()
}

func (a *app) markAlive() {
	a.statsMu.Lock()
	a.stats.alive++
	if a.opts.fastReachable > 0 && a.stats.alive >= a.opts.fastReachable {
		if a.cancel != nil {
			a.cancel()
		}
	}
	a.statsMu.Unlock()
}

func (a *app) markUnreachable() {
	a.statsMu.Lock()
	a.stats.unreachable++
	a.statsMu.Unlock()
}

func (a *app) emitJSON(event string, payload interface{}) {
	if a.jsonEncoder == nil {
		a.jsonEncoder = json.NewEncoder(os.Stdout)
	}
	a.withStdout(func() {
		if err := a.jsonEncoder.Encode(map[string]interface{}{event: payload}); err != nil {
			a.stderrf("json encode error: %v\n", err)
		}
	})
}

func (a *app) printGlobalStatsJSON() {
	payload := map[string]interface{}{
		"targets":     len(a.hosts),
		"alive":       a.stats.alive,
		"unreachable": a.stats.unreachable,
		"timeouts":    a.stats.timeouts,
		"sent":        a.stats.sent,
		"received":    a.stats.recv,
	}
	a.emitJSON("stats", payload)
}

func (h *host) printReplyJSON(a *app, idx int, rep reply, rtt time.Duration) {
	payload := map[string]interface{}{
		"host":   h.display,
		"seq":    idx,
		"size":   rep.size,
		"rtt_ms": float64(rtt.Microseconds()) / 1000.0,
	}
	if rep.ttl >= 0 {
		payload["ttl"] = rep.ttl
	} else if h.opts.printTTL {
		payload["ttl"] = nil
	}
	if rep.tos >= 0 {
		payload["tos"] = rep.tos
	} else if h.opts.printTOS {
		payload["tos"] = nil
	}
	if rep.timestamps != nil {
		payload["timestamps"] = map[string]uint32{
			"originate": rep.timestamps.originate,
			"receive":   rep.timestamps.receive,
			"transmit":  rep.timestamps.transmit,
			"local":     msSinceMidnight(rep.recvTime),
		}
	}
	if h.opts.elapsed {
		payload["elapsed_ms"] = float64(rtt.Microseconds()) / 1000.0
	}
	ts := h.opts.timestampPrefix(rep.recvTime)
	if ts != "" {
		payload["timestamp"] = ts
	}
	a.emitJSON("resp", payload)
}

func (h *host) printTimeoutJSON(a *app, idx int) {
	payload := map[string]interface{}{
		"host": h.display,
		"seq":  idx,
	}
	ts := h.opts.timestampPrefix(time.Now())
	if ts != "" {
		payload["timestamp"] = ts
	}
	a.emitJSON("timeout", payload)
}

func (h *host) printSummaryJSON(a *app) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.opts.reportAllRtts && len(h.stats.perPing) > 0 {
		values := make([]interface{}, len(h.stats.perPing))
		for i, v := range h.stats.perPing {
			if v <= 0 {
				values[i] = nil
			} else {
				values[i] = float64(v.Microseconds()) / 1000.0
			}
		}
		a.emitJSON("vSum", map[string]interface{}{
			"host":   h.display,
			"values": values,
		})
		return
	}
	loss := 0
	if h.stats.sent > 0 {
		loss = (h.stats.sent - h.stats.recv) * 100 / h.stats.sent
	}
	payload := map[string]interface{}{
		"host": h.display,
		"xmt":  h.stats.sent,
		"rcv":  h.stats.recv,
		"loss": loss,
	}
	if h.opts.outage && h.opts.perHostInterval > 0 {
		payload["outage_ms"] = outageMillis(h.stats.sent, h.stats.recv, h.opts.perHostInterval)
	}
	if h.stats.recv > 0 {
		avg := h.stats.totalRTT / time.Duration(h.stats.recv)
		payload["rttMin"] = float64(h.stats.minRTT.Microseconds()) / 1000.0
		payload["rttAvg"] = float64(avg.Microseconds()) / 1000.0
		payload["rttMax"] = float64(h.stats.maxRTT.Microseconds()) / 1000.0
	}
	a.emitJSON("summary", payload)
}

func (h *host) printIntervalJSON(a *app, now time.Time) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	payload := map[string]interface{}{
		"time": now.Unix(),
		"host": h.display,
		"xmt":  h.stats.intervalSent,
		"rcv":  h.stats.intervalRecv,
	}
	if h.stats.intervalSent > 0 {
		payload["loss"] = (h.stats.intervalSent - h.stats.intervalRecv) * 100 / h.stats.intervalSent
	} else {
		payload["loss"] = 0
	}
	if h.opts.outage && h.opts.perHostInterval > 0 {
		payload["outage_ms"] = outageMillis(h.stats.intervalSent, h.stats.intervalRecv, h.opts.perHostInterval)
	}
	if h.stats.intervalRecv > 0 {
		avg := h.stats.intervalTotal / time.Duration(h.stats.intervalRecv)
		payload["rttMin"] = float64(h.stats.intervalMin.Microseconds()) / 1000.0
		payload["rttAvg"] = float64(avg.Microseconds()) / 1000.0
		payload["rttMax"] = float64(h.stats.intervalMax.Microseconds()) / 1000.0
	}
	a.emitJSON("intSum", payload)
}

func (a *app) printNetdata(now time.Time) {
	if a.opts.reportInterval <= 0 {
		return
	}
	a.withStdout(func() {
		period := float64(a.opts.reportInterval) / float64(time.Second)
		for _, h := range a.hosts {
			if h.netdataName == "" {
				h.netdataName = sanitizeMetricName(h.display)
			}
			if !a.netdataSent {
				fmt.Printf("CHART goping.%s_packets '' 'goping Packets' packets '%s' goping.packets line 110020 %.0f\n", h.netdataName, h.display, period)
				fmt.Println("DIMENSION xmt sent absolute 1 1")
				fmt.Println("DIMENSION rcv received absolute 1 1")
			}
			fmt.Printf("BEGIN goping.%s_packets\n", h.netdataName)
			fmt.Printf("SET xmt = %d\n", h.stats.intervalSent)
			fmt.Printf("SET rcv = %d\n", h.stats.intervalRecv)
			fmt.Println("END")

			if !a.netdataSent {
				fmt.Printf("CHART goping.%s_quality '' 'goping Quality' percentage '%s' goping.quality area 110010 %.0f\n", h.netdataName, h.display, period)
				fmt.Println("DIMENSION returned '' absolute 1 1")
			}
			fmt.Printf("BEGIN goping.%s_quality\n", h.netdataName)
			returned := 0
			if h.stats.intervalSent > 0 {
				returned = (h.stats.intervalRecv * 100) / h.stats.intervalSent
			}
			fmt.Printf("SET returned = %d\n", returned)
			fmt.Println("END")

			if !a.netdataSent {
				fmt.Printf("CHART goping.%s_latency '' 'goping Latency' ms '%s' goping.latency area 110000 %.0f\n", h.netdataName, h.display, period)
				fmt.Println("DIMENSION min minimum absolute 1 1000000")
				fmt.Println("DIMENSION max maximum absolute 1 1000000")
				fmt.Println("DIMENSION avg average absolute 1 1000000")
			}
			fmt.Printf("BEGIN goping.%s_latency\n", h.netdataName)
			if h.stats.intervalRecv > 0 {
				avg := h.stats.intervalTotal / time.Duration(h.stats.intervalRecv)
				fmt.Printf("SET min = %d\n", h.stats.intervalMin.Microseconds())
				fmt.Printf("SET avg = %d\n", avg.Microseconds())
				fmt.Printf("SET max = %d\n", h.stats.intervalMax.Microseconds())
			}
			fmt.Println("END")

			if !a.opts.cumulativeStats {
				h.resetIntervalStats()
			}
		}
		a.netdataSent = true
	})
}

func outageMillis(sent, recv int, interval time.Duration) int {
	if interval <= 0 || sent <= recv {
		return 0
	}
	diff := sent - recv
	ms := float64(interval) / float64(time.Millisecond)
	return int(float64(diff) * ms)
}

func applyInterfaceSource(opts *options) error {
	if opts.iface == "" {
		return nil
	}
	iface, err := net.InterfaceByName(opts.iface)
	if err != nil {
		return err
	}
	addrs, err := iface.Addrs()
	if err != nil {
		return err
	}
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if ip4 := ip.To4(); ip4 != nil && opts.sourceAddr4 == "" {
			opts.sourceAddr4 = net.IP(ip4).String()
		} else if ip.To16() != nil && opts.sourceAddr6 == "" {
			opts.sourceAddr6 = ip.String()
		}
	}
	return nil
}
