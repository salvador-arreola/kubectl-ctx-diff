# kubectl-ctx-diff

A kubectl plugin that diffs Kubernetes resources between two kubeconfig contexts or namespaces side by side.

Compares any namespaced resource - ConfigMaps, Secrets (keys only, values never exposed), Deployments, Services, CRDs, and more. Resource types are discovered automatically from the cluster.

```
KIND        NAME                       KEY                                                          STATUS     CONTEXT-1                  CONTEXT-2
ConfigMap   default/app-config         data.DB_HOST                                                 modified   dev-postgres.default.svc   prod-postgres.default.svc
ConfigMap   default/app-config         data.LOG_LEVEL                                               modified   debug                      warn
Deployment  default/api-server         spec.replicas                                                modified   1                          3
Deployment  default/api-server         spec.template.spec.containers[0].image                       modified   myapp/api:1.2.0            myapp/api:1.5.0
Deployment  default/api-server         spec.template.spec.containers[0].resources.limits.memory     modified   256Mi                      1Gi
Secret      default/app-secrets        data.DB_PASSWORD                                             modified   [redacted]                 [redacted]
Secret      default/app-secrets        data.EXTRA_TOKEN                                             only-in-2  [redacted]                 [redacted]
Widget      default/my-widget          spec.image                                                   modified   myapp:1.0                  myapp:2.0
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

Accepts plural names, singular names, or Kind names (case-insensitive):

```bash
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter configmaps
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter Secret
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter configmaps,secrets,widgets
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
| `--filter` | `-f` | all | Resource types to include, e.g. `configmaps,secrets` |
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

All namespaced resources are discovered automatically via the cluster's API. For each resource, all fields except cluster-assigned metadata (`uid`, `resourceVersion`, `clusterIP`, etc.) and `status` are compared.

| Resource | Notes |
|---|---|
| ConfigMap | All `data` keys and values |
| Secret | Key names only; values never exposed |
| Deployment, StatefulSet, DaemonSet | Full spec including image, replicas, resources |
| Service | Spec fields; `clusterIP` excluded (cluster-assigned) |
| CRDs | Automatically discovered and diffed - no config needed |
| Any other namespaced resource | Discovered and diffed automatically |

## Building from source

```bash
git clone https://github.com/salvador-arreola/kubectl-ctx-diff.git
cd kubectl-ctx-diff
go build -o kubectl-ctx-diff .
```

## License

MIT - see [LICENSE](LICENSE)
