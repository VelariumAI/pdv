package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/velariumai/pdv/internal/api"
	"github.com/velariumai/pdv/internal/config"
	"github.com/velariumai/pdv/internal/database"
	"github.com/velariumai/pdv/internal/download"
	"github.com/velariumai/pdv/internal/events"
	"github.com/velariumai/pdv/internal/tui"
	"github.com/velariumai/pdv/pkg/output"
	"golang.org/x/term"
)

type appState struct {
	configPath string
	dbPath     string
	cfg        *config.Config
	db         *database.DB
	engine     *download.Engine
}

func main() {
	if err := run(context.Background(), os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stdout, stderr *os.File, args []string) error {
	paths, err := defaultPaths()
	if err != nil {
		return err
	}
	state := &appState{
		configPath: paths.configPath,
		dbPath:     paths.dbPath,
	}

	var (
		flagConfigPath string
		flagVersion    bool
		flagLogLevel   string
	)

	root := &cobra.Command{
		Use:   "pdv",
		Short: "PDV is a self-hosted download manager",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if flagLogLevel != "" {
				if err := os.Setenv("PDV_LOG_LEVEL", flagLogLevel); err != nil {
					return fmt.Errorf("main: set log level env: %w", err)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagVersion {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "pdv %s (%s)\n", Version, BuildDate)
				return err
			}
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&flagConfigPath, "config", state.configPath, "path to config file")
	root.PersistentFlags().StringVar(&flagLogLevel, "log-level", "info", "log level")
	root.Flags().BoolVar(&flagVersion, "version", false, "show version and exit")

	root.AddCommand(newAddCommand(state, &flagConfigPath))
	root.AddCommand(newProbeCommand(state, &flagConfigPath))
	root.AddCommand(newListCommand(state, &flagConfigPath))
	root.AddCommand(newCancelCommand(state, &flagConfigPath))
	root.AddCommand(newPauseCommand(state, &flagConfigPath))
	root.AddCommand(newResumeCommand(state, &flagConfigPath))
	root.AddCommand(newRetryCommand(state, &flagConfigPath))
	root.AddCommand(newHistoryCommand(state, &flagConfigPath))
	root.AddCommand(newStatusCommand(state, &flagConfigPath))
	root.AddCommand(newServeCommand(state, &flagConfigPath))
	root.AddCommand(newTUICommand(state, &flagConfigPath))
	root.AddCommand(newGetCommand(state, &flagConfigPath))
	root.AddCommand(newSetCommand(state, &flagConfigPath))
	root.AddCommand(newConfigCommand(&flagConfigPath))

	root.SetArgs(args)
	return root.ExecuteContext(ctx)
}

type resolvedPaths struct {
	configPath string
	dbPath     string
}

func defaultPaths() (*resolvedPaths, error) {
	exePath, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("main: resolve executable path: %w", err)
	}
	baseDir := filepath.Dir(exePath)
	return &resolvedPaths{
		configPath: filepath.Join(baseDir, "pdv.json"),
		dbPath:     filepath.Join(baseDir, "pdv.db"),
	}, nil
}

func resolveStaticDir(baseDir string) string {
	candidates := []string{
		filepath.Join(baseDir, "web"),
		baseDir,
	}
	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(cwd, "web"), cwd)
	}
	for _, dir := range candidates {
		info, err := os.Stat(filepath.Join(dir, "index.html"))
		if err == nil && !info.IsDir() {
			return dir
		}
	}
	return ""
}

func (a *appState) init(ctx context.Context, cfgPath string) error {
	if cfgPath == "" {
		return errors.New("main: config path is empty")
	}
	cfg, err := ensureConfig(cfgPath)
	if err != nil {
		return err
	}
	dbPath := filepath.Join(filepath.Dir(cfgPath), "pdv.db")
	db, err := database.Open(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("main: open database: %w", err)
	}
	a.configPath = cfgPath
	a.dbPath = dbPath
	if override := strings.TrimSpace(os.Getenv("PDV_LOG_LEVEL")); override != "" {
		cfg.LogLevel = override
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("main: validate config: %w", err)
	}
	a.cfg = cfg
	a.db = db
	configureLogging(cfg)
	a.engine = download.NewEngine(cfg, db)
	return nil
}

