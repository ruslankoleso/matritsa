-- name: UpsertCompany :one
INSERT INTO companies (inn, name, kind, registered_at, okved, okved_group, director, address, monthly_obligations)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (inn) DO UPDATE SET name = EXCLUDED.name, kind = EXCLUDED.kind, registered_at = EXCLUDED.registered_at,
  okved = EXCLUDED.okved, okved_group = EXCLUDED.okved_group, director = EXCLUDED.director,
  address = EXCLUDED.address, monthly_obligations = EXCLUDED.monthly_obligations
RETURNING *;

-- name: GetCompany :one
SELECT * FROM companies WHERE id = $1;

-- name: GetCompanyByINN :one
SELECT * FROM companies WHERE inn = $1;

-- name: UpsertFinancial :exec
INSERT INTO company_financials (company_id, month, revenue) VALUES ($1, $2, $3)
ON CONFLICT (company_id, month) DO UPDATE SET revenue = EXCLUDED.revenue;

-- name: ListFinancials :many
SELECT * FROM company_financials WHERE company_id = $1 ORDER BY month;

-- name: UpsertUser :one
INSERT INTO users (login, password_hash, role, company_id, full_name)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (login) DO UPDATE SET password_hash = EXCLUDED.password_hash, role = EXCLUDED.role,
  company_id = EXCLUDED.company_id, full_name = EXCLUDED.full_name
RETURNING *;

-- name: GetUserByLogin :one
SELECT * FROM users WHERE login = $1;

-- name: GetUser :one
SELECT * FROM users WHERE id = $1;

-- name: CountCompanies :one
SELECT count(*) FROM companies;

-- name: CountUsers :one
SELECT count(*) FROM users;

-- name: InsertCompany :one
-- Unlike UpsertCompany it never touches an existing company: sign-up must not take one over.
INSERT INTO companies (inn, name, kind, registered_at, okved, okved_group, director, address, monthly_obligations)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: InsertClient :one
-- A new client account from sign-up; fails on a taken login instead of overwriting it.
INSERT INTO users (login, password_hash, role, company_id, full_name, consented_at)
VALUES ($1, $2, 'client', $3, $4, now())
RETURNING *;
