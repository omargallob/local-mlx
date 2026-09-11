package mlx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewClientHostNormalization(t *testing.T) {
	cases := map[string]string{
		"localhost:8080":         "http://localhost:8080",
		"http://localhost:8080":  "http://localhost:8080",
		"http://localhost:8080/": "http://localhost:8080",
		"https://example.com":    "https://example.com",
		"1.2.3.4:8080":           "http://1.2.3.4:8080",
	}
	for in, want := range cases {
		if got := NewClient(in).BaseURL; got != want {
			t.Errorf("NewClient(%q).BaseURL = %q, want %q", in, got, want)
		}
	}
}

func TestModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{"object":"list","data":[{"id":"a"},{"id":"b"}]}`))
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL).Models(context.Background())
	if err != nil {
		t.Fatalf("Models: %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Errorf("Models = %v, want [a b]", got)
	}
}

func TestModelsErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL).Models(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error %q should contain server body", err)
	}
}

func TestChat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"hello there"}}]}`))
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL).Chat(context.Background(), "m", []Message{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if got != "hello there" {
		t.Errorf("Chat = %q, want %q", got, "hello there")
	}
}

func TestChatStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Include a blank line and a malformed keep-alive to exercise skipping.
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n"))
		w.Write([]byte("data: not-json\n\n"))
		w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n"))
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer srv.Close()

	var tokens []string
	full, err := NewClient(srv.URL).ChatStream(context.Background(), "m",
		[]Message{{Role: "user", Content: "hi"}},
		func(tok string) { tokens = append(tokens, tok) })
	if err != nil {
		t.Fatalf("ChatStream: %v", err)
	}
	if full != "Hello" {
		t.Errorf("assembled = %q, want %q", full, "Hello")
	}
	if strings.Join(tokens, "") != "Hello" {
		t.Errorf("tokens = %v, want to join to Hello", tokens)
	}
}

func TestPingUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"a"}]}`))
	}))
	defer srv.Close()
	if err := NewClient(srv.URL).Ping(context.Background()); err != nil {
		t.Fatalf("Ping (up): %v", err)
	}
}

func TestPingErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if err := NewClient(srv.URL).Ping(context.Background()); err == nil {
		t.Fatal("expected error for non-200")
	}
}

func TestPingUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // now nothing is listening

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := NewClient(url).Ping(ctx); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestChatStreamErrorOnNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL).ChatStream(context.Background(), "m", nil, nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q should contain server body", err)
	}
}
