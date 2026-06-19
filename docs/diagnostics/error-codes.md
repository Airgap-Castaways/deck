# Diagnostics: error codes

deck renders CLI errors in the form `CODE: message` (see `internal/errcode`). The codes below are stable, machine-readable identifiers you can match against in scripts and logs.

> **Contributors:** When adding a new `E_*` code in source, add a row to the appropriate section below — the `internal/doccheck` drift test fails the build until the code is documented.

## Bundle

| Code | Meaning | Context |
|------|---------|---------|
| `E_MANIFEST_MISSING` | `.deck/manifest.json` (or the manifest entry in a tar archive) was not found | Bundle integrity check at apply start; `bundle verify`; bundle merge |
| `E_MANIFEST_EMPTY` | The bundle manifest exists but contains no tracked entries | `bundle verify`; apply-start manifest verification |
| `E_BUNDLE_INTEGRITY` | A bundle artifact is missing, has a size or SHA-256 mismatch, or a manifest entry is structurally invalid | `bundle verify`, apply-start verification, bundle merge |
| `E_BUNDLE_IMPORT_PATH_TRAVERSAL` | A tar entry in the bundle archive contains a path that escapes the destination root (e.g. `../`) | `bundle import` archive extraction |
| `E_BUNDLE_IMPORT_INVALID_PREFIX` | A tar entry path does not start with the required `bundle/` prefix | `bundle import` archive extraction |
| `E_BUNDLE_IMPORT_UNSUPPORTED_TYPE` | A tar entry has an unsupported file type (not a regular file or directory) | `bundle import` archive extraction |

## Workflow Validation

| Code | Meaning | Context |
|------|---------|---------|
| `E_SCHEMA_INVALID` | A workflow step or phase has an invalid or deprecated field value | Schema validation before apply/prepare; `deck validate` |
| `E_KIND_ROLE_MISMATCH` | A step kind is used in a role (apply/prepare) that does not support it | Step kind validation during workflow loading |
| `E_DUPLICATE_PHASE_NAME` | Two or more phases share the same name (including empty name) | Phase name uniqueness check during validation |
| `E_DUPLICATE_STEP_ID` | Two workflow steps share the same `id` | Workflow semantic validation |
| `E_RUNTIME_VAR_RESERVED` | A variable name used in `register` conflicts with a built-in runtime variable | Semantic validation of register outputs; runtime variable registration |
| `E_REGISTER_VAR_INVALID` | A step's `register` variable name is invalid (does not match the required pattern) | Workflow semantic validation |
| `E_RUNTIME_VAR_REDEFINED` | A `runtime.*` variable is defined more than once across workflow steps | Workflow semantic validation |
| `E_TEMPLATE_SINGLE_BRACE` | A template expression uses single-brace syntax (`{var}`) instead of the required double-brace syntax (`{{var}}`) | Template syntax check during step validation |
| `E_REGISTER_OUTPUT_NOT_FOUND` | A `register` block references an output key that the step does not produce | Semantic validation; runtime step output registration |
| `E_CONDITION_EVAL` | A `when` CEL condition failed to evaluate at runtime | Step condition evaluation in install and prepare runners |

## Parallel Groups

| Code | Meaning | Context |
|------|---------|---------|
| `E_PARALLEL_KIND_UNSAFE` | A step kind that is not safe for concurrent execution appears inside a `parallelGroup` | Parallel group validation during workflow load |
| `E_PARALLEL_PATH_CONFLICT` | Two steps in the same `parallelGroup` both write to the same filesystem path | Parallel group conflict detection |
| `E_PARALLEL_OUTPUT_CONFLICT` | Two steps in the same `parallelGroup` both write the same `register` output key | Parallel group conflict detection |
| `E_PARALLEL_RUNTIME_DEPENDENCY` | A step reads a `runtime.*` variable that is produced by another step in the same `parallelGroup` | Parallel group dependency check |
| `E_PARALLEL_GROUP_DISCONTIGUOUS` | Steps sharing a `parallelGroup` are not contiguous within their phase | Workflow validation |

## Prepare Phase

