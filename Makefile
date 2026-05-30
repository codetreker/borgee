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
		COVERPROFILE="$$tmp_dir/coverage.out" \
		go run github.com/codetreker/go-cov/cmd/go-cov@v0.1.0
	@echo "==> client vitest"
	@cd packages/client && ./node_modules/.bin/vitest run --reporter=dot --testTimeout=10000
	@echo "==> typecheck"
	@cd packages/client && ./node_modules/.bin/tsc --noEmit 2>&1 | tail -5

precheck-fast:
	@echo "Skip cov, only typecheck + fast node-side vitest"
	@cd packages/client && ./node_modules/.bin/tsc --noEmit 2>&1 | tail -5
	@cd packages/client && ./node_modules/.bin/vitest run --project=node --reporter=dot --testTimeout=10000
