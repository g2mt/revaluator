GO      ?= go
BINDIR  ?= bin

.PHONY: all python clean

all: python

# python build requires cgo + libpython dev headers/libs
python:
	CGO_ENABLED=1 $(GO) build -tags python -o $(BINDIR)/server-python ./cmd/server

clean:
	rm -rf $(BINDIR)
