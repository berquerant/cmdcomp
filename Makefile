GOBUILD = go build -trimpath -v
GOTEST = go test -cover -race
CMD = ./cmd/cmdcomp
BIN = bin/cmdcomp
THIRD_PARTY_LICENSES = NOTICE

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
lint: check-licenses vet vuln

.PHONY: vuln
vuln:
	go tool govulncheck ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: check-licenses-diff
check-licenses-diff: $(THIRD_PARTY_LICENSES)
	git diff --exit-code $(THIRD_PARTY_LICENSES)

.PHONY: check-licenses
check-licenses: check-licenses-diff
	./hack/license.sh check

.PHONY: $(THIRD_PARTY_LICENSES)
$(THIRD_PARTY_LICENSES):
	./hack/license.sh report > $@
