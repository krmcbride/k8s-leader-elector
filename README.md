# k8s-leader-elector

This is a maintained, Lease-backed replacement for the archived `k8s.gcr.io/leader-elector` sidecar image from [`kubernetes-retired/contrib/election`](https://github.com/kubernetes-retired/contrib/tree/master/election).

The sidecar keeps the old user-facing contract: each participant elects a pod name as leader, and `--http=0.0.0.0:4040` serves the current leader as:

```json
{"name":"pod-name"}
```

## Compatibility Notes

The original `leader-elector` stored election state in a `core/v1` Endpoints annotation named `control-plane.alpha.kubernetes.io/leader`. Kubernetes v1.33 deprecates direct use of the Endpoints API and emits warnings when clients read or write Endpoints resources.

This project intentionally does not reimplement the legacy Endpoints lock. It uses `coordination.k8s.io/v1` `Lease` objects, which are the current Kubernetes leader-election primitive.

The HTTP contract is preserved, but the election storage mechanism is not wire-compatible with the old image. During a normal rolling update from the old image to this image, old pods would elect through Endpoints while new pods elect through Leases. Workloads that cannot tolerate two simultaneous leaders must use a non-overlapping cutover.

## Migrating From `k8s.gcr.io/leader-elector`

For workloads that require strict single leadership, do not use a normal rolling update from the old image to this one. Use a cutover such as:

1. Scale the workload to zero, or otherwise stop all old leader-elector participants.
2. Apply RBAC that grants access to `coordination.k8s.io` `leases`.
3. Update the sidecar image to this project.
4. Scale the workload back up.

If the workload can tolerate temporary dual leaders during rollout, a normal rolling update may be acceptable, but the old and new sidecars will run separate elections until all old pods are gone.

The old flags are preserved:

```console
leader-elector --election=example --http=0.0.0.0:4040
```

Supported flags:

- `--election`: required lock/election name.
- `--id`: participant identity. Defaults to the container hostname, matching the old image entrypoint behavior.
- `--election-namespace`: namespace for the lock object. Defaults to `default`.
- `--ttl`: leader lease duration. Defaults to `10s`.
- `--use-cluster-credentials`: force in-cluster client credentials.
- `--kubeconfig`: use a kubeconfig file for local testing.
- `--http`: bind address for the JSON leader endpoint.
- `--version`: print build version information and exit.

The original source used here as behavioral reference is:

- [`election/example/main.go`](https://github.com/kubernetes-retired/contrib/blob/master/election/example/main.go)
- [`election/lib/election.go`](https://github.com/kubernetes-retired/contrib/blob/master/election/lib/election.go)
- [`election/run.sh`](https://github.com/kubernetes-retired/contrib/blob/master/election/run.sh)

See `NOTICE` for attribution and licensing provenance.

## Development

Use the local flake for a consistent toolchain:

```console
nix develop
make check
```

The same checks are available through Nix:

```console
nix flake check
```

Build the binary:

```console
nix build
```

Build an OCI image tarball:

```console
nix build .#image
```

## RBAC

Grant the sidecar access to Leases in the election namespace:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: leader-elector
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: leader-elector
rules:
  - apiGroups: ["coordination.k8s.io"]
    resources: ["leases"]
    verbs: ["get", "create", "update"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: leader-elector
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: leader-elector
subjects:
  - kind: ServiceAccount
    name: leader-elector
```

Old Endpoints-only RBAC is not sufficient for this image.
