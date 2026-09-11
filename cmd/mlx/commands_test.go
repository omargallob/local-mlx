package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/omargallob/local-mlx/mlx"
)

// fakeServer serves /v1/models and /v1/chat/completions (streaming and not).
func fakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			w.Write([]byte(`{"data":[{"id":"model-a"},{"id":"model-b"}]}`))
		case "/v1/chat/completions":
			var body struct {
				Stream bool `json:"stream"`
			}
			raw, _ := io.ReadAll(r.Body)
			json.Unmarshal(raw, &body)
			if body.Stream {
				w.Header().Set("Content-Type", "text/event-stream")
				w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"pong\"}}]}\n\n"))
				w.Write([]byte("data: [DONE]\n\n"))
				return
			}
			w.Write([]byte(`{"choices":[{"message":{"content":"pong"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
}

func newEnv(srv *httptest.Server, in string) (*appEnv, *bytes.Buffer) {
	out := &bytes.Buffer{}
	return &appEnv{
		client: mlx.NewClient(srv.URL),
		in:     strings.NewReader(in),
		out:    out,
	}, out
}

func TestCmdModels(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e, out := newEnv(srv, "")
	if err := cmdModels(context.Background(), e); err != nil {
		t.Fatalf("cmdModels: %v", err)
	}
	if got := out.String(); got != "model-a\nmodel-b\n" {
		t.Errorf("output = %q", got)
	}
}

func TestResolveModelDefaultsToFirst(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e, _ := newEnv(srv, "")
	got, err := e.resolveModel(context.Background())
	if err != nil {
		t.Fatalf("resolveModel: %v", err)
	}
	if got != "model-a" {
		t.Errorf("resolveModel = %q, want model-a", got)
	}
}

func TestCmdRunFromArg(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e, out := newEnv(srv, "")
	if err := cmdRun(context.Background(), e, []string{"say", "hi"}); err != nil {
		t.Fatalf("cmdRun: %v", err)
	}
	if strings.TrimSpace(out.String()) != "pong" {
		t.Errorf("output = %q, want pong", out.String())
	}
}

func TestCmdRunFromStdin(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e, out := newEnv(srv, "hello from stdin")
	if err := cmdRun(context.Background(), e, nil); err != nil {
		t.Fatalf("cmdRun: %v", err)
	}
	if strings.TrimSpace(out.String()) != "pong" {
		t.Errorf("output = %q, want pong", out.String())
	}
}

func TestCmdRunEmptyPromptErrors(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e, _ := newEnv(srv, "   \n")
	if err := cmdRun(context.Background(), e, nil); err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestCmdChatStreamsAndQuits(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e, out := newEnv(srv, "hi\n/quit\n")
	if err := cmdChat(context.Background(), e, nil); err != nil {
		t.Fatalf("cmdChat: %v", err)
	}
	if !strings.Contains(out.String(), "pong") {
		t.Errorf("expected streamed reply in output, got %q", out.String())
	}
}

func TestCmdChatModelSwitch(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e, out := newEnv(srv, "/model model-b\n/quit\n")
	if err := cmdChat(context.Background(), e, nil); err != nil {
		t.Fatalf("cmdChat: %v", err)
	}
	if !strings.Contains(out.String(), "model set to model-b") {
		t.Errorf("expected model-switch confirmation, got %q", out.String())
	}
}

func TestCmdHeartbeatTicksThenStops(t *testing.T) {
	srv := fakeServer(t)
	defer srv.Close()
	e, out := newEnv(srv, "")
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() { done <- cmdHeartbeat(ctx, e, []string{"--interval", "20ms"}) }()

	time.Sleep(70 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("cmdHeartbeat: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("heartbeat did not stop on cancel")
	}

	if n := strings.Count(out.String(), "heartbeat OK"); n < 2 {
		t.Errorf("expected multiple heartbeat lines, got %d in %q", n, out.String())
	}
}
