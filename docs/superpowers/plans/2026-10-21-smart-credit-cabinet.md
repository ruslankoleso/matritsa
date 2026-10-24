# Умный кредитный кабинет — план реализации

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Работающий прототип кредитного конвейера для малого бизнеса: лендинг, кабинет предпринимателя, рабочее место сотрудника банка и единая машина состояний заявки, запускаемый одной командой `docker compose up`.

**Architecture:** Go-монолит (`cmd/api`) со слоями handler → service → repository поверх PostgreSQL; два отдельных Go-мока внешних систем (БКИ, ЕГРЮЛ); конвейер заявки — чистая машина состояний плюс оркестратор, пишущий аудит-журнал и события в SSE. Фронтенд — React SPA, типизированный клиент генерируется из `api/openapi.yaml`. ИИ — Ollama с обязательным запасным режимом на правилах.

**Tech Stack:** Go 1.23, chi, oapi-codegen, pgx, sqlc, goose, PostgreSQL 16, bcrypt, JWT; React 18 + TypeScript + Vite, Tailwind, shadcn/ui, Framer Motion, Vitest, Playwright; Docker Compose; Ollama.

**Spec:** `docs/superpowers/specs/2026-10-21-smart-credit-cabinet-design.md`

## Global Constraints

- Только веб-интерфейс; сценарий целиком внутри продукта, без email/SMS/мессенджеров (вход по логину и паролю, уведомления только внутри кабинета).
- Никаких обращений к системам Альфа-Банка; БКИ и ЕГРЮЛ — только локальные моки.
- Только синтетические данные; никаких платных API, подписок, данных. LLM только с открытыми весами.
- Запуск одной командой `docker compose up`; версии зафиксированы (`go.mod`/`go.sum`, `package-lock.json`, теги образов Docker без `latest`).
- Нет официального логотипа, фирменного шрифта и торговых знаков банка; шрифт бесплатный (Onest). Стиль: красный только на главном действии, без градиентов, свечений, стекла, иконок в плитках, выдуманных цифр.
- Тексты интерфейса на «вы», тёплые и человеческие, на русском.
- Деньги хранятся целыми числами в копейках (`int64`), на фронте форматируются через `Intl.NumberFormat('ru-RU')`.
- Имя продукта — настраиваемое: Go `config.ProductName` (env `PRODUCT_NAME`, по умолчанию «Матрица»; подпись «команда МАТРИЦА РУСИ» в футере и README), фронт `VITE_PRODUCT_NAME`. Менять имя можно одной правкой.
- Коммиты: репозиторий пока не инициализирован (команда заведёт позже). Пока его нет, шаги «Commit» пропускаются; после `git init` каждая задача коммитится сообщением вида `feat: …` с трейлерами `Co-Authored-By` для команды и для Claude.
- Дедлайн 2026-10-26; заморозка и тег 2026-10-25 вечером.

## Review Focus

Входы и условия, которые спецификация подразумевает, но которых нет в основных тестах; каждый получает тест в задаче-владельце.

1. Двойной клик «Подать заявку» / повторный POST с тем же `Idempotency-Key` → одна заявка, один переход, тот же ответ (Task 10).
2. Одновременное действие клиента и аналитика над одной заявкой → одно проходит, другое получает 409, состояние не портится (Task 11).
3. Моки БКИ/ЕГРЮЛ недоступны или отвечают медленнее таймаута → заявка остаётся в `ENRICHMENT` с понятным сообщением и ретраится, API не падает (Task 8, 11).
4. Клиент запрашивает чужую заявку по ID, клиент вызывает `/staff/*` → 404 и 403 соответственно (Task 9, 10, 12).
5. Нулевая/пустая выручка (деление на ноль в долговой нагрузке), сумма вне диапазона продукта, ИНН неверной длины или с нецифрами, файл больше 5 МБ или недопустимого типа → понятная ошибка валидации, не 500 (Task 6, 10).
6. Ollama недоступна или вернула мусор → ассистент и пояснения работают на запасных правилах (Task 15).

---

## File Structure

```
cmd/api/main.go                  # сборка зависимостей, запуск, graceful shutdown
cmd/mock-bki/main.go             # мок БКИ
cmd/mock-egrul/main.go           # мок ЕГРЮЛ/ФНС
cmd/seed/main.go                 # синтетические данные и тестовые аккаунты
internal/config/                 # env-конфигурация
internal/products/               # каталог продуктов и параметры
internal/workflow/               # чистая машина состояний
internal/scoring/                # скоринг: Input → Result
internal/decision/               # решение по результату скоринга
internal/integrations/           # клиенты БКИ/ЕГРЮЛ (таймаут, ретраи)
internal/auth/                   # пароли, JWT, RBAC middleware
internal/store/                  # sqlc-запросы, транзакции
internal/pipeline/               # оркестратор конвейера, аудит
internal/events/                 # SSE-хаб и уведомления
internal/feed/                   # правила персональной ленты
internal/ai/                     # Ollama-клиент, ассистент, пояснения, запасной режим
internal/httpapi/                # реализация OpenAPI-сервера
migrations/                      # goose
api/openapi.yaml                 # контракт (источник истины)
web/                             # React SPA
e2e/                             # Playwright
docs/                            # архитектура, диаграммы, сценарии
docker-compose.yml, Makefile, README.md, THIRD_PARTY_NOTICES.md
```

Порядок задач позволяет вести фронтенд параллельно с бэкендом после Task 3 (контракт).

---

### Task 1: Каркас, конфигурация, Docker

**Files:**
- Create: `go.mod`, `Makefile`, `docker-compose.yml`, `Dockerfile.api`, `cmd/api/main.go`, `internal/config/config.go`, `internal/httpapi/health.go`
- Test: `internal/config/config_test.go`, `internal/httpapi/health_test.go`

**Interfaces:**
- Produces: `config.Load() (Config, error)`; `Config{HTTPAddr, DatabaseURL, JWTSecret, ProductName, BKIURL, EGRULURL, OllamaURL string; AIEnabled bool}`; маршруты `GET /healthz` (всегда 200), `GET /readyz` (200, если БД отвечает, иначе 503).

