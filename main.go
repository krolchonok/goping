package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	opts, targets, err := parseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}

	if opts.showHelp {
		printUsage()
		return
	}

	if opts.showVersion {
		fmt.Println(versionString())
		return
	}

	if len(targets) == 0 && opts.targetFile == "" && opts.generateSpec == nil {
		fmt.Fprintln(os.Stderr, "no targets specified")
		printUsage()
		os.Exit(3)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	app, err := newApp(opts)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := app.loadTargets(targets); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	app.initProgressBar()

	if err := app.run(ctx); err != nil {
		if err == context.Canceled {
			app.printFinalStats()
			return
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	app.printFinalStats()
	if opts.minReachable > 0 {
		if app.stats.alive >= opts.minReachable {
			os.Exit(0)
		}
		os.Exit(1)
	}
	if app.stats.alive != len(app.hosts) {
		os.Exit(1)
	}
	os.Exit(0)
}
