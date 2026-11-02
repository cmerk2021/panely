// Command panely runs the configurable TUI dashboard.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cmerk2021/panely/internal/app"
	"github.com/cmerk2021/panely/internal/appinfo"
	_ "github.com/cmerk2021/panely/internal/builtin"
	"github.com/cmerk2021/panely/internal/config"
	"github.com/cmerk2021/panely/internal/panel"
	"github.com/cmerk2021/panely/internal/storage"
	"github.com/cmerk2021/panely/internal/theme"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", appinfo.Name, err)
		os.Exit(1)
	}
}

func usage(fs *flag.FlagSet) func() {
	return func() {
		fmt.Fprintf(os.Stderr, `%[1]s — a configurable TUI dashboard

Usage:
  %[1]s [flags]             run the dashboard
  %[1]s version             print version information
  %[1]s themes              list available themes
  %[1]s config path         print the config file path
  %[1]s config validate     validate the config file
  %[1]s config init         write the default config file

Flags:
`, appinfo.Name)
		fs.PrintDefaults()
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet(appinfo.Name, flag.ExitOnError)
	cfgPath := fs.String("config", "", "path to config file (default: XDG config dir)")
	debug := fs.Bool("debug", false, "enable debug logging to the data dir")
	fs.Usage = usage(fs)

	// Allow "panely <subcommand> [flags]" ordering.
	var sub []string
	rest := args
	for len(rest) > 0 && rest[0] != "" && rest[0][0] != '-' {
		sub = append(sub, rest[0])
		rest = rest[1:]
	}
	if err := fs.Parse(rest); err != nil {
		return err
	}
	// Also allow "panely [flags] <subcommand>" ordering.
	if len(sub) == 0 && fs.NArg() > 0 {
		sub = fs.Args()
	}

	path := *cfgPath
	if path == "" {
		p, err := storage.DefaultConfigPath()
		if err != nil {
			return err
		}
		path = p
	}

	if len(sub) > 0 {
		switch sub[0] {
		case "version":
			fmt.Printf("%s %s (commit %s, built %s)\n", appinfo.Name, appinfo.Version, appinfo.Commit, appinfo.Date)
			return nil
		case "themes":
			cfg, _ := config.Load(path, panel.Types())
			for _, n := range theme.Names(cfg) {
				fmt.Println(n)
			}
			return nil
		case "config":
			if len(sub) < 2 {
				return errors.New("usage: config path|validate|init")
			}
			switch sub[1] {
			case "path":
				fmt.Println(path)
				return nil
			case "validate":
				if _, err := config.Load(path, panel.Types()); err != nil {
					return err
				}
				fmt.Println("config is valid")
				return nil
			case "init":
				if err := config.WriteDefault(path); err != nil {
					return err
				}
				fmt.Println("wrote", path)
				return nil
			default:
				return fmt.Errorf("unknown config subcommand %q", sub[1])
			}
		default:
			return fmt.Errorf("unknown command %q (try --help)", sub[0])
		}
	}

	return runTUI(path, *debug)
}

func runTUI(cfgPath string, debug bool) error {
	dataDir, err := storage.DataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	// Structured logging to a file; never stdout while the TUI runs.
	var logger *slog.Logger
	if debug {
		logFile, err := os.OpenFile(filepath.Join(dataDir, appinfo.Name+".log"),
			os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		defer func() { _ = logFile.Close() }()
		logger = slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelDebug}))
	} else {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// First run: write a default config.
	firstRun := false
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := config.WriteDefault(cfgPath); err != nil {
			return fmt.Errorf("write default config: %w", err)
		}
		firstRun = true
	}

	cfg, err := config.Load(cfgPath, panel.Types())
	if err != nil {
		return err
	}

	m := app.New(cfgPath, cfg, dataDir, logger, firstRun)
	p := tea.NewProgram(m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	_, err = p.Run()
	return err
}