- [ ] **Step 1: Write failing tests** `TestLoad_DefaultsAndOverrides` (пустое окружение → `ProductName=="Матрица"`, `AIEnabled==true`; `PRODUCT_NAME=X` → `X`; пустой `JWT_SECRET` в режиме `APP_ENV=prod` → ошибка) и `TestReadyz_DBDown_503`.
- [ ] **Step 2: Run** `go test ./internal/... -run 'TestLoad|TestReadyz'` — Expected: FAIL (пакеты не определены).
- [ ] **Step 3: Implement** `config.Load`, health-хендлеры (chi), `main.go` с graceful shutdown (SIGTERM, 10 с), структурным логом `slog` (JSON). Compose: `postgres:16.4`, `api`; Makefile-цели `up`, `test`, `gen`, `seed`.
- [ ] **Step 4: Run** `go test ./... && docker compose up -d postgres api && curl -fs localhost:8080/readyz` — Expected: PASS и `ok`.
- [ ] **Step 5: Commit** `feat: project skeleton, config, health`.

### Task 2: Схема БД, sqlc, синтетические данные

**Files:**
- Create: `migrations/0001_init.sql`, `sqlc.yaml`, `internal/store/queries/*.sql`, `cmd/seed/main.go`, `internal/seed/companies.go`
- Test: `internal/store/store_test.go` (тестовая БД через testcontainers или `DATABASE_URL`)

**Interfaces:**
- Produces таблицы: `users(id, login, password_hash, role, company_id, full_name)`, `companies(id, inn, name, kind, registered_at, okved, director, address)`, `company_financials(company_id, month, revenue)`, `applications(id, company_id, user_id, product_id, amount, term_months, purpose, status, data jsonb, version int, created_at, updated_at)`, `application_events(id, application_id, at, actor, from_status, to_status, comment, payload jsonb)`, `documents(id, application_id, kind, filename, size, path, uploaded_at)`, `scoring_results(application_id, score, grade, factors jsonb, hard_stops jsonb, created_at)`, `offers(id, application_id, amount, term_months, rate_bp, monthly_payment, source, expires_at)`, `notifications(id, user_id, application_id, text, read_at, created_at)`. `store.Store` с методом `WithTx(ctx, func(Queries) error) error`. Роли: `client`, `staff`.
- Seed: 6 компаний (ИНН-значения синтетические): надёжная пекарня (класс A), закредитованный ИП (D), молодая компания без истории (C), ликвидированная (hard-stop → отказ), пограничная (B/C), с просрочкой более 90 дней (hard-stop); для каждой — 12 месяцев выручки; аккаунты `client1..client6` и `staff1` (пароль `demo1234`, только для демо).

- [ ] **Step 1: Write failing test** `TestMigrate_Up_Down_Up` и `TestSeed_Idempotent` (повторный запуск не дублирует; ровно 6 компаний и 7 пользователей).
- [ ] **Step 2: Run** `go test ./internal/store -run 'TestMigrate|TestSeed'` — Expected: FAIL.
- [ ] **Step 3: Implement** миграции goose, запросы sqlc, `Store.WithTx`, seed. Версионное поле `applications.version` — для оптимистичной блокировки (используется в Task 11).
- [ ] **Step 4: Run** `make gen && go test ./internal/store` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: schema, sqlc, seed data`.

### Task 3: Контракт OpenAPI и кодогенерация

**Files:**
- Create: `api/openapi.yaml`, `oapi-codegen.yaml`, `internal/httpapi/gen.go` (сгенерировано), `web/src/api/schema.ts` (сгенерировано)
- Test: `internal/httpapi/contract_test.go`

**Interfaces:**
- Produces эндпоинты (все под `/api/v1`, JSON, ошибки `{code, message, fields?}`):
  - Auth: `POST /auth/login {login,password} → {accessToken, refreshToken, role}`; `POST /auth/refresh`; `GET /me`.
  - Справочники: `GET /products`; `GET /company/lookup?inn=` (мок ЕГРЮЛ для автозаполнения); `POST /assistant/recommend {text} → {productId, amount?, termMonths?, rationale, source: "ai"|"rules"}`.
  - Клиент: `GET /applications`; `POST /applications` (черновик); `GET /applications/{id}`; `PATCH /applications/{id}`; `POST /applications/{id}/submit` (заголовок `Idempotency-Key`); `POST /applications/{id}/documents` (multipart); `POST /applications/{id}/respond {answers}`; `POST /applications/{id}/offer/accept`; `POST /applications/{id}/offer/decline`; `POST /applications/{id}/cancel`; `GET /applications/{id}/timeline`; `GET /feed`; `GET /notifications`; `POST /notifications/{id}/read`; `GET /events` (SSE).
  - Сотрудник: `GET /staff/queue?status=&q=`; `GET /staff/applications/{id}`; `POST /staff/applications/{id}/claim`; `POST /staff/applications/{id}/decision {action: approve|counter|request_info|reject, comment, offer?}`; `GET /staff/applications/{id}/explanation`.
- Модель `Application` содержит `status`, `allowedActions[]`, `nextStep` (человеческий текст), `offer?`, `missingItems[]`.

- [ ] **Step 1: Write failing test** `TestOpenAPI_Valid` (спецификация проходит валидацию kin-openapi; каждый `operationId` уникален; у каждого защищённого пути есть `security`).
- [ ] **Step 2: Run** `go test ./internal/httpapi -run TestOpenAPI` — Expected: FAIL (нет файла).
- [ ] **Step 3: Write** `openapi.yaml` по списку выше, подключить oapi-codegen (chi-server, strict) и `openapi-typescript` для фронта; цели `make gen`.
- [ ] **Step 4: Run** `make gen && go build ./... && (cd web && npx tsc --noEmit)` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: OpenAPI contract and codegen`.

### Task 4: Каталог продуктов

**Files:**
- Create: `internal/products/catalog.go`
- Test: `internal/products/catalog_test.go`

