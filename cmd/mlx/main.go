// Command mlx is a small CLI for a Mac-hosted mlx_lm.server. It runs on the Mac
// or on a Raspberry Pi that reaches the Mac over the LAN.
//
// Usage:
//
//	mlx [--host h] [--model m] <command> [args]
//
// Global flags must precede the command. Commands: models, run, chat, heartbeat.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/omargallob/local-mlx/mlx"
)

func main() {
	root := flag.NewFlagSet("mlx", flag.ExitOnError)
	host := root.String("host", envOr("MLX_HOST", "localhost:8080"), "mlx server host (host:port or URL); env MLX_HOST")
	model := root.String("model", os.Getenv("MLX_MODEL"), "model id (default: the server's first); env MLX_MODEL")
	root.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: mlx [--host h] [--model m] <command> [args]\n\n")
		fmt.Fprintf(os.Stderr, "commands:\n")
		fmt.Fprintf(os.Stderr, "  ping                check the server is reachable\n")
		fmt.Fprintf(os.Stderr, "  models              list available models\n")
		fmt.Fprintf(os.Stderr, "  run [prompt]        one-shot completion (prompt from arg or stdin)\n")
		fmt.Fprintf(os.Stderr, "  chat                interactive streaming chat\n")
		fmt.Fprintf(os.Stderr, "  heartbeat           long-lived server ping loop (container default)\n\n")
		fmt.Fprintf(os.Stderr, "global flags:\n")
		root.PrintDefaults()
	}
	_ = root.Parse(os.Args[1:])

	args := root.Args()
	if len(args) == 0 {
		root.Usage()
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	e := &appEnv{
		client: mlx.NewClient(*host),
		model:  *model,
		in:     os.Stdin,
		out:    os.Stdout,
	}

	var err error
	switch cmd := args[0]; cmd {
	case "ping":
		// The command is itself the health check.
		err = cmdPing(ctx, e)
	case "models":
		if err = preflight(ctx, e, preflightTimeout); err == nil {
			err = cmdModels(ctx, e)
		}
	case "run":
		if err = preflight(ctx, e, preflightTimeout); err == nil {
			err = cmdRun(ctx, e, args[1:])
		}
	case "chat":
		if err = preflight(ctx, e, preflightTimeout); err == nil {
			err = cmdChat(ctx, e, args[1:])
		}
	case "heartbeat":
		// A monitor must keep running even when the target is down, so the
		// preflight here is informational, not fatal.
		if perr := preflight(ctx, e, preflightTimeout); perr != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", perr)
		}
		err = cmdHeartbeat(ctx, e, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", cmd)
		root.Usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
