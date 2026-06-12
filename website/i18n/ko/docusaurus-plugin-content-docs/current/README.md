---
source: docs/README.md
source_hash: ec2e5f828dbd96a6dccffb058f559c711a200904
---
# deck 문서

`deck`는 에어갭(망분리) 및 운영 제약 환경을 위한 워크플로 도구입니다. 워크플로를 작성하고, 검증하고, 사이트에 필요한 것을 번들로 묶은 뒤, 대상 머신에서 로컬로 실행하세요.

## 여기서 시작

- **[Quick Start](quick-start.md)**: 워크스페이스를 만들고, 린트하고, 아티팩트를 준비하고, 번들을 빌드한 뒤, 로컬에서 적용합니다.
- **[Using deck ask](ask.md)**: 플랜 모드와 진단을 포함해 `deck ask`를 구성하고 사용합니다.
- **[Examples](examples/README.md)**: 사이트 절차에 맞게 변형할 수 있는 구체적인 워크플로 파일에서 시작합니다.

## 워크플로 작성

- **[Workflow Model](workflow-model.md)**: 워크플로 구조, 공통 스텝 필드, 변수, 단계, 검증 규칙.
- **[Step Kinds](step-kinds.md)**: 워크플로 스텝 종류를 선택하고 작성하기 위한 단계 및 작업 중심 레퍼런스.
- **[Workspace Layout](workspace-layout.md)**: 워크스페이스 구조와 컴포넌트 프래그먼트 계약.

## 운영

- **[CLI Reference](cli.md)**: 명령줄 사용법과 플래그.
- **[Apply State](apply-state.md)**: 단계 기반 적용 재개 및 `--fresh` 동작.
- **[Apply Run Logs](apply-runlogs.md)**: XDG 상태 루트 아래에 기록되는 실행별 `record.json` / `events.jsonl` 진단.
- **[Error Codes](diagnostics/error-codes.md)**: deck가 방출하는 안정적인 `CODE: message` 진단 카탈로그.
- **[Bundle Layout](bundle-layout.md)**: 자체 완결형 번들 형식.
- **[Server Audit Log](server-audit-log.md)**: 서버 감사 로그 레코드 형태.
- **[Server Registry](server/registry.md)**: 준비된 이미지를 제공하는 읽기 전용 OCI `/v2` 레지스트리.
- **[Server Daemon](server/daemon.md)**: `deck server up`을 백그라운드에서 실행합니다(Linux에서는 systemd, macOS/Windows에서는 pid-file 프로세스).
- **[.deckignore](deckignore.md)**: gitignore 스타일 패턴을 사용해 번들과 서버에서 파일을 제외합니다.

## 보조 섹션

- **[Core Concepts](core-concepts/README.md)**: deck가 존재하는 이유와 아키텍처가 어떻게 맞물리는지.
- **[Contributing](contributing/README.md)**: 개발 프로세스, 스타일, 릴리스, 호환성 참고 사항.
- **[Release Notes](releases/README.md)**: 각 버전과 함께 제공되는 릴리스별 한국어 주요 변경 사항(`주요 변경 사항`).

## 자주 찾는 경로

- deck가 처음이라면: [Quick Start](quick-start.md)에서 시작하세요
- 오프라인 Kubernetes 워크플로를 계획 중이라면: [Offline Kubernetes Tutorial](offline-kubernetes.md)를 읽으세요
- 워크플로를 작성한다면: [Workflow Model](workflow-model.md)을 사용하세요
- 정확한 명령 구문을 찾는다면: [CLI Reference](cli.md)를 사용하세요
