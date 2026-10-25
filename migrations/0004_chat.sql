-- +goose Up
-- A support specialist answers client chats; they cannot decide applications.
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('client', 'staff', 'support'));

-- One running talk per client. 'bot': the helper answers; 'waiting': the client asked for a person and the
-- helper keeps answering until one joins; 'live': a support specialist writes, the helper is silent.
CREATE TABLE chat_threads (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL UNIQUE REFERENCES users(id),
    status       text NOT NULL DEFAULT 'bot' CHECK (status IN ('bot', 'waiting', 'live')),
    agent_id     uuid REFERENCES users(id),
    requested_at timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_threads_open_idx ON chat_threads (status, requested_at) WHERE status <> 'bot';

CREATE TABLE chat_messages (
    id         bigserial PRIMARY KEY,
    thread_id  uuid NOT NULL REFERENCES chat_threads(id) ON DELETE CASCADE,
    author     text NOT NULL CHECK (author IN ('client', 'assistant', 'agent', 'system')),
    text       text NOT NULL,
    source     text CHECK (source IN ('ai', 'rules')),
    off_topic  boolean NOT NULL DEFAULT false,
    agent_name text,
    at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_thread_idx ON chat_messages (thread_id, id);

-- +goose Down
DROP TABLE chat_messages;
DROP TABLE chat_threads;
DELETE FROM users WHERE role = 'support';
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check CHECK (role IN ('client', 'staff'));