| Code | Meaning | Context |
|------|---------|---------|
| `E_PREPARE_KIND_UNSUPPORTED` | A step kind is not supported during the prepare phase | Step dispatch in prepare runner |
| `E_PREPARE_NO_ARTIFACTS` | A package-download step produced no artifacts in its output directory | Package artifact collection in prepare |
| `E_PREPARE_SOURCE_NOT_FOUND` | A `source.path` referenced in a step could not be resolved to any configured fetch source | File and artifact download in prepare |
| `E_PREPARE_CHECKSUM_MISMATCH` | Downloaded artifact checksum does not match the expected value | File download verification in prepare |
| `E_PREPARE_OFFLINE_POLICY_BLOCK` | A `source.url` download was blocked by the offline policy | File download in prepare when offline mode is enforced |
| `E_PREPARE_RUNTIME_NOT_FOUND` | No supported container runtime (docker/podman) is available for package preparation | Container runtime detection in prepare |
| `E_PREPARE_RUNTIME_UNSUPPORTED` | The configured container runtime is not supported | Container runtime selection in prepare |
| `E_PREPARE_ENGINE_UNSUPPORTED` | The configured image engine is not supported | Image pull/push engine selection in prepare |
| `E_PREPARE_CHECKHOST_FAILED` | A `CheckHost` step failed during the prepare phase | Host pre-check execution in prepare |
| `E_PREPARE_OUTPUT_ROOT_INVALID` | A step's output path escapes the allowed output root directory | Output path validation in prepare parallel steps |

## Install (Apply) Phase — General

| Code | Meaning | Context |
|------|---------|---------|
| `E_INSTALL_KIND_UNSUPPORTED` | A step kind is not supported during the install (apply) phase | Step dispatch in install runner |
| `E_INSTALL_SOURCE_NOT_FOUND` | A required source artifact was not found in the bundle | Artifact lookup during install step execution |
| `E_INSTALL_CHECKSUM_MISMATCH` | An artifact's checksum does not match the expected value | Artifact integrity verification during install |
| `E_INSTALL_OFFLINE_POLICY_BLOCK` | A network download was blocked by the offline policy during install | Network access gate during install |
| `E_INSTALL_CHECKHOST_FAILED` | A `CheckHost` step failed during the install phase | Host pre-check execution in install |
| `E_INSTALL_CLUSTER_CHECK_FAILED` | A `CheckKubernetesCluster` step failed | Kubernetes cluster readiness check during install |
| `E_INSTALL_INTERACTION_FAILED` | An operator interaction step (prompt/input) failed | `Input`, `Confirm` step execution |
| `E_INSTALL_INTERACTION_UNSUPPORTED` | An interaction step tried to render a secret runtime value in its message | Operator interaction message rendering |

## Install — Package Management

| Code | Meaning | Context |
|------|---------|---------|
| `E_INSTALL_PACKAGES_REQUIRED` | A package install step has no packages listed | `InstallPackage`/`InstallAptPackage`/`InstallDnfPackage` step validation |
| `E_INSTALL_PACKAGES_MANAGER_NOT_FOUND` | No supported package manager (apt/dnf/yum) was found on the host | Package manager detection during install |
| `E_INSTALL_PACKAGES_OPTION_INVALID` | A package install option (e.g., extra flags) is invalid | Package install option validation |
| `E_INSTALL_PACKAGES_SOURCE_INVALID` | The specified package source is invalid or unrecognised | Package source validation |
| `E_INSTALL_PACKAGES_INSTALL_FAILED` | The package manager command failed during install | Package installation execution |
| `E_INSTALL_REPOCONFIG_PATH_REQUIRED` | A `ConfigureRepository` step has no `path` field | `ConfigureRepository` step validation |
| `E_INSTALL_PACKAGECACHE_MANAGER_INVALID` | A `RefreshRepository` step specifies an invalid package manager | `RefreshRepository` step validation |

## Install — File Operations

