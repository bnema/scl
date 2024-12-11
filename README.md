# Search Container Logs (scl)

A simple CLI tool to search through logs of all running containers simultaneously and as fast as possible.

## Installation

```bash
go install github.com/bnema/scl@latest
```

## Example

```bash
scl "error in database"

scl "error in database" --since 1h

scl "error in database" --tail 1000
```
