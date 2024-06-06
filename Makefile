BUILD_DIR ?= bin
BUILD_TARGET ?= $(BUILD_DIR)/clusterctl-bootstrap

all: clean fmt test vet build

.PHONY: build
build: $(BUILD_TARGET)
$(BUILD_TARGET):
	go build -o $@ main.go

.PHONY: build
test:
	go test -v ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: clean
clean:
	-rm -f $(BUILD_TARGET)