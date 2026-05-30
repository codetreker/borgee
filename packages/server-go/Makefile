.PHONY: build test test-race run clean fmt vet coverage coverage-html regression

# CV-6 #cv-6: sqlite_fts5 build tag enables FTS5 virtual tables (full-text
# search) in mattn/go-sqlite3. Default build does not include FTS5.
GOTAGS := sqlite_fts5

build:
	go build -tags "$(GOTAGS)" -o bin/collab ./cmd/collab

test:
	go test -tags "$(GOTAGS)" ./... -v

test-race:
	go test -tags "$(GOTAGS)" ./... -v -race

run:
	go run -tags "$(GOTAGS)" ./cmd/collab

clean:
	rm -rf bin/

fmt:
	go fmt ./...

vet:
	go vet -tags "$(GOTAGS)" ./...

coverage:
	./scripts/coverage.sh

coverage-html:
	GENERATE_HTML=true HTML_OUT=coverage.html ./scripts/coverage.sh

# regression — runs every test registered via internal/testutil/regression.
# Closed milestones add a blank import in regression_suite/suite_test.go so
# their 4.1 acceptance auto-rolls into this run (闸 5 of README.md).
regression:
	go test -tags "$(GOTAGS)" ./internal/testutil/regression_suite -run TestRegressionSuite -v
