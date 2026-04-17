package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/velariumai/pdv/internal/config"
	"github.com/velariumai/pdv/internal/database"
)

type appState struct {
	configPath string
	dbPath     string
	cfg        *config.Config
	db         *database.DB
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
	)
	root := &cobra.Command{
		Use:   "pdv",
		Short: "PDV is a self-hosted download manager",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagVersion {
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "pdv %s (%s)\n", Version, BuildDate)
				return err
			}
			if err := state.init(cmd.Context(), flagConfigPath); err != nil {
				return err
			}
			defer state.close()
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "pdv initialized: config=%s db=%s\n", state.configPath, state.dbPath)
			return err
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&flagConfigPath, "config", state.configPath, "path to config file")
	root.Flags().BoolVar(&flagVersion, "version", false, "show version and exit")
	root.AddCommand(newGetCommand(state, &flagConfigPath))
	root.AddCommand(newSetCommand(state, &flagConfigPath))
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		return err
	}
	return nil
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
	a.cfg = cfg
	a.db = db
	return nil
}

func (a *appState) close() {
	if a.db == nil {
		return
	}
	_ = a.db.Close()
	a.db = nil
}

func ensureConfig(path string) (*config.Config, error) {
	if _, err := os.Stat(path); err == nil {
		cfg, loadErr := config.Load(path)
		if loadErr != nil {
			return nil, fmt.Errorf("main: load config: %w", loadErr)
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
	return loaded, nil
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
