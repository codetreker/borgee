.PHONY: registry-totals precheck precheck-fast

registry-totals:
	@echo "Total: $$(grep -cE '^- REG-' docs/qa/regression-registry.md)"
	@echo "Active: $$(grep -cE '^- REG-.*🟢' docs/qa/regression-registry.md)"
	@echo "Pending: $$(grep -cE '^- REG-.*⚪' docs/qa/regression-registry.md)"

precheck:
	@echo "==> go coverage (CI parity)"
	@tmp_root="$${BORGE_PRECHECK_TMPDIR:-$${TMPDIR:-/tmp}}"; \
	mkdir -p "$$tmp_root"; \
	tmp_dir="$$(mktemp -d "$$tmp_root/borgee-precheck.XXXXXX")"; \
	trap 'rm -rf "$$tmp_dir"' EXIT; \
	mkdir -p "$$tmp_dir/go-build"; \
	cd packages/server-go && env \
		GOTMPDIR="$$tmp_dir/go-build" \
		THRESHOLD_TOTAL=85 \
		THRESHOLD_FUNC=50 \
		THRESHOLD_PACKAGE=70 \
		THRESHOLD_PRINT=85 \
		BUILD_TAGS="sqlite_fts5 race_heavy" \
		COVERPROFILE="$$tmp_dir/coverage.out" \
		FAIL_ON_CRITICAL_BLOCKS=false \
		GENERATE_HTML=false \
		RACE_DETECTION=false \
		go run ./scripts/lib/coverage/
	@echo "==> client vitest"
	@cd packages/client && ./node_modules/.bin/vitest run --reporter=dot --testTimeout=10000
	@echo "==> typecheck"
	@cd packages/client && ./node_modules/.bin/tsc --noEmit 2>&1 | tail -5

precheck-fast:
	@echo "Skip cov, only typecheck + fast node-side vitest"
	@cd packages/client && ./node_modules/.bin/tsc --noEmit 2>&1 | tail -5
	@cd packages/client && ./node_modules/.bin/vitest run --project=node --reporter=dot --testTimeout=10000
