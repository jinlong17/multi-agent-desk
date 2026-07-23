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
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/controlplane"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("multidesk-server failed", "error", err)
		os.Exit(1)
	}
}

func run(arguments []string) error {
	if len(arguments) > 0 && arguments[0] == "bootstrap" {
		return runBootstrap(arguments[1:])
	}
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
	processLock, err := controlplane.AcquireProcessLock(config.DatabasePath)
	if err != nil {
		return err
	}
	defer processLock.Close()
	store, err := controlplane.OpenStore(ctx, controlplane.StoreOptions{Path: config.DatabasePath, BusyTimeout: config.BusyTimeout()})
	if err != nil {
		return err
	}
	defer store.Close()
	if token, created, err := store.EnsureBootstrapToken(ctx, time.Now().UTC()); err != nil {
		return err
	} else if created {
		fmt.Printf("Bootstrap token (shown once; expires in 10 minutes): %s\n", token)
	}
	server, err := controlplane.NewServer(config, store)
	if err != nil {
		return err
	}
	return server.Run(ctx)
}

func runBootstrap(arguments []string) error {
	if len(arguments) == 0 || arguments[0] != "rotate" {
		return fmt.Errorf("bootstrap command must be rotate")
	}
	flags := flag.NewFlagSet("multidesk-server bootstrap rotate", flag.ContinueOnError)
	configPath := flags.String("config", "", "absolute path to owner-only server JSON config")
	confirm := flags.Bool("confirm-uninitialized", false, "confirm that this server has not been initialized")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *configPath == "" || !*confirm {
		return fmt.Errorf("bootstrap rotate requires --config and --confirm-uninitialized")
	}
	config, err := controlplane.LoadConfig(*configPath)
	if err != nil {
		return err
	}
	processLock, err := controlplane.AcquireProcessLock(config.DatabasePath)
	if err != nil {
		return err
	}
	defer processLock.Close()
	store, err := controlplane.OpenStore(context.Background(), controlplane.StoreOptions{Path: config.DatabasePath, BusyTimeout: config.BusyTimeout()})
	if err != nil {
		return err
	}
	defer store.Close()
	token, err := store.RotateBootstrapToken(context.Background(), time.Now().UTC())
	if err != nil {
		return err
	}
	fmt.Printf("Bootstrap token (shown once; expires in 10 minutes): %s\n", token)
	return nil
}
