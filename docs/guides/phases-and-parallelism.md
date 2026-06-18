# Phases and parallelism

This guide explains when to use named phases instead of a flat step list, how
to name and import phases for operator legibility, and how to run safe steps
concurrently inside a phase using `parallelGroup` and `maxParallelism`.

---

## Flat steps vs. named phases

A workflow can express all its work in one `steps:` list or break it into
named `phases:`. The two forms are mutually exclusive — a workflow must choose
one at the top level.

```yaml
# Flat — fine for small, self-contained procedures.
version: v1alpha1
steps:
  - id: ensure-state-dir
    kind: EnsureDirectory
    spec:
      path: /var/lib/deck
      mode: "0755"
  - id: write-config
    kind: WriteFile
    spec:
      path: /etc/myapp.conf
      content: "mode: production\n"
```

Flat steps execute as an implicit phase named `default`. That is fine until
the procedure grows past a handful of steps — at that point naming the phases
makes the intent visible before an operator has to read every step.

Use named phases when:

- The procedure has two or more natural boundaries (e.g. host prerequisites →
  runtime installation → application bootstrap).
- You want meaningful resume checkpoints (see below).
- You are composing reusable component fragments from `workflows/components/`.

---

## Phases as resume checkpoints

Phases are the persisted resume boundary for `deck apply`.

- Completed phases are skipped on the next run when state is present.
- A failed phase is rerun from its **first step** on the next run. Partial
  progress inside a failed phase is not reused.
- Step-level resume is not supported; only phase-level checkpoints are
  persisted.

This means that a phase represents a meaningful unit of work that is either
entirely done or starts over. Choose phase boundaries at natural "safe to
restart from here" points — after packages are installed, after the runtime is
configured, before a one-way operation like `kubeadm init`.

For full details on what is persisted, see [Apply State](../apply-state.md).

---

## Naming phases for operator legibility

Pick concise, action-oriented names. An operator reading `deck plan` output
should understand the procedure at a glance:

```yaml
version: v1alpha1
phases:
  - name: host-prereqs        # OS-level checks and packages
  - name: runtime             # containerd, kubelet
  - name: image-source        # configure containerd registry mirror
  - name: bootstrap           # kubeadm init, announce join
  - name: verify              # readiness checks
```

