GO      ?= go
BINDIR  ?= bin

.PHONY: all python javascript clean

all: python javascript

python:
	CGO_ENABLED=1 $(GO) build -tags python -o $(BINDIR)/server-python ./cmd/server

javascript:
	cd $(BINDIR)/js && npm install

clean:
	rm -rf $(BINDIR)