**Interfaces:**
- Produces: `type ID string` (`working_capital`, `equipment`, `overdraft`, `express`); `type Product struct{ID ID; Title, Description, Hint string; MinAmount, MaxAmount int64; MinTerm, MaxTerm int; BaseRateBP int; AutoApproveLimit int64; Documents []DocRequirement}`; `Catalog() []Product`; `Get(ID) (Product, bool)`; `RiskPremiumBP(grade scoring.Grade) (int, bool)`; `ValidateRequest(p Product, amount int64, term int) []FieldError`.
- Значения (₽, в копейках в коде): working_capital 300 000–10 000 000, 3–24 мес., 16,5 %, автолимит 3 000 000; equipment 500 000–20 000 000, 12–60 мес., 15,0 %, автолимит 3 000 000; overdraft 100 000–5 000 000, 12 мес., 18,0 %, автолимит 1 500 000; express 50 000–1 000 000, 3–12 мес., 21,0 %, автолимит 1 000 000. Надбавка за риск: A 0, B 200, C 450, D 800 б.п.; E — нет (`ok=false`).

- [ ] **Step 1: Write failing tests** `TestCatalog_FourProducts`, `TestRiskPremium_E_NotAvailable`, `TestValidateRequest_OutOfRange_ReturnsFieldErrors` (сумма 10 копеек и 10^12 → ошибки по полю `amount`; срок 0 → `termMonths`).
- [ ] **Step 2: Run** `go test ./internal/products` — Expected: FAIL.
- [ ] **Step 3: Implement** по значениям выше; тексты `Description`/`Hint` простым языком (что это, кому подходит).
- [ ] **Step 4: Run** `go test ./internal/products` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: product catalog`.

### Task 5: Машина состояний

**Files:**
- Create: `internal/workflow/workflow.go`, `docs/state-diagram.md` (Mermaid, генерируется из таблицы переходов тестом-генератором)
- Test: `internal/workflow/workflow_test.go`

**Interfaces:**
- Produces: `type Status string` (`DRAFT, SUBMITTED, INTAKE_CHECK, NEEDS_INFO, ENRICHMENT, SCORING, ANALYST_REVIEW, OFFER_MADE, REJECTED, SIGNING, DISBURSED, DECLINED_BY_CLIENT, EXPIRED, CANCELLED`); `type Event string` (`Submit, IntakePassed, IntakeNeedsInfo, ClientProvidedInfo, EnrichmentDone, AutoOffer, SendToReview, AutoReject, AnalystOffer, AnalystRequestInfo, AnalystReject, ClientAccept, ClientDecline, SigningDone, Disburse, OfferExpired, Cancel`); `Next(from Status, ev Event) (Status, error)` (ошибка `ErrInvalidTransition`); `Allowed(from Status) []Event`; `IsTerminal(Status) bool`; `ClientMessage(Status) string` (понятный человеческий текст «что сейчас происходит»).

- [ ] **Step 1: Write failing table tests** `TestNext_ValidTransitions` (строки: `DRAFT+Submit→SUBMITTED`, `SCORING→ANALYST_REVIEW` по `SendToReview`, `ANALYST_REVIEW+AnalystRequestInfo→NEEDS_INFO`, `NEEDS_INFO+ClientProvidedInfo→ENRICHMENT`, `OFFER_MADE+ClientAccept→SIGNING`, `SIGNING+Disburse→DISBURSED`), `TestNext_Invalid` (`REJECTED+ClientAccept`, `DRAFT+AnalystOffer`, терминальные статусы → `ErrInvalidTransition`), `TestEveryStatusHasClientMessage`, `TestTerminalStatesHaveNoAllowedEvents`.
- [ ] **Step 2: Run** `go test ./internal/workflow` — Expected: FAIL.
- [ ] **Step 3: Implement** таблицу `map[Status]map[Event]Status`; переход `SCORING` выбирается событиями `AutoOffer|SendToReview|AutoReject`; `Cancel` допустим из нетерминальных состояний до `SIGNING`. Тест-генератор выводит Mermaid в `docs/state-diagram.md`.
- [ ] **Step 4: Run** `go test ./internal/workflow` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: application state machine`.

### Task 6: Скоринг

**Files:**
- Create: `internal/scoring/scoring.go`, `internal/scoring/config.go`, `internal/scoring/annuity.go`
- Test: `internal/scoring/scoring_test.go`

**Interfaces:**
- Consumes: `products.ID`, типы интеграций из Task 8 (`integrations.BKIReport`, `integrations.EGRULRecord`) — объявить в пакете `integrations/types.go` в этой задаче, чтобы избежать цикла.
- Produces: `type Grade string` (`A..E`); `type Input struct{CompanyAgeMonths int; MonthlyRevenue []int64; MonthlyObligations int64; BKI *integrations.BKIReport; EGRUL integrations.EGRULRecord; Amount int64; TermMonths int; RateBP int; OKVEDGroup int}`; `type Factor struct{Key string; Points, Max int; Note string}`; `type Result struct{Score int; Grade Grade; Factors []Factor; HardStops []string; MaxAffordableAmount int64}`; `Score(in Input, cfg Config) Result`; `DefaultConfig() Config`; `MonthlyPayment(amount int64, rateBP, termMonths int) int64` (аннуитет).
- Веса (сумма 100): возраст бизнеса 10 (≥36 мес. 10; 12–35 → 7; 6–11 → 4; <6 → 0); стабильность выручки 15 (коэф. вариации за 12 мес.: <0,15 → 15; <0,30 → 10; <0,50 → 5; иначе 0); долговая нагрузка 25 ((обязательства + платёж) / средняя выручка: <0,20 → 25; <0,35 → 18; <0,50 → 10; иначе 0); кредитный балл БКИ 20 (≥750 → 20; 650–749 → 14; 550–649 → 7; <550 → 0; нет истории → 8); просрочки 15 (нет → 15; ≤30 дн. → 8; 31–90 → 0); сумма к годовой выручке 10 (<10 % → 10; <25 % → 7; <50 % → 3; иначе 0); группа риска ОКВЭД 5 (группа 1 → 5, 2 → 3, 3 → 0). Классы: A ≥80, B 65–79, C 50–64, D 35–49, E <35. Жёсткие стопы: статус ЕГРЮЛ ≠ `active`; признак банкротства; максимальная просрочка >90 дней. `MaxAffordableAmount` — наибольшая сумма, при которой нагрузка ≤ 0,35.

