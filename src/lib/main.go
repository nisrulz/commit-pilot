package lib

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// Main is the entry point for the commit-pilot CLI. It parses the command-line
// arguments, resolves the configuration, and dispatches to the requested
// sub-command (list models, doctor, or the commit workflow).
func Main() {
	flags, showHelp := ParseArgs(os.Args[1:])
	if showHelp {
		printHelp()
		return
	}
	SetOutputMode(flags.Quiet, flags.JSON)
	if flags.Error != "" {
		Die("arguments: %s", flags.Error)
	}

	cfg, err := ResolveConfig(flags)
	if err != nil {
		Die("config: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	cfg.Context = ctx
	SetOutputMode(cfg.Quiet, cfg.JSON)
	if cfg.JSON {
		// Keep stdout clean for the JSON result; progress and prompts go to stderr.
		cfg.Output = os.Stderr
	}
	switch {
	case flags.ListModels:
		PrintBanner()
		PrintSeparator()
		runListModels(cfg)
	case flags.Doctor:
		PrintBanner()
		PrintSeparator()
		runDoctorCheck(cfg)
	default:
		PrintBanner()
		PrintSeparator()
		cfg = AnnounceProvider(cfg)
		PrintSeparator()
		runWorkflow(cfg)
	}
}