func (a *appState) close() {
	if a.db != nil {
		_ = a.db.Close()
		a.db = nil
	}
}

func ensureConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err == nil {
		cfg, loadErr := config.Load(path)
		if loadErr != nil {
			return nil, fmt.Errorf("main: load config: %w", loadErr)
		}
		if err := cfg.Validate(); err != nil {
			return nil, fmt.Errorf("main: validate config: %w", err)
		}
		return cfg, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("main: stat config %q: %w", path, err)
	}

	cfg := config.New()
	if err := cfg.Save(path); err != nil {
		return nil, fmt.Errorf("main: create default config: %w", err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("main: reload default config: %w", err)
	}
	if err := loaded.Validate(); err != nil {
		return nil, fmt.Errorf("main: validate config: %w", err)
	}
	return loaded, nil
}

func configureLogging(cfg *config.Config) {
	level := slog.LevelInfo
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	var writer io.Writer = os.Stdout
	if cfg.LogFile != "" {
		if f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			writer = io.MultiWriter(os.Stdout, f)
		}
	}
	opts := &slog.HandlerOptions{Level: level}
	if os.Getenv("DEBUG") == "1" {
		slog.SetDefault(slog.New(slog.NewTextHandler(writer, opts)))
		return
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(writer, opts)))
}

func parseID(arg string) (int64, error) {
	id, err := strconv.ParseInt(arg, 10, 64)
	if err != nil || id < 1 {
		return 0, fmt.Errorf("invalid id %q", arg)
	}
	return id, nil
}

func newAddCommand(state *appState, configPath *string) *cobra.Command {
	var (
		quality     string
		format      string
		template    string
		category    string
		isPlaylist  bool
		enqueueOnly bool
		waitTimeout time.Duration
	)
	cmd := &cobra.Command{
		Use:   "add <url>",
		Short: "Add a URL to the queue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()

			engineCtx, cancelEngine := context.WithCancel(context.Background())
			defer cancelEngine()
			if err := state.engine.Start(engineCtx); err != nil {
				return err
			}
			defer state.engine.Stop(context.Background())

			entry, err := state.engine.Add(cmd.Context(), args[0], &output.AddOpts{
				Quality:    quality,
				Format:     format,
				Template:   template,
				Category:   category,
				IsPlaylist: isPlaylist,
			})
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "queued id=%d status=%s\n", entry.ID, entry.Status); err != nil {
				return err
			}
			if enqueueOnly {
				return nil
			}

			completedCh := make(chan interface{}, 16)
			failedCh := make(chan interface{}, 16)
			offCompleted := state.engine.Subscribe("DownloadCompleted", completedCh)
			offFailed := state.engine.Subscribe("DownloadFailed", failedCh)
			defer offCompleted()
			defer offFailed()

			waitCtx, cancelWait := context.WithTimeout(cmd.Context(), waitTimeout)
			defer cancelWait()
			for {
				select {
				case ev := <-completedCh:
					if c, ok := ev.(*events.DownloadCompleted); ok && c.ID == entry.ID {
						_, err := fmt.Fprintf(cmd.OutOrStdout(), "completed id=%d path=%s\n", c.ID, c.FilePath)
						return err
					}
				case ev := <-failedCh:
					if f, ok := ev.(*events.DownloadFailed); ok && f.ID == entry.ID {
						return fmt.Errorf("download %d failed: %s", f.ID, f.Error)
					}
				case <-waitCtx.Done():
					return fmt.Errorf("download %d did not complete in %s", entry.ID, waitTimeout)
				}
			}
		},
	}
	cmd.Flags().StringVar(&quality, "quality", "", "download quality selector")
	cmd.Flags().StringVar(&format, "format", "", "yt-dlp format selector")
	cmd.Flags().StringVar(&template, "template", "", "output template override")
	cmd.Flags().StringVar(&category, "category", "", "logical category override")
	cmd.Flags().BoolVar(&isPlaylist, "playlist", false, "mark URL as playlist for template routing")
	cmd.Flags().BoolVar(&enqueueOnly, "enqueue-only", false, "only enqueue without waiting for completion")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 20*time.Minute, "maximum wait time for completion")
	return cmd
}

