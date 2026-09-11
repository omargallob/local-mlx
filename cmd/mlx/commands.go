package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/omargallob/local-mlx/mlx"
)

// appEnv holds everything the subcommand handlers need. Injecting the client,
// reader, and writer keeps the handlers unit-testable without real network or
// os.Stdin/os.Stdout.
type appEnv struct {
	client *mlx.Client
	model  string // default model; empty means "use the server's first"
	in     io.Reader
	out    io.Writer
}

// resolveModel returns the configured model, or the server's first if unset.
func (e *appEnv) resolveModel(ctx context.Context) (string, error) {
	if e.model != "" {
		return e.model, nil
	}
	models, err := e.client.Models(ctx)
	if err != nil {
		return "", err
	}
	if len(models) == 0 {
		return "", fmt.Errorf("server reports no models; set --model or MLX_MODEL")
	}
	return models[0], nil
}

// cmdModels prints the available model ids, one per line.
func cmdModels(ctx context.Context, e *appEnv) error {
	models, err := e.client.Models(ctx)
	if err != nil {
		return err
	}
	for _, m := range models {
		fmt.Fprintln(e.out, m)
	}
	return nil
}

// cmdRun is the one-shot automation primitive: prompt in (arg or stdin),
// completion out.
func cmdRun(ctx context.Context, e *appEnv, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(e.out)
	system := fs.String("system", "", "optional system prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}

	prompt := strings.TrimSpace(strings.Join(fs.Args(), " "))
	if prompt == "" {
		b, err := io.ReadAll(e.in)
		if err != nil {
			return err
		}
		prompt = strings.TrimSpace(string(b))
	}
	if prompt == "" {
		return fmt.Errorf("no prompt: pass it as an argument or on stdin")
	}

	model, err := e.resolveModel(ctx)
	if err != nil {
		return err
	}

	msgs := make([]mlx.Message, 0, 2)
	if *system != "" {
		msgs = append(msgs, mlx.Message{Role: "system", Content: *system})
	}
	msgs = append(msgs, mlx.Message{Role: "user", Content: prompt})

	reply, err := e.client.Chat(ctx, model, msgs)
	if err != nil {
		return err
	}
	fmt.Fprintln(e.out, reply)
	return nil
}

// cmdChat is an interactive streaming REPL.
func cmdChat(ctx context.Context, e *appEnv, args []string) error {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	fs.SetOutput(e.out)
	system := fs.String("system", "", "optional system prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}

	model, err := e.resolveModel(ctx)
	if err != nil {
		return err
	}

	newHistory := func() []mlx.Message {
		if *system != "" {
			return []mlx.Message{{Role: "system", Content: *system}}
		}
		return nil
	}
	history := newHistory()

	sc := bufio.NewScanner(e.in)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for {
		fmt.Fprint(e.out, "you> ")
		if !sc.Scan() {
			fmt.Fprintln(e.out)
			return nil
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "/") {
			switch {
			case line == "/quit" || line == "/exit":
				return nil
			case line == "/reset":
				history = newHistory()
				fmt.Fprintln(e.out, "(conversation reset)")
			case line == "/models":
				models, err := e.client.Models(ctx)
				if err != nil {
					return err
				}
				fmt.Fprintln(e.out, strings.Join(models, ", "))
			case strings.HasPrefix(line, "/model "):
				model = strings.TrimSpace(strings.TrimPrefix(line, "/model "))
				fmt.Fprintf(e.out, "(model set to %s)\n", model)
			default:
				fmt.Fprintln(e.out, "unknown command; try /model <id>, /models, /reset, /quit")
			}
			continue
		}

		history = append(history, mlx.Message{Role: "user", Content: line})
		fmt.Fprint(e.out, "bot> ")
		reply, err := e.client.ChatStream(ctx, model, history, func(tok string) {
			fmt.Fprint(e.out, tok)
		})
		fmt.Fprintln(e.out)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			fmt.Fprintf(e.out, "error: %v\n", err)
			history = history[:len(history)-1] // drop the failed user turn
			continue
		}
		history = append(history, mlx.Message{Role: "assistant", Content: reply})
	}
}

// cmdHeartbeat is a long-lived loop that pings the server and logs status. It is
// the container's default command and the seed of the future agent loop.
func cmdHeartbeat(ctx context.Context, e *appEnv, args []string) error {
	fs := flag.NewFlagSet("heartbeat", flag.ContinueOnError)
	fs.SetOutput(e.out)
	interval := fs.Duration("interval", 30*time.Second, "time between pings")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ping := func() {
		ts := time.Now().Format(time.RFC3339)
		models, err := e.client.Models(ctx)
		if err != nil {
			fmt.Fprintf(e.out, "%s heartbeat DOWN %s: %v\n", ts, e.client.BaseURL, err)
			return
		}
		fmt.Fprintf(e.out, "%s heartbeat OK %s (%d models)\n", ts, e.client.BaseURL, len(models))
	}

	ping() // report immediately, then on each tick
	t := time.NewTicker(*interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			ping()
		}
	}
}
