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

func updateRoutes(server *bgpserver.BgpServer, lastURL string) string {
	slog.Info("checking for latest CAIDA prefix2as data")

	url, err := downloader.LatestPrefix2ASURL()
	if err != nil {
		slog.Error("failed to find latest prefix2as URL", "err", err)
		return lastURL
	}
	if url == lastURL {
		slog.Info("prefix2as data unchanged, skipping download", "url", url)
		return lastURL
	}

	body, err := downloader.Download(url)
	if err != nil {
		slog.Error("failed to download prefix2as data", "err", err)
		return lastURL
	}
	defer body.Close()

	records, err := gzparser.Parse(body)
	if err != nil {
		slog.Error("failed to parse prefix2as data", "err", err)
		return lastURL
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
	for prefix, asn := range records {
		changed, err := server.AddPath(prefix, asn)
		if err != nil {
			slog.Error("failed to add route", "prefix", prefix, "err", err)
			errs++
		} else if changed {
			added++
		}
	}

	slog.Info("route update complete",
		"added", added,
		"deleted", deleted,
		"unchanged", len(records)-added-errs,
		"active", len(server.ActivePrefixes()),
		"errors", errs,
	)
	return url
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

	// SIGUSR1: check for a new prefix2as file out of cycle (downloads only if a
	// newer one is available). SIGUSR2: force a re-download even if unchanged.
	sigRefresh := make(chan os.Signal, 1)
	sigForce := make(chan os.Signal, 1)
	signal.Notify(sigRefresh, syscall.SIGUSR1)
	signal.Notify(sigForce, syscall.SIGUSR2)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Initial update
		lastURL := updateRoutes(server, "")

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				lastURL = updateRoutes(server, lastURL)
			case <-sigRefresh:
				slog.Info("received SIGUSR1, checking for new prefix2as data")
				lastURL = updateRoutes(server, lastURL)
			case <-sigForce:
				slog.Info("received SIGUSR2, forcing prefix2as re-download")
				lastURL = updateRoutes(server, "")
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
