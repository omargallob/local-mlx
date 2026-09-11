package opencode

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStripJSONC(t *testing.T) {
	in := []byte(`{
  // line comment
  "url": "http://x/*not a comment*/y", /* trailing */
  /* block
     comment */
  "n": 1
}`)
	got := StripJSONC(in)
	var m map[string]any
	if err := json.Unmarshal(got, &m); err != nil {
		t.Fatalf("stripped output is not valid JSON: %v\n%s", err, got)
	}
	if m["url"] != "http://x/*not a comment*/y" {
		t.Errorf("comment marker inside string was mangled: %v", m["url"])
	}
	if m["n"].(float64) != 1 {
		t.Errorf("n = %v", m["n"])
	}
}

func TestDetectConfigured(t *testing.T) {
	b := []byte(`{"provider":{"mlx":{"options":{"baseURL":"http://h:8080/v1"},"models":{"a":{},"b":{}}}}}`)
	res, err := Detect(b)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !res.Configured || res.BaseURL != "http://h:8080/v1" || res.NumModels != 2 {
		t.Errorf("res = %+v", res)
	}
}

func TestDetectNotConfigured(t *testing.T) {
	b := []byte(`{"provider":{"other":{"options":{"baseURL":"x"}}}}`)
	res, err := Detect(b)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if res.Configured {
		t.Errorf("expected not configured, got %+v", res)
	}
}

func TestDetectJSONC(t *testing.T) {
	b := []byte(`{
  // my config
  "provider": { "mlx": { "options": { "baseURL": "http://h/v1" }, "models": { "a": {} } } }
}`)
	res, err := Detect(b)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if !res.Configured || res.NumModels != 1 {
		t.Errorf("res = %+v", res)
	}
}

func TestSuggestProviderWithModels(t *testing.T) {
	out, err := SuggestProvider("http://h:8080/v1", []string{"m1", "m2"})
	if err != nil {
		t.Fatalf("SuggestProvider: %v", err)
	}
	// Must be valid JSON with the mlx provider and both models.
	var doc struct {
		Provider struct {
			Mlx struct {
				NPM     string `json:"npm"`
				Options struct {
					BaseURL string `json:"baseURL"`
				} `json:"options"`
				Models map[string]struct {
					Name string `json:"name"`
				} `json:"models"`
			} `json:"mlx"`
		} `json:"provider"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("output not valid JSON: %v\n%s", err, out)
	}
	if doc.Provider.Mlx.NPM != "@ai-sdk/openai-compatible" {
		t.Errorf("npm = %q", doc.Provider.Mlx.NPM)
	}
	if doc.Provider.Mlx.Options.BaseURL != "http://h:8080/v1" {
		t.Errorf("baseURL = %q", doc.Provider.Mlx.Options.BaseURL)
	}
	if _, ok := doc.Provider.Mlx.Models["m1"]; !ok {
		t.Errorf("missing m1 in %s", out)
	}
	if doc.Provider.Mlx.Models["m2"].Name != "m2" {
		t.Errorf("m2 name = %q", doc.Provider.Mlx.Models["m2"].Name)
	}
}

func TestSuggestProviderPlaceholder(t *testing.T) {
	out, err := SuggestProvider("http://h/v1", nil)
	if err != nil {
		t.Fatalf("SuggestProvider: %v", err)
	}
	if !strings.Contains(string(out), "default_model") {
		t.Errorf("expected placeholder default_model, got %s", out)
	}
}
