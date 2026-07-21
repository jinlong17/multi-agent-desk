// Command multidesk-server runs the MultiAgentDesk Control Plane foundation.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jinlong17/multi-agent-desk/internal/controlplane"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("multidesk-server failed", "error", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	flags := flag.NewFlagSet("multidesk-server", flag.ContinueOnError)
	configPath := flags.String("config", "", "absolute path to owner-only server JSON config")
	showVersion := flags.Bool("version", false, "print build version")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	if *showVersion {
		fmt.Printf("multidesk-server %s (%s)\n", controlplane.BuildVersion, controlplane.BuildCommit)
		return nil
	}
	if *configPath == "" {
		return fmt.Errorf("--config is required")
	}
	config, err := controlplane.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := controlplane.OpenStore(ctx, controlplane.StoreOptions{Path: config.DatabasePath, BusyTimeout: config.BusyTimeout()})
	if err != nil {
		return err
	}
	defer store.Close()
	return controlplane.NewServer(config, store).Run(ctx)
}
