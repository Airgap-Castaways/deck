# Running the content server

`deck server up` turns a prepared bundle root into a shared HTTP endpoint. All
nodes in an air-gapped site can pull bundles, browse available scenarios, and
have containerd pull container images from the same source without any
external network access.

---

## When to run the server (and when not to)

**Run the server** when you have multiple nodes that all need to apply
workflows from the same bundle. Instead of copying the bundle to every node,
one node (typically the control-plane) hosts the content and all other nodes
point at it with `--server`.

**Skip the server** when you are running a workflow on a single node from a
local bundle. In that case, `deck apply --root . --scenario apply` works
directly against the local filesystem.

---

## Starting the server

The server requires a bundle root: the directory that contains `workflows/`
and `outputs/` (the output of `deck prepare`).

### Foreground mode

```bash
deck server up --root /path/to/bundle --addr :5000
```

The server blocks the terminal and logs requests to stderr as well as to the
audit log at `<root>/.deck/logs/server-audit.log`. Use foreground mode during
initial setup to watch requests in real time, or in short-lived automation
where the process is managed externally (a container, a supervisor, a CI step).

Stop it with `Ctrl-C`.

### Daemon mode

```bash
deck server up --root /path/to/bundle --addr :5000 --daemon
```

`--daemon` starts the server in the background. The mechanism is
platform-specific:

- **Linux**: `deck server up --daemon` launches a transient `systemd-run`
  unit. Both `systemd-run` and `systemctl` must be on `PATH`.
- **macOS / Windows**: deck spawns a detached child process and writes a pid
  file under `~/.local/state/deck/server/`.

On startup, the command prints the unit name (Linux) or the pid and log file
path (macOS/Windows):

```
server up: ok (deck-server, pid 12345)
server log: /home/ubuntu/.local/state/deck/server/deck-server.log
```

Use daemon mode when the server needs to stay running between `deck apply`
invocations on worker nodes: for example, on a control-plane node that serves
all workers during a multi-day rollout.

For full platform details including journal access on Linux, see
[Server Daemon Mode](../server/daemon.md).

---

## TLS options

By default the server listens on plain HTTP. For sites that require encrypted
transport, two options are available.

### Self-signed certificate

```bash
deck server up --root /path/to/bundle --addr :5443 --tls-self-signed
```

Deck generates a self-signed certificate at startup. Worker nodes will need to
configure `skipVerify: true` in their containerd registry mirror entries (the
offline-kubernetes example already does this; see
`vars.runtime.containerd.mirrorHosts[*].skipVerify`).

### Bring your own certificate

```bash
deck server up \
  --root /path/to/bundle \
  --addr :5443 \
  --tls-cert /etc/deck/server.crt \
  --tls-key  /etc/deck/server.key
```

Pass the path to a PEM-encoded certificate and private key. Both `--tls-cert`
and `--tls-key` must be provided together.

---

## What the server serves

| Path prefix | What it provides |
|-------------|-----------------|
| `/workflows/` | Scenario files; resolved by `--server` + `--scenario` in `deck apply` and `deck plan`. |
| `/v2/` | Read-only OCI Distribution v2 registry backed by image tarballs under `outputs/images/`. |
| Browse UI | Static site at `/` for exploring available bundles (human-readable). |
| `/healthz` | Health probe endpoint; returns `200 OK` when the server is ready. |

For registry details, endpoint list, alias rules, and read-only enforcement, see
[Server Registry](../server/registry.md).

---

## Pointing containerd at the registry mirror

The server's `/v2` registry is most useful when containerd on every node is
configured to treat it as a mirror for upstream registries. This lets kubeadm
pull images without touching the internet.

### The `WriteContainerdRegistryHosts` step

The offline-kubernetes example configures mirrors using
`WriteContainerdRegistryHosts` in
`components/runtime/registry-mirror.yaml`:

```yaml
# workflows/components/runtime/registry-mirror.yaml
steps:
  - id: configure-containerd-registry-mirrors
    kind: WriteContainerdRegistryHosts
    spec:
      path: "{{ .vars.runtime.containerd.certsDir }}"
      registryHosts: "{{ .vars.runtime.containerd.mirrorHosts }}"
  - id: restart-containerd-after-registry-mirror
    kind: ManageService
    spec:
      name: "{{ .vars.runtime.containerd.serviceName }}"
      state: restarted
  - id: wait-containerd-after-registry-mirror
    kind: WaitForService
    spec:
      name: "{{ .vars.runtime.containerd.serviceName }}"
      timeout: 5m
      interval: 2s
```

