.PHONY: up down reset test test-go test-web e2e demo-video smoke gen docs

E2E = docker compose -p matritsa-e2e -f docker-compose.yml -f docker-compose.e2e.yml

up:        ## run the whole stack (web on :3000, api on :8080)
	docker compose up -d --build

down:
	docker compose down

reset:     ## drop the database and uploads and start again with the demo data only
	docker compose down -v
	docker compose up -d --build --wait

test: test-go test-web   ## unit and integration tests (needs `docker compose up -d postgres` for Go integration tests)

test-go:
	go test ./... -race

test-web:
	cd web && npm test

e2e:       ## Playwright end-to-end scenarios on a separate stack (site on :3100); the demo data stays untouched
	$(E2E) up -d --build --wait
	cd e2e && npm ci && BASE_URL=http://localhost:3100 npx playwright test; status=$$?; cd .. && $(E2E) down -v; exit $$status

demo-video: ## record the key scenario on a separate stack and cut it into e2e/demo-video/out/matritsa-demo.mp4 (needs ffmpeg)
	$(E2E) up -d --build --wait
	cd e2e && npm ci && npx playwright install ffmpeg && BASE_URL=http://localhost:3100 npx playwright test -c demo-video/demo.config.ts; status=$$?; cd .. && $(E2E) down -v; \
	  [ $$status -eq 0 ] && node e2e/demo-video/edit.mjs || exit $$status

smoke:     ## quick check of the three main scenarios against the running stack
	BASE_URL=http://localhost:3000 go run ./cmd/smoke

gen:       ## regenerate code from the contract and SQL
	go tool oapi-codegen -config oapi-codegen.yaml api/openapi.yaml
	go tool sqlc generate
	cd web && npm run gen:api

docs:      ## regenerate generated documents (state diagram, Postman collection)
	UPDATE_DOCS=1 go test ./internal/workflow
	go run ./cmd/postman > docs/postman/matritsa.postman_collection.json
