// Command chatcompare puts the helper's answers side by side: from the handbook alone and from a model.
// It is a development aid for choosing how the chat answers (see docs/ai-chat.md); it needs a running Ollama.
//
//	go run ./cmd/chatcompare -model hf.co/t-tech/T-lite-it-2.1-GGUF:Q4_K_M
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"matritsa/internal/ai"
)

func main() {
	model := flag.String("model", "", "Ollama model; empty runs the handbook only")
	url := flag.String("url", "http://localhost:11434", "Ollama address")
	flag.Parse()
	facts := ai.GoldenFacts // the reference set is shared with the regression test in internal/ai
	rules := ai.NewChat(nil)
	var llm *ai.Chat
	if *model != "" {
		llm = ai.NewChat(ai.NewOllama(*url, *model))
	}
	ctx := context.Background()
	var aiCount, ruleCount int
	var total time.Duration
	for i, q := range ai.GoldenSet {
		fmt.Printf("\n[%d] %s\n", i+1, q.Text)
		r := rules.Reply(ctx, q.History, q.Text, facts)
		fmt.Printf("  справочник: %s\n", oneLine(r.Text))
		if llm == nil {
			continue
		}
		start := time.Now()
		a := llm.Reply(ctx, q.History, q.Text, facts)
		d := time.Since(start)
		total += d
		if a.Source == "ai" {
			aiCount++
		} else {
			ruleCount++
		}
		fmt.Printf("  %-10s (%.1f с, %d симв.): %s\n", a.Source, d.Seconds(), utf8.RuneCountInString(a.Text), oneLine(a.Text))
	}
	if llm != nil {
		fmt.Fprintf(os.Stderr, "\nответов модели: %d, откатов на справочник: %d, среднее время %.1f с\n", aiCount, ruleCount, total.Seconds()/float64(len(ai.GoldenSet)))
	}
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }
