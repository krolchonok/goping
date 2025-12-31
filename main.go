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
		printUsage(opts.locale)
		return
	}

	if opts.showVersion {
		fmt.Println(versionString())
		return
	}

	if len(targets) == 0 && opts.targetFile == "" && opts.generateSpec == nil {
		msg := "no targets specified"
		if opts.locale == "ru" {
			msg = "не указаны цели"
		}
		fmt.Fprintln(os.Stderr, msg)
		printUsage(opts.locale)
		os.Exit(3)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupSignalHandlers(ctx, cancel, opts.locale)

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

func setupSignalHandlers(ctx context.Context, cancel context.CancelFunc, locale string) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	messageFirst := "\nInterrupt received, cancelling scan... (press Ctrl+C again to force exit)"
	messageForce := "\nForce exit requested"
	if locale == "ru" {
		messageFirst = "\nПолучен сигнал прерывания, завершаем скан... (нажмите Ctrl+C ещё раз для немедленного выхода)"
		messageForce = "\nЭкстренный выход по Ctrl+C"
	}
	go func() {
		defer signal.Stop(sigCh)
		first := true
		for {
			select {
			case <-ctx.Done():
				return
			case <-sigCh:
				if first {
					first = false
					fmt.Fprintln(os.Stderr, messageFirst)
					cancel()
				} else {
					fmt.Fprintln(os.Stderr, messageForce)
					os.Exit(130)
				}
			}
		}
	}()
}
