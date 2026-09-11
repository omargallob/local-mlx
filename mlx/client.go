// Package mlx is a small standard-library client for an mlx_lm.server
// (OpenAI-compatible) instance. It is the reusable core shared by the CLI and
// any future automation/agent code.
package mlx

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to an mlx_lm.server (OpenAI-compatible) instance.
type Client struct {
	BaseURL string // e.g. http://localhost:8080
	HTTP    *http.Client
}

// NewClient returns a Client for the given host. host may be "localhost:8080"
// or a full "http://host:port" URL.
func NewClient(host string) *Client {
	base := host
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}
	base = strings.TrimRight(base, "/")
	return &Client{
		BaseURL: base,
		HTTP:    &http.Client{Timeout: 5 * time.Minute},
	}
}

// Message is a single chat turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type model struct {
	ID string `json:"id"`
}

type modelsResponse struct {
	Data []model `json:"data"`
}

// Models lists the model IDs the server has available.
func (c *Client) Models(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/models", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("models: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var mr modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&mr); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(mr.Data))
	for _, m := range mr.Data {
		ids = append(ids, m.ID)
	}
	return ids, nil
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Chat sends messages and returns the complete assistant reply in one shot
// (non-streaming). It is the convenient primitive for automation callers.
func (c *Client) Chat(ctx context.Context, model string, messages []Message) (string, error) {
	resp, err := c.postChat(ctx, model, messages, false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := checkOK("chat", resp); err != nil {
		return "", err
	}
	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", err
	}
	if len(cr.Choices) == 0 {
		return "", nil
	}
	return cr.Choices[0].Message.Content, nil
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

// ChatStream sends messages and calls onToken for each streamed token.
// It returns the full assembled response.
func (c *Client) ChatStream(ctx context.Context, model string, messages []Message, onToken func(string)) (string, error) {
	resp, err := c.postChat(ctx, model, messages, true)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if err := checkOK("chat", resp); err != nil {
		return "", err
	}

	var sb strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue // skip malformed keep-alive lines
		}
		for _, ch := range chunk.Choices {
			if ch.Delta.Content != "" {
				sb.WriteString(ch.Delta.Content)
				if onToken != nil {
					onToken(ch.Delta.Content)
				}
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return sb.String(), err
	}
	return sb.String(), nil
}

func (c *Client) postChat(ctx context.Context, model string, messages []Message, stream bool) (*http.Response, error) {
	payload, err := json.Marshal(chatRequest{Model: model, Messages: messages, Stream: stream})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.HTTP.Do(req)
}

func checkOK(op string, resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("%s: %s: %s", op, resp.Status, strings.TrimSpace(string(body)))
}
