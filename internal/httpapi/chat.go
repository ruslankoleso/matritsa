package httpapi

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"matritsa/internal/ai"
	"matritsa/internal/auth"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/workflow"
)

// ChatWithAssistant answers a question about credit and the cabinet. The person's own applications
// are the only personal facts the helper gets.
func (s *Server) ChatWithAssistant(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	var req gen.ChatRequest
	if !decodeBody(w, r, &req) {
		return
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Напишите вопрос", []fieldErr{{"message", "Напишите вопрос о кредитах или работе кабинета"}})
		return
	}
	if utf8.RuneCountInString(msg) > 500 {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Вопрос слишком длинный", []fieldErr{{"message", "Сократите вопрос до 500 символов"}})
		return
	}
	if !s.ChatLimiter.Allowed(claims.UserID) {
		writeError(w, http.StatusTooManyRequests, "too_many_messages", "Слишком много вопросов за короткое время. Подождите несколько минут.")
		return
	}
	s.ChatLimiter.Fail(claims.UserID) // every question counts

	var history []ai.ChatTurn
	if req.History != nil {
		for _, t := range *req.History {
			if t.Role == "user" || t.Role == "assistant" {
				history = append(history, ai.ChatTurn{Role: string(t.Role), Text: t.Text})
			}
		}
	}
	var facts ai.ChatContext
	if apps, err := s.Store.ListApplicationsByUser(r.Context(), claims.UserID); err == nil {
		for _, a := range apps {
			st := workflow.Status(a.Status)
			facts.Applications = append(facts.Applications, ai.AppFact{
				ProductTitle: productTitle(a.ProductID), Amount: a.Amount, TermMonths: int(a.TermMonths),
				StatusLabel: workflow.ClientLabel(st), StatusMessage: workflow.ClientMessage(st),
			})
		}
	}
	rep := s.Chat.Reply(r.Context(), history, msg, facts)
	writeJSON(w, http.StatusOK, gen.ChatReply{Text: rep.Text, Source: gen.ChatReplySource(rep.Source), OffTopic: rep.OffTopic})
}
