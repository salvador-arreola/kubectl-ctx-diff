# kubectl-ctx-diff

[![Krew](https://img.shields.io/badge/kubectl%20plugin-krew-blue?logo=kubernetes&logoColor=white)](https://krew.sigs.k8s.io/plugins/)
[![Go Report Card](https://goreportcard.com/badge/github.com/salvador-arreola/kubectl-ctx-diff)](https://goreportcard.com/report/github.com/salvador-arreola/kubectl-ctx-diff)
[![GitHub release](https://img.shields.io/github/release/salvador-arreola/kubectl-ctx-diff.svg)](https://github.com/salvador-arreola/kubectl-ctx-diff/releases)
[![License](https://img.shields.io/github/license/salvador-arreola/kubectl-ctx-diff)](LICENSE)

![kubect-ctx-diff demo](static/demo.gif)

A kubectl plugin that diffs Kubernetes resources between two contexts or namespaces, side by side, field by field.

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

## Highlights

- **Automatic discovery** - diffs every namespaced resource type in the cluster, including CRDs, with no configuration needed.
- **Secret-safe** - Secret key names are diffed; values are never exposed. SHA-256 hashes detect changes without leaking data.
- **Flexible targeting** - compare two clusters, two namespaces in the same cluster, or any cross-cluster/cross-namespace combination.
- **Composable output** - colored table for humans, JSON for scripts and CI pipelines.
- **External diff tool support** - pipe large or multiline values to `diff`, `vimdiff`, or any `$DIFFTOOL`.

## Installation

**Via [krew](https://krew.sigs.k8s.io/)** (recommended):

```sh
kubectl krew install ctx-diff
```

**Via `go install`:**

```sh
go install github.com/salvador-arreola/kubectl-ctx-diff@latest
```

**Manual download:**

```sh
curl -L https://github.com/salvador-arreola/kubectl-ctx-diff/releases/latest/download/kubectl-ctx-diff_$(uname -s)_$(uname -m).tar.gz | tar xz
mv kubectl-ctx-diff /usr/local/bin/
```

## Quick start

```sh
# diff current context against another context, same namespace
kubectl ctx-diff diff --context-2 prod-cluster -n my-app

# diff two namespaces in the same cluster
kubectl ctx-diff diff --context-2 prod-cluster --namespace-1 staging --namespace-2 production
```

`--context-1` defaults to your current kubeconfig context.

## Usage

```
kubectl ctx-diff diff --context-2 <context> [flags]
```

### Compare two clusters, same namespace

```sh
kubectl ctx-diff diff --context-1 kind-dev-cluster --context-2 kind-prod-cluster -n my-app
```

### Compare two namespaces in the same cluster

```sh
kubectl ctx-diff diff --context-2 kind-prod-cluster --namespace-1 staging --namespace-2 production
```

### Compare two namespaces across different clusters

```sh
kubectl ctx-diff diff \
  --context-1 kind-dev-cluster --namespace-1 payments \
  --context-2 kind-prod-cluster --namespace-2 billing
```

### Filter by resource type

Accepts plural names, singular names, or Kind names (case-insensitive):

```sh
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter configmaps
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter Secret
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter configmaps,secrets,widgets
```

Pods, ReplicaSets, Endpoints, EndpointSlices, and Events are excluded by default (auto-managed runtime state). Opt in explicitly:

```sh
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter pods
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --filter pods,replicasets
```

### JSON output

```sh
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app -o json
```

### Inspect large values with a diff tool

```sh
# uses diff by default, or $DIFFTOOL if set
kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --full

DIFFTOOL=vimdiff kubectl ctx-diff diff --context-2 kind-prod-cluster -n my-app --full
```

## Flags

| Flag            | Short | Default             | Description                                           |
| --------------- | ----- | ------------------- | ----------------------------------------------------- |
| `--context-1`   |       | current-context     | First kubeconfig context                              |
| `--context-2`   |       | (required)          | Second kubeconfig context                             |
| `--namespace-1` | `-n`  | `default`           | Namespace for context-1                               |
| `--namespace-2` |       | same as namespace-1 | Namespace for context-2                               |
| `--filter`      | `-f`  | all                 | Resource types to include, e.g. `configmaps,secrets`  |
| `--output`      | `-o`  | `table`             | Output format: `table` or `json`                      |
| `--full`        |       | false               | Show full value diffs via `$DIFFTOOL`                 |
| `--kubeconfig`  |       | `~/.kube/config`    | Path to kubeconfig file                               |

## Output

**Table (default):** colored, truncated. Large or multiline values shown as `sha256:<8hex> [NB]`. Secret values always shown as `[redacted]`.

**JSON:** full values included. Secret values are empty strings (`""`), `"Redacted": true` field set.

**Status values:**

| Status     | Meaning                                    |
| ---------- | ------------------------------------------ |
| `modified` | Key exists in both contexts, values differ |
| `only-in-1`| Key exists only in context-1               |
| `only-in-2`| Key exists only in context-2               |

## What gets compared

All namespaced resources are discovered automatically via the cluster's API. For each resource, all fields except cluster-assigned metadata (`uid`, `resourceVersion`, `clusterIP`, etc.) and `status` are compared.

| Resource                           | Notes                                                  |
| ---------------------------------- | ------------------------------------------------------ |
| ConfigMap                          | All `data` keys and values                             |
| Secret                             | Key names only; values never exposed                   |
| Deployment, StatefulSet, DaemonSet | Full spec including image, replicas, resources         |
| Service                            | Spec fields; `clusterIP` excluded (cluster-assigned)   |
| CRDs                               | Automatically discovered and diffed - no config needed |
| Any other namespaced resource      | Discovered and diffed automatically                    |

## Building from source

```sh
git clone https://github.com/salvador-arreola/kubectl-ctx-diff.git
cd kubectl-ctx-diff
go build -o kubectl-ctx-diff .
```

## License

MIT - see [LICENSE](LICENSE)