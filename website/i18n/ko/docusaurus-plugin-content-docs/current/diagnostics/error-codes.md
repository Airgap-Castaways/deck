---
source: docs/diagnostics/error-codes.md
source_hash: a8c0538185d6eab7444ee7ec59b9d42049da00bc
---
# 진단: 에러 코드

deck은 CLI 에러를 `CODE: message` 형식으로 렌더링합니다(`internal/errcode` 참조). 아래 코드들은 안정적이고 기계가 읽을 수 있는 식별자로, 스크립트와 로그에서 매칭할 수 있습니다.

> **기여자:** 소스에 새로운 `E_*` 코드를 추가할 때는 아래 적절한 섹션에 행을 추가하세요, `internal/doccheck` 드리프트 테스트가 해당 코드가 문서화될 때까지 빌드를 실패시킵니다.

## 번들

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_MANIFEST_MISSING` | `.deck/manifest.json`(또는 tar 아카이브 내 매니페스트 엔트리)을 찾을 수 없음 | 적용 시작 시 번들 무결성 검사; `bundle verify`; 번들 병합 |
| `E_MANIFEST_EMPTY` | 번들 매니페스트는 존재하지만 추적된 엔트리가 없음 | `bundle verify`; 적용 시작 시 매니페스트 검증 |
| `E_BUNDLE_INTEGRITY` | 번들 아티팩트가 누락되었거나, 크기 또는 SHA-256 불일치가 있거나, 매니페스트 엔트리가 구조적으로 유효하지 않음 | `bundle verify`, 적용 시작 시 검증, 번들 병합 |
| `E_BUNDLE_IMPORT_PATH_TRAVERSAL` | 번들 아카이브의 tar 엔트리가 대상 루트를 벗어나는 경로(예: `../`)를 포함함 | `bundle import` 아카이브 추출 |
| `E_BUNDLE_IMPORT_INVALID_PREFIX` | tar 엔트리 경로가 필수 `bundle/` 접두사로 시작하지 않음 | `bundle import` 아카이브 추출 |
| `E_BUNDLE_IMPORT_UNSUPPORTED_TYPE` | tar 엔트리가 지원되지 않는 파일 유형(일반 파일 또는 디렉터리가 아님)을 가짐 | `bundle import` 아카이브 추출 |

## 워크플로 검증

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_SCHEMA_INVALID` | 워크플로 스텝 또는 단계에 유효하지 않거나 더 이상 사용되지 않는 필드 값이 있음 | 적용/준비 전 스키마 검증; `deck validate` |
| `E_KIND_ROLE_MISMATCH` | 스텝 종류가 이를 지원하지 않는 역할(apply/prepare)에서 사용됨 | 워크플로 로딩 중 스텝 종류 검증 |
| `E_DUPLICATE_PHASE_NAME` | 둘 이상의 단계가 동일한 이름(빈 이름 포함)을 공유함 | 검증 중 단계 이름 고유성 검사 |
| `E_DUPLICATE_STEP_ID` | 두 워크플로 스텝이 동일한 `id`를 공유함 | 워크플로 의미 검증 |
| `E_RUNTIME_VAR_RESERVED` | `register`에서 사용된 변수 이름이 내장 런타임 변수와 충돌함 | register 출력의 의미 검증; 런타임 변수 등록 |
| `E_REGISTER_VAR_INVALID` | 스텝의 `register` 변수 이름이 유효하지 않음(필수 패턴과 일치하지 않음) | 워크플로 의미 검증 |
| `E_RUNTIME_VAR_REDEFINED` | `runtime.*` 변수가 워크플로 스텝 전반에서 두 번 이상 정의됨 | 워크플로 의미 검증 |
| `E_TEMPLATE_SINGLE_BRACE` | 템플릿 표현식이 필수 이중 중괄호 구문(`{{var}}`) 대신 단일 중괄호 구문(`{var}`)을 사용함 | 스텝 검증 중 템플릿 구문 검사 |
| `E_REGISTER_OUTPUT_NOT_FOUND` | `register` 블록이 스텝이 생성하지 않는 출력 키를 참조함 | 의미 검증; 런타임 스텝 출력 등록 |
| `E_CONDITION_EVAL` | `when` CEL 조건이 런타임에 평가에 실패함 | install 및 prepare 러너에서의 스텝 조건 평가 |

## 병렬 그룹

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_PARALLEL_KIND_UNSAFE` | 동시 실행에 안전하지 않은 스텝 종류가 `parallelGroup` 내부에 나타남 | 워크플로 로드 중 병렬 그룹 검증 |
| `E_PARALLEL_PATH_CONFLICT` | 동일한 `parallelGroup`의 두 스텝이 모두 동일한 파일 시스템 경로에 씀 | 병렬 그룹 충돌 감지 |
| `E_PARALLEL_OUTPUT_CONFLICT` | 동일한 `parallelGroup`의 두 스텝이 모두 동일한 `register` 출력 키에 씀 | 병렬 그룹 충돌 감지 |
| `E_PARALLEL_RUNTIME_DEPENDENCY` | 스텝이 동일한 `parallelGroup` 내 다른 스텝이 생성한 `runtime.*` 변수를 읽음 | 병렬 그룹 의존성 검사 |
| `E_PARALLEL_GROUP_DISCONTIGUOUS` | `parallelGroup`을 공유하는 스텝들이 해당 단계 내에서 연속적이지 않음 | 워크플로 검증 |

## 준비 단계

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_PREPARE_KIND_UNSUPPORTED` | 준비 단계에서 지원되지 않는 스텝 종류 | prepare 러너에서의 스텝 디스패치 |
| `E_PREPARE_NO_ARTIFACTS` | 패키지 다운로드 스텝이 출력 디렉터리에 아티팩트를 생성하지 않음 | 준비 단계의 패키지 아티팩트 수집 |
| `E_PREPARE_SOURCE_NOT_FOUND` | 스텝에서 참조된 `source.path`가 구성된 fetch 소스로 해석될 수 없음 | 준비 단계의 파일 및 아티팩트 다운로드 |
| `E_PREPARE_CHECKSUM_MISMATCH` | 다운로드된 아티팩트 체크섬이 예상 값과 일치하지 않음 | 준비 단계의 파일 다운로드 검증 |
| `E_PREPARE_OFFLINE_POLICY_BLOCK` | `source.url` 다운로드가 오프라인 정책에 의해 차단됨 | 오프라인 모드가 강제될 때 준비 단계의 파일 다운로드 |
| `E_PREPARE_RUNTIME_NOT_FOUND` | 패키지 준비에 사용할 수 있는 지원 컨테이너 런타임(docker/podman)이 없음 | 준비 단계의 컨테이너 런타임 감지 |
| `E_PREPARE_RUNTIME_UNSUPPORTED` | 구성된 컨테이너 런타임이 지원되지 않음 | 준비 단계의 컨테이너 런타임 선택 |
| `E_PREPARE_ENGINE_UNSUPPORTED` | 구성된 이미지 엔진이 지원되지 않음 | 준비 단계의 이미지 pull/push 엔진 선택 |
| `E_PREPARE_CHECKHOST_FAILED` | 준비 단계 중 `CheckHost` 스텝이 실패함 | 준비 단계의 호스트 사전 검사 실행 |
| `E_PREPARE_OUTPUT_ROOT_INVALID` | 스텝의 출력 경로가 허용된 출력 루트 디렉터리를 벗어남 | 준비 단계 병렬 스텝의 출력 경로 검증 |

## 설치(적용) 단계: 일반

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_INSTALL_KIND_UNSUPPORTED` | 설치(적용) 단계에서 지원되지 않는 스텝 종류 | install 러너에서의 스텝 디스패치 |
| `E_INSTALL_SOURCE_NOT_FOUND` | 필요한 소스 아티팩트를 번들에서 찾을 수 없음 | 설치 스텝 실행 중 아티팩트 조회 |
| `E_INSTALL_CHECKSUM_MISMATCH` | 아티팩트의 체크섬이 예상 값과 일치하지 않음 | 설치 중 아티팩트 무결성 검증 |
| `E_INSTALL_OFFLINE_POLICY_BLOCK` | 설치 중 네트워크 다운로드가 오프라인 정책에 의해 차단됨 | 설치 중 네트워크 접근 게이트 |
| `E_INSTALL_CHECKHOST_FAILED` | 설치 단계 중 `CheckHost` 스텝이 실패함 | 설치 단계의 호스트 사전 검사 실행 |
| `E_INSTALL_CLUSTER_CHECK_FAILED` | `CheckKubernetesCluster` 스텝이 실패함 | 설치 중 쿠버네티스 클러스터 준비 상태 검사 |
| `E_INSTALL_INTERACTION_FAILED` | 운영자 상호작용 스텝(prompt/input)이 실패함 | `Input`, `Confirm` 스텝 실행 |
| `E_INSTALL_INTERACTION_UNSUPPORTED` | 상호작용 스텝이 메시지에 비밀 런타임 값을 렌더링하려고 함 | 운영자 상호작용 메시지 렌더링 |

## 설치: 패키지 관리

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_INSTALL_PACKAGES_REQUIRED` | 패키지 설치 스텝에 나열된 패키지가 없음 | `InstallPackage`/`InstallAptPackage`/`InstallDnfPackage` 스텝 검증 |
| `E_INSTALL_PACKAGES_MANAGER_NOT_FOUND` | 호스트에서 지원되는 패키지 관리자(apt/dnf/yum)를 찾을 수 없음 | 설치 중 패키지 관리자 감지 |
| `E_INSTALL_PACKAGES_OPTION_INVALID` | 패키지 설치 옵션(예: 추가 플래그)이 유효하지 않음 | 패키지 설치 옵션 검증 |
| `E_INSTALL_PACKAGES_SOURCE_INVALID` | 지정된 패키지 소스가 유효하지 않거나 인식되지 않음 | 패키지 소스 검증 |
| `E_INSTALL_PACKAGES_INSTALL_FAILED` | 설치 중 패키지 관리자 명령이 실패함 | 패키지 설치 실행 |
| `E_INSTALL_REPOCONFIG_PATH_REQUIRED` | `ConfigureRepository` 스텝에 `path` 필드가 없음 | `ConfigureRepository` 스텝 검증 |
| `E_INSTALL_PACKAGECACHE_MANAGER_INVALID` | `RefreshRepository` 스텝이 유효하지 않은 패키지 관리자를 지정함 | `RefreshRepository` 스텝 검증 |

## 설치: 파일 작업

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_INSTALL_WRITEFILE_PATH_REQUIRED` | `WriteFile` 스텝에 `path` 필드가 없음 | `WriteFile` 스텝 검증 |
| `E_INSTALL_EDITFILE_PATH_REQUIRED` | `EditFile`/`EditJSON`/`EditYAML`/`EditTOML` 스텝에 `path` 필드가 없음 | edit-file 스텝 검증 |
| `E_INSTALL_EDITFILE_EDITS_REQUIRED` | edit-file 스텝에 지정된 편집 내용이 없음 | edit-file 스텝 검증 |
| `E_INSTALL_COPYFILE_PATH_REQUIRED` | `CopyFile` 스텝에 `path` 필드가 없음 | `CopyFile` 스텝 검증 |
| `E_INSTALL_INSTALLFILE_PATH_REQUIRED` | `InstallFile` 스텝에 `path` 필드가 없음 | `InstallFile` 스텝 검증 |
| `E_INSTALL_INSTALLFILE_CONTENT_REQUIRED` | `InstallFile` 스텝에 지정된 내용이 없음 | `InstallFile` 스텝 검증 |
| `E_INSTALL_TEMPLATEFILE_PATH_REQUIRED` | `TemplateFile` 스텝에 `path` 필드가 없음 | `TemplateFile` 스텝 검증 |
| `E_INSTALL_TEMPLATEFILE_TEMPLATE_REQUIRED` | `TemplateFile` 스텝에 템플릿 본문이 없음 | `TemplateFile` 스텝 검증 |
| `E_INSTALL_ENSUREDIR_PATH_REQUIRED` | `EnsureDirectory` 스텝에 `path` 필드가 없음 | `EnsureDirectory` 스텝 검증 |
| `E_INSTALL_SYMLINK_PATH_REQUIRED` | `CreateSymlink` 스텝에 `path` 필드가 없음 | `CreateSymlink` 스텝 검증 |
| `E_INSTALL_SYMLINK_TARGET_REQUIRED` | `CreateSymlink` 스텝에 `target` 필드가 없음 | `CreateSymlink` 스텝 검증 |

## 설치: 시스템 구성

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_INSTALL_SYSCTL_PATH_REQUIRED` | `Sysctl` 스텝에 `path` 필드가 없음 | `Sysctl` 스텝 검증 |
| `E_INSTALL_SYSCTL_VALUES_REQUIRED` | `Sysctl` 스텝에 지정된 값이 없음 | `Sysctl` 스텝 검증 |
| `E_INSTALL_MODPROBE_MODULES_REQUIRED` | 커널 모듈 스텝에 나열된 모듈이 없음 | modprobe 액션이 있는 `KernelModule` 스텝 |
| `E_INSTALL_KERNELMODULE_NAME_REQUIRED` | `KernelModule` 스텝에 모듈 이름이 없음 | `KernelModule` 스텝 검증 |
| `E_INSTALL_SERVICE_NAME_REQUIRED` | `ManageService` 스텝에 서비스 이름이 없음 | `ManageService` 스텝 검증 |
| `E_INSTALL_SYSTEMD_UNIT_PATH_REQUIRED` | `WriteSystemdUnit` 스텝에 `path` 필드가 없음 | `WriteSystemdUnit` 스텝 검증 |
| `E_INSTALL_SYSTEMD_UNIT_CONTENT_REQUIRED` | `WriteSystemdUnit` 스텝에 인라인 콘텐츠도 소스 파일도 없음 | `WriteSystemdUnit` 스텝 검증 |
| `E_INSTALL_SYSTEMD_UNIT_CONTENT_CONFLICT` | `WriteSystemdUnit` 스텝이 인라인 콘텐츠와 소스 파일을 모두 지정함 | `WriteSystemdUnit` 스텝 검증 |
| `E_INSTALL_SYSTEMD_UNIT_SERVICE_NAME_REQUIRED` | `WriteSystemdUnit` 스텝에 서비스 이름이 필요하지만 제공되지 않음 | `WriteSystemdUnit` 스텝 검증 |

## 설치: 명령 실행

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_INSTALL_RUNCOMMAND_REQUIRED` | `Command` 스텝에 지정된 명령이 없음 | `Command` 스텝 검증 |
| `E_INSTALL_RUNCOMMAND_TIMEOUT` | `Command` 스텝이 구성된 타임아웃을 초과함 | `Command` 스텝 실행 |
| `E_INSTALL_RUNCOMMAND_FAILED` | `Command` 스텝의 프로세스가 0이 아닌 상태로 종료됨 | `Command` 스텝 실행 |

## 설치: Wait 스텝

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_INSTALL_WAIT_TIMEOUT` | wait 스텝(WaitForFile, WaitForService 등)이 조건이 충족되기 전에 타임아웃됨 | 설치 중 모든 wait 스텝 종류 |
| `E_INSTALL_WAITPATH_PATH_REQUIRED` | 경로 기반 wait 스텝에 `path` 필드가 없음 | `WaitForFile`/`WaitForMissingFile` 스텝 검증 |
| `E_INSTALL_WAITPATH_STATE_INVALID` | 경로 기반 wait 스텝이 유효하지 않은 예상 상태를 지정함 | `WaitForFile`/`WaitForMissingFile` 스텝 검증 |
| `E_INSTALL_WAITPATH_TYPE_INVALID` | 경로 기반 wait 스텝이 유효하지 않은 경로 유형(`file`, `dir`, `any`가 아님)을 지정함 | `WaitForFile` 스텝 검증 |
| `E_INSTALL_WAITPATH_POLL_INTERVAL_INVALID` | 경로 기반 wait 스텝이 유효하지 않은 폴링 간격을 지정함 | `WaitForFile`/`WaitForMissingFile` 스텝 검증 |
| `E_INSTALL_WAITPATH_TIMEOUT` | 경로 기반 wait 스텝이 타임아웃됨 | `WaitForFile`/`WaitForMissingFile` 스텝 실행 |

