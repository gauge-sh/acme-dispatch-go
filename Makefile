GO      ?= go
DB      ?= dispatch.db
PKGS    := ./packages/core/... ./apps/api/... ./apps/web/...

.PHONY: build test lint fmt seed web api clean

build:
	$(GO) build $(PKGS)

test:
	$(GO) test $(PKGS)

lint:
	@out=$$(gofmt -l apps packages); if [ -n "$$out" ]; then echo "gofmt:"; echo "$$out"; exit 1; fi
	$(GO) vet $(PKGS)

fmt:
	gofmt -w apps packages

seed:
	$(GO) run ./apps/web -db $(DB) -seed

web:
	$(GO) run ./apps/web -db $(DB)

api:
	$(GO) run ./apps/api -db $(DB)

clean:
	rm -f $(DB) $(DB)-wal $(DB)-shm
