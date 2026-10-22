// Package ai wraps an open-weights model served by Ollama. The model only
// suggests and explains; it never makes the credit decision. Every call has a
// rule-based fallback so the product works with the model switched off.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// LLM is a minimal chat-completion interface.
type LLM interface {
	Complete(ctx context.Context, system, user string) (string, error)
}

type Ollama struct {
	baseURL string
	model   string
	http    *http.Client
	Timeout time.Duration
}

func NewOllama(baseURL, model string) *Ollama {
	return &Ollama{baseURL: baseURL, model: model, http: &http.Client{}, Timeout: 12 * time.Second}
}

func (o *Ollama) Complete(ctx context.Context, system, user string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	body, _ := json.Marshal(map[string]any{
		"model":    o.model,
		"stream":   false,
		"options":  map[string]any{"temperature": 0.2},
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("ollama status %d: %s", resp.StatusCode, b)
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("ollama decode: %w", err)
	}
	return out.Message.Content, nil
}
