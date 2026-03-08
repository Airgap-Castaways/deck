# CLI Reference

This page describes the current top-level command surface exposed by `deck`.

## Core flow

- `init`: create starter workflow files under `workflows/`
- `validate`: validate a workflow file against the top-level workflow schema and the relevant step schema
- `pack`: discover `workflows/pack.yaml`, gather artifacts, copy workflows, embed the `deck` binary, and write `bundle.tar`
- `apply`: execute the `apply` workflow locally

## Offline source and repo flow

- `serve`: expose a bundle root over HTTP
- `source`: persist a default source mode (`server` or `local-root`)
- `list`: list workflows from the current local root or configured server
- `health`: call `/healthz` on the configured server

## Diagnostics and lifecycle

- `bundle`: bundle lifecycle operations
- `diff`: show which apply steps would run or skip
- `doctor`: generate a validation or preflight-style report
- `logs`: read server audit logs
- `cache`: inspect or clean the artifact cache
- `service`: generate or inspect service-related behavior for `deck serve`

## Common examples

```bash
deck init --out ./demo
deck validate --file ./demo/workflows/apply.yaml
deck pack --out ./bundle.tar
deck apply
deck serve --root ./bundle --addr :8080
deck source set --server http://127.0.0.1:8080
deck list
deck health
```

## Notes

- `pack` expects a workflow directory containing `pack.yaml`, `apply.yaml`, and `vars.yaml`.
- `apply` defaults to the `install` phase when phases are used.
- `source` helps remove repeated `--server` flags in shared offline repo flows.
