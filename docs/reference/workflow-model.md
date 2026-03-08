# Workflow Model

`deck` uses a YAML workflow model designed to stay readable for operators who already think in manifests.

## Top-level fields

- `role`: required, either `pack` or `apply`
- `version`: currently `v1alpha1`
- `vars`: optional variable map
- `varImports`: optional external variable imports
- `imports`: optional workflow imports
- `steps`: top-level step list
- `phases`: named phase list for more structured execution

The schema allows either top-level `steps`, named `phases`, or imported workflow fragments.

## Minimal workflow

```yaml
role: apply
version: v1alpha1
steps:
  - id: disable-swap
    apiVersion: deck/v1alpha1
    kind: RunCommand
    spec:
      command: ["swapoff", "-a"]
```

## Step shape

Every step is centered on:

- `id`
- `apiVersion`
- `kind`
- `spec`

Optional execution controls:

- `when`: conditional execution expression
- `retry`: retry count
- `timeout`: duration string such as `30s` or `5m`
- `register`: export step outputs into later runtime values

## Supported step kinds

- `CheckHost`
- `DownloadPackages`
- `DownloadK8sPackages`
- `DownloadImages`
- `DownloadFile`
- `InstallPackages`
- `WriteFile`
- `EditFile`
- `CopyFile`
- `Sysctl`
- `Modprobe`
- `RunCommand`
- `VerifyImages`
- `KubeadmInit`
- `KubeadmJoin`

## Why the model looks this way

- It is explicit enough for offline troubleshooting.
- It is YAML-shaped for Kubernetes-familiar users.
- It supports schema validation without introducing a remote orchestration dependency.

## Related references

- `schema-reference.md`
- `bundle-layout.md`
- `../schemas/deck-workflow.schema.json`