func newProbeCommand(state *appState, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "probe <url>",
		Short: "Probe URL metadata and formats",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()

			result, err := download.Probe(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Title: %s\nUploader: %s\nDuration: %ds\n", result.Title, result.Uploader, result.Duration)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "FORMAT\tRESOLUTION\tCODEC\tSIZE_EST\tEXT")
			for _, f := range result.Formats {
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%d\t%s\n", f.FormatID, f.Resolution, f.Codec, f.FileSizeEstimate, f.Ext)
			}
			return tw.Flush()
		},
	}
}

func newListCommand(state *appState, configPath *string) *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List queue entries",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()
			items, err := state.engine.ListQueue(cmd.Context(), status)
			if err != nil {
				return err
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "ID\tSTATUS\tTITLE\tPROGRESS\tSPEED")
			for _, it := range items {
				_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%.1f%%\t-\n", it.ID, it.Status, it.Title, it.Progress)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	return cmd
}

func newCancelCommand(state *appState, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a queue entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()
			if err := state.engine.Cancel(cmd.Context(), id); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "cancelled id=%d\n", id)
			return err
		},
	}
}

func newPauseCommand(state *appState, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "pause <id>",
		Short: "Pause a queue entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()
			if err := state.engine.Pause(cmd.Context(), id); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "paused id=%d\n", id)
			return err
		},
	}
}

func newResumeCommand(state *appState, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "resume <id>",
		Short: "Resume a queue entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()
			if err := state.engine.Resume(cmd.Context(), id); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "resumed id=%d\n", id)
			return err
		},
	}
}

func newRetryCommand(state *appState, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "retry <id>",
		Short: "Retry a queue entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseID(args[0])
			if err != nil {
				return err
			}
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()
			if err := state.engine.Retry(cmd.Context(), id); err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "retried id=%d\n", id)
			return err
		},
	}
}

func newHistoryCommand(state *appState, configPath *string) *cobra.Command {
	var status string
	cmd := &cobra.Command{
		Use:   "history [limit]",
		Short: "List history entries",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			limit := 20
			if len(args) == 1 {
				v, err := strconv.Atoi(args[0])
				if err != nil || v < 1 {
					return fmt.Errorf("invalid limit %q", args[0])
				}
				limit = v
			}
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()
			items, err := state.engine.ListHistory(cmd.Context(), status)
			if err != nil {
				return err
			}
			if len(items) > limit {
				items = items[:limit]
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 8, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "ID\tTITLE\tSTATUS\tSIZE\tDATE")
			for _, it := range items {
				_, _ = fmt.Fprintf(tw, "%d\t%s\t%s\t%d\t%s\n", it.ID, it.Title, it.FinalStatus, it.FileSize, it.DownloadedAt.Format(time.RFC3339))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by final status")
	return cmd
}

func newStatusCommand(state *appState, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show worker and queue status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()
			queueItems, err := state.engine.ListQueue(cmd.Context(), "")
			if err != nil {
				return err
			}
			historyItems, err := state.engine.ListHistory(cmd.Context(), "")
			if err != nil {
				return err
			}
			workers := state.engine.Workers()
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "workers=%d queue=%d history=%d\n", len(workers), len(queueItems), len(historyItems))
			for _, w := range workers {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "worker[%d]=%s current=%d\n", w.ID, w.State, w.CurrentID)
			}
			return nil
		},
	}
}

