// Package opencode inspects an OpenCode config for an mlx provider and can build
// a suggested provider block populated with the server's models. It never writes
// files — callers print the suggestion for the user to paste.
package opencode

import (
	"bytes"
	"encoding/json"
)

// Result summarizes whether an OpenCode config has an mlx provider.
type Result struct {
	Configured bool   // provider.mlx exists
	BaseURL    string // provider.mlx.options.baseURL
	NumModels  int    // number of models listed under provider.mlx
}

type configFile struct {
	Provider map[string]providerEntry `json:"provider"`
}

type providerEntry struct {
	Options struct {
		BaseURL string `json:"baseURL"`
	} `json:"options"`
	Models map[string]json.RawMessage `json:"models"`
}

// Detect parses OpenCode config bytes (JSON or JSONC) and reports mlx status.
func Detect(b []byte) (Result, error) {
	var cf configFile
	if err := json.Unmarshal(StripJSONC(b), &cf); err != nil {
		return Result{}, err
	}
	entry, ok := cf.Provider["mlx"]
	if !ok {
		return Result{Configured: false}, nil
	}
	return Result{
		Configured: true,
		BaseURL:    entry.Options.BaseURL,
		NumModels:  len(entry.Models),
	}, nil
}

// SuggestProvider returns pretty-printed JSON for an mlx provider block (wrapped
// under a "provider" key) ready to paste into an OpenCode config. If modelIDs is
// empty, a single placeholder model is used.
func SuggestProvider(baseURL string, modelIDs []string) ([]byte, error) {
	models := map[string]any{}
	if len(modelIDs) == 0 {
		models["default_model"] = map[string]any{"name": "Default MLX Model"}
	} else {
		for _, id := range modelIDs {
			models[id] = map[string]any{"name": id}
		}
	}
	block := map[string]any{
		"provider": map[string]any{
			"mlx": map[string]any{
				"npm":     "@ai-sdk/openai-compatible",
				"name":    "MLX (local)",
				"options": map[string]any{"baseURL": baseURL},
				"models":  models,
			},
		},
	}
	return json.MarshalIndent(block, "", "  ")
}

// StripJSONC removes // line and /* */ block comments, ignoring markers inside
// strings, so a JSONC document parses with encoding/json.
func StripJSONC(b []byte) []byte {
	var out bytes.Buffer
	inString, escaped := false, false
	for i := 0; i < len(b); i++ {
		c := b[i]
		if inString {
			out.WriteByte(c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			out.WriteByte(c)
		case c == '/' && i+1 < len(b) && b[i+1] == '/':
			for i < len(b) && b[i] != '\n' {
				i++
			}
			if i < len(b) {
				out.WriteByte('\n')
			}
		case c == '/' && i+1 < len(b) && b[i+1] == '*':
			i += 2
			for i+1 < len(b) && (b[i] != '*' || b[i+1] != '/') {
				i++
			}
			i++ // skip past the closing '/', outer loop advances again
		default:
			out.WriteByte(c)
		}
	}
	return out.Bytes()
}
