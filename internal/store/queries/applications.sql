-- name: CreateApplication :one
INSERT INTO applications (company_id, user_id, product_id, amount, term_months, purpose, data)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetApplication :one
SELECT * FROM applications WHERE id = $1;

-- name: GetApplicationForUpdate :one
SELECT * FROM applications WHERE id = $1 FOR UPDATE;

-- name: ListApplicationsByUser :many
SELECT * FROM applications WHERE user_id = $1 ORDER BY created_at DESC;

-- name: UpdateApplicationDraft :one
UPDATE applications
SET product_id = $2, amount = $3, term_months = $4, purpose = $5, data = $6, updated_at = now()
WHERE id = $1 AND status IN ('DRAFT', 'NEEDS_INFO')
RETURNING *;

-- name: SetApplicationData :exec
UPDATE applications SET data = $2, updated_at = now() WHERE id = $1;

-- name: TransitionApplication :one
UPDATE applications
SET status = $3, version = version + 1, updated_at = now()
WHERE id = $1 AND version = $2
RETURNING *;

-- name: SetIdempotencyKey :exec
UPDATE applications SET idempotency_key = $2 WHERE id = $1;

-- name: GetApplicationByIdempotencyKey :one
SELECT * FROM applications WHERE user_id = $1 AND idempotency_key = $2;

-- name: AssignApplication :exec
UPDATE applications SET assigned_to = $2, updated_at = now() WHERE id = $1;

-- name: ListQueue :many
SELECT a.id, a.product_id, a.amount, a.term_months, a.status, a.updated_at, a.assigned_to,
       c.name AS company_name, c.inn AS company_inn,
       sr.grade AS grade, sr.score AS score,
       COALESCE((a.data->'review'->>'manualCheck')::boolean, false) AS manual_check,
       COALESCE((a.data->'rejection'->>'by') = 'system', false)
         OR EXISTS (SELECT 1 FROM offers o WHERE o.application_id = a.id AND o.source = 'auto') AS automatic
FROM applications a
JOIN companies c ON c.id = a.company_id
LEFT JOIN scoring_results sr ON sr.application_id = a.id
WHERE a.status <> 'DRAFT'
  AND (sqlc.arg(status)::text = '' OR a.status = sqlc.arg(status)::text)
  AND (sqlc.arg(q)::text = '' OR c.name ILIKE '%' || sqlc.arg(q)::text || '%' OR c.inn LIKE sqlc.arg(q)::text || '%')
ORDER BY (a.status = 'ANALYST_REVIEW') DESC, a.updated_at ASC
LIMIT 200;

-- name: CountApplicationsByStatus :many
SELECT status, count(*)::int AS n FROM applications WHERE status <> 'DRAFT' GROUP BY status;

-- name: ListApplicationsByStatus :many
SELECT * FROM applications WHERE status = ANY($1::text[]) ORDER BY updated_at ASC LIMIT $2;

-- name: InsertEvent :one
INSERT INTO application_events (application_id, actor, event, from_status, to_status, comment, payload)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListEvents :many
SELECT * FROM application_events WHERE application_id = $1 ORDER BY id;

-- name: InsertDocument :one
INSERT INTO documents (application_id, kind, filename, content_type, size, path)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: ListDocuments :many
SELECT * FROM documents WHERE application_id = $1 ORDER BY uploaded_at;

-- name: GetDocument :one
SELECT * FROM documents WHERE id = $1 AND application_id = $2;

-- name: UpsertScoringResult :exec
INSERT INTO scoring_results (application_id, score, grade, factors, hard_stops, max_affordable, debt_load, enrichment, metrics)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (application_id) DO UPDATE SET score = EXCLUDED.score, grade = EXCLUDED.grade, factors = EXCLUDED.factors,
  hard_stops = EXCLUDED.hard_stops, max_affordable = EXCLUDED.max_affordable, debt_load = EXCLUDED.debt_load,
  enrichment = EXCLUDED.enrichment, metrics = EXCLUDED.metrics, created_at = now();

-- name: GetScoringResult :one
SELECT * FROM scoring_results WHERE application_id = $1;

-- name: DeactivateOffers :exec
UPDATE offers SET active = false WHERE application_id = $1 AND active;

-- name: InsertOffer :one
INSERT INTO offers (application_id, amount, term_months, rate_bp, monthly_payment, source, expires_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetActiveOffer :one
SELECT * FROM offers WHERE application_id = $1 AND active ORDER BY created_at DESC LIMIT 1;

-- name: ListExpiredOfferApplications :many
SELECT a.id FROM applications a JOIN offers o ON o.application_id = a.id AND o.active
WHERE a.status = 'OFFER_MADE' AND o.expires_at < now();

-- name: InsertNotification :one
INSERT INTO notifications (user_id, application_id, text) VALUES ($1, $2, $3) RETURNING *;

-- name: ListNotifications :many
SELECT * FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 50;

-- name: MarkNotificationRead :exec
UPDATE notifications SET read_at = now() WHERE id = $1 AND user_id = $2 AND read_at IS NULL;