func newServeCommand(state *appState, configPath *string) *cobra.Command {
	var (
		host    string
		port    int
		withTUI bool
		noTUI   bool
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start REST API server",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()

			if host == "" {
				host = state.cfg.APIHost
			}
			if port == 0 {
				port = state.cfg.APIPort
			}
			if noTUI {
				withTUI = false
			}
			addr := fmt.Sprintf("%s:%d", host, port)

			engineCtx, cancelEngine := context.WithCancel(context.Background())
			defer cancelEngine()
			if err := state.engine.Start(engineCtx); err != nil {
				return err
			}

			server := api.NewServer(addr, state.engine, state.cfg)
			staticDir := resolveStaticDir(filepath.Dir(state.configPath))
			if staticDir != "" {
				server.SetStaticDir(staticDir)
			}
			slog.Info("serve startup",
				"api_addr", addr,
				"config_path", state.configPath,
				"db_path", state.dbPath,
				"static_dir", staticDir,
				"workers", state.cfg.MaxConcurrentQueue,
			)
			server.SetShutdownHook(func(ctx context.Context) {
				_ = server.Stop(ctx)
				_ = state.engine.Stop(ctx)
			})

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
			defer signal.Stop(sigCh)

			errCh := make(chan error, 1)
			go func() {
				errCh <- server.Start(context.Background())
			}()

			if withTUI && term.IsTerminal(int(os.Stdin.Fd())) {
				go func() {
					model := tui.NewModel(state.engine, state.cfg)
					p := tea.NewProgram(model, tea.WithAltScreen())
					_, _ = p.Run()
				}()
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "serving on http://%s\n", addr)

			select {
			case err := <-errCh:
				if err != nil {
					return err
				}
			case <-sigCh:
			}

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_ = server.Stop(shutdownCtx)
			_ = state.engine.Stop(shutdownCtx)
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "api-host", "", "API bind host")
	cmd.Flags().IntVar(&port, "api-port", 0, "API bind port")
	cmd.Flags().BoolVar(&withTUI, "tui", true, "attach TUI when running in an interactive terminal")
	cmd.Flags().BoolVar(&noTUI, "no-tui", false, "disable TUI even in interactive terminals")
	return cmd
}

func newTUICommand(state *appState, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch terminal UI directly",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("tui: stdin is not an interactive terminal")
			}
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()

			engineCtx, cancelEngine := context.WithCancel(context.Background())
			defer cancelEngine()
			if err := state.engine.Start(engineCtx); err != nil {
				return err
			}
			defer state.engine.Stop(context.Background())

			model := tui.NewModel(state.engine, state.cfg)
			p := tea.NewProgram(model, tea.WithAltScreen())
			if _, err := p.Run(); err != nil {
				return fmt.Errorf("tui: run: %w", err)
			}
			return nil
		},
	}
}

func newGetCommand(state *appState, configPath *string) *cobra.Command {
	var showAll bool
	cmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Get a config value",
		Args: func(cmd *cobra.Command, args []string) error {
			if showAll {
				return nil
			}
			if len(args) != 1 {
				return fmt.Errorf("%s requires [key] or --all", cmd.CommandPath())
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()
			if showAll {
				data, err := json.MarshalIndent(state.cfg, "", "  ")
				if err != nil {
					return fmt.Errorf("main: marshal config: %w", err)
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			val, ok := state.cfg.Get(args[0])
			if !ok {
				return fmt.Errorf("main: unknown config key %q", args[0])
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), val)
			return err
		},
	}
	cmd.Flags().BoolVar(&showAll, "all", false, "print the full config JSON")
	return cmd
}

func newSetCommand(state *appState, configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.init(cmd.Context(), *configPath); err != nil {
				return err
			}
			defer state.close()
			if err := state.cfg.Set(args[0], args[1]); err != nil {
				return err
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", args[0], args[1])
			return err
		},
	}
}

func newConfigCommand(configPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Configuration utilities",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "validate",
		Short: "Validate config file values",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := ensureConfig(*configPath)
			if err != nil {
				return err
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("config invalid: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "config valid: %s\n", *configPath)
			return err
		},
	})
	return cmd
}