- [ ] **Step 1: Write failing tests** (table-driven): `TestScore_ReliableBakery_GradeA`, `TestScore_NoHistory_GetsEightBKIPoints`, `TestScore_HardStop_Liquidated`, `TestScore_HardStop_Overdue91Days`, `TestScore_ZeroRevenue_NoPanic_GradeE` (пустой/нулевой срез выручки → нагрузка = «максимальная», без деления на ноль), `TestFactorsSumToScore`, `TestMonthlyPayment_Annuity` (1 000 000 ₽, 12 %, 12 мес. → 88 849 ₽ ±1), `TestMaxAffordable_KeepsLoadAtMost035`.
- [ ] **Step 2: Run** `go test ./internal/scoring` — Expected: FAIL.
- [ ] **Step 3: Implement** чистые функции без I/O; каждый фактор возвращает `Note` на человеческом языке («Бизнес работает 4 года — это плюс»); пороги и веса — в `Config`, `DefaultConfig` содержит значения выше.
- [ ] **Step 4: Run** `go test ./internal/scoring -race` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: transparent scoring model`.

### Task 7: Кредитное решение

**Files:**
- Create: `internal/decision/decision.go`
- Test: `internal/decision/decision_test.go`

**Interfaces:**
- Consumes: `scoring.Result`, `products.Product`.
- Produces: `type Outcome string` (`AutoOffer, Review, Reject`); `type Offer struct{Amount int64; TermMonths int; RateBP int; MonthlyPayment int64; Source string}`; `type Decision struct{Outcome Outcome; Offer *Offer; Recommended *Offer; Reasons []string}`; `Decide(res scoring.Result, p products.Product, amount int64, term int) Decision`.
- Правила: hard-стоп или класс E → `Reject` с причинами; класс A, сумма ≤ `AutoApproveLimit` и сумма ≤ `MaxAffordableAmount` → `AutoOffer` (ставка = базовая + надбавка); иначе классы A–D → `Review`, где `Recommended` — встречное предложение: сумма = min(запрошенная, `MaxAffordableAmount`, `MaxAmount`), ставка с надбавкой; если `MaxAffordableAmount` < `MinAmount` продукта → `Reject` с причиной «долговая нагрузка слишком высока для минимальной суммы».

- [ ] **Step 1: Write failing tests** `TestDecide_GradeA_Small_AutoOffer`, `TestDecide_GradeA_AboveAutoLimit_Review`, `TestDecide_GradeC_ReviewWithReducedCounter` (рекомендованная сумма < запрошенной), `TestDecide_GradeE_Reject`, `TestDecide_AffordableBelowMin_Reject`, `TestDecide_RateIncludesRiskPremium` (B на working_capital → 1650+200 б.п.).
- [ ] **Step 2: Run** `go test ./internal/decision` — Expected: FAIL.
- [ ] **Step 3: Implement** `Decide` как чистую функцию.
- [ ] **Step 4: Run** `go test ./internal/decision` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: credit decision rules`.

### Task 8: Моки БКИ и ЕГРЮЛ и клиенты

**Files:**
- Create: `cmd/mock-bki/main.go`, `cmd/mock-egrul/main.go`, `internal/integrations/types.go`, `internal/integrations/client.go`, `Dockerfile.mock`
- Test: `internal/integrations/client_test.go`, `cmd/mock-bki/main_test.go`

**Interfaces:**
- Produces: `BKIReport{HasHistory bool; ActiveLoans int; TotalDebt int64; MaxOverdueDays int; Score int}`; `EGRULRecord{INN, Name, Status string; RegisteredAt time.Time; OKVED, Director string; Bankrupt bool}`; `integrations.Client{BKI(ctx, inn string) (*BKIReport, error); EGRUL(ctx, inn string) (EGRULRecord, error)}` с таймаутом 2 с, 3 ретрая с экспоненциальной паузой, ошибка `ErrUnavailable`. Моки: `GET /bki/{inn}`, `GET /egrul/{inn}` — детерминированные ответы для ИНН из seed; неизвестный ИНН БКИ → `HasHistory=false`; неизвестный ИНН ЕГРЮЛ → 404. Переменная `MOCK_LATENCY_MS` и заголовок `X-Mock-Fail` для демонстрации отказа.

