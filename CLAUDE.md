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
main.go                   entry point; imports cloud provider auth; suppresses klog output
cmd/root.go               cobra root; --context-1, --context-2, --kubeconfig persistent flags
cmd/diff.go               diff subcommand; validation, client setup, AllResources call, output
pkg/client/client.go      ResolveContextName, ValidateNamespace, New, NewDynamic, NewDiscovery
pkg/diff/generic.go       AllResources, diffGVR, flattenObject, DiffResult/KeyDiff types
```

**Data flow:** `cmd/diff` resolves and validates both contexts/namespaces, builds typed clients (namespace validation), dynamic clients and a discovery client (diffing), calls `diff.AllResources`, prints table or JSON.

**Discovery:** `AllResources` calls `disc.ServerPreferredNamespacedResources()` to enumerate all namespaced listable resources. Partial failures (e.g. metrics.k8s.io) are ignored. Resources that fail to list are skipped silently.

**`pkg/diff`** uses `dynamic.Interface` so tests use `k8s.io/client-go/dynamic/fake` with no real cluster needed.

**`DiffResult`** holds `Kind`, `Name`, `Namespace1`, `Namespace2`, `[]KeyDiff`. `KeyDiff` has `Key`, `Value1`, `Value2`, `Status`, `Redacted`. Status constants: `equal`, `modified`, `only-in-1`, `only-in-2`.

**`flattenObject`** walks unstructured objects recursively into dot-notation keys (`spec.containers[0].image`). Skips: `status`, `apiVersion`, `kind`, `metadata.{resourceVersion,uid,creationTimestamp,generation,managedFields,selfLink}`, `spec.clusterIP`, `spec.clusterIPs*`, `metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"]`.

**Secret redaction:** detected by `kind == "Secret"`. Values under `data.*` and `stringData.*` are sha256-hashed for change detection; `flatVal.redacted = true` propagates to `KeyDiff.Redacted = true`; `Value1`/`Value2` are empty strings in output.

## Key behaviors

- `--context-1` defaults to current kubeconfig context; `--context-2` is required
- `--namespace-2` defaults to `--namespace-1` value if not set
- Same context + same namespace is rejected; same context + different namespace is allowed
- Both contexts validated against kubeconfig before any API calls
- Both namespaces validated against their respective clusters before diffing
- `--filter` accepts plural resource names (`configmaps`), singular (`configmap`), or Kind names (`ConfigMap`) - case-insensitive; default is all resources
- CRDs discovered and diffed automatically - no code changes needed
- Large or multiline values shown as `sha256:<8hex> [NB]` in table; full values in JSON
- Secret values are always `[redacted]` in table; empty strings with `Redacted: true` in JSON
- `--full` writes values to temp files and calls `$DIFFTOOL` (fallback: `diff`); skips redacted keys
- `--output json` excludes equal keys from output
- Help text shows `kubectl ctx-diff` when binary is invoked with `kubectl-` prefix (krew installs as `kubectl-ctx_diff` with underscore; code normalizes underscores to dashes)

## Code style

- Errors: `fmt.Errorf("context: %w", err)`
- No em dashes in any text
- Kubeconfig loaded via `clientcmd.NewDefaultClientConfigLoadingRules()` with optional `ExplicitPath`
- Table output: stdlib `text/tabwriter` + `github.com/fatih/color`

## Module and release

Module: `github.com/salvador-arreola/kubectl-ctx-diff`. Go 1.26, k8s.io/client-go v0.36.x.

Release: tag `vX.Y.Z` triggers `.github/workflows/release.yml` via GoReleaser. GoReleaser outputs to `.goreleaser-dist/` (not `dist/`). After release, update sha256 values in `deploy/krew.yaml` from `checksums.txt`, then submit PR to `kubernetes-sigs/krew-index` with the manifest renamed to `ctx-diff.yaml`.

Version injected at build time: `-X github.com/salvador-arreola/kubectl-ctx-diff/cmd.version={{.Version}}`.
