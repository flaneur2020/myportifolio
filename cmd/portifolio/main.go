package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"portifolio/internal/config"
	"portifolio/internal/provider"
	"portifolio/internal/storage"
	"portifolio/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	configPath := flag.String("config", "", "path to TOML configuration")
	dbPath := flag.String("db", "", "path to SQLite price cache")
	flag.Parse()

	cfgPath := *configPath
	if cfgPath == "" {
		root, err := os.UserConfigDir()
		if err != nil {
			fatal(err)
		}
		cfgPath = filepath.Join(root, "portifolio", "config.toml")
	}
	databasePath := *dbPath
	if databasePath == "" {
		root, err := os.UserConfigDir()
		if err != nil {
			fatal(err)
		}
		databasePath = filepath.Join(root, "portifolio", "cache.sqlite")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	store, err := storage.Open(ctx, databasePath)
	if err != nil {
		fatal(err)
	}
	defer store.Close()

	cfg, err := config.Load(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		fatal(err)
	}
	var initial *config.Config
	if err == nil {
		initial = &cfg
	}

	registry := provider.NewRegistry()
	registry.Register(provider.NewYahoo())
	registry.Register(provider.NewStooq())
	registry.Register(provider.NewEastmoney())

	program := tea.NewProgram(ui.New(initial, cfgPath, store, registry), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "portifolio:", err)
	os.Exit(1)
}
