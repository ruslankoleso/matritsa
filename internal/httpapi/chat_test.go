package httpapi_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"matritsa/internal/httpapi/gen"
	"matritsa/internal/products"
	"matritsa/internal/seed"
	"matritsa/internal/store/db"
)

func chatBody(msg string) map[string]any { return map[string]any{"message": msg} }

const chatPath = "/api/v1/assistant/chat"

func (e *testEnv) say(login, msg string) gen.ChatThread {
	e.t.Helper()
	code, raw := e.do("POST", chatPath, e.token(login), chatBody(msg))
	if code != 200 {
		e.t.Fatalf("%s says %q: %d %s", login, msg, code, raw)
	}
	var th gen.ChatThread
	mustJSON(e.t, raw, &th)
	return th
}

func (e *testEnv) thread(login string) gen.ChatThread {
	e.t.Helper()
	code, raw := e.do("GET", chatPath, e.token(login), nil)
	if code != 200 {
		e.t.Fatalf("%d %s", code, raw)
	}
	var th gen.ChatThread
	mustJSON(e.t, raw, &th)
	return th
}

func lastOf(th gen.ChatThread, author gen.ChatMessageAuthor) (gen.ChatMessage, bool) {
	for i := len(th.Messages) - 1; i >= 0; i-- {
		if th.Messages[i].Author == author {
			return th.Messages[i], true
		}
	}
	return gen.ChatMessage{}, false
}

func (e *testEnv) supportChat(login, id string) gen.SupportChat {
	e.t.Helper()
	code, raw := e.do("GET", "/api/v1/support/chats/"+id, e.token(login), nil)
	if code != 200 {
		e.t.Fatalf("%d %s", code, raw)
	}
	var sc gen.SupportChat
	mustJSON(e.t, raw, &sc)
	return sc
}

func TestChat_AnswersFromTheHandbook_AndRefusesOtherTopics(t *testing.T) {
	e := newEnv(t)
	th := e.say("client1", "Какие документы нужны для овердрафта?")
	a, ok := lastOf(th, gen.ChatMessageAuthorAssistant)
	if th.Status != gen.ChatThreadStatusBot || !ok || a.OffTopic || !strings.Contains(a.Text, "Выписка по расчётному счёту") || a.Source == nil || *a.Source != gen.ChatMessageSourceRules {
		t.Fatalf("%+v", th)
	}
	th = e.say("client1", "Расскажи анекдот про программистов")
	a, _ = lastOf(th, gen.ChatMessageAuthorAssistant)
	if !a.OffTopic || !strings.Contains(a.Text, "кредит") {
		t.Fatalf("%+v", a)
	}
	if len(th.Messages) != 4 || th.Messages[0].Author != gen.ChatMessageAuthorClient {
		t.Fatalf("the talk is kept in order, oldest first: %+v", th.Messages)
	}
}

func TestChat_TalkIsKeptBetweenCalls_PerPerson(t *testing.T) {
	e := newEnv(t)
	e.say("client1", "Какие у вас ставки?")
	if th := e.thread("client1"); len(th.Messages) != 2 {
		t.Fatalf("the talk must survive a reload: %+v", th.Messages)
	}
	if th := e.thread("client2"); len(th.Messages) != 0 {
		t.Fatalf("another person's talk is not visible: %+v", th.Messages)
	}
}

func TestChat_StatusAnswerUsesOnlyTheCallersOwnApplications(t *testing.T) {
	e := newEnv(t)
	e.run("client2", e.draft("client2", products.Overdraft, 500_000*rub, 12, true))
	ask := func(login string) string {
		th := e.say(login, "Что с моей заявкой?")
		a, _ := lastOf(th, gen.ChatMessageAuthorAssistant)
		return a.Text
	}
	if own := ask("client2"); !strings.Contains(own, "Овердрафт") || !strings.Contains(own, "500 000") {
		t.Fatalf("the owner sees the application: %q", own)
	}
	if other := ask("client3"); strings.Contains(other, "Овердрафт") || !strings.Contains(other, "нет заявок") {
		t.Fatalf("another person must not see it: %q", other)
	}
}

func TestChat_Validation_AndLimits(t *testing.T) {
	e := newEnv(t)
	tok := e.token("client1")
	if code, _ := e.do("POST", chatPath, tok, chatBody("   ")); code != 422 {
		t.Errorf("empty message: %d", code)
	}
	if code, _ := e.do("POST", chatPath, tok, chatBody(strings.Repeat("кредит ", 100))); code != 422 {
		t.Errorf("a message over 500 characters: %d", code)
	}
	if code, _ := e.do("POST", chatPath, "", chatBody("кредит")); code != 401 {
		t.Errorf("without a token: %d", code)
	}
	for _, login := range []string{"staff1", "support1"} {
		if code, _ := e.do("POST", chatPath, e.token(login), chatBody("кредит")); code != 403 {
			t.Errorf("%s uses their own workspace: %d", login, code)
		}
	}
}

func TestChat_RateLimitedPerPerson(t *testing.T) {
	e := newEnv(t)
	tok := e.token("client1")
	var last int
	for i := 0; i < 40; i++ {
		last, _ = e.do("POST", chatPath, tok, chatBody("Какие у вас ставки?"))
		if last == 429 {
			if i < 20 {
				t.Fatalf("the limit must leave room for a real talk, stopped at %d", i)
			}
			return
		}
	}
	t.Fatalf("a flood of messages must be stopped, last status %d", last)
}

