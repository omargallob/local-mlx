package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/omargallob/local-mlx/opencode"
)

// cmdOpencode checks whether OpenCode has an mlx provider, or suggests one.
//
//	mlx opencode [check|suggest] [--config path] [--dir path]
//
// It is read-only / suggest-only: it never writes any OpenCode file.
func cmdOpencode(ctx context.Context, e *appEnv, args []string) error {
	fs := flag.NewFlagSet("opencode", flag.ContinueOnError)
	fs.SetOutput(e.out)
	configPath := fs.String("config", "", "path to a specific opencode config file")
	dir := fs.String("dir", "", "opencode config dir (default: $XDG_CONFIG_HOME/opencode or ~/.config/opencode)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	mode := "check"
	if fs.NArg() > 0 {
		mode = fs.Arg(0)
	}
	switch mode {
	case "check":
		return opencodeCheck(e, *configPath, *dir)
	case "suggest":
		return opencodeSuggest(ctx, e)
	default:
		return fmt.Errorf("unknown opencode mode %q (want check or suggest)", mode)
	}
}

// opencodeConfigFiles resolves which config files to inspect.
func opencodeConfigFiles(configPath, dir string) ([]string, error) {
	if configPath != "" {
		return []string{configPath}, nil
	}
	d := dir
	if d == "" {
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			d = filepath.Join(xdg, "opencode")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, err
			}
			d = filepath.Join(home, ".config", "opencode")
		}
	}
	var files []string
	for _, name := range []string{"opencode.jsonc", "opencode.json"} {
		p := filepath.Join(d, name)
		if _, err := os.Stat(p); err == nil {
			files = append(files, p)
		}
	}
	return files, nil
}

func opencodeCheck(e *appEnv, configPath, dir string) error {
	files, err := opencodeConfigFiles(configPath, dir)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no opencode config found (looked for opencode.jsonc / opencode.json)")
	}

	configured := false
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		res, err := opencode.Detect(b)
		if err != nil {
			fmt.Fprintf(e.out, "%s: parse error: %v\n", f, err)
			continue
		}
		if !res.Configured {
			fmt.Fprintf(e.out, "%s: mlx provider NOT configured — run `mlx opencode suggest`\n", f)
			continue
		}
		configured = true
		fmt.Fprintf(e.out, "%s: mlx provider configured (baseURL=%s, %d models)\n", f, res.BaseURL, res.NumModels)
		if want := e.client.BaseURL + "/v1"; res.BaseURL != want {
			fmt.Fprintf(e.out, "  note: baseURL differs from current --host (%s)\n", want)
		}
		if res.NumModels <= 1 {
			fmt.Fprintf(e.out, "  note: only a placeholder model listed — run `mlx opencode suggest` to fill real models\n")
		}
	}
	if !configured {
		return fmt.Errorf("opencode has no mlx provider")
	}
	return nil
}

func opencodeSuggest(ctx context.Context, e *appEnv) error {
	models, err := e.client.Models(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "note: could not reach server (%v); suggesting a placeholder model\n", err)
		models = nil
	}
	block, err := opencode.SuggestProvider(e.client.BaseURL+"/v1", models)
	if err != nil {
		return err
	}
	fmt.Fprintln(e.out, string(block))
	return nil
}
