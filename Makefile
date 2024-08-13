BUILD_DIR ?= bin
BUILD_TARGET ?= $(BUILD_DIR)/clusterctl-bootstrap

GOLANGCI_LINT_VERSION ?= v1.57.2
MOCKGEN_VERSION ?= v0.4.0
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
MOCKGEN ?= $(LOCALBIN)/mockgen-$(MOCKGEN_VERSION)
INFRASTRUCTURE_PROVIDERS ?= linode
all: clean fmt test vet build

.PHONY: build
build: $(BUILD_TARGET)
$(BUILD_TARGET):
	go build -o $@ main.go

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

.PHONY: build
test: generate
	go test -race -v ./... -coverprofile cover.out

.PHONY: coverage
coverage: test
	go tool cover -html cover.out

.PHONY: generate
generate: mockgen
	for provider in $(INFRASTRUCTURE_PROVIDERS) ; do \
            $(MOCKGEN) -destination=providers/infrastructure/linode/mock/mock_$$provider.go -source=providers/infrastructure/$$provider/$$provider.go ; \
	done
	$(MOCKGEN) -destination=providers/controlplane/mock/mock_controlplane.go -source=providers/controlplane/types.go
	$(MOCKGEN) -destination=providers/infrastructure/mock/mock_types.go -source=providers/infrastructure/types.go


.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint: golangci-lint
	$(GOLANGCI_LINT) --timeout 10m run

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT)
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})

.PHONY: mockgen
mockgen: $(MOCKGEN)
$(MOCKGEN): $(LOCALBIN)
	$(call go-install-tool,$(MOCKGEN),go.uber.org/mock/mockgen,${MOCKGEN_VERSION})

.PHONY: clean
clean:
	-rm -f $(BUILD_TARGET)
	-rm -f $(LOCALBIN)

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef
