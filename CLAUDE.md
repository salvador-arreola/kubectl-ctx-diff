# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## About

`kubectl-ctx-diff` is a kubectl plugin that diffs Kubernetes resources (ConfigMaps, Secrets, env vars, resource limits) between two kubeconfig contexts or namespaces. Binary named `kubectl-ctx-diff` makes it runnable as `kubectl env`.

## Commands

```bash
go build ./...          # compile all packages
go test ./...           # run all tests
go test ./pkg/diff/...  # run diff package tests only
go run . diff --context-1 staging --context-2 prod -n my-namespace
```

## Architecture

```
main.go              → cmd.Execute()
cmd/root.go          → cobra root; persistent flags --context-1, --context-2
cmd/diff.go          → "diff" subcommand; builds clients, calls pkg/diff, prints colored table
pkg/client/client.go → New(contextName) → kubernetes.Interface via kubeconfig context override
pkg/diff/configmap.go → ConfigMaps(ctx, c1, c2, ns) → []DiffResult
```

**Data flow:** `cmd/diff` calls `client.New()` twice, passes both `kubernetes.Interface` values to `diff.ConfigMaps`, renders `[]DiffResult` via `text/tabwriter` + `fatih/color`.

**`pkg/diff`** takes `kubernetes.Interface` — decoupled from `pkg/client` so tests use `k8s.io/client-go/kubernetes/fake` without real clusters.

**`DiffResult`** holds `[]KeyDiff{Key, Value1, Value2, Status}`. Status constants: `equal`, `modified`, `only-in-1`, `only-in-2`.

## Code style

- Errors: `fmt.Errorf("context: %w", err)`
- Kubeconfig: read via `clientcmd.NewDefaultClientConfigLoadingRules()` + `ConfigOverrides{CurrentContext}`
- Table output: stdlib `text/tabwriter` + `github.com/fatih/color`

## Module

`github.com/salvador-arreola/kubectl-ctx-diff` — Go 1.26, k8s.io/client-go v0.36.x.
