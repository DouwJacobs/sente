GO ?= go
.PHONY: build test dev clean
build:
	cd web && npm ci && npm run build
	mkdir -p bin
	$(GO) build -trimpath -ldflags="$$(sh scripts/build-metadata.sh)" -o bin/finance ./cmd/finance
test:
	$(GO) test -race ./...
	cd web && npm test && npm run build
dev:
	GO="$(GO)" python3 scripts/dev.py
clean:
	rm -rf bin web/dist

.PHONY: demo dev-demo
demo:
	GO="$(GO)" python3 scripts/demo.py
dev-demo:
	GO="$(GO)" python3 scripts/demo.py --serve

.PHONY: release-check
release-check:
	python3 scripts/release-metadata.test.py
	python3 scripts/publish-release.test.py
	python3 scripts/ci-scope.test.py
	python3 scripts/test-browser.test.py
