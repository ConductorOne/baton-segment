GOOS = $(shell go env GOOS)
GOARCH = $(shell go env GOARCH)
BUILD_DIR = dist/${GOOS}_${GOARCH}
GENERATED_CONF := pkg/config/conf.gen.go

ifeq ($(GOOS),windows)
OUTPUT_PATH = ${BUILD_DIR}/baton-segment.exe
else
OUTPUT_PATH = ${BUILD_DIR}/baton-segment
endif

# Set the build tag conditionally based on ENABLE_LAMBDA
ifdef BATON_LAMBDA_SUPPORT
	BUILD_TAGS=-tags baton_lambda_support
else
	BUILD_TAGS=
endif

.PHONY: build
build: ${GENERATED_CONF}
	go build ${BUILD_TAGS} -o ${OUTPUT_PATH} ./cmd/baton-segment

$(GENERATED_CONF): pkg/config/config.go go.mod
	@echo "Generating $(GENERATED_CONF)..."
	go generate -tags=generate ./pkg/config

.PHONY: generate
generate: $(GENERATED_CONF)

.PHONY: update-deps
update-deps:
	go get -d -u ./...
	go mod tidy -v
	go mod vendor

.PHONY: add-dep
add-dep:
	go mod tidy -v
	go mod vendor

.PHONY: update-baton-sdk-deps
update-baton-sdk-deps:
	go get -u github.com/conductorone/baton-sdk
	go mod tidy -v
	go mod vendor
	@echo "✅ Baton-sdk dependencies updated successfully"

.PHONY: lint
lint:
	golangci-lint run --timeout=3m

.PHONY: test-server
test-server:
	go run ./cmd/test-server

.PHONY: test-with-server
test-with-server: build
	@echo "Starting test server in background..."; \
	go run ./cmd/test-server & SERVER_PID=$$!; \
	trap "kill -TERM $$SERVER_PID 2>/dev/null || kill -9 $$SERVER_PID 2>/dev/null" EXIT; \
	sleep 2; \
	echo "Running connector sync..."; \
	${OUTPUT_PATH} \
		--base-url http://localhost:8080 \
		--token test-segment-token-12345
