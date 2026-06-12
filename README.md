# kubectl-ctx-diff

A kubectl plugin that diffs Kubernetes resources between two kubeconfig contexts or namespaces side by side.

Compares ConfigMaps, Secrets (keys only, values never exposed), and Deployment resource limits and env vars.

```
KIND        NAME                      KEY                    STATUS     CONTEXT-1     CONTEXT-2
ConfigMap   test-ctx-diff/app-config  DB_HOST                modified   postgres-dev  postgres-prod
ConfigMap   test-ctx-diff/app-config  FEATURE_FLAGS          only-in-1  all
ConfigMap   test-ctx-diff/app-config  LOG_LEVEL              modified   debug         warn
Secret      test-ctx-diff/db-creds    api-key                modified   [redacted]    [redacted]
Secret      test-ctx-diff/db-creds    extra-key              only-in-2  [redacted]    [redacted]
Deployment  test-ctx-diff/api-server  nginx.requests.cpu     modified   100m          500m
Deployment  test-ctx-diff/api-server  nginx.limits.memory    modified   256Mi         1Gi
Deployment  test-ctx-diff/api-server  nginx.ENV              modified   development   production
Deployment  test-ctx-diff/api-server  nginx.DEBUG            only-in-1  true
```

## Installation

**Via krew:**
```bash
kubectl krew install ctx-diff
```

**Manual:**
```bash
curl -L https://github.com/salvador-arreola/kubectl-ctx-diff/releases/latest/download/kubectl-ctx-diff_$(uname -s)_$(uname -m).tar.gz | tar xz
mv kubectl-ctx-diff /usr/local/bin/
```

## Usage

```bash
kubectl ctx-diff diff --context-2 <context> [flags]
```

`--context-1` defaults to the current kubeconfig context.

### Compare two clusters, same namespace

```bash
kubectl ctx-diff diff --context-1 kind-dev-cluster --context-2 kind-prod-cluster -n my-app
```

### Compare two namespaces in the same cluster

```bash
kubectl ctx-diff diff --context-2 kind-prod-cluster --namespace-1 staging --namespace-2 production
```

### Compare two namespaces across different clusters

```bash
kubectl ctx-diff diff \
  --context-1 kind-dev-cluster --namespace-1 payments \
  --context-2 kind-prod-cluster --namespace-2 billing
```

### Filter by resource type

```bash
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter configmaps
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter secrets
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter deployments
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter configmaps,secrets
```

### JSON output

```bash
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app -o json
```

### Inspect large values with a diff tool

```bash
# uses diff by default, or $DIFFTOOL if set
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --full

DIFFTOOL=vimdiff kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --full
```

## Flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--context-1` | | current-context | First kubeconfig context |
| `--context-2` | | (required) | Second kubeconfig context |
| `--namespace-1` | `-n` | `default` | Namespace for context-1 |
| `--namespace-2` | | same as namespace-1 | Namespace for context-2 |
| `--filter` | `-f` | all | Resource types: `configmaps`, `secrets`, `deployments` |
| `--output` | `-o` | `table` | Output format: `table` or `json` |
| `--full` | | false | Show full value diffs via `$DIFFTOOL` |
| `--kubeconfig` | | `~/.kube/config` | Path to kubeconfig file |

## Output

**Table (default):** colored, truncated. Large or multiline values shown as `sha256:<8hex> [NB]`. Secret values always shown as `[redacted]`.

**JSON:** full values included. Secret values are empty strings (`""`), `"Redacted": true` field set.

**Status values:**
- `modified` - key exists in both, values differ
- `only-in-1` - key exists only in context-1
- `only-in-2` - key exists only in context-2

## What gets compared

| Resource | What is diffed |
|---|---|
| ConfigMap | All keys and values |
| Secret | Key names only; values never exposed |
| Deployment | Per-container CPU/memory requests and limits; literal env vars (`valueFrom` refs skipped) |

## Building from source

```bash
git clone https://github.com/salvador-arreola/kubectl-ctx-diff.git
cd kubectl-ctx-diff
go build -o kubectl-ctx-diff .
```

## License

MIT - see [LICENSE](LICENSE)
