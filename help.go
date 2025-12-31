package main

import "fmt"

const version = "5.0.0-go"

func versionString() string {
	return fmt.Sprintf("goping %s (Go rewrite)", version)
}

type usageLocale struct {
	usageLine   string
	optionLines []string
}

var usageLocales = map[string]usageLocale{
	"en": {
		usageLine: "Usage: goping [options] target [target...]",
		optionLines: []string{
			"Examples:",
			"  goping -a -q -P 8.8.8.8 1.1.1.1",
			"  goping -a -q -P -g 192.168.1.1/24",
			"  goping -a -q -P -g 192.168.1.1/24 > alive.txt",
			"  goping -a -q -P -f targets.txt > alive.txt",
			"",
			"  -4, --ipv4           Use IPv4 only",
			"  -6, --ipv6           Use IPv6 only",
			"  -a, --alive          Show alive hosts",
			"  -A, --addr           Show resolved address instead of name",
			"  -b, --size=BYTES     Set payload size (default 56)",
			"  -B, --backoff=N      Backoff factor for retries (default 1.5)",
			"  -c, --count=N        Number of requests to send to each target",
			"  -C, --vcount=N       Same as -c but prints per-request RTTs",
			"  -d, --rdns           Force reverse DNS lookups",
			"  -D, --timestamp      Print timestamps on output",
			"  -e, --elapsed        Show elapsed time for each reply",
			"  -f, --file=FILE      Read targets from file ('-' for stdin)",
			"  -g, --generate NET   Generate targets from CIDR or start/end",
			"  -i, --interval=MSEC  Global interval between packets (default 10ms)",
			"  -l, --loop           Loop sending pings indefinitely",
			"      -l LOCALE, --lang=LOCALE  Override help language (en or ru)",
			"  -L, --traffic        Show traffic statistics for the whole scan",
			"  -y, --tcp-probe      Use TCP connect probes instead of ICMP",
			"  -Y, --tcp-port=PORT  TCP probe port (default 80)",
			"  -m, --all            Ping all addresses returned for a host",
			"  -n, --name           Resolve numeric targets to hostnames",
			"  -o, --outage         Print outage counters",
			"  -p, --period=MSEC    Per-host interval (default 1000ms)",
			"  -P, --progress       Show a progress bar while hosts finish",
			"  -q, --quiet          Quiet output",
			"  -Q, --squiet=SEC     Print periodic summaries",
			"  -r, --retry=N        Retries for unreachable hosts (default 3)",
			"  -R, --random         Fill payload with random data",
			"  -s, --stats          Print global statistics",
			"  -S, --src=ADDR       Use the given source address",
			"  -t, --timeout=MSEC   Initial timeout (default 500ms)",
			"  -u, --unreach        Show unreachable hosts",
			"  -v, --version        Show version information",
			"  -x, --reachable=N    Exit successfully if at least N hosts are reachable",
			"  -X, --fast-reachable=N  Exit as soon as N hosts are reachable",
		},
	},
	"ru": {
		usageLine: "\u0418\u0441\u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u043d\u0438\u0435: goping [\u043e\u043f\u0446\u0438\u0438] \u0446\u0435\u043b\u044c [\u0446\u0435\u043b\u044c...]",
		optionLines: []string{
			"\u041f\u0440\u0438\u043c\u0435\u0440\u044b:",
			"  goping -a -q -P 8.8.8.8 1.1.1.1",
			"  goping -a -q -P -g 192.168.1.1/24",
			"  goping -a -q -P -g 192.168.1.1/24 > alive.txt",
			"  goping -a -q -P -f targets.txt > alive.txt",
			"",
			"  -4, --ipv4           \u0418\u0441\u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u0442\u044c \u0442\u043e\u043b\u044c\u043a\u043e IPv4",
			"  -6, --ipv6           \u0418\u0441\u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u0442\u044c \u0442\u043e\u043b\u044c\u043a\u043e IPv6",
			"  -a, --alive          \u041f\u043e\u043a\u0430\u0437\u044b\u0432\u0430\u0442\u044c \u0434\u043e\u0441\u0442\u0443\u043f\u043d\u044b\u0435 \u0445\u043e\u0441\u0442\u044b",
			"  -A, --addr           \u041f\u043e\u043a\u0430\u0437\u044b\u0432\u0430\u0442\u044c \u0430\u0434\u0440\u0435\u0441 \u0432\u043c\u0435\u0441\u0442\u043e \u0438\u043c\u0435\u043d\u0438",
			"  -b, --size=BYTES     \u0420\u0430\u0437\u043c\u0435\u0440 \u043f\u043e\u043b\u0435\u0437\u043d\u043e\u0439 \u043d\u0430\u0433\u0440\u0443\u0437\u043a\u0438 (\u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e 56)",
			"  -B, --backoff=N      \u041c\u043d\u043e\u0436\u0438\u0442\u0435\u043b\u044c \u0442\u0430\u0439\u043c\u0430\u0443\u0442\u0430 \u043f\u043e\u0432\u0442\u043e\u0440\u043e\u0432 (\u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e 1.5)",
			"  -c, --count=N        \u0427\u0438\u0441\u043b\u043e \u0437\u0430\u043f\u0440\u043e\u0441\u043e\u0432 \u043d\u0430 \u043a\u0430\u0436\u0434\u044b\u0439 \u0445\u043e\u0441\u0442",
			"  -C, --vcount=N       \u041a\u0430\u043a -c, \u043d\u043e \u0432\u044b\u0432\u043e\u0434\u0438\u0442 RTT \u043a\u0430\u0436\u0434\u043e\u0433\u043e \u0437\u0430\u043f\u0440\u043e\u0441\u0430",
			"  -d, --rdns           \u041f\u0440\u0438\u043d\u0443\u0434\u0438\u0442\u0435\u043b\u044c\u043d\u043e \u0434\u0435\u043b\u0430\u0442\u044c \u043e\u0431\u0440\u0430\u0442\u043d\u043e\u0435 DNS",
			"  -D, --timestamp      \u041f\u0435\u0447\u0430\u0442\u0430\u0442\u044c \u043e\u0442\u043c\u0435\u0442\u043a\u0438 \u0432\u0440\u0435\u043c\u0435\u043d\u0438",
			"  -e, --elapsed        \u041f\u043e\u043a\u0430\u0437\u044b\u0432\u0430\u0442\u044c \u0432\u0440\u0435\u043c\u044f \u0432\u044b\u043f\u043e\u043b\u043d\u0435\u043d\u0438\u044f \u043a\u0430\u0436\u0434\u043e\u0433\u043e \u043e\u0442\u0432\u0435\u0442\u0430",
			"  -f, --file=FILE      \u0427\u0438\u0442\u0430\u0442\u044c \u0446\u0435\u043b\u0438 \u0438\u0437 \u0444\u0430\u0439\u043b\u0430 ('-' \u0434\u043b\u044f stdin)",
			"  -g, --generate NET   \u0413\u0435\u043d\u0435\u0440\u0438\u0440\u043e\u0432\u0430\u0442\u044c \u0446\u0435\u043b\u0438 \u0438\u0437 CIDR \u0438\u043b\u0438 \u0434\u0438\u0430\u043f\u0430\u0437\u043e\u043d\u0430",
			"  -i, --interval=MSEC  \u041e\u0431\u0449\u0438\u0439 \u0438\u043d\u0442\u0435\u0440\u0432\u0430\u043b \u043c\u0435\u0436\u0434\u0443 \u043f\u0430\u043a\u0435\u0442\u0430\u043c\u0438 (\u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e 10\u043c\u0441)",
			"  -l, --loop           \u0417\u0430\u0446\u0438\u043a\u043b\u0438\u0442\u044c \u043e\u0442\u043f\u0440\u0430\u0432\u043a\u0443 \u0437\u0430\u043f\u0440\u043e\u0441\u043e\u0432",
			"      -l LOCALE, --lang=LOCALE  \u0412\u044b\u0431\u0440\u0430\u0442\u044c \u044f\u0437\u044b\u043a \u043f\u043e\u0434\u0441\u043a\u0430\u0437\u043e\u043a (ru \u0438\u043b\u0438 en)",
			"  -L, --traffic        \u041f\u043e\u043a\u0430\u0437\u0430\u0442\u044c \u0441\u0442\u0430\u0442\u0438\u0441\u0442\u0438\u043a\u0443 \u0442\u0440\u0430\u0444\u0438\u043a\u0430 \u0437\u0430 \u0441\u043a\u0430\u043d",
			"  -y, --tcp-probe      \u0418\u0441\u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u0442\u044c TCP-\u043f\u0440\u043e\u0432\u0435\u0440\u043a\u0438 \u0432\u043c\u0435\u0441\u0442\u043e ICMP",
			"  -Y, --tcp-port=PORT  TCP-\u043f\u043e\u0440\u0442 \u0434\u043b\u044f \u043f\u0440\u043e\u0432\u0435\u0440\u043a\u0438 (\u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e 80)",
			"  -m, --all            \u041f\u0438\u043d\u0433\u043e\u0432\u0430\u0442\u044c \u0432\u0441\u0435 \u0430\u0434\u0440\u0435\u0441\u0430, \u0432\u043e\u0437\u0432\u0440\u0430\u0449\u0451\u043d\u043d\u044b\u0435 DNS",
			"  -n, --name           \u0420\u0430\u0437\u0440\u0435\u0448\u0430\u0442\u044c \u0447\u0438\u0441\u043b\u043e\u0432\u044b\u0435 \u0446\u0435\u043b\u0438 \u0432 \u0438\u043c\u0435\u043d\u0430",
			"  -o, --outage         \u041f\u0435\u0447\u0430\u0442\u0430\u0442\u044c \u0441\u0447\u0451\u0442\u0447\u0438\u043a\u0438 \u043f\u0440\u043e\u0441\u0442\u043e\u0435\u0432",
			"  -p, --period=MSEC    \u0418\u043d\u0442\u0435\u0440\u0432\u0430\u043b \u0434\u043b\u044f \u043e\u0434\u043d\u043e\u0433\u043e \u0445\u043e\u0441\u0442\u0430 (\u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e 1000\u043c\u0441)",
			"  -P, --progress       \u041f\u043e\u043a\u0430\u0437\u044b\u0432\u0430\u0442\u044c \u0438\u043d\u0434\u0438\u043a\u0430\u0442\u043e\u0440 \u043f\u0440\u043e\u0433\u0440\u0435\u0441\u0441\u0430",
			"  -q, --quiet          \u0422\u0438\u0445\u0438\u0439 \u0432\u044b\u0432\u043e\u0434",
			"  -Q, --squiet=SEC     \u041f\u0435\u0440\u0438\u043e\u0434\u0438\u0447\u0435\u0441\u043a\u0438\u0435 \u0441\u0432\u043e\u0434\u043a\u0438",
			"  -r, --retry=N        \u0427\u0438\u0441\u043b\u043e \u043f\u043e\u0432\u0442\u043e\u0440\u043e\u0432 \u0434\u043b\u044f \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u044b\u0445 (\u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e 3)",
			"  -R, --random         \u0417\u0430\u043f\u043e\u043b\u043d\u044f\u0442\u044c \u043f\u043e\u043b\u0435\u0437\u043d\u0443\u044e \u043d\u0430\u0433\u0440\u0443\u0437\u043a\u0443 \u0441\u043b\u0443\u0447\u0430\u0439\u043d\u044b\u043c\u0438 \u0434\u0430\u043d\u043d\u044b\u043c\u0438",
			"  -s, --stats          \u041f\u0435\u0447\u0430\u0442\u0430\u0442\u044c \u0433\u043b\u043e\u0431\u0430\u043b\u044c\u043d\u0443\u044e \u0441\u0442\u0430\u0442\u0438\u0441\u0442\u0438\u043a\u0443",
			"  -S, --src=ADDR       \u0418\u0441\u043f\u043e\u043b\u044c\u0437\u043e\u0432\u0430\u0442\u044c \u0443\u043a\u0430\u0437\u0430\u043d\u043d\u044b\u0439 \u0438\u0441\u0445\u043e\u0434\u043d\u044b\u0439 \u0430\u0434\u0440\u0435\u0441",
			"  -t, --timeout=MSEC   \u041d\u0430\u0447\u0430\u043b\u044c\u043d\u044b\u0439 \u0442\u0430\u0439\u043c\u0430\u0443\u0442 (\u043f\u043e \u0443\u043c\u043e\u043b\u0447\u0430\u043d\u0438\u044e 500\u043c\u0441)",
			"  -u, --unreach        \u041f\u043e\u043a\u0430\u0437\u044b\u0432\u0430\u0442\u044c \u043d\u0435\u0434\u043e\u0441\u0442\u0443\u043f\u043d\u044b\u0435 \u0445\u043e\u0441\u0442\u044b",
			"  -v, --version        \u041f\u043e\u043a\u0430\u0437\u0430\u0442\u044c \u0432\u0435\u0440\u0441\u0438\u044e",
			"  -x, --reachable=N    \u0423\u0441\u043f\u0435\u0445, \u0435\u0441\u043b\u0438 \u0434\u043e\u0441\u0442\u0438\u0433\u043d\u0443\u0442\u043e N \u0434\u043e\u0441\u0442\u0443\u043f\u043d\u044b\u0445 \u0445\u043e\u0441\u0442\u043e\u0432",
			"  -X, --fast-reachable=N  \u041e\u0441\u0442\u0430\u043d\u043e\u0432\u0438\u0442\u044c \u0441\u043a\u0430\u043d \u043f\u0440\u0438 \u0434\u043e\u0441\u0442\u0438\u0436\u0435\u043d\u0438\u0438 N \u0434\u043e\u0441\u0442\u0443\u043f\u043d\u044b\u0445",
		},
	},
}

func printUsage(locale string) {
	data, ok := usageLocales[locale]
	if !ok {
		data = usageLocales["en"]
	}
	fmt.Println(data.usageLine)
	for _, line := range data.optionLines {
		fmt.Println(line)
	}
}
