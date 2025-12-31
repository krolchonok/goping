# goping

`goping` is a fast, parallel ICMP scanner inspired by `fping`, rewritten in Go. It efficiently probes thousands of IPv4 or IPv6 hosts, offers detailed per-host statistics, and can export results as human-readable text, JSON, or Netdata-compatible metrics.

## Features

- **High-performance scanning** – concurrent workers with configurable global and per-host intervals keep latency low even across large /16 networks.
- **Flexible target input** – mix plain targets, files (`-f file`), or dynamic generation from CIDR or start/end ranges (`-g`, `--generate`).
- **IPv4 and IPv6** – force either family with `-4`/`-6`, or scan both simultaneously.
- **Progress and reporting** – progress bar (`-P`), periodic summaries (`-Q 5`), per-host outage tracking, JSON output (`-J/--json`), Netdata charts (`-N/--netdata`), and cumulative reporting.
- **Traffic accounting** – enable the `-L/--traffic` flag to print total packets, sent/received bandwidth, and combined network load for the entire scan.
- **TCP probe mode** – switch to `-y/--tcp-probe` (with optional `-Y PORT`) to detect hosts via TCP connects when ICMP is blocked or raw sockets are unavailable (e.g., Termux).
- **Operational niceties** – quiet mode (`-q`), timestamped replies, TTL/TOS printing, reverse DNS, random payloads, and configurable retry/backoff policies.

## Installation

Requirements: Go 1.24 or newer (see `go.mod`).

```powershell
# Clone this repository, then:
go build -o goping.exe .
# Optionally install into GOPATH/bin
go install ./...
```

The resulting binary is self-contained and does not require elevated privileges on most systems (raw socket access may still need admin rights depending on the OS).

## Usage

```powershell
goping [options] target [target...]
```

Basic sweep of two hosts with quick output:

```powershell
goping -a -q -P -i 1 10.0.0.1 10.0.0.2
```

Scan an entire subnet generated from CIDR notation while collecting link utilization data:

```powershell
goping -a -q -P -i 1 -L -g 10.0.0.0/24
```

Send JSON to stdout for downstream processing:

```powershell
goping --json --count 3 example.com
```

Export Netdata charts every 10 seconds:

```powershell
goping -N -Q 10,cumulative targets.txt
```

## Flag highlights

| Flag | Description |
| ---- | ----------- |
| `-a`, `--alive` | Print hosts that respond (default behaviour). |
| `-q`, `--quiet` | Suppress per-reply stats; combine with `-a` for IP-only lists. |
| `-P`, `--progress` | Show a progress bar that updates as hosts finish. |
| `-i`, `--interval` | Global delay between packets (e.g., `-i 1` for 1 ms). |
| `-p`, `--period` | Per-host interval (default 1 s). |
| `-c`, `--count` | Number of probes per target (default indefinite until success). |
| `-g`, `--generate` | Expand CIDR/start-end ranges into targets. |
| `-f`, `--file` | Read targets from a file (`-` for stdin). |
| `-J`, `--json` | Emit structured JSON events instead of text. |
| `-N`, `--netdata` | Produce Netdata charts compatible with the stock fping collector. |
| `-L`, `--traffic` | Show total sent/received packets and Mbps for the whole scan. |
| `-y`, `--tcp-probe` | Use TCP connect probes instead of ICMP. |
| `-Y`, `--tcp-port=PORT` | TCP probe port (default 80). |
| `-Q`, `--squiet SEC[,cumulative]` | Print periodic interval summaries. |
| `-S`, `--src` | Force a specific source IP. |
| `-t`, `--timeout` / `-r`, `--retry` | Control probe timeout and retry counts. |

Run `goping --help` for the full option list.

## Development

```powershell
gofmt -w .
go build ./...
# Optional sanity run
go test ./...
```

When contributing:

1. Keep code comments concise and favor readable logic.
2. Ensure new flags are wired through `options.go`, documented in `help.go`, and covered in the README.
3. Prefer `rg`/`ripgrep` for searches to keep the workflow fast on large trees.

## License

Released under the [MIT License](LICENSE).
