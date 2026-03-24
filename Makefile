VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X github.com/jacksonfernando/a-kit/internal/version.Version=$(VERSION)"
BINARY   := a-kit

# Derive the latest semver tag and split into parts for bumping.
LATEST_TAG    := $(shell git describe --tags --match "v*" --abbrev=0 2>/dev/null || echo "v0.0.0")
MAJOR         := $(shell echo $(LATEST_TAG) | sed 's/v//' | cut -d. -f1)
MINOR         := $(shell echo $(LATEST_TAG) | sed 's/v//' | cut -d. -f2)
PATCH         := $(shell echo $(LATEST_TAG) | sed 's/v//' | cut -d. -f3)

.PHONY: build install clean version tag-patch tag-minor tag-major

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

## tag-patch: bump patch → v1.0.x
tag-patch:
	$(eval NEXT := v$(MAJOR).$(MINOR).$(shell echo $$(($(PATCH)+1))))
	@echo "Tagging $(NEXT)"
	@git tag -a $(NEXT) -m "Release $(NEXT)"
	@echo "Run 'git push origin $(NEXT)' to push the tag."

## tag-minor: bump minor → v1.x.0
tag-minor:
	$(eval NEXT := v$(MAJOR).$(shell echo $$(($(MINOR)+1))).0)
	@echo "Tagging $(NEXT)"
	@git tag -a $(NEXT) -m "Release $(NEXT)"
	@echo "Run 'git push origin $(NEXT)' to push the tag."

## tag-major: bump major → vx.0.0
tag-major:
	$(eval NEXT := v$(shell echo $$(($(MAJOR)+1))).0.0)
	@echo "Tagging $(NEXT)"
	@git tag -a $(NEXT) -m "Release $(NEXT)"
	@echo "Run 'git push origin $(NEXT)' to push the tag."