| Code | Meaning | Context |
|------|---------|---------|
| `E_INSTALL_WRITEFILE_PATH_REQUIRED` | A `WriteFile` step has no `path` field | `WriteFile` step validation |
| `E_INSTALL_EDITFILE_PATH_REQUIRED` | An `EditFile`/`EditJSON`/`EditYAML`/`EditTOML` step has no `path` field | Edit-file step validation |
| `E_INSTALL_EDITFILE_EDITS_REQUIRED` | An edit-file step has no edits specified | Edit-file step validation |
| `E_INSTALL_COPYFILE_PATH_REQUIRED` | A `CopyFile` step has no `path` field | `CopyFile` step validation |
| `E_INSTALL_INSTALLFILE_PATH_REQUIRED` | An `InstallFile` step has no `path` field | `InstallFile` step validation |
| `E_INSTALL_INSTALLFILE_CONTENT_REQUIRED` | An `InstallFile` step has no content specified | `InstallFile` step validation |
| `E_INSTALL_TEMPLATEFILE_PATH_REQUIRED` | A `TemplateFile` step has no `path` field | `TemplateFile` step validation |
| `E_INSTALL_TEMPLATEFILE_TEMPLATE_REQUIRED` | A `TemplateFile` step has no template body | `TemplateFile` step validation |
| `E_INSTALL_ENSUREDIR_PATH_REQUIRED` | An `EnsureDirectory` step has no `path` field | `EnsureDirectory` step validation |
| `E_INSTALL_SYMLINK_PATH_REQUIRED` | A `CreateSymlink` step has no `path` field | `CreateSymlink` step validation |
| `E_INSTALL_SYMLINK_TARGET_REQUIRED` | A `CreateSymlink` step has no `target` field | `CreateSymlink` step validation |

## Install — System Configuration

| Code | Meaning | Context |
|------|---------|---------|
| `E_INSTALL_SYSCTL_PATH_REQUIRED` | A `Sysctl` step has no `path` field | `Sysctl` step validation |
| `E_INSTALL_SYSCTL_VALUES_REQUIRED` | A `Sysctl` step has no values specified | `Sysctl` step validation |
| `E_INSTALL_MODPROBE_MODULES_REQUIRED` | A kernel module step has no modules listed | `KernelModule` step with modprobe action |
| `E_INSTALL_KERNELMODULE_NAME_REQUIRED` | A `KernelModule` step has no module name | `KernelModule` step validation |
| `E_INSTALL_SERVICE_NAME_REQUIRED` | A `ManageService` step has no service name | `ManageService` step validation |
| `E_INSTALL_SYSTEMD_UNIT_PATH_REQUIRED` | A `WriteSystemdUnit` step has no `path` field | `WriteSystemdUnit` step validation |
| `E_INSTALL_SYSTEMD_UNIT_CONTENT_REQUIRED` | A `WriteSystemdUnit` step has neither inline content nor a source file | `WriteSystemdUnit` step validation |
| `E_INSTALL_SYSTEMD_UNIT_CONTENT_CONFLICT` | A `WriteSystemdUnit` step specifies both inline content and a source file | `WriteSystemdUnit` step validation |
| `E_INSTALL_SYSTEMD_UNIT_SERVICE_NAME_REQUIRED` | A `WriteSystemdUnit` step requires a service name but none was provided | `WriteSystemdUnit` step validation |

## Install — Command Execution

| Code | Meaning | Context |
|------|---------|---------|
| `E_INSTALL_RUNCOMMAND_REQUIRED` | A `Command` step has no command specified | `Command` step validation |
| `E_INSTALL_RUNCOMMAND_TIMEOUT` | A `Command` step exceeded its configured timeout | `Command` step execution |
| `E_INSTALL_RUNCOMMAND_FAILED` | A `Command` step's process exited with a non-zero status | `Command` step execution |

## Install — Wait Steps