`vars.runtime.containerd.mirrorHosts` in the example's `vars.yaml` maps each
upstream registry (`registry.k8s.io`, `quay.io`, `docker.io`, `ghcr.io`) to
the deck server host. The example uses `http://192.0.2.10:5000` with
`skipVerify: true`.

Run this component as part of the `image-source` phase on every node before
the bootstrap or join step that needs images:

```yaml
phases:
  - name: image-source
    imports:
      - path: runtime/registry-mirror.yaml
```

After `WriteContainerdRegistryHosts` runs and containerd restarts, `kubeadm
init` / `kubeadm join` with `imagePullPolicy: Never` will resolve images from
the server's `/v2` registry instead of the internet.

### Pointing worker apply runs at the server

When a worker node runs `deck apply` against a remote scenario, use
`--server <host:port>` to tell deck where to fetch workflows:

```bash
deck apply --server 192.0.2.10:5000 --scenario join
```

Deck resolves the scenario file from
`http://192.0.2.10:5000/workflows/scenarios/join.yaml` and stores apply state
in the user-local XDG state root (not the bundle directory, since the workflow
source is remote).

You can also save the server address as a persistent default so you do not
have to type it on every invocation:

```bash
deck server remote set http://192.0.2.10:5000
deck apply --scenario join          # uses the saved remote
```

---

## Health check

After starting the server (in either mode), verify it is responding before
pointing nodes at it:

```bash
deck server health --server http://192.0.2.10:5000
```

```bash
# Machine-readable output:
deck server health --server http://192.0.2.10:5000 -o json
```

`deck server health` sends a `GET` to `/healthz` and reports `health: ok` on
HTTP 200. If the server is not yet ready, retry after a few seconds.

---

## Logs and audit records

### Viewing audit logs

Every HTTP request is written to a JSONL audit log at:

```
<root>/.deck/logs/server-audit.log
```

Read it with:

```bash
deck server logs --root /path/to/bundle --source file
```

Or stream the journal on Linux:

```bash
deck server logs --source journal --unit deck-server.service
```

The audit log rotates automatically when it exceeds the configured size limit
(default 50 MB, up to 10 retained files). Adjust with `--audit-max-size-mb`
and `--audit-max-files` on `deck server up`.

For the full audit record schema, see [Server Audit Log](../server-audit-log.md).

### Verbosity

Add `--v=1` or `--v=2` to `deck server up` to emit structured diagnostics to
stderr alongside the audit log.

---

## Stopping the daemon

```bash
deck server down
```

On Linux, this calls `systemctl stop deck-server.service`. On macOS/Windows,
it sends `SIGTERM` to the tracked process and removes the pid file. If the
process is already gone (stale pid), the stop still succeeds and the pid file
is cleaned up.

To stop a daemon started with a custom `--unit`:

```bash
deck server down --unit my-bundle-server
```

---

## Worked example: control-plane server + worker join

This end-to-end walkthrough matches the offline-kubernetes example layout.

### On the control-plane node (cp-1, 192.0.2.10)

```bash
# 1. Extract the bundle that was transferred from the connected environment.
tar -xf bundle.tar

# 2. Start the server as a daemon on port 5000.
deck server up --root . --addr :5000 --daemon

# 3. Verify it is ready.
deck server health --server http://192.0.2.10:5000

# 4. Apply the bootstrap scenario (uses local filesystem, not the server).
deck apply --root . --scenario bootstrap
```

During bootstrap the operator is prompted for a passphrase. The kubeadm join
command is printed to the terminal as an encrypted block only. Copy the text
between `BEGIN ENCRYPTED JOIN` and `END ENCRYPTED JOIN`.

### On each worker node (worker-1, 192.0.2.21)

```bash
# 1. Extract the bundle (same bundle as the control plane).
tar -xf bundle.tar

# 2. Apply the join scenario, pointing at the control-plane server.
#    deck fetches the join.yaml workflow from http://192.0.2.10:5000.
deck apply --server 192.0.2.10:5000 --scenario join
```

The join scenario:
1. Prompts for the encrypted join block (paste the ciphertext).
2. Prompts for the passphrase.
3. Configures containerd registry mirrors (the `image-source` phase) so
   kubeadm can pull images from `192.0.2.10:5000/v2` without internet access.
4. Decrypts the join command locally and runs `kubeadm join`.

### Shut down after all workers have joined

```bash
# On cp-1, after all workers have joined successfully:
deck server down
```

---

## Related references

- [Server Registry](../server/registry.md): OCI `/v2` endpoint details
- [Server Daemon Mode](../server/daemon.md): systemd and pid-file mechanics
- [Server Audit Log](../server-audit-log.md): audit record schema
- [Offline Kubernetes example](../examples/README.md): full multi-node walkthrough
- [CLI Reference, deck server up](../cli/deck_server_up.md)
