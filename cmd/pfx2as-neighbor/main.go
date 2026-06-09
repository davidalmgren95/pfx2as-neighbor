package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"pfx2as-neighbor/internal/bgpserver"
	"pfx2as-neighbor/internal/downloader"
	"pfx2as-neighbor/internal/gzparser"

	"gopkg.in/yaml.v3"
)

func updateRoutes(server *bgpserver.BgpServer) {
	slog.Info("fetching latest CAIDA prefix2as data")

	body, err := downloader.DownloadLatestPrefix2AS()
	if err != nil {
		slog.Error("failed to download prefix2as data", "err", err)
		return
	}

	records, err := gzparser.Parse(body)
	if err != nil {
		slog.Error("failed to parse prefix2as data", "err", err)
		return
	}
	slog.Info("parsed prefix2as data", "prefixes", len(records))

	var deleted, added, errs int
	for activePrefix := range server.ActivePrefixes() {
		if _, exists := records[activePrefix]; !exists {
			if err := server.DeletePath(activePrefix); err != nil {
				slog.Error("failed to delete stale route", "prefix", activePrefix, "err", err)
				errs++
			} else {
				deleted++
			}
		}
	}
	for prefix, asns := range records {
		if err := server.AddPath(prefix, asns); err != nil {
			slog.Error("failed to add route", "prefix", prefix, "err", err)
			errs++
		} else {
			added++
		}
	}

	slog.Info("route update complete",
		"added", added,
		"deleted", deleted,
		"unchanged", len(records)-added,
		"active", len(server.ActivePrefixes()),
		"errors", errs,
	)
}

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	configPath := flag.String("c", "", "path to config file")
	flag.Parse()

	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "usage: pfx2as-neighbor -c <config>")
		os.Exit(1)
	}

	data, err := os.ReadFile(*configPath)
	if err != nil {
		slog.Error("failed to read config", "path", *configPath, "err", err)
		os.Exit(1)
	}

	var cfg *bgpserver.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		slog.Error("failed to parse config", "path", *configPath, "err", err)
		os.Exit(1)
	}
	interval := 6 * time.Hour
	if cfg.UpdateInterval != "" {
		d, err := time.ParseDuration(cfg.UpdateInterval)
		if err != nil {
			slog.Error("invalid update_interval, using default 6h", "value", cfg.UpdateInterval, "err", err)
		} else {
			interval = d
		}
	}
	slog.Info("config loaded", "asn", cfg.ASN, "router_id", cfg.RouterID, "neighbors", len(cfg.Neighbors), "update_interval", interval)

	server, err := bgpserver.Start(ctx, cfg)
	if err != nil {
		slog.Error("failed to start BGP server", "err", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		
		// Initial update
		updateRoutes(server)

      	ticker := time.NewTicker(interval)
      	defer ticker.Stop()

      	for {
      	    select {
      	    case <-ticker.C:
				updateRoutes(server)
      	    case <-ctx.Done():
      	        return
      	    }
      	}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	wg.Wait()

	stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Stop(stopCtx); err != nil {
		slog.Error("error stopping BGP server", "err", err)
	}
}
