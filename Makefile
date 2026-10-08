# gh-board local pipeline. Every target is one plain command per line so
# that GNU make runs it unchanged on macOS, Linux and Windows runners; the
# repository tools run through go run and need no shell features.

.PHONY: fmt lint laws vuln secrets test coverage build ci

fmt:
	gofumpt -w .

lint:
	golangci-lint run ./...

laws:
	go run ./tools/laws -root .

vuln:
	govulncheck ./...

secrets:
	gitleaks detect --no-git -v --redact
	gitleaks detect -v --redact

test:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...

coverage:
	go run ./tools/coverage -profile coverage.out -root . -min 80

build:
	go build -o gh-board ./cmd/gh-board

ci: fmt lint laws vuln secrets test coverage build

.PHONY: acceptance

acceptance:
	go run ./tools/acceptance
