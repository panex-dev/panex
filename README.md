# Panex

A development runtime for Chrome extensions. Save and instantly see behavior across contexts with state inspection and replay.

> **Status:** Early development. Not usable yet.

## Prerequisites

- Go 1.24+
- [golangci-lint](https://golangci-lint.run/welcome/install/) v1.64.5
- [goimports](https://pkg.go.dev/golang.org/x/tools/cmd/goimports)

## Setup

```bash
go mod verify
```

## Development
```bash
make fmt    # format code
make lint   # run linters
make test   # run tests with race detector
make build  # compile to ./bin/panex
```

## Architecture Decisions

See [docs/adr/](docs/adr/) for all architecture decision records.
