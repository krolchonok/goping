package main

import "fmt"

const version = "5.0.0-go"

func versionString() string {
	return fmt.Sprintf("goping %s (Go rewrite)", version)
}

func printUsage() {
	fmt.Println("Usage: goping [options] target [target...]")
	fmt.Println("  -4, --ipv4           Use IPv4 only")
	fmt.Println("  -6, --ipv6           Use IPv6 only")
	fmt.Println("  -a, --alive          Show alive hosts")
	fmt.Println("  -A, --addr           Show resolved address instead of name")
	fmt.Println("  -b, --size=BYTES     Set payload size (default 56)")
	fmt.Println("  -B, --backoff=N      Backoff factor for retries (default 1.5)")
	fmt.Println("  -c, --count=N        Number of requests to send to each target")
	fmt.Println("  -C, --vcount=N       Same as -c but prints per-request RTTs")
	fmt.Println("  -d, --rdns           Force reverse DNS lookups")
	fmt.Println("  -D, --timestamp      Print timestamps on output")
	fmt.Println("  -e, --elapsed        Show elapsed time for each reply")
	fmt.Println("  -f, --file=FILE      Read targets from file ('-' for stdin)")
	fmt.Println("  -g, --generate NET   Generate targets from CIDR or start/end")
	fmt.Println("  -i, --interval=MSEC  Global interval between packets (default 10ms)")
	fmt.Println("  -l, --loop           Loop sending pings indefinitely")
	fmt.Println("  -L, --traffic        Show traffic statistics for the whole scan")
	fmt.Println("  -m, --all            Ping all addresses returned for a host")
	fmt.Println("  -n, --name           Resolve numeric targets to hostnames")
	fmt.Println("  -o, --outage         Print outage counters")
	fmt.Println("  -p, --period=MSEC    Per-host interval (default 1000ms)")
	fmt.Println("  -P, --progress       Show a progress bar while hosts finish")
	fmt.Println("  -q, --quiet          Quiet output")
	fmt.Println("  -Q, --squiet=SEC     Print periodic summaries")
	fmt.Println("  -r, --retry=N        Retries for unreachable hosts (default 3)")
	fmt.Println("  -R, --random         Fill payload with random data")
	fmt.Println("  -s, --stats          Print global statistics")
	fmt.Println("  -S, --src=ADDR       Use the given source address")
	fmt.Println("  -t, --timeout=MSEC   Initial timeout (default 500ms)")
	fmt.Println("  -u, --unreach        Show unreachable hosts")
	fmt.Println("  -v, --version        Show version information")
	fmt.Println("  -x, --reachable=N    Exit successfully if at least N hosts are reachable")
	fmt.Println("  -X, --fast-reachable=N  Exit as soon as N hosts are reachable")
}