- [ ] **Step 1: Write failing tests** `TestMockBKI_SeededINN_Deterministic`, `TestClient_RetriesThenErrUnavailable` (httptest-сервер отвечает 500 три раза → `ErrUnavailable`, вызвано ровно 3 раза), `TestClient_TimeoutRespected` (сервер спит 3 с → ошибка за <2,5 с).
- [ ] **Step 2: Run** `go test ./internal/integrations ./cmd/...` — Expected: FAIL.
- [ ] **Step 3: Implement** оба мока на одних и тех же seed-данных (`internal/seed`); клиент на `net/http` с `context`.
- [ ] **Step 4: Run** `go test ./internal/integrations ./cmd/... -race` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: BKI and EGRUL mocks with resilient clients`.

### Task 9: Аутентификация и роли

**Files:**
- Create: `internal/auth/auth.go`, `internal/auth/middleware.go`, `internal/httpapi/auth_handlers.go`
- Test: `internal/auth/auth_test.go`, `internal/httpapi/auth_handlers_test.go`

**Interfaces:**
- Produces: `HashPassword(pw string) (string, error)`; `CheckPassword(hash, pw string) bool`; `Issuer.Issue(userID string, role string) (access, refresh string, err error)`; `Issuer.Parse(token string) (Claims{UserID, Role string}, error)`; middleware `RequireRole(roles ...string)` кладёт `Claims` в контекст (`auth.FromContext`). Access — 15 мин, refresh — 7 дней. Лимит попыток входа: 5 в минуту на логин (429).

- [ ] **Step 1: Write failing tests** `TestLogin_WrongPassword_401`, `TestLogin_RateLimited_429`, `TestStaffRoute_ClientToken_403`, `TestProtectedRoute_NoToken_401`, `TestExpiredToken_401`.
- [ ] **Step 2: Run** `go test ./internal/auth ./internal/httpapi -run 'TestLogin|TestStaffRoute|TestProtected|TestExpired'` — Expected: FAIL.
- [ ] **Step 3: Implement** bcrypt (cost 10), HS256 JWT, in-memory limiter.
- [ ] **Step 4: Run** те же тесты — Expected: PASS.
- [ ] **Step 5: Commit** `feat: auth and RBAC`.

### Task 10: API заявок (клиентская часть)

**Files:**
- Create: `internal/httpapi/applications.go`, `internal/httpapi/documents.go`, `internal/httpapi/lookup.go`
- Test: `internal/httpapi/applications_test.go` (интеграционные, с реальной БД)

**Interfaces:**
- Consumes: `store`, `products`, `workflow`, `auth`, `integrations.Client`; `pipeline.Start(ctx, appID)` из Task 11 (в этой задаче — интерфейс `Runner{Start(ctx, id) error}` с заглушкой).
- Produces: реализации эндпоинтов клиента из Task 3. Хранение: файлы в `DATA_DIR/uploads/{appID}/`, допустимые типы `application/pdf`, `image/png`, `image/jpeg`, размер ≤ 5 МБ. `Idempotency-Key` хранится на заявке; повторный `submit` с тем же ключом возвращает 200 с тем же телом. Поле `nextStep` берётся из `workflow.ClientMessage`; `missingItems` — из обязательных документов продукта и незаполненных полей. Валидация ИНН: 10 или 12 цифр.

- [ ] **Step 1: Write failing tests** `TestCreateDraft_Then_Patch_Persists`, `TestSubmit_Twice_SameKey_SingleTransition` (в `application_events` ровно одна запись `DRAFT→SUBMITTED`), `TestSubmit_MissingFields_422_WithFieldList`, `TestGetApplication_OtherUsers_404`, `TestUpload_TooLarge_413`, `TestUpload_BadType_415`, `TestLookup_InvalidINN_422`, `TestAmountOutOfRange_422`.
- [ ] **Step 2: Run** `go test ./internal/httpapi -run 'TestCreateDraft|TestSubmit|TestGetApplication|TestUpload|TestLookup|TestAmount'` — Expected: FAIL.
- [ ] **Step 3: Implement** хендлеры; смена статуса только через `pipeline`/`workflow.Next` внутри транзакции с проверкой `version`.
- [ ] **Step 4: Run** те же тесты — Expected: PASS.
- [ ] **Step 5: Commit** `feat: client application API`.

### Task 11: Оркестратор конвейера и аудит

**Files:**
- Create: `internal/pipeline/pipeline.go`, `internal/pipeline/worker.go`
- Test: `internal/pipeline/pipeline_test.go`

**Interfaces:**
- Consumes: Task 4–8, 2, 13 (`events.Hub.Publish`).
- Produces: `Pipeline.Start(ctx, appID string) error` (запускает асинхронную обработку: `SUBMITTED → INTAKE_CHECK → ENRICHMENT → SCORING → решение`); `Pipeline.Apply(ctx, appID string, ev workflow.Event, actor string, comment string, expectedVersion int) error` — единственная точка смены статуса: транзакция, `workflow.Next`, запись в `application_events`, увеличение `version`, публикация события; при несовпадении `version` → `ErrConflict` (HTTP 409). Проверка полноты (`INTAKE_CHECK`): обязательные поля и документы продукта; не хватает → `IntakeNeedsInfo`. Обогащение: ЕГРЮЛ + БКИ; при `ErrUnavailable` статус остаётся `ENRICHMENT`, в журнал пишется «внешний источник временно недоступен», повтор через 5 с, до 6 попыток, затем событие в очередь сотрудника с пометкой «нужна ручная проверка». Результат скоринга сохраняется в `scoring_results`, предложение — в `offers` (срок действия 7 дней). Исход `Review` создаёт рекомендованное встречное предложение в `data`.

- [ ] **Step 1: Write failing tests** `TestPipeline_ReliableCompany_AutoOffer` (end-to-end на seed-компании A: итоговый статус `OFFER_MADE`, в журнале цепочка статусов по порядку), `TestPipeline_Liquidated_Rejected_WithReasons`, `TestPipeline_Borderline_GoesToAnalystReview`, `TestPipeline_MissingDocs_NeedsInfo_ThenResumes`, `TestApply_StaleVersion_ErrConflict`, `TestPipeline_BKIDown_StaysInEnrichment_AndRetries` (мок отдаёт 500 два раза, затем успех → заявка дошла до решения), `TestApply_ConcurrentClientAndAnalyst_OneWins`.
- [ ] **Step 2: Run** `go test ./internal/pipeline -race` — Expected: FAIL.
- [ ] **Step 3: Implement** worker на горутине с очередью в БД (`SELECT … FOR UPDATE SKIP LOCKED` по статусам `SUBMITTED/ENRICHMENT` с таймером), чтобы пережить рестарт.
- [ ] **Step 4: Run** `go test ./internal/pipeline -race` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: pipeline orchestrator with audit trail`.

### Task 12: API сотрудника

**Files:**
- Create: `internal/httpapi/staff.go`
- Test: `internal/httpapi/staff_test.go`

**Interfaces:**
- Consumes: `pipeline.Apply`, `ai.Explainer` (Task 15, пока интерфейс с шаблонной реализацией).
- Produces: очередь (сортировка: сначала `ANALYST_REVIEW`, затем по давности), карточка (заявка, компания, ответы БКИ/ЕГРЮЛ, `scoring_results` с факторами, журнал, `Recommended`), `claim`, `decision`: `approve` → `AnalystOffer` с условиями заявки либо рекомендованными; `counter` → `AnalystOffer` с условиями из тела (проверка по границам продукта и `MaxAffordableAmount` не обязательна, но предупреждение возвращается); `request_info` → `AnalystRequestInfo` (текст запроса обязателен); `reject` → `AnalystReject` (комментарий обязателен). Передача `version` обязательна.

