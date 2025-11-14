GOBUILD = go build -trimpath -v
GOTEST = go test -cover -race
CMD = ./cmd/cmdcomp
BIN = bin/cmdcomp

.PHONY: $(BIN)
$(BIN):
	$(GOBUILD) -o $@ $(CMD)

.PHONY: test
test:
	$(GOTEST) ./...

.PHONY: init
init:
	$(GOMOD) tidy -v

.PHONY: lint
lint: vet vuln

.PHONY: vuln
vuln:
	go tool govulncheck ./...

.PHONY: vet
vet:
	go vet ./...
