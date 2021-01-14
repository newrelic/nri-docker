INTEGRATION     := docker
BINARY_NAME      = nri-$(INTEGRATION)
SRC_DIR          = ./src/
VALIDATE_DEPS    = golang.org/x/lint/golint
TEST_DEPS        = github.com/axw/gocov/gocov github.com/AlekSi/gocov-xml
INTEGRATIONS_DIR = /var/db/newrelic-infra/newrelic-integrations/
CONFIG_DIR       = /etc/newrelic-infra/integrations.d
GO_FILES        := ./src/
WORKDIR         := $(shell pwd)
TARGET          := target
TARGET_DIR       = $(WORKDIR)/$(TARGET)
GOOS             = GOOS=linux
GO               = $(GOOS) go
GOCOV            = $(GOOS) gocov

all: build

build: clean validate compile test

clean: compile-deps
	@echo "=== $(INTEGRATION) === [ clean ]: removing binaries and coverage file..."
	@rm -rfv bin coverage.xml $(TARGET)

validate-deps:
	@echo "=== $(INTEGRATION) === [ validate-deps ]: installing validation dependencies..."
	@$(GO) get -v $(VALIDATE_DEPS)

validate-only:
	@printf "=== $(INTEGRATION) === [ validate ]: running gofmt... "
	@OUTPUT="$(shell gofmt -l $(GO_FILES))" ;\
	if [ -z "$$OUTPUT" ]; then \
		echo "passed." ;\
	else \
		echo "failed. Incorrect syntax in the following files:" ;\
		echo "$$OUTPUT" ;\
		exit 1 ;\
	fi
	@printf "=== $(INTEGRATION) === [ validate ]: running golint... "
	@OUTPUT="$(shell golint $(SRC_DIR)...)" ;\
	if [ -z "$$OUTPUT" ]; then \
		echo "passed." ;\
	else \
		echo "failed. Issues found:" ;\
		echo "$$OUTPUT" ;\
		exit 1 ;\
	fi
	@printf "=== $(INTEGRATION) === [ validate ]: running go vet... "
	@OUTPUT="$(shell $(GO) vet $(SRC_DIR)...)" ;\
	if [ -z "$$OUTPUT" ]; then \
		echo "passed." ;\
	else \
		echo "failed. Issues found:" ;\
		echo "$$OUTPUT" ;\
		exit 1;\
	fi

validate: validate-deps validate-only

run-docker-dev:
	docker run --rm -it -v $(WORKDIR):/go/src/github.com/newrelic/nri-docker golang:1.10

compile-deps:
	@echo "=== $(INTEGRATION) === [ compile-deps ]: installing build dependencies..."
	@$(GO) get -v -d -t ./...

bin/$(BINARY_NAME):
	@echo "=== $(INTEGRATION) === [ compile ]: building $(BINARY_NAME)..."
	@$(GO) build -v -o bin/$(BINARY_NAME) $(GO_FILES)

compile: compile-deps bin/$(BINARY_NAME)

test-deps: compile-deps
	@echo "=== $(INTEGRATION) === [ test-deps ]: installing testing dependencies..."
	@$(GO) get -v $(TEST_DEPS)
	@docker build -t stress:latest src/biz/

test-only:
	@echo "=== $(INTEGRATION) === [ test ]: running unit tests..."
	@$(GOCOV) test $(SRC_DIR)/... | gocov-xml > coverage.xml

test: test-deps test-only

integration-test: test-deps
	@echo "=== $(INTEGRATION) === [ test ]: running integration tests..."
	@docker-compose -f tests/integration/docker-compose.yml up -d --build
	@$(GO) test -v -tags=integration ./tests/integration/. || (ret=$$?; docker-compose -f tests/integration/docker-compose.yml down && exit $$ret)
	@docker-compose -f tests/integration/docker-compose.yml down

install: bin/$(BINARY_NAME)
	@echo "=== $(INTEGRATION) === [ install ]: installing bin/$(BINARY_NAME)..."
	@sudo install -D --mode=755 --owner=root --strip $(ROOT)bin/$(BINARY_NAME) $(INTEGRATIONS_DIR)/bin/$(BINARY_NAME)
	@sudo install -D --mode=644 --owner=root $(ROOT)$(INTEGRATION)-definition.yml $(INTEGRATIONS_DIR)/$(INTEGRATION)-definition.yml
	@sudo install -D --mode=644 --owner=root $(ROOT)$(INTEGRATION)-config.yml.sample $(CONFIG_DIR)/$(INTEGRATION)-config.yml.sample

# Include thematic Makefiles
include Makefile-*.mk
include $(CURDIR)/build/ci.mk
include $(CURDIR)/build/release.mk

.PHONY: all build clean validate-deps validate-only validate compile-deps compile test-deps test-only test integration-test install
