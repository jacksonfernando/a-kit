VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X github.com/jacksonfernando/a-kit/internal/version.Version=$(VERSION)"
BINARY   := a-kit

.PHONY: build install clean version

build:
	go build $(LDFLAGS) -o bin/$(BINARY) .

install:
	go install $(LDFLAGS) .

test:
	go test ./...

clean:
	rm -rf bin/

version:
	@echo $(VERSION)