- [ ] **Step 1: Write failing tests** `TestStaffQueue_ReviewFirst`, `TestStaffDecision_Reject_RequiresComment_422`, `TestStaffDecision_Counter_CreatesOffer_ClientSeesIt`, `TestStaffDecision_StaleVersion_409`, `TestClientToken_OnStaffRoutes_403`, `TestStaffCard_IncludesFactors`.
- [ ] **Step 2: Run** `go test ./internal/httpapi -run 'TestStaff'` — Expected: FAIL.
- [ ] **Step 3: Implement** хендлеры.
- [ ] **Step 4: Run** те же тесты — Expected: PASS.
- [ ] **Step 5: Commit** `feat: staff workspace API`.

### Task 13: События SSE и уведомления

**Files:**
- Create: `internal/events/hub.go`, `internal/httpapi/sse.go`, `internal/httpapi/notifications.go`
- Test: `internal/events/hub_test.go`, `internal/httpapi/sse_test.go`

**Interfaces:**
- Produces: `Hub.Subscribe(userID string) (<-chan Event, func())`; `Hub.Publish(userID string, Event)`; `Event{Type string; ApplicationID string; Status string; Text string}`. `GET /events` отдаёт `text/event-stream` с heartbeat каждые 20 с; клиент получает события только по своим заявкам, сотрудники — по очереди. Уведомление создаётся на каждый видимый клиенту переход (текст на человеческом языке из `workflow.ClientMessage`).

- [ ] **Step 1: Write failing tests** `TestHub_PublishReachesOnlyTarget`, `TestSSE_ClientGetsStatusChangeAfterStaffDecision` (интеграционный), `TestSSE_Unsubscribe_OnDisconnect`, `TestNotification_CreatedPerTransition_MarkRead`.
- [ ] **Step 2: Run** `go test ./internal/events ./internal/httpapi -run 'TestHub|TestSSE|TestNotification'` — Expected: FAIL.
- [ ] **Step 3: Implement** хаб на каналах с буфером и неблокирующей отправкой.
- [ ] **Step 4: Run** те же тесты `-race` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: SSE events and in-app notifications`.

### Task 14: Персональная лента

**Files:**
- Create: `internal/feed/feed.go`, `internal/httpapi/feed.go`
- Test: `internal/feed/feed_test.go`

**Interfaces:**
- Produces: `type Card struct{ID, Kind, Title, Body, CTA string; ProductID *products.ID; Priority int}`; `Build(profile Profile) []Card`, где `Profile{OKVED string; CompanyAgeMonths int; AvgMonthlyRevenue int64; HasObligations bool; ActiveApplication *AppSummary}`. Правила: нет активной заявки → карточки подходящих продуктов (торговля/производство → оборотный кредит; техника/оборудование по ОКВЭД → оборудование; ежемесячные кассовые разрывы → овердрафт); есть заявка в `NEEDS_INFO` → карточка «Нужно уточнение» с приоритетом 1; `OFFER_MADE` → «Ваше предложение действует до …»; `DISBURSED` → сопутствующее предложение; для компании младше 12 месяцев — совет собрать историю платежей; отраслевая подсказка по ОКВЭД. Не более 6 карточек, сортировка по `Priority`.

- [ ] **Step 1: Write failing tests** `TestFeed_Bakery_NoApp_ShowsWorkingCapital`, `TestFeed_NeedsInfo_ShowsClarificationFirst`, `TestFeed_ConstructionCompany_ShowsEquipment`, `TestFeed_DifferentProfiles_DifferentCards` (карточки для двух seed-клиентов не совпадают), `TestFeed_MaxSixCards`.
- [ ] **Step 2: Run** `go test ./internal/feed` — Expected: FAIL.
- [ ] **Step 3: Implement** чистую функцию-правила; эндпоинт `GET /feed` строит профиль из БД.
- [ ] **Step 4: Run** `go test ./internal/feed` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: personalised feed`.

### Task 15: ИИ-ассистент и пояснения

**Files:**
- Create: `internal/ai/ollama.go`, `internal/ai/assistant.go`, `internal/ai/explain.go`, `internal/ai/fallback.go`
- Test: `internal/ai/ai_test.go`

**Interfaces:**
- Produces: `Assistant.Recommend(ctx, text string) (Recommendation{ProductID products.ID; Amount int64; TermMonths int; Rationale string; Source string}, error)` — всегда возвращает результат: при отключённом ИИ, таймауте (3 с) или невалидном JSON от модели использует `fallback.Recommend` (ключевые слова: «станок|оборудован|техник» → equipment; «зарплат|закупк|товар|оборот» → working_capital; «касс|разрыв» → overdraft; иначе express), `Source` = `"ai"` или `"rules"`. `Explainer.Explain(res scoring.Result, d decision.Decision, audience string) (string, string)` — текст и источник; запасной режим собирает связный текст из `Factor.Note`. Ollama вызывается по HTTP с `format: json`, температура 0,2; модель из `OLLAMA_MODEL` (по умолчанию `qwen2.5:3b`). ИИ не принимает решений: ответ модели валидируется (существующий продукт, сумма в границах).

