package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"mmm-revived/internal/account"
	"mmm-revived/internal/bootstrap"
	"mmm-revived/internal/calendar"
	"mmm-revived/internal/config"
	"mmm-revived/internal/gate"
	"mmm-revived/internal/handlers"
	"mmm-revived/internal/patchserver"
	"mmm-revived/internal/server"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	path := flag.String("config", defaultConfigPath(), "server configuration file")
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))
	c, err := config.Load(*path)
	if err != nil {
		log.Error("invalid configuration", "path", *path, "error", err)
		os.Exit(2)
	}
	if err = run(log, c); err != nil {
		log.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func defaultConfigPath() string {
	executable, err := os.Executable()
	if err != nil {
		return config.FileName
	}
	dir := filepath.Dir(executable)
	for _, marker := range []string{config.FileName, config.AssetsDir} {
		if _, err = os.Stat(filepath.Join(dir, marker)); err == nil {
			return filepath.Join(dir, config.FileName)
		}
	}
	return config.FileName
}

func run(log *slog.Logger, c config.Config) error {
	key, err := gate.LoadKey(c.PrivateKey(), c.PublicKey())
	if err != nil {
		return fmt.Errorf("load keys: %w", err)
	}
	store, err := account.Open(c.Database(), c.Catalog())
	if err != nil {
		return fmt.Errorf("load accounts: %w", err)
	}
	defer store.Close()
	store.Start = c.StartMode()
	store.Fresh = c.FreshStart
	store.ConfigureCalendarLoop(c.CalendarLoop)
	store.ConfigureEvents(c.AllEvents)
	shift, err := store.ConfigureCalendar(nil)
	if err != nil {
		return fmt.Errorf("configure content calendar: %w", err)
	}
	start, period := calendar.Loop()
	log.Info("content calendar", "offset_seconds", shift, "loop", c.CalendarLoop, "loop_start", time.Unix(start, 0).UTC(), "loop_days", period/calendar.DaySeconds, "loop_cycle", calendar.Cycle())
	if err = store.ConfigureEconomy(c.Economy()); err != nil {
		return fmt.Errorf("configure economy: %w", err)
	}
	if err = store.ConfigureTeamSize(c.TeamSize); err != nil {
		return fmt.Errorf("configure team size: %w", err)
	}
	if err = store.ConfigureEnergyRegen(c.EnergyRegen); err != nil {
		return fmt.Errorf("configure energy regen: %w", err)
	}
	if err = store.ConfigureRewardMultiplier(c.RewardMultiplier); err != nil {
		return fmt.Errorf("configure reward multiplier: %w", err)
	}
	web := bootstrap.Server{GateAddress: c.Address()}
	patches, err := openPatches(log, c.Patches())
	if err != nil {
		return err
	}
	if patches != nil {
		defer patches.Close()
		publicKey, err := os.ReadFile(c.PublicKey())
		if err != nil {
			return err
		}
		if err = patches.ConfigureRetail("http://"+c.HTTPAddress(), publicKey); err != nil {
			return err
		}
		web.NativePatches = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Info("native patch request", "method", r.Method, "path", r.URL.Path, "peer", r.RemoteAddr)
			patches.ServeHTTP(w, r)
		})
	}
	listener, err := net.Listen("tcp", c.Address())
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	httpListener, err := net.Listen("tcp", c.HTTPAddress())
	if err != nil {
		listener.Close()
		return fmt.Errorf("bootstrap listen: %w", err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	service := server.NewService(store, server.Config{}, handlers.Register)
	server := gate.Server{Key: key, Handler: service, Logger: log}
	log.Info("server ready", "gate", listener.Addr(), "http", httpListener.Addr(), "fresh_start", c.FreshStart, "story_complete", c.StoryComplete, "infinite_energy", c.InfiniteEnergy, "free_draws", c.FreeDraws, "dreamstone_shop", c.DreamstoneShop, "all_banners", c.AllBanners, "featured_banners", c.FeaturedBanners, "all_events", c.AllEvents, "team_size", c.TeamSize, "energy_regen", c.EnergyRegen, "reward_multiplier", c.RewardMultiplier, "public_key", c.PublicKey())
	var running sync.WaitGroup
	running.Go(func() { service.Run(ctx) })
	results := make(chan error, 2)
	go func() { results <- server.Serve(ctx, listener) }()
	go func() { results <- web.Serve(ctx, httpListener) }()
	err = <-results
	cancel()
	err = errors.Join(err, <-results)
	running.Wait()
	if err != nil {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}

func openPatches(log *slog.Logger, dir string) (*patchserver.NativeCatalog, error) {
	if _, err := os.Stat(filepath.Join(dir, "patchlist_win32.json")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read native game patches: %w", err)
	}
	catalog, err := patchserver.OpenNativeCatalog(dir)
	if err != nil {
		return nil, fmt.Errorf("load native game patches: %w", err)
	}
	log.Info("native game patches ready", "directory", dir)
	return catalog, nil
}
