.PHONY: run db-up up down test-s1 test-s2 test-s3 test-s4 test-s5 test-s6 test-s7 test-s8 test-s9 test-s10 test-s11 test-s12

run:
	go run ./cmd/blog

db-up:
	docker compose up -d db

up:
	docker compose up -d --build

down:
	docker compose down

test-s1:
	go test ./... -run 'TestHealth' -v

test-s2:
	@if ! docker compose ps --status running db >/dev/null 2>&1; then \
		echo "db 未运行，先执行: docker compose up -d db"; \
		exit 1; \
	fi
	@DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/blog?sslmode=disable}" \
		go test ./... -run 'TestS2|TestHealth_StillNoAuth|TestListen_RequiresStore' -v

test-s3:
	@if ! docker compose ps --status running db >/dev/null 2>&1; then \
		echo "db 未运行，先执行: docker compose up -d db"; \
		exit 1; \
	fi
	@DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/blog?sslmode=disable}" \
		go test ./... -run 'TestLogin|TestHealth_StillOpen' -v

test-s4:
	@if ! docker compose ps --status running db >/dev/null 2>&1; then \
		echo "db 未运行，先执行: docker compose up -d db"; \
		exit 1; \
	fi
	@DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/blog?sslmode=disable}" \
		go test ./... -run 'TestArticle' -v

test-s5:
	@if ! docker compose ps --status running db >/dev/null 2>&1; then \
		echo "db 未运行，先执行: docker compose up -d db"; \
		exit 1; \
	fi
	@DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/blog?sslmode=disable}" \
		go test ./... -run 'TestProject' -v

test-s6:
	@if ! docker compose ps --status running db >/dev/null 2>&1; then \
		echo "db 未运行，先执行: docker compose up -d db"; \
		exit 1; \
	fi
	@DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/blog?sslmode=disable}" \
		go test ./... -run 'TestMessage' -v

test-s7:
	@if ! docker compose ps --status running db >/dev/null 2>&1; then \
		echo "db 未运行，先执行: docker compose up -d db"; \
		exit 1; \
	fi
	@DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/blog?sslmode=disable}" \
		go test ./... -run 'TestSearch|TestTags' -v

test-s8:
	@if ! docker compose ps --status running db >/dev/null 2>&1; then \
		echo "db 未运行，先执行: docker compose up -d db"; \
		exit 1; \
	fi
	@DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/blog?sslmode=disable}" \
		go test ./... -run 'TestSettings|TestSuggestions' -v

test-s9:
	@if ! docker compose ps --status running db >/dev/null 2>&1; then \
		echo "db 未运行，先执行: docker compose up -d db"; \
		exit 1; \
	fi
	@DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/blog?sslmode=disable}" \
		go test ./... -run 'TestAdminPage|TestPreview|TestCORS|TestGuestbookHTML|TestSearchHTML|TestPublicNav|TestRequireAdmin_Cookie|TestAdminAssets' -v

test-s10:
	go test ./... -run 'TestPostsHTML_UsesAPI|TestIndexHTML_UsesAPI|TestArticleHTML_UsesAPI|TestProjectsHTML_UsesAPI' -v

test-s11:
	go test ./internal/compose/... -run 'TestCompose' -v -timeout 5m

test-s12:
	@if ! docker compose ps --status running db >/dev/null 2>&1; then \
		echo "db 未运行，先执行: docker compose up -d db"; \
		exit 1; \
	fi
	@DATABASE_URL="$${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/blog?sslmode=disable}" \
		go test ./... -run 'TestChat_|TestS12_|TestTools_|TestStream_|TestNew_NoAPIKey|TestSystemPrompt_|TestBuildInput_|TestSeedKnowledge_|TestSuggestions_|TestChatHTML_UsesAPI|TestSettingsHTML_NoEmbedding' -v
