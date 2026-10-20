.PHONY: up down test test-go test-web e2e smoke gen docs

up:        ## run the whole stack (web on :3000, api on :8080)
	docker compose up -d --build

down:
	docker compose down

test: test-go test-web   ## unit and integration tests (needs `docker compose up -d postgres` for Go integration tests)

test-go:
	go test ./... -race

test-web:
	cd web && npm test

e2e:       ## Playwright end-to-end scenarios against the running stack
	cd e2e && npm install && npx playwright test

smoke:     ## quick check of the three main scenarios against the running stack
	BASE_URL=http://localhost:3000 go run ./cmd/smoke

gen:       ## regenerate code from the contract and SQL
	go tool oapi-codegen -config oapi-codegen.yaml api/openapi.yaml
	go tool sqlc generate
	cd web && npm run gen:api

docs:      ## regenerate generated documents (state diagram, Postman collection)
	UPDATE_DOCS=1 go test ./internal/workflow
	go run ./cmd/postman > docs/postman/matritsa.postman_collection.json
