.SILENT:
.PHONY: lint test race converage up

lint:
	go tool -modfile=go.tool.mod golangci-lint run ./...

test:
	go test ./... -coverprofile cover.out

race:
	go test ./... -race

coverage:
	go tool cover -html cover.out

up:
	docker compose build tgfeed
	docker compose up -d
	docker image prune --force
