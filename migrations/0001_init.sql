-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE companies (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    inn                 text NOT NULL UNIQUE,
    name                text NOT NULL,
    kind                text NOT NULL CHECK (kind IN ('ooo', 'ip')),
    registered_at       date NOT NULL,
    okved               text NOT NULL,
    okved_group         int  NOT NULL CHECK (okved_group BETWEEN 1 AND 3),
    director            text NOT NULL,
    address             text NOT NULL,
    monthly_obligations bigint NOT NULL DEFAULT 0
);

CREATE TABLE company_financials (
    company_id uuid   NOT NULL REFERENCES companies(id) ON DELETE CASCADE,
    month      date   NOT NULL,
    revenue    bigint NOT NULL,
    PRIMARY KEY (company_id, month)
);

CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    login         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    role          text NOT NULL CHECK (role IN ('client', 'staff')),
    company_id    uuid REFERENCES companies(id),
    full_name     text NOT NULL
);

CREATE TABLE applications (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    company_id      uuid   NOT NULL REFERENCES companies(id),
    user_id         uuid   NOT NULL REFERENCES users(id),
    product_id      text   NOT NULL,
    amount          bigint NOT NULL,
    term_months     int    NOT NULL,
    purpose         text   NOT NULL DEFAULT '',
    status          text   NOT NULL DEFAULT 'DRAFT',
    data            jsonb  NOT NULL DEFAULT '{}'::jsonb,
    version         int    NOT NULL DEFAULT 1,
    idempotency_key text,
    assigned_to     uuid REFERENCES users(id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX applications_user_idx ON applications (user_id, created_at DESC);
CREATE INDEX applications_status_idx ON applications (status, updated_at);
CREATE UNIQUE INDEX applications_idem_idx ON applications (user_id, idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE TABLE application_events (
    id             bigserial PRIMARY KEY,
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    at             timestamptz NOT NULL DEFAULT now(),
    actor          text NOT NULL,
    event          text NOT NULL,
    from_status    text NOT NULL,
    to_status      text NOT NULL,
    comment        text NOT NULL DEFAULT '',
    payload        jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX application_events_app_idx ON application_events (application_id, id);

CREATE TABLE documents (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    kind           text NOT NULL,
    filename       text NOT NULL,
    content_type   text NOT NULL,
    size           bigint NOT NULL,
    path           text NOT NULL,
    uploaded_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX documents_app_idx ON documents (application_id);

CREATE TABLE scoring_results (
    application_id uuid PRIMARY KEY REFERENCES applications(id) ON DELETE CASCADE,
    score          int    NOT NULL,
    grade          text   NOT NULL,
    factors        jsonb  NOT NULL,
    hard_stops     jsonb  NOT NULL,
    max_affordable bigint NOT NULL,
    debt_load      double precision NOT NULL,
    enrichment     jsonb  NOT NULL DEFAULT '{}'::jsonb,
    created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE offers (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id  uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    amount          bigint NOT NULL,
    term_months     int    NOT NULL,
    rate_bp         int    NOT NULL,
    monthly_payment bigint NOT NULL,
    source          text   NOT NULL,
    active          boolean NOT NULL DEFAULT true,
    expires_at      timestamptz NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX offers_app_idx ON offers (application_id, created_at DESC);

CREATE TABLE notifications (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    application_id uuid REFERENCES applications(id) ON DELETE CASCADE,
    text           text NOT NULL,
    read_at        timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_idx ON notifications (user_id, created_at DESC);

-- +goose Down
DROP TABLE notifications, offers, scoring_results, documents, application_events, applications, users, company_financials, companies;
