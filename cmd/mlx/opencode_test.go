package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/omargallob/local-mlx/mlx"
)

func writeConfig(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestOpencodeCheckConfigured(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	dir := t.TempDir()
	writeConfig(t, dir, "opencode.json",
		`{"provider":{"mlx":{"options":{"baseURL":"`+srv.URL+`/v1"},"models":{"a":{},"b":{}}}}}`)

	e := &appEnv{client: mlx.NewClient(srv.URL), out: &bytes.Buffer{}}
	if err := cmdOpencode(context.Background(), e, []string{"--dir", dir, "check"}); err != nil {
		t.Fatalf("check (configured): %v", err)
	}
	out := e.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "configured") || !strings.Contains(out, "2 models") {
		t.Errorf("output = %q", out)
	}
}

func TestOpencodeCheckNotConfigured(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	dir := t.TempDir()
	writeConfig(t, dir, "opencode.json", `{"provider":{"other":{}}}`)

	e := &appEnv{client: mlx.NewClient(srv.URL), out: &bytes.Buffer{}}
	err := cmdOpencode(context.Background(), e, []string{"--dir", dir, "check"})
	if err == nil {
		t.Fatal("expected error (exit 1) when mlx not configured")
	}
}

func TestOpencodeCheckNoConfigFile(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e := &appEnv{client: mlx.NewClient(srv.URL), out: &bytes.Buffer{}}
	if err := cmdOpencode(context.Background(), e, []string{"--dir", t.TempDir(), "check"}); err == nil {
		t.Fatal("expected error when no config file exists")
	}
}

func TestOpencodeSuggestLiveModels(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e := &appEnv{client: mlx.NewClient(srv.URL), out: &bytes.Buffer{}}
	if err := cmdOpencode(context.Background(), e, []string{"suggest"}); err != nil {
		t.Fatalf("suggest: %v", err)
	}
	out := e.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "@ai-sdk/openai-compatible") || !strings.Contains(out, "model-a") {
		t.Errorf("suggest output = %q", out)
	}
	if !strings.Contains(out, srv.URL+"/v1") {
		t.Errorf("expected baseURL in output, got %q", out)
	}
}

func TestOpencodeSuggestServerDownPlaceholder(t *testing.T) {
	srv := fakeServer(t)
	url := srv.URL
	srv.Close() // nothing listening
	e := &appEnv{client: mlx.NewClient(url), out: &bytes.Buffer{}}
	if err := cmdOpencode(context.Background(), e, []string{"suggest"}); err != nil {
		t.Fatalf("suggest (down): %v", err)
	}
	out := e.out.(*bytes.Buffer).String()
	if !strings.Contains(out, "default_model") {
		t.Errorf("expected placeholder model when server down, got %q", out)
	}
}
