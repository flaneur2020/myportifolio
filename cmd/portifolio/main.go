package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"portifolio/core"
	"portifolio/plugins/builtin"
	tomlcfg "portifolio/plugins/config/toml"
	"portifolio/plugins/storage/sqlite"
	luart "portifolio/runtime/lua"
	"portifolio/ui"
)

func main() {
	configPath := flag.String("config", "", "path to TOML configuration")
	dbPath := flag.String("db", "", "path to SQLite price cache")
	pluginDirs := flag.String("plugins", "", "comma-separated extra Lua plugin directories")
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

	kernel := core.NewKernel()
	defer kernel.Close()

	// Persistence plugins
	store, err := sqlite.Open(ctx, databasePath)
	if err != nil {
		fatal(err)
	}
	kernel.SetPriceStore(store)
	kernel.SetDirectoryStore(store)
	kernel.OnClose(store.Close)
	kernel.SetConfigStore(tomlcfg.New())

	// Built-in embedded Lua plugins (markets, providers, directories, cards, default theme)
	luaHost := luart.NewHost(luart.HostOptions{})
	builtin.Register(kernel, luaHost)

	// User Lua plugins from <config-dir>/plugins and --plugins
	luaHost.LoadForConfig(kernel, cfgPath)
	for _, dir := range strings.Split(*pluginDirs, ",") {
		dir = strings.TrimSpace(dir)
		if dir != "" {
			luaHost.LoadDir(kernel, dir)
		}
	}

	cfg, err := kernel.LoadConfig(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		fatal(err)
	}
	var initial *core.Config
	if err == nil {
		initial = &cfg
	}

	model := ui.New(kernel, initial, cfgPath)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := program.Run(); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "portifolio:", err)
	os.Exit(1)
}