| Code | Meaning | Context |
|------|---------|---------|
| `E_INSTALL_WAIT_TIMEOUT` | A wait step (WaitForFile, WaitForService, etc.) timed out before the condition was met | All wait step kinds during install |
| `E_INSTALL_WAITPATH_PATH_REQUIRED` | A path-based wait step has no `path` field | `WaitForFile`/`WaitForMissingFile` step validation |
| `E_INSTALL_WAITPATH_STATE_INVALID` | A path-based wait step specifies an invalid expected state | `WaitForFile`/`WaitForMissingFile` step validation |
| `E_INSTALL_WAITPATH_TYPE_INVALID` | A path-based wait step specifies an invalid path type (not `file`, `dir`, or `any`) | `WaitForFile` step validation |
| `E_INSTALL_WAITPATH_POLL_INTERVAL_INVALID` | A path-based wait step specifies an invalid poll interval | `WaitForFile`/`WaitForMissingFile` step validation |
| `E_INSTALL_WAITPATH_TIMEOUT` | A path-based wait step timed out | `WaitForFile`/`WaitForMissingFile` step execution |

## Install — Image Verification

| Code | Meaning | Context |
|------|---------|---------|
| `E_INSTALL_VERIFY_IMAGES_REQUIRED` | A `VerifyImage` step has no images listed | `VerifyImage` step validation |
| `E_INSTALL_VERIFY_IMAGES_COMMAND_FAILED` | The image verification command (e.g., `crictl`) failed | `VerifyImage` step execution |
| `E_INSTALL_VERIFY_IMAGES_NOT_FOUND` | One or more required images are not present on the host | `VerifyImage` step execution |

## Install — Artifacts

| Code | Meaning | Context |
|------|---------|---------|
| `E_INSTALL_ARTIFACTS_REQUIRED` | A step that depends on bundle artifacts found none available | Artifact lookup during install |
| `E_INSTALL_ARTIFACT_ARCH_UNSUPPORTED` | No artifact in the bundle matches the host architecture | Artifact architecture selection |
| `E_INSTALL_ARTIFACT_SOURCE_INVALID` | The artifact source specification is invalid | Artifact source validation during install |

## Install — Kubernetes (kubeadm)

| Code | Meaning | Context |
|------|---------|---------|
| `E_INSTALL_KUBEADM_INIT_MODE_INVALID` | The `InitKubeadm` step specifies an invalid mode | `InitKubeadm` step validation |
| `E_INSTALL_KUBEADM_INIT_JOINFILE_REQUIRED` | The `InitKubeadm` step requires a join-file path but none was provided | `InitKubeadm` step validation |
| `E_INSTALL_KUBEADM_INIT_FAILED` | `kubeadm init` exited with an error | `InitKubeadm` step execution |
| `E_INSTALL_KUBEADM_JOIN_MODE_INVALID` | The `JoinKubeadm` step specifies an invalid join mode | `JoinKubeadm` step validation |
| `E_INSTALL_KUBEADM_JOIN_JOINFILE_REQUIRED` | The `JoinKubeadm` step requires a join-file path but none was provided | `JoinKubeadm` step validation |
| `E_INSTALL_KUBEADM_JOIN_FILE_NOT_FOUND` | The join-configuration file referenced by a `JoinKubeadm` step does not exist | `JoinKubeadm` step execution |
| `E_INSTALL_KUBEADM_JOIN_INPUT_CONFLICT` | A `JoinKubeadm` step specifies conflicting join inputs (e.g., both a join command and a join file) | `JoinKubeadm` step validation |
| `E_INSTALL_KUBEADM_JOIN_COMMAND_MISSING` | A `JoinKubeadm` step requires a join command but none was found | `JoinKubeadm` step validation |
| `E_INSTALL_KUBEADM_JOIN_COMMAND_INVALID` | The join command provided to a `JoinKubeadm` step is malformed | `JoinKubeadm` step validation |
| `E_INSTALL_KUBEADM_JOIN_FAILED` | `kubeadm join` exited with an error | `JoinKubeadm` step execution |
| `E_INSTALL_KUBEADM_RESET_FAILED` | `kubeadm reset` exited with an error | `ResetKubeadm` step execution |
| `E_INSTALL_KUBEADM_UPGRADE_FAILED` | `kubeadm upgrade` exited with an error | `UpgradeKubeadm` step execution |

## Server

| Code | Meaning | Context |
|------|---------|---------|
| `E_SERVER_TLS_PARTIAL_FILES` | Exactly one of the TLS certificate and key files exists (both must be provided together or neither) | TLS configuration at server startup |
