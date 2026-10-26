package ai

import (
	"context"
	"strings"
	"testing"
)

// The reference set of docs/ai-chat.md, as a regression test of the handbook (the model is off): every
// question gets the behaviour it was compared on, and no answer invents a figure, a deadline or a promise.
func TestGoldenSet_Handbook(t *testing.T) {
	chat := NewChat(nil)
	for _, q := range GoldenSet {
		t.Run(q.Text, func(t *testing.T) {
			if q.Expect == Operator {
				if !WantsOperator(q.Text) {
					t.Fatal("a request for a person must be recognised before any model")
				}
				return
			}
			r := chat.Reply(context.Background(), q.History, q.Text, GoldenFacts)
			if r.Source != "rules" || strings.TrimSpace(r.Text) == "" {
				t.Fatalf("no handbook answer: %+v", r)
			}
			if (q.Expect == Refuse) != r.OffTopic {
				t.Fatalf("off topic = %v, want %v: %s", r.OffTopic, q.Expect == Refuse, r.Text)
			}
			low := strings.ToLower(r.Text)
			for _, m := range append(append([]string{}, promiseMarks...), inventedMarks...) {
				if strings.Contains(low, m) && !strings.Contains(strings.ToLower(q.Text), m) {
					t.Errorf("the answer says %q, which the handbook never claims: %s", m, r.Text)
				}
			}
		})
	}
}

// With a model that makes things up, the answers still pass the same checks: the code replaces them.
func TestGoldenSet_InventiveModelIsContained(t *testing.T) {
	chat := NewChat(&fakeLLM{reply: "Обычно решение занимает 3-5 рабочих дней, звоните на горячую линию, одобрение гарантировано."})
	for _, q := range GoldenSet {
		if q.Expect != Answer {
			continue
		}
		r := chat.Reply(context.Background(), q.History, q.Text, GoldenFacts)
		low := strings.ToLower(r.Text)
		for _, m := range []string{"рабочих дн", "горяч", "гарантир"} {
			if strings.Contains(low, m) && !strings.Contains(strings.ToLower(q.Text), m) {
				t.Errorf("%q: an invented claim reached the client: %s", q.Text, r.Text)
			}
		}
	}
}
