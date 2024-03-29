# Plugtalk 🗣

An instant chat application that allows users to spontaneously create chat rooms and communicate with each other in real-time. Plugtalk is built with Go, HTMX, Templ, SQLite, and Websockets.

## Getting Started

### Prerequisites

- Go 1.16
- SQLite
- Air (for live reload)

### Installation

Clone the repository

```bash
git clone <repo-url>
```

Navigate to the project directory

```bash
cd plugtalk
```

Install the dependencies

```bash
go mod download
```

### Usage

run all make commands with clean tests

```bash
make all build
```

build the application

```bash
make build
```

run the application

```bash
make run
```

Create DB container

```bash
make docker-run
```

Shutdown DB container

```bash
make docker-down
```

live reload the application

```bash
make watch
```

run the test suite

```bash
make test
```

clean up binary from the last build

```bash
make clean
```
