# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build ./...                         # compile all packages
go build -o kubectl-ctx-diff .         # build binary
go test ./...                          # run all tests
go test ./pkg/diff/... -v              # run diff package tests verbosely
go run . diff --context-2 <ctx> -n <ns>
```

## Architecture

```
main.go                   entry point; imports _ "k8s.io/client-go/plugin/pkg/client/auth"
cmd/root.go               cobra root; --context-1, --context-2, --kubeconfig persistent flags
cmd/diff.go               diff subcommand; validation, client setup, resource fetching, output
pkg/client/client.go      ResolveContextName, ValidateNamespace, New - all accept kubeconfig path
pkg/diff/configmap.go     ConfigMaps() + shared DiffResult/KeyDiff types + diffData()
pkg/diff/secret.go        Secrets() - keys only, values never populated, Redacted: true
pkg/diff/deployment.go    DeploymentResources() + DeploymentEnvVars() via containerExtractor
```

**Data flow:** `cmd/diff` resolves and validates both contexts and namespaces, builds two `kubernetes.Interface` clients, calls all diff functions, aggregates `[]DiffResult`, prints table or JSON.

**`pkg/diff`** accepts `kubernetes.Interface` so tests use `k8s.io/client-go/kubernetes/fake` with no real cluster needed.

**`DiffResult`** holds `Kind`, `Name`, `Namespace1`, `Namespace2`, `[]KeyDiff`. `KeyDiff` has `Key`, `Value1`, `Value2`, `Status`, `Redacted`. Status constants in `configmap.go`: `equal`, `modified`, `only-in-1`, `only-in-2`.

**`diffData()`** in `configmap.go` is the shared key-by-key diff function used by both ConfigMaps and Deployment extractors. Secrets use `diffSecretData()` to avoid ever populating values.

## Key behaviors

- `--context-1` defaults to current kubeconfig context; `--context-2` is required
- `--namespace-2` defaults to `--namespace-1` value if not set
- Same context + same namespace is rejected; same context + different namespace is allowed
- Both contexts validated against kubeconfig before any API calls
- Both namespaces validated against their respective clusters before diffing
- `--filter` accepts comma-separated: `configmaps`, `secrets`, `deployments` (deployments runs both resources and env vars)
- Large or multiline values shown as `sha256:<8hex> [NB]` in table; full values in JSON
- Secret values are always `[redacted]` in table; empty strings with `Redacted: true` in JSON
- `--full` writes values to temp files and calls `$DIFFTOOL` (fallback: `diff`); skips redacted keys
- `--output json` excludes equal keys from output
- Help text shows `kubectl ctx-diff` when binary is invoked with `kubectl-` prefix

## Code style

- Errors: `fmt.Errorf("context: %w", err)`
- No em dashes in any text
- Kubeconfig loaded via `clientcmd.NewDefaultClientConfigLoadingRules()` with optional `ExplicitPath`
- Table output: stdlib `text/tabwriter` + `github.com/fatih/color`

## Module and release

Module: `github.com/salvador-arreola/kubectl-ctx-diff`. Go 1.26, k8s.io/client-go v0.36.x.

Release: tag `vX.Y.Z` triggers `.github/workflows/release.yml` via GoReleaser. GoReleaser outputs to `.goreleaser-dist/` (not `dist/`). After release, update sha256 values in `deploy/krew.yaml` from `checksums.txt`, then submit PR to `kubernetes-sigs/krew-index` with the manifest renamed to `ctx-diff.yaml`.

Version injected at build time: `-X github.com/salvador-arreola/kubectl-ctx-diff/cmd.version={{.Version}}`.