This five-phase structure comes directly from the
[offline-kubernetes example](https://github.com/Airgap-Castaways/deck/blob/main/docs/examples/offline-kubernetes/workflows/scenarios/bootstrap.yaml).
Reading the phase names alone gives an operator a useful mental model of the
deployment flow before they open a single component file.

---

## Importing component fragments

Phases can import reusable step files from `workflows/components/`. The path
is always relative to the `components/` directory — never a `../` path.

```yaml
phases:
  - name: host-prereqs
    imports:
      - path: host-prereqs.yaml
      - path: repo/offline-repo-debian.yaml
        when: runtime.host.os.family == "debian"
  - name: runtime
    imports:
      - path: runtime/k8s-binaries.yaml
      - path: runtime/containerd.yaml
      - path: runtime/kubelet.yaml
  - name: image-source
    imports:
      - path: runtime/registry-mirror.yaml
```

Component files contain only a `steps:` list. They cannot have their own
`phases:` or `vars:` — shared defaults belong in `workflows/vars.yaml` or the
importing scenario's `vars:` block.

### Conditional imports

Each import entry accepts an optional `when` CEL condition. Deck AND-combines
the import condition with each step's own `when`:

- If only the import has `when`, all steps from that file inherit it.
- If only the step has `when`, it keeps its own condition.
- If both are present, the step runs only when `(import-when) && (step-when)`.

```yaml
phases:
  - name: host-prereqs
    imports:
      - path: repo/offline-repo-debian.yaml
        when: runtime.host.os.family == "debian"   # all steps in this file
      - path: repo/offline-repo-rhel.yaml
        when: runtime.host.os.family == "rhel"
      - path: host-prereqs.yaml                    # no filter — all nodes
```

A phase may also mix imports with inline `steps:`:

```yaml
phases:
  - name: verify
    imports:
      - path: verify/bootstrap-cluster.yaml
    steps:
      - id: check-node-ready
        kind: Command
        spec:
          command: [kubectl, get, nodes]
```

---

## Parallel batches with `parallelGroup`

Within a phase, consecutive steps that share the same `parallelGroup` value
run as a single concurrent batch. Steps in different groups, or with no group,
run sequentially.

```yaml
phases:
  - name: packages
    maxParallelism: 2
    steps:
      - id: download-ubuntu
        kind: DownloadPackage
        parallelGroup: distro-downloads
        spec:
          packages: [containerd]
          distro:
            family: debian
            release: ubuntu2404
          repo:
            type: deb-flat
          backend:
            mode: container
            runtime: docker
            image: ubuntu:24.04

      - id: download-rhel
        kind: DownloadPackage
        parallelGroup: distro-downloads
        spec:
          packages: [containerd]
          distro:
            family: rhel
            release: rhel9
          repo:
            type: rpm
          backend:
            mode: container
            runtime: docker
            image: rockylinux:9

      # This step runs after the batch above completes, sequentially.
      - id: verify-downloads
        kind: Command
        spec:
          command: [ls, -lh, outputs/packages/]
```

### Rules for `parallelGroup`

1. **Contiguous only.** Only consecutive steps with the same value belong to
   the same batch. Once a batch closes (because a step with a different group
   or no group appears), the same group name cannot reopen later in the phase.

2. **Apply-time kind allowlist.** At apply time, only a specific set of step
   kinds may appear in a parallel batch: `Command`, `CopyFile`,
   `EnsureDirectory`, `ExtractArchive`, `WaitForCommand`, `WaitForFile`,
   `WaitForMissingFile`, `WaitForService`, `WaitForTCPPort`,
   `WaitForMissingTCPPort`, and `WriteFile`. Other kinds must run
   sequentially.

3. **No shared output paths.** Same-batch apply steps cannot target the same
   literal output path or node path. Same-batch prepare steps cannot write to
   the same prepared root path.

4. **No cross-batch `register` consumption.** A step cannot consume a
   `register` value produced by another step in the same batch. Registered
   values from a batch become visible only after the entire batch succeeds.
   See [Capturing step output with register](capturing-output.md) for details
   on this restriction.

### `maxParallelism` on a phase

`maxParallelism` caps how many steps inside a batch run simultaneously. The
cap applies per-batch inside the phase. Without it, all steps in a batch start
at once.

```yaml
phases:
  - name: runtime
    maxParallelism: 2          # run at most 2 batch steps at a time
    imports:
      - path: runtime/containerd.yaml
```

This is useful when running many parallel downloads on a machine with limited
resources — set `maxParallelism: 4` to keep concurrency bounded without
serializing work entirely.

---

## Worked example: multi-phase scenario with a parallel download group

The following scenario uses three phases. The `prepare-artifacts` phase runs
two downloads concurrently, then proceeds to the `install` and `verify` phases
sequentially.

```yaml
# workflows/scenarios/apply.yaml
version: v1alpha1
phases:
  - name: prepare-artifacts
    maxParallelism: 2
    steps:
      # Both steps share parallelGroup "downloads" — they start at the same time.
      - id: extract-containerd
        kind: ExtractArchive
        parallelGroup: downloads
        spec:
          src: "{{ .context.paths.bundleRoot }}/files/bin/linux/amd64/containerd.tar.gz"
          dest: /opt/containerd-k8s
          strip: 1

      - id: extract-cni-plugins
        kind: ExtractArchive
        parallelGroup: downloads
        spec:
          src: "{{ .context.paths.bundleRoot }}/files/bin/linux/amd64/cni-plugins.tgz"
          dest: /opt/cni/bin
          strip: 0

      # Runs sequentially after the batch above completes.
      - id: set-cni-permissions
        kind: Command
        spec:
          command: [chmod, -R, "0755", /opt/cni/bin]

  - name: install
    imports:
      - path: runtime/kubelet.yaml

  - name: verify
    steps:
      - id: check-kubelet
        kind: WaitForService
        spec:
          name: kubelet
          timeout: 2m
          interval: 5s
```

**Resume behavior:** if the `install` phase fails mid-way and you rerun `deck
apply`, the `prepare-artifacts` phase is skipped (already completed), and
`install` restarts from its first step.

---

## Anti-patterns

### Over-splitting phases

A phase with a single two-line step adds ceremony without benefit. Merge
logically related steps into one phase rather than giving every step its own
phase. A good rule of thumb: a phase should represent 30 seconds to several
minutes of work with a meaningful name.

### Parallelizing steps that share output or order dependencies

If step B reads a file that step A writes, they must run sequentially. Putting
them in the same `parallelGroup` violates the rule and will either fail with a
path-conflict error or produce a race.

### Consuming a `register` value in the same batch

```yaml
steps:
  # BAD: both steps are in the same batch.
  # "join-node" cannot see joinFile until after the batch completes.
  - id: init-cluster
    kind: InitKubeadm
    parallelGroup: kube-init
    register:
      joinFile: joinFile
    spec:
      outputJoinFile: /tmp/deck/join.txt

  - id: join-node
    kind: JoinKubeadm
    parallelGroup: kube-init          # WRONG — same batch
    spec:
      joinFile: "{{ .runtime.joinFile }}"
```

Fix: remove `parallelGroup` from both steps, or put them in separate phases.

---

## Related references

- [Workflow Model — Phases](../workflow-model.md#phases)
- [Workflow Model — Parallel batches](../workflow-model.md#parallel-batches)
- [Apply State — Phase-based resume](../apply-state.md#phase-based-resume)
- [Capturing step output with register](capturing-output.md)
