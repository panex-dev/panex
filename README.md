# Panex

A developemtn runtime for Chrome extensions. Save -> Instantly see behavior across contexts with state inspection and replay.

> **Status:** Early developent. Not usable yet.

## Prerequisites

- Go 1.24+
- [golangci-lint](https://golangci-lint.run/welcome/install/) v1.64.5

## Development
```bash
make fmt    # format code
make lint   # run linters
make test   # run tests with race detector
make build  # compile to ./bin/panex
```

## Architecture Decisions

See [docs/adr/](docs/adr/) for all architecture decision records.