## 설치: 이미지 검증

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_INSTALL_VERIFY_IMAGES_REQUIRED` | `VerifyImage` 스텝에 나열된 이미지가 없음 | `VerifyImage` 스텝 검증 |
| `E_INSTALL_VERIFY_IMAGES_COMMAND_FAILED` | 이미지 검증 명령(예: `crictl`)이 실패함 | `VerifyImage` 스텝 실행 |
| `E_INSTALL_VERIFY_IMAGES_NOT_FOUND` | 하나 이상의 필요한 이미지가 호스트에 존재하지 않음 | `VerifyImage` 스텝 실행 |

## 설치: 아티팩트

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_INSTALL_ARTIFACTS_REQUIRED` | 번들 아티팩트에 의존하는 스텝이 사용 가능한 아티팩트를 찾지 못함 | 설치 중 아티팩트 조회 |
| `E_INSTALL_ARTIFACT_ARCH_UNSUPPORTED` | 번들의 어떤 아티팩트도 호스트 아키텍처와 일치하지 않음 | 아티팩트 아키텍처 선택 |
| `E_INSTALL_ARTIFACT_SOURCE_INVALID` | 아티팩트 소스 명세가 유효하지 않음 | 설치 중 아티팩트 소스 검증 |

## 설치: 쿠버네티스 (kubeadm)

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_INSTALL_KUBEADM_INIT_MODE_INVALID` | `InitKubeadm` 스텝이 유효하지 않은 모드를 지정함 | `InitKubeadm` 스텝 검증 |
| `E_INSTALL_KUBEADM_INIT_JOINFILE_REQUIRED` | `InitKubeadm` 스텝에 join-file 경로가 필요하지만 제공되지 않음 | `InitKubeadm` 스텝 검증 |
| `E_INSTALL_KUBEADM_INIT_FAILED` | `kubeadm init`이 에러로 종료됨 | `InitKubeadm` 스텝 실행 |
| `E_INSTALL_KUBEADM_JOIN_MODE_INVALID` | `JoinKubeadm` 스텝이 유효하지 않은 join 모드를 지정함 | `JoinKubeadm` 스텝 검증 |
| `E_INSTALL_KUBEADM_JOIN_JOINFILE_REQUIRED` | `JoinKubeadm` 스텝에 join-file 경로가 필요하지만 제공되지 않음 | `JoinKubeadm` 스텝 검증 |
| `E_INSTALL_KUBEADM_JOIN_FILE_NOT_FOUND` | `JoinKubeadm` 스텝이 참조하는 join-configuration 파일이 존재하지 않음 | `JoinKubeadm` 스텝 실행 |
| `E_INSTALL_KUBEADM_JOIN_INPUT_CONFLICT` | `JoinKubeadm` 스텝이 충돌하는 join 입력(예: join 명령과 join 파일 둘 다)을 지정함 | `JoinKubeadm` 스텝 검증 |
| `E_INSTALL_KUBEADM_JOIN_COMMAND_MISSING` | `JoinKubeadm` 스텝에 join 명령이 필요하지만 찾을 수 없음 | `JoinKubeadm` 스텝 검증 |
| `E_INSTALL_KUBEADM_JOIN_COMMAND_INVALID` | `JoinKubeadm` 스텝에 제공된 join 명령의 형식이 잘못됨 | `JoinKubeadm` 스텝 검증 |
| `E_INSTALL_KUBEADM_JOIN_FAILED` | `kubeadm join`이 에러로 종료됨 | `JoinKubeadm` 스텝 실행 |
| `E_INSTALL_KUBEADM_RESET_FAILED` | `kubeadm reset`이 에러로 종료됨 | `ResetKubeadm` 스텝 실행 |
| `E_INSTALL_KUBEADM_UPGRADE_FAILED` | `kubeadm upgrade`가 에러로 종료됨 | `UpgradeKubeadm` 스텝 실행 |

## 서버

| Code | 의미 | 컨텍스트 |
|------|---------|---------|
| `E_SERVER_TLS_PARTIAL_FILES` | TLS 인증서와 키 파일 중 정확히 하나만 존재함(둘 다 함께 제공하거나 둘 다 제공하지 않아야 함) | 서버 시작 시 TLS 구성 |
