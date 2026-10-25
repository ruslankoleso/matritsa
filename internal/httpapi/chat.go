package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"matritsa/internal/ai"
	"matritsa/internal/auth"
	"matritsa/internal/events"
	"matritsa/internal/httpapi/gen"
	"matritsa/internal/store/db"
	"matritsa/internal/workflow"
)

// One running talk per client. The helper answers while the thread is 'bot' or 'waiting'; once a support
// specialist joins ('live') the helper is silent until the specialist closes the talk or the client leaves it.
const (
	chatBot     = "bot"
	chatWaiting = "waiting"
	chatLive    = "live"

	chatWindow     = 100 // messages returned for a thread
	chatHistory    = 12  // messages the helper sees
	maxClientRunes = 500
	maxAgentRunes  = 1000
)

const (
	noteQueued   = "Запрос передан сотруднику поддержки. Он подключится к разговору, как освободится. Пока вы ждёте, помощник продолжит отвечать на вопросы по кредитам."
	noteReleased = "Вы вернулись к помощнику."
	noteClosed   = "Сотрудник завершил разговор. Дальше вам отвечает помощник."
	noteLeft     = "Клиент вернулся к помощнику, разговор завершён."
)

// agentLabel turns "Мария Игоревна Громова, специалист поддержки" into what a client should see:
// "Мария Игоревна, специалист поддержки" (no surname).
func agentLabel(fullName string) string {
	name, role, _ := strings.Cut(fullName, ",")
	if f := strings.Fields(name); len(f) >= 3 {
		name = f[0] + " " + f[1]
	}
	name = strings.TrimSpace(name)
	if role = strings.TrimSpace(role); role != "" {
		return name + ", " + role
	}
	return name
}

func (s *Server) chatMessageView(m db.ChatMessage) gen.ChatMessage {
	v := gen.ChatMessage{Id: m.ID, Author: gen.ChatMessageAuthor(m.Author), Text: m.Text, OffTopic: m.OffTopic, At: m.At.Time}
	if m.Source.Valid {
		src := gen.ChatMessageSource(m.Source.String)
		v.Source = &src
	}
	if m.AgentName.Valid {
		v.AgentName = &m.AgentName.String
	}
	return v
}

func (s *Server) threadView(ctx context.Context, t db.ChatThread) (gen.ChatThread, error) {
	msgs, err := s.Store.ListChatMessages(ctx, db.ListChatMessagesParams{ThreadID: t.ID, Limit: chatWindow})
	if err != nil {
		return gen.ChatThread{}, err
	}
	v := gen.ChatThread{Id: parseUUID(t.ID), Status: gen.ChatThreadStatus(t.Status), Messages: make([]gen.ChatMessage, 0, len(msgs))}
	for _, m := range msgs {
		v.Messages = append(v.Messages, s.chatMessageView(m))
	}
	if t.Status == chatLive && t.AgentID != nil {
		if a, err := s.Store.GetUser(ctx, *t.AgentID); err == nil {
			label := agentLabel(a.FullName)
			v.AgentName = &label
		}
	}
	return v, nil
}

func (s *Server) note(ctx context.Context, threadID, text string) error {
	_, err := s.Store.InsertChatMessage(ctx, db.InsertChatMessageParams{ThreadID: threadID, Author: "system", Text: text})
	return err
}

// ringSupport and ringClient tell open screens to refetch the thread.
func (s *Server) ringSupport(threadID, status string) {
	s.Hub.Publish("support", events.Event{Type: "chat", ThreadID: threadID, Status: status})
}

func (s *Server) ringUser(userID, threadID, status string) {
	s.Hub.Publish(userID, events.Event{Type: "chat", ThreadID: threadID, Status: status})
}

// ---------------------------------------------------------------- client

