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
	"regexp"
	"strings"
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

// JSONCompleter is implemented by models that can be told to answer in valid JSON only.
type JSONCompleter interface {
	CompleteJSON(ctx context.Context, system, user string) (string, error)
}

// Complete asks for a plain-text answer.
func (o *Ollama) Complete(ctx context.Context, system, user string) (string, error) {
	return o.chat(ctx, system, user, false)
}

// CompleteJSON asks the model to answer in valid JSON only: small models otherwise break the quoting.
func (o *Ollama) CompleteJSON(ctx context.Context, system, user string) (string, error) {
	return o.chat(ctx, system, user, true)
}

func (o *Ollama) chat(ctx context.Context, system, user string, asJSON bool) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()
	payload := map[string]any{
		"model":  o.model,
		"stream": false,
		// no chain of reasoning: it only lengthens the answer and may leak into the text
		"think": false,
		// 4096 tokens hold the system prompt, the facts and the answer; the default context of some models is larger and slower.
		"options":  map[string]any{"temperature": 0.2, "num_ctx": 4096},
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": user}},
	}
	if asJSON {
		payload["format"] = "json"
	}
	body, _ := json.Marshal(payload)
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
	return cleanAnswer(out.Message.Content), nil
}

// cleanAnswer drops what some models leak into the text: the end of Qwen3 reasoning ("</think>"),
// a stray ">" line, the role name T-lite prints before its answer and the empty lines before the answer.
func cleanAnswer(s string) string {
	if i := strings.LastIndex(s, "</think>"); i >= 0 {
		s = s[i+len("</think>"):]
	}
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimPrefix(s, ">\n"))
	return leakedRole.ReplaceAllString(s, "")
}

// leakedRole is a chat-template role name at the very start: "assistant Text", "assistant\nText", "Assistant: Text".
var leakedRole = regexp.MustCompile(`^(?i:assistant)(?::\s*|\s+)`)