- [ ] **Step 1: Write failing tests** `TestRecommend_OllamaDown_FallsBackToRules`, `TestRecommend_GarbageJSON_FallsBack`, `TestRecommend_ModelReturnsUnknownProduct_FallsBack`, `TestRecommend_KeywordStanok_Equipment`, `TestExplain_Fallback_MentionsTopFactors`, `TestRecommend_AmountClampedToProductRange`.
- [ ] **Step 2: Run** `go test ./internal/ai` — Expected: FAIL.
- [ ] **Step 3: Implement** с интерфейсом `LLM` для подмены в тестах.
- [ ] **Step 4: Run** `go test ./internal/ai` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: AI assistant with rules fallback`.

### Task 16: Фронтенд — основа и дизайн-система

**Files:**
- Create: `web/` (Vite + React + TS), `web/src/design/tokens.css`, `web/src/design/*` (кнопки, поля, карточка статуса, маршрут заявки), `web/src/api/client.ts`, `web/src/auth/*`, `web/src/router.tsx`, `web/src/lib/format.ts`
- Test: `web/src/lib/format.test.ts`, `web/src/auth/guard.test.tsx`

**Interfaces:**
- Consumes: `web/src/api/schema.ts` (Task 3).
- Produces: `formatMoney(kopecks: number): string` («1 250 000 ₽»), `formatPayment`, `useApi()` с автообновлением токена, `<RequireRole role="client"|"staff">` (редирект по роли после входа: client → `/cabinet`, staff → `/staff`), `useEvents()` (SSE с автопереподключением), компонент `<Route Track status steps />` — «маршрут заявки» (главный визуальный мотив).
- Дизайн: перед стартом выполнить процесс taste-skill: `node <skill>/scripts/draw.mjs --sector finance --brief "<бриф>" --sections … --keep palette,type --tone warm` для лендинга и `draw.mjs --app --kind dashboard …` для кабинетов; токены: красный `#EF3124` только для главного действия, белая основа, ступени тёплого серого, шрифт Onest (локально через `@fontsource/onest`), навыки: `taste-skill`, `framer-motion`, `ui-styling`, `design-system`.

- [ ] **Step 1: Write failing tests** `formatMoney(125000000)==="1 250 000 ₽"` (с неразрывными пробелами), `formatMoney(0)`, `RequireRole` перенаправляет client со `/staff` на `/cabinet` и неавторизованного на `/login`.
- [ ] **Step 2: Run** `cd web && npx vitest run` — Expected: FAIL.
- [ ] **Step 3: Implement** основу, токены, базовые компоненты, API-клиент поверх сгенерированных типов, роутинг.
- [ ] **Step 4: Run** `cd web && npx vitest run && npx tsc --noEmit && npm run build` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: web foundation and design system`.

### Task 17: Лендинг и вход

**Files:**
- Create: `web/src/pages/Landing/*`, `web/src/pages/Login.tsx`, `web/src/components/Calculator.tsx`
- Test: `web/src/components/Calculator.test.tsx`

**Interfaces:**
- Consumes: `GET /products`, `POST /auth/login`.
- Produces: лендинг (идея «заявка = маршрут»: линия с остановками, которая проходится при скролле; одна первая-экранная фраза и одна кнопка «Войти»; разделы: как это работает, продукты простыми словами, часто задаваемые вопросы, футер с именем продукта на всю ширину). Калькулятор (живая деталь): ползунки суммы и срока, платёж пересчитывается на глазах (аннуитет по базовой ставке выбранного продукта); есть запасной режим без скрипта (форма с значением по умолчанию). Экран входа: логин и пароль + кнопки «Войти как демо-предприниматель» и «Войти как сотрудник банка».

- [ ] **Step 1: Write failing tests** `Calculator: платёж для 1 000 000 ₽ / 12 мес. / 16,5 % показывает значение аннуитета ±1 ₽`, `Calculator: сумма ограничена границами продукта`, `Login: неверный пароль показывает человеческое сообщение`.
- [ ] **Step 2: Run** `cd web && npx vitest run src/components src/pages` — Expected: FAIL.
- [ ] **Step 3: Implement** страницы по бриф-блокам; анимации Framer Motion с `prefers-reduced-motion`.
- [ ] **Step 4: Run** `cd web && npx vitest run && node <skill>/scripts/lint.mjs web/src && node <skill>/scripts/inspect.mjs http://localhost:5173` — Expected: тесты PASS, `inspect` без FAIL на 1440/768/390.
- [ ] **Step 5: Commit** `feat: landing page and login`.

### Task 18: Кабинет предпринимателя

**Files:**
- Create: `web/src/pages/cabinet/*` (Home, ProductPicker, ApplicationWizard, Documents, Status, Offer, Notifications)
- Test: `web/src/pages/cabinet/wizard.test.tsx`

**Interfaces:**
- Consumes: эндпоинты клиента (Task 3), `useEvents`.
- Produces: Главная (лента карточек, активные заявки с маршрутом); подбор продукта (диалог с ассистентом → рекомендация с обоснованием и пометкой «подсказка ИИ»/«по правилам»); мастер заявки по шагам (цель и параметры, компания с автозаполнением по ИНН, финансовые сведения, документы), автосохранение черновика на каждом шаге, у каждого шага пояснение «зачем нам это»; экран статуса (маршрут с текущей остановкой и «что дальше», живое обновление по SSE без перезагрузки); запрос уточнения (форма ответа и загрузка документов); предложение (условия, различия со встречным предложением, принять/отказаться, имитация подписания с понятной пометкой «демо»); колокольчик уведомлений.

- [ ] **Step 1: Write failing tests** `wizard: шаг не пускает дальше при невалидном ИНН и показывает подсказку`, `wizard: черновик восстанавливается после перезагрузки`, `status: событие SSE меняет статус на экране без запроса страницы`, `offer: встречное предложение показывает, что именно изменилось`.
- [ ] **Step 2: Run** `cd web && npx vitest run src/pages/cabinet` — Expected: FAIL.
- [ ] **Step 3: Implement** экраны на компонентах дизайн-системы (режим taste-skill «app screens»).
- [ ] **Step 4: Run** `cd web && npx vitest run && npx tsc --noEmit` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: client cabinet`.

### Task 19: Рабочее место сотрудника

**Files:**
- Create: `web/src/pages/staff/*` (Queue, ApplicationCard, ScoringBreakdown, DecisionPanel)
- Test: `web/src/pages/staff/decision.test.tsx`

**Interfaces:**
- Consumes: эндпоинты `/staff/*`, SSE.
- Produces: очередь с фильтрами и поиском, новая заявка появляется без перезагрузки; карточка (данные клиента и компании, ответы мок-проверок, журнал); разбор скоринга (вклад каждого фактора в баллах, цвет по знаку, баллы суммируются в итог, класс риска) — график по правилам skill `dataviz`; панель решения (одобрить / изменить условия / запросить уточнение / отказать, комментарий обязателен для двух последних, пояснение решения простым языком из `/explanation`); предупреждение при конфликте версий («заявку уже изменили, обновите»).

- [ ] **Step 1: Write failing tests** `decision: отказ без комментария заблокирован`, `breakdown: сумма вкладов равна итоговому баллу`, `conflict: ответ 409 показывает понятное сообщение и перезагружает карточку`.
- [ ] **Step 2: Run** `cd web && npx vitest run src/pages/staff` — Expected: FAIL.
- [ ] **Step 3: Implement** экраны.
- [ ] **Step 4: Run** `cd web && npx vitest run && npx tsc --noEmit` — Expected: PASS.
- [ ] **Step 5: Commit** `feat: staff workspace UI`.

### Task 20: Упаковка в Docker и ИИ-сервис

**Files:**
- Modify: `docker-compose.yml`
- Create: `Dockerfile.web`, `nginx.conf`, `scripts/pull-model.sh`, `.env.example`
- Test: `scripts/smoke.sh`

**Interfaces:**
- Produces: сервисы `postgres`, `api`, `mock-bki`, `mock-egrul`, `web` (nginx + статика, прокси `/api`), `ollama` (профиль `ai`, включается `docker compose --profile ai up`; без него `AI_ENABLED=false` и всё работает на правилах); автоматические миграции и seed при старте `api`; healthcheck для каждого сервиса; версии образов закреплены.

- [ ] **Step 1: Write** `scripts/smoke.sh`: поднимает стек, ждёт `readyz`, логинится как `client1`, создаёт и подаёт заявку, ожидает статус `OFFER_MADE` в течение 30 с, логинится как `staff1` и получает очередь.
- [ ] **Step 2: Run** `docker compose down -v && bash scripts/smoke.sh` — Expected: FAIL до готовности упаковки.
- [ ] **Step 3: Implement** compose, Dockerfile'ы (многостадийные, непривилегированный пользователь), `.env.example`.
- [ ] **Step 4: Run** `docker compose down -v && docker compose up -d --build && bash scripts/smoke.sh` — Expected: `SMOKE OK`.
- [ ] **Step 5: Commit** `feat: docker packaging`.

### Task 21: Сквозные тесты

**Files:**
- Create: `e2e/playwright.config.ts`, `e2e/happy-path.spec.ts`, `e2e/counter-offer.spec.ts`, `e2e/reject-and-needs-info.spec.ts`

**Interfaces:**
- Consumes: запущенный compose-стек.

- [ ] **Step 1: Write failing specs** (по seed-клиентам): (a) надёжная пекарня подаёт заявку и без участия человека получает предложение, принимает, видит «подписано/выдано (демо)»; (b) закредитованный ИП → заявка в очереди сотрудника, сотрудник делает встречное предложение, у клиента статус обновляется без перезагрузки, клиент принимает; (c) ликвидированная компания получает отказ с понятными причинами; (d) сотрудник запрашивает уточнение, клиент отвечает, заявка возвращается в обработку.
- [ ] **Step 2: Run** `cd e2e && npx playwright test` — Expected: FAIL до готовности UI.
- [ ] **Step 3: Fix** найденные расхождения в UI/API.
- [ ] **Step 4: Run** `cd e2e && npx playwright test` — Expected: все спецификации PASS.
- [ ] **Step 5: Commit** `test: end-to-end scenarios`.

### Task 22: Документация и артефакты для жюри

**Files:**
- Create: `README.md`, `THIRD_PARTY_NOTICES.md`, `docs/architecture.md` (диаграмма Mermaid), `docs/data-model.md` (ER Mermaid), `docs/test-scenarios.md`, `docs/demo-script.md`, `docs/scoring.md`, `docs/postman/collection.json`

**Interfaces:**
- Consumes: готовый стек.

- [ ] **Step 1: Write** README: назначение, архитектура, требования к окружению, запуск одной командой (+ профиль `ai`), переменные окружения (таблица), список компонентов, пошаговый сценарий проверки для жюри, тестовые аккаунты, ограничения и допущения, план развития (в том числе роль администратора правил). THIRD_PARTY_NOTICES: все библиотеки, шрифт, модель Ollama, смоделированные интеграции (БКИ, ЕГРЮЛ, ФНС, подписание, выдача), пометка, что стиль вдохновлён брендом из кейса и официальные знаки не используются.
- [ ] **Step 2: Verify** `docs/state-diagram.md` соответствует `workflow` (тест Task 5 проходит), коллекция запросов сгенерирована из `openapi.yaml`, все ссылки в README существуют (`grep`-проверка путей).
- [ ] **Step 3: Run** `bash scripts/smoke.sh` по инструкции из README в чистом клоне — Expected: `SMOKE OK`.
- [ ] **Step 4: Commit** `docs: README, notices, diagrams`.

### Task 23: Проверка дизайна и полировка

**Files:**
- Modify: по замечаниям

- [ ] **Step 1: Run** `node <skill>/scripts/lint.mjs web/src` и `node <skill>/scripts/inspect.mjs <url>` для лендинга, входа, главной кабинета, мастера, статуса, очереди и карточки сотрудника; Expected: 0 FAIL на 1440/768/390.
- [ ] **Step 2: Look** — открыть каждый скриншот, провести критику по `references/critique.md`, исправить три главных замечания, повторить (не более 3 раундов).
- [ ] **Step 3: Run** `web-design-guidelines` по `web/src` — исправить замечания по доступности (контраст, фокус, подписи).
- [ ] **Step 4: Run** `cd web && npx vitest run && cd ../e2e && npx playwright test && go test ./... -race` — Expected: всё PASS.
- [ ] **Step 5: Freeze** 25 окт вечером: проверка «с чистого листа» (`git clone`, `docker compose up`, `scripts/smoke.sh`), тег `v1.0-hackathon`, фиксация commit hash в README.