func TestChat_AskingForAPerson_QueuesTheTalk_AndTheHelperKeepsAnswering(t *testing.T) {
	e := newEnv(t)
	ring, cancel := e.hub.Subscribe("support")
	defer cancel()

	th := e.say("client1", "Хочу поговорить с живым сотрудником")
	if th.Status != gen.ChatThreadStatusWaiting {
		t.Fatalf("a request for a person queues the talk: %+v", th)
	}
	if n, ok := lastOf(th, gen.ChatMessageAuthorSystem); !ok || !strings.Contains(n.Text, "передан") {
		t.Fatalf("the client is told the request was passed on: %+v", th.Messages)
	}
	select {
	case ev := <-ring:
		if ev.Type != "chat" || ev.ThreadID != th.Id.String() {
			t.Fatalf("%+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("support screens must be told about a new request")
	}

	// while waiting, the helper still answers
	th = e.say("client1", "Какая ставка по кредиту на оборудование?")
	a, ok := lastOf(th, gen.ChatMessageAuthorAssistant)
	if th.Status != gen.ChatThreadStatusWaiting || !ok || !strings.Contains(a.Text, "15 %") {
		t.Fatalf("the helper must keep answering while the person waits: %+v", th)
	}
	// asking again does not queue twice
	th = e.say("client1", "Соедините с оператором")
	queued := 0
	for _, m := range th.Messages {
		if m.Author == gen.ChatMessageAuthorSystem {
			queued++
		}
	}
	if queued != 1 {
		t.Fatalf("one request, one note: %+v", th.Messages)
	}

	code, raw := e.do("GET", "/api/v1/support/chats", e.token("support1"), nil)
	var list gen.SupportChatList
	mustJSON(t, raw, &list)
	if code != 200 || list.Waiting != 1 || len(list.Items) != 1 || list.Items[0].Status != gen.SupportChatItemStatusWaiting || list.Items[0].ClientName == "" {
		t.Fatalf("%d %+v", code, list)
	}
}

func TestChat_ASupportSpecialistJoinsAnswersAndCloses(t *testing.T) {
	e := newEnv(t)
	e.run("client2", e.draft("client2", products.Overdraft, 500_000*rub, 12, true))
	th := e.say("client2", "Нужен живой человек")
	id := th.Id.String()

	sc := e.supportChat("support1", id)
	if len(sc.Applications) != 1 || sc.Applications[0].ProductTitle != "Овердрафт" || sc.Mine {
		t.Fatalf("the specialist sees the client's applications by status: %+v", sc)
	}
	if _, raw := e.do("GET", "/api/v1/support/chats/"+id, e.token("support1"), nil); strings.Contains(string(raw), "grade") || strings.Contains(string(raw), "scoring") {
		t.Fatalf("a specialist gets no scoring: %s", raw)
	}

	// cannot answer before joining
	if code, _ := e.do("POST", "/api/v1/support/chats/"+id+"/messages", e.token("support1"), chatBody("Здравствуйте")); code != 409 {
		t.Fatalf("answering without joining: %d", code)
	}
	code, raw := e.do("POST", "/api/v1/support/chats/"+id+"/join", e.token("support1"), nil)
	var joined gen.SupportChat
	mustJSON(t, raw, &joined)
	if code != 200 || joined.Thread.Status != gen.ChatThreadStatusLive || !joined.Mine {
		t.Fatalf("%d %s", code, raw)
	}
	th = e.thread("client2")
	if th.Status != gen.ChatThreadStatusLive || th.AgentName == nil || *th.AgentName != "Мария Игоревна, специалист поддержки" {
		t.Fatalf("the client sees who joined, without a surname: %+v", th)
	}

	// the helper is silent while a person is in the talk
	before := len(th.Messages)
	th = e.say("client2", "Подскажите, что с моей заявкой?")
	if len(th.Messages) != before+1 {
		t.Fatalf("only the client's message is added while a specialist is in the talk: %+v", th.Messages[before:])
	}

	code, raw = e.do("POST", "/api/v1/support/chats/"+id+"/messages", e.token("support1"), chatBody("Заявка на проверке, решение будет сегодня."))
	if code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	th = e.thread("client2")
	m, ok := lastOf(th, gen.ChatMessageAuthorAgent)
	if !ok || m.AgentName == nil || !strings.Contains(m.Text, "на проверке") {
		t.Fatalf("the client sees the specialist's answer: %+v", th.Messages)
	}

	if code, _ := e.do("POST", "/api/v1/support/chats/"+id+"/messages", e.token("support1"), chatBody("  ")); code != 422 {
		t.Errorf("empty answer: %d", code)
	}
	if code, _ := e.do("POST", "/api/v1/support/chats/"+id+"/messages", e.token("support1"), chatBody(strings.Repeat("а", 1001))); code != 422 {
		t.Errorf("answer over 1000 characters: %d", code)
	}

	code, raw = e.do("POST", "/api/v1/support/chats/"+id+"/close", e.token("support1"), nil)
	if code != 200 {
		t.Fatalf("%d %s", code, raw)
	}
	th = e.say("client2", "Какие у вас ставки?")
	if a, ok := lastOf(th, gen.ChatMessageAuthorAssistant); th.Status != gen.ChatThreadStatusBot || !ok || !strings.Contains(a.Text, "ставк") {
		t.Fatalf("after the talk is closed the helper answers again: %+v", th)
	}
	if code, _ := e.do("GET", "/api/v1/support/chats/"+id, e.token("support1"), nil); code != 404 {
		t.Fatalf("a talk with the helper alone is not for the specialist to read: %d", code)
	}
}

func TestChat_OnlyOneSpecialistTakesAWaitingTalk(t *testing.T) {
	e := newEnv(t)
	hash, _ := bcrypt.GenerateFromPassword([]byte(seed.DemoPassword), 4)
	if _, err := e.s.UpsertUser(context.Background(), db.UpsertUserParams{Login: "support2", PasswordHash: string(hash), Role: "support", FullName: "Олег Петрович Рябов, специалист поддержки"}); err != nil {
		t.Fatal(err)
	}
	id := e.say("client1", "Соедините с оператором").Id.String()
	if code, _ := e.do("POST", "/api/v1/support/chats/"+id+"/join", e.token("support1"), nil); code != 200 {
		t.Fatalf("first join: %d", code)
	}
	if code, _ := e.do("POST", "/api/v1/support/chats/"+id+"/join", e.token("support2"), nil); code != 409 {
		t.Fatalf("a talk already taken: %d", code)
	}
	if code, _ := e.do("POST", "/api/v1/support/chats/"+id+"/messages", e.token("support2"), chatBody("Здравствуйте")); code != 409 {
		t.Fatalf("another specialist must not write into the talk: %d", code)
	}
	if code, _ := e.do("POST", "/api/v1/support/chats/"+id+"/close", e.token("support2"), nil); code != 409 {
		t.Fatalf("another specialist must not close the talk: %d", code)
	}
	if code, _ := e.do("POST", "/api/v1/support/chats/"+id+"/join", e.token("support1"), nil); code != 200 {
		t.Fatalf("the same specialist may reopen their own talk: %d", code)
	}
	if th := e.thread("client1"); countOf(th, gen.ChatMessageAuthorSystem) != 2 {
		t.Fatalf("one note for the request, one for the join: %+v", th.Messages)
	}
}

func countOf(th gen.ChatThread, author gen.ChatMessageAuthor) int {
	n := 0
	for _, m := range th.Messages {
		if m.Author == author {
			n++
		}
	}
	return n
}

func TestChat_TheClientMayReturnToTheHelper(t *testing.T) {
	e := newEnv(t)
	id := e.say("client1", "Соедините с оператором").Id.String()
	e.do("POST", "/api/v1/support/chats/"+id+"/join", e.token("support1"), nil)

	code, raw := e.do("DELETE", chatPath+"/operator", e.token("client1"), nil)
	var th gen.ChatThread
	mustJSON(t, raw, &th)
	if code != 200 || th.Status != gen.ChatThreadStatusBot || th.AgentName != nil {
		t.Fatalf("%d %+v", code, th)
	}
	if code, _ := e.do("POST", "/api/v1/support/chats/"+id+"/messages", e.token("support1"), chatBody("Вы здесь?")); code != 409 {
		t.Fatalf("the specialist can no longer write: %d", code)
	}

	// the button works too
	code, raw = e.do("POST", chatPath+"/operator", e.token("client1"), nil)
	mustJSON(t, raw, &th)
	if code != 200 || th.Status != gen.ChatThreadStatusWaiting {
		t.Fatalf("%d %+v", code, th)
	}
	code, raw = e.do("POST", chatPath+"/operator", e.token("client1"), nil)
	mustJSON(t, raw, &th)
	if code != 200 || th.Status != gen.ChatThreadStatusWaiting || countOf(th, gen.ChatMessageAuthorSystem) != 4 {
		t.Fatalf("asking twice queues once: %d %+v", code, th.Messages)
	}
}

func TestChat_RolesAreSeparated(t *testing.T) {
	e := newEnv(t)
	id := e.say("client1", "Соедините с оператором").Id.String()
	for _, c := range []struct {
		login, method, path string
		want                int
	}{
		{"client1", "GET", "/api/v1/support/chats", 403},
		{"client1", "POST", "/api/v1/support/chats/" + id + "/join", 403},
		{"staff1", "GET", "/api/v1/support/chats", 403},
		{"staff1", "POST", "/api/v1/support/chats/" + id + "/join", 403},
		{"support1", "GET", "/api/v1/staff/queue", 403},
		{"support1", "GET", "/api/v1/applications", 403},
		{"support1", "GET", chatPath, 403},
		{"support1", "GET", "/api/v1/support/chats", 200},
	} {
		if code, _ := e.do(c.method, c.path, e.token(c.login), nil); code != c.want {
			t.Errorf("%s %s as %s: want %d, got %d", c.method, c.path, c.login, c.want, code)
		}
	}
	if code, _ := e.do("GET", "/api/v1/support/chats", "", nil); code != 401 {
		t.Errorf("anonymous: %d", code)
	}
	if code, _ := e.do("GET", "/api/v1/support/chats/"+"00000000-0000-0000-0000-000000000000", e.token("support1"), nil); code != 404 {
		t.Errorf("unknown talk: %d", code)
	}
}

func TestChat_SpecialistCanSignIn_AndSeesTheirRole(t *testing.T) {
	e := newEnv(t)
	code, raw := e.do("GET", "/api/v1/me", e.token("support1"), nil)
	var me gen.Me
	mustJSON(t, raw, &me)
	if code != 200 || me.Role != gen.MeRoleSupport || !strings.Contains(me.FullName, "специалист поддержки") {
		t.Fatalf("%d %+v", code, me)
	}
}
