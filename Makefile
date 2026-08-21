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

.PHONY: golden
golden:
	go test ./pkg/cli -update-golden

.PHONY: init
init:
	$(GOMOD) tidy -v

.PHONY: lint
lint: check-licenses check-readme vet go-fix

.PHONY: vet
vet:
	go vet ./...

.PHONY: go-fix
go-fix:
	go fix -diff ./...

.PHONY: go-fix-fix
go-fix-fix:
	go fix ./...

.PHONY: check-licenses-diff
check-licenses-diff: $(THIRD_PARTY_LICENSES)
	git diff --exit-code $(THIRD_PARTY_LICENSES)

.PHONY: check-licenses
check-licenses: check-licenses-diff
	./hack/license.sh check

.PHONY: $(THIRD_PARTY_LICENSES)
$(THIRD_PARTY_LICENSES):
	./hack/license.sh report > $@

.PHONY: check-readme-diff
check-readme-diff: README.md
	git diff --exit-code README.md

.PHONY: check-readme
check-readme: check-readme-diff

.PHONY: README.md
README.md: $(BIN)
	./hack/readme.sh $@ $(BIN)
