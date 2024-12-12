# SCL - Search Container Logs

A simple CLI tool to search through logs of all running containers simultaneously and as fast as possible.

## Installation

```bash
go install github.com/bnema/scl@latest
```

## Usage

You can use scl in different ways:

### View logs without search pattern
```bash
# Follow logs in real-time
scl --follow

# Show last 100 lines
scl --tail 100

# Show logs from last hour
scl --since 1h

# Combine flags
scl --follow --tail 100 --since 1h
```

### Search in logs
```bash
# Basic search
scl "error"

# Search with flags
scl "error" --follow
scl "error" --tail 100
scl "error" --since 1h

# Combine flags
scl "error" --follow --tail 100 --since 1h
```

## Flags

- `--follow` or `-f`: Follow log output in real-time
- `--tail` or `-t`: Number of lines to show from the end of logs
- `--since` or `-s`: Show logs since duration (e.g., 1h, 30m, 24h)
