INTEGRATION     	:= docker
BINARY_NAME      	= nri-$(INTEGRATION)
SRC_DIR          	= ./src/
VALIDATE_DEPS    	= golang.org/x/lint/golint
INTEGRATIONS_DIR 	= /var/db/newrelic-infra/newrelic-integrations/
CONFIG_DIR       	= /etc/newrelic-infra/integrations.d
GO_FILES        	:= ./src/
GOOS             	= GOOS=linux
GO               	= $(GOOS) go
GO_VERSION 			?= $(shell grep '^go ' go.mod | awk '{print $$2}')
BUILDER_IMAGE 		?= "ghcr.io/newrelic/coreint-automation:latest-go$(GO_VERSION)-ubuntu16.04"

all: build

build: clean validate compile test

clean: compile-deps
	@echo "=== $(INTEGRATION) === [ clean ]: removing binaries and coverage file..."
	@rm -rfv bin coverage.xml

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

compile-deps:
	@echo "=== $(INTEGRATION) === [ compile-deps ]: installing build dependencies..."
	@$(GO) get -v -d -t ./...

bin/$(BINARY_NAME):
	@echo "=== $(INTEGRATION) === [ compile ]: building $(BINARY_NAME)..."
	@CGO_ENABLED=0 $(GO) build -v -o bin/$(BINARY_NAME) $(GO_FILES)

compile: compile-deps bin/$(BINARY_NAME)

test:
	@echo "=== $(INTEGRATION) === [ test ]: running unit tests..."
	@CGO_ENABLED=1 go test -race $(SRC_DIR)/...

integration-test-deps: compile-deps
	@echo "=== $(INTEGRATION) === [ integration-test-deps ]: installing testing dependencies..."
	@docker build -t stress:latest src/biz/

integration-test: integration-test-deps
	@echo "=== $(INTEGRATION) === [ test ]: running integration tests..."
	@$(GO) test -v -tags=integration ./test/integration/.

install: bin/$(BINARY_NAME)
	@echo "=== $(INTEGRATION) === [ install ]: installing bin/$(BINARY_NAME)..."
	@sudo install -D --mode=755 --owner=root --strip $(ROOT)bin/$(BINARY_NAME) $(INTEGRATIONS_DIR)/bin/$(BINARY_NAME)
	@sudo install -D --mode=644 --owner=root $(ROOT)$(INTEGRATION)-definition.yml $(INTEGRATIONS_DIR)/$(INTEGRATION)-definition.yml
	@sudo install -D --mode=644 --owner=root $(ROOT)$(INTEGRATION)-config.yml $(CONFIG_DIR)/$(INTEGRATION)-config.yml.sample

# rt-update-changelog runs the release-toolkit run.sh script by piping it into bash to update the CHANGELOG.md.
# It also passes down to the script all the flags added to the make target. To check all the accepted flags,
# see: https://github.com/newrelic/release-toolkit/blob/main/contrib/ohi-release-notes/run.sh
#  e.g. `make rt-update-changelog -- -v`
rt-update-changelog:
	curl "https://raw.githubusercontent.com/newrelic/release-toolkit/v1/contrib/ohi-release-notes/run.sh" | bash -s -- $(filter-out $@,$(MAKECMDGOALS))

# Include thematic Makefiles
include $(CURDIR)/build/ci.mk
include $(CURDIR)/build/release.mk

.PHONY: all build clean validate-deps validate-only validate compile-deps compile test-deps test-only test integration-test install
