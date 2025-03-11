package main

import (
	"context"
	"flag"
	"github.com/live-labs/lokiactor/config"
	"github.com/live-labs/lokiactor/flows"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	{
		log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelDebug,
		}))

		log.Handler()
		slog.SetDefault(log)
	}

	argConfigFile := flag.String("config", "./lokiactor.yml", "Configuration file for the application")
	flag.Parse()

	slog.Info("Using configuration file", "file", *argConfigFile)

	cfg, err := config.Load(*argConfigFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	slog.Debug("Configuration loaded", "config", cfg)

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig := <-ch
		slog.Info("Terminating Loki-actor, received signal", "signal", sig.String())
		cancel()
	}()

	fls := make([]*flows.Flow, 0, len(cfg.Flows))

	for _, flowCfg := range cfg.Flows {
		flow, err := flows.New(ctx, flowCfg, cfg.Loki)
		if err != nil {
			slog.Error("Failed to create flow", "error", err)
			os.Exit(1)
		}
		fls = append(fls, flow)
		slog.Debug("Flow created", "name", flowCfg.Name, "query", flowCfg.Query)
	}

	for _, flow := range fls {
		go flow.Run()
		slog.Debug("Flow started", "name", flow.Name())
	}

	<-ctx.Done()
	slog.Info("Loki-actor terminated")

}