func (s *Server) GetChat(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	t, err := s.Store.EnsureChatThread(r.Context(), claims.UserID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	s.respondThread(w, r, t)
}

func (s *Server) respondThread(w http.ResponseWriter, r *http.Request, t db.ChatThread) {
	v, err := s.threadView(r.Context(), t)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// SendChatMessage stores the client's message and, unless a specialist is in the talk, the helper's answer.
// A request for a person puts the talk in the support queue.
func (s *Server) SendChatMessage(w http.ResponseWriter, r *http.Request) {
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
	if utf8.RuneCountInString(msg) > maxClientRunes {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Вопрос слишком длинный", []fieldErr{{"message", "Сократите вопрос до 500 символов"}})
		return
	}
	if !s.ChatLimiter.Allowed(claims.UserID) {
		writeError(w, http.StatusTooManyRequests, "too_many_messages", "Слишком много сообщений за короткое время. Подождите несколько минут.")
		return
	}
	s.ChatLimiter.Fail(claims.UserID) // every message counts

	ctx := r.Context()
	t, err := s.Store.EnsureChatThread(ctx, claims.UserID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	history, err := s.helperHistory(ctx, t.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if _, err := s.Store.InsertChatMessage(ctx, db.InsertChatMessageParams{ThreadID: t.ID, Author: "client", Text: msg}); err != nil {
		internalError(w, r, err)
		return
	}

	switch {
	case t.Status == chatLive:
		// A person is in the talk: the helper stays silent, the specialist is told.
		if t.AgentID != nil {
			s.ringUser(*t.AgentID, t.ID, chatLive)
		}
		s.ringSupport(t.ID, chatLive)
	case t.Status == chatBot && ai.WantsOperator(msg):
		if t, err = s.queue(ctx, t); err != nil {
			internalError(w, r, err)
			return
		}
	default:
		cc := s.chatContext(ctx, claims.UserID, t.Status == chatWaiting)
		rep := s.Chat.Reply(ctx, history, msg, cc)
		src := rep.Source
		if _, err := s.Store.InsertChatMessage(ctx, db.InsertChatMessageParams{ThreadID: t.ID, Author: "assistant", Text: rep.Text, Source: pgtype.Text{String: src, Valid: true}, OffTopic: rep.OffTopic}); err != nil {
			internalError(w, r, err)
			return
		}
		if t.Status == chatWaiting {
			s.ringSupport(t.ID, chatWaiting) // the person is still waiting and has written again
		}
	}
	t, err = s.Store.GetChatThread(ctx, t.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	s.respondThread(w, r, t)
}

// queue moves a 'bot' thread to the support queue; a thread that is already queued is returned unchanged.
func (s *Server) queue(ctx context.Context, t db.ChatThread) (db.ChatThread, error) {
	q, err := s.Store.RequestChatOperator(ctx, t.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return t, nil
	}
	if err != nil {
		return t, err
	}
	if err := s.note(ctx, q.ID, noteQueued); err != nil {
		return q, err
	}
	s.ringSupport(q.ID, chatWaiting)
	return q, nil
}

// helperHistory is what the helper has seen of the talk: the client's messages and its own answers.
func (s *Server) helperHistory(ctx context.Context, threadID string) ([]ai.ChatTurn, error) {
	msgs, err := s.Store.ListChatMessages(ctx, db.ListChatMessagesParams{ThreadID: threadID, Limit: chatHistory})
	if err != nil {
		return nil, err
	}
	var h []ai.ChatTurn
	for _, m := range msgs {
		switch m.Author {
		case "client":
			h = append(h, ai.ChatTurn{Role: "user", Text: m.Text})
		case "assistant":
			h = append(h, ai.ChatTurn{Role: "assistant", Text: m.Text})
		}
	}
	return h, nil
}

// chatContext holds the person's own applications, the only personal facts the helper gets.
func (s *Server) chatContext(ctx context.Context, userID string, waiting bool) ai.ChatContext {
	cc := ai.ChatContext{OperatorRequested: waiting}
	if apps, err := s.Store.ListApplicationsByUser(ctx, userID); err == nil {
		for _, a := range apps {
			st := workflow.Status(a.Status)
			cc.Applications = append(cc.Applications, ai.AppFact{
				ProductTitle: productTitle(a.ProductID), Amount: a.Amount, TermMonths: int(a.TermMonths),
				StatusLabel: workflow.ClientLabel(st), StatusMessage: workflow.ClientMessage(st),
			})
		}
	}
	return cc
}

func (s *Server) RequestChatOperator(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	t, err := s.Store.EnsureChatThread(r.Context(), claims.UserID)
	if err == nil {
		t, err = s.queue(r.Context(), t)
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	t, err = s.Store.GetChatThread(r.Context(), t.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	s.respondThread(w, r, t)
}

func (s *Server) CancelChatOperator(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	ctx := r.Context()
	t, err := s.Store.EnsureChatThread(ctx, claims.UserID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	prev := t
	if rel, err := s.Store.ReleaseChatThread(ctx, t.ID); err == nil {
		t = rel
		if err := s.note(ctx, t.ID, noteReleased); err != nil {
			internalError(w, r, err)
			return
		}
		if prev.AgentID != nil {
			s.ringUser(*prev.AgentID, t.ID, chatBot)
		}
		s.ringSupport(t.ID, chatBot)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		internalError(w, r, err)
		return
	}
	s.respondThread(w, r, t)
}

// ---------------------------------------------------------------- support

func (s *Server) ListSupportChats(w http.ResponseWriter, r *http.Request) {
	claims, _ := auth.FromContext(r.Context())
	rows, err := s.Store.ListSupportChats(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	out := gen.SupportChatList{Items: make([]gen.SupportChatItem, 0, len(rows))}
	for _, row := range rows {
		it := gen.SupportChatItem{
			Id: parseUUID(row.ID), Status: gen.SupportChatItemStatus(row.Status), ClientName: row.ClientName, CompanyName: row.CompanyName,
			LastText: row.LastText, Mine: row.AgentID != nil && *row.AgentID == claims.UserID,
		}
		if row.Status == chatWaiting {
			out.Waiting++
			if row.RequestedAt.Valid {
				it.WaitingMinutes = int(timeSince(row.RequestedAt.Time).Minutes())
			}
		} else {
			out.Live++
			if row.AgentName != "" {
				label := agentLabel(row.AgentName)
				it.AgentName = &label
			}
		}
		out.Items = append(out.Items, it)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) supportThread(w http.ResponseWriter, r *http.Request, id string) (db.ChatThread, bool) {
	t, err := s.Store.GetChatThread(r.Context(), id)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "Обращение не найдено")
		return t, false
	}
	if err != nil {
		internalError(w, r, err)
		return t, false
	}
	return t, true
}

func (s *Server) supportView(ctx context.Context, userID string, t db.ChatThread) (gen.SupportChat, error) {
	th, err := s.threadView(ctx, t)
	if err != nil {
		return gen.SupportChat{}, err
	}
	cl, err := s.Store.GetChatClient(ctx, t.ID)
	if err != nil {
		return gen.SupportChat{}, err
	}
	v := gen.SupportChat{Thread: th, ClientName: cl.FullName, CompanyName: cl.CompanyName, Mine: t.AgentID != nil && *t.AgentID == userID, Applications: []gen.SupportApplicationFact{}}
	if cl.Inn != "" {
		v.Inn = &cl.Inn
	}
	// Statuses only, as the client sees them: a support specialist does not decide applications and gets no scoring.
	if apps, err := s.Store.ListApplicationsByUser(ctx, cl.UserID); err == nil {
		for _, a := range apps {
			v.Applications = append(v.Applications, gen.SupportApplicationFact{
				ProductTitle: productTitle(a.ProductID), Amount: a.Amount, TermMonths: int(a.TermMonths), StatusLabel: workflow.ClientLabel(workflow.Status(a.Status)),
			})
		}
	}
	return v, nil
}

func (s *Server) respondSupport(w http.ResponseWriter, r *http.Request, t db.ChatThread) {
	claims, _ := auth.FromContext(r.Context())
	v, err := s.supportView(r.Context(), claims.UserID, t)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) GetSupportChat(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	t, ok := s.supportThread(w, r, id.String())
	if !ok {
		return
	}
	if t.Status == chatBot { // a talk with the helper alone is not for the specialist to read
		writeError(w, http.StatusNotFound, "not_found", "Обращение закрыто")
		return
	}
	s.respondSupport(w, r, t)
}

func (s *Server) JoinSupportChat(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	claims, _ := auth.FromContext(r.Context())
	t, ok := s.supportThread(w, r, id.String())
	if !ok {
		return
	}
	joined, err := s.Store.JoinChatThread(r.Context(), db.JoinChatThreadParams{ID: t.ID, AgentID: &claims.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusConflict, "chat_taken", "Разговор уже ведёт другой сотрудник или клиент вернулся к помощнику")
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if t.Status == chatWaiting { // a rejoin by the same specialist adds no second note
		me, err := s.Store.GetUser(r.Context(), claims.UserID)
		if err != nil {
			internalError(w, r, err)
			return
		}
		if err := s.note(r.Context(), joined.ID, "К разговору подключилась "+agentLabel(me.FullName)+"."); err != nil {
			internalError(w, r, err)
			return
		}
		s.ringUser(joined.UserID, joined.ID, chatLive)
		s.ringSupport(joined.ID, chatLive)
	}
	s.respondSupport(w, r, joined)
}

func (s *Server) SendSupportMessage(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	claims, _ := auth.FromContext(r.Context())
	var req gen.SupportMessageRequest
	if !decodeBody(w, r, &req) {
		return
	}
	text := strings.TrimSpace(req.Message)
	if text == "" {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Напишите ответ", []fieldErr{{"message", "Напишите ответ клиенту"}})
		return
	}
	if utf8.RuneCountInString(text) > maxAgentRunes {
		writeFieldErrors(w, http.StatusUnprocessableEntity, "Ответ слишком длинный", []fieldErr{{"message", "Сократите ответ до 1000 символов"}})
		return
	}
	t, ok := s.supportThread(w, r, id.String())
	if !ok {
		return
	}
	if t.Status != chatLive || t.AgentID == nil || *t.AgentID != claims.UserID {
		writeError(w, http.StatusConflict, "not_your_chat", "Сначала подключитесь к разговору")
		return
	}
	me, err := s.Store.GetUser(r.Context(), claims.UserID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	name := agentLabel(me.FullName)
	if _, err := s.Store.InsertChatMessage(r.Context(), db.InsertChatMessageParams{ThreadID: t.ID, Author: "agent", Text: text, AgentName: pgtype.Text{String: name, Valid: true}}); err != nil {
		internalError(w, r, err)
		return
	}
	s.ringUser(t.UserID, t.ID, chatLive)
	s.respondSupport(w, r, t)
}

func (s *Server) CloseSupportChat(w http.ResponseWriter, r *http.Request, id gen.ApplicationId) {
	claims, _ := auth.FromContext(r.Context())
	t, ok := s.supportThread(w, r, id.String())
	if !ok {
		return
	}
	if t.Status != chatLive || t.AgentID == nil || *t.AgentID != claims.UserID {
		writeError(w, http.StatusConflict, "not_your_chat", "Завершить можно только свой разговор")
		return
	}
	rel, err := s.Store.ReleaseChatThread(r.Context(), t.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := s.note(r.Context(), rel.ID, noteClosed); err != nil {
		internalError(w, r, err)
		return
	}
	s.ringUser(rel.UserID, rel.ID, chatBot)
	s.ringSupport(rel.ID, chatBot)
	s.respondSupport(w, r, rel)
}
