# deck

<img align="right" src="assets/logo.png" width="120" alt="logo">

[English README](./README.md) | [문서 홈](./docs/README.md)

**기존의 Bash 기반 폐쇄망 운영 절차를 검증 가능한 번들 형태의 워크플로우로 전환하는 도구**

<br clear="right" />

## 프로젝트 소개

인터넷이 차단된 폐쇄망(Air-gapped) 환경에서 쿠버네티스 구축, 패키지 설치, 호스트 설정 등의 운영 작업은 대개 셸 스크립트로 시작됩니다. 하지만 운영 절차가 복잡해질수록 스크립트의 크기는 걷잡을 수 없이 커지고, 결국 유지보수나 검토가 불가능한 수준에 이르게 됩니다.

`deck`은 이러한 한계를 극복하기 위해 설계되었습니다. 불안정한 Bash 스크립트를 명확한 의도가 드러나는 '타입 기반 단계(Typed Steps)'로 대체하고, 실행 전 검증 과정을 거칩니다. 또한 작업에 필요한 모든 리소스를 독립적인 번들(Self-contained Bundle)로 묶어, 현장에서 추가 의존성 없이 즉시 실행할 수 있는 환경을 제공합니다.

## 시작하기

워크플로우 작성부터 번들 제작, 현장 실행까지의 과정은 매우 직관적입니다.

```bash
# 1. 데모 프로젝트 초기화
deck init --out ./demo

cd ./demo

# 2. 워크플로우 검증 (구조 및 문법 체크)
deck lint

# 3. 정의된 아티팩트(파일, 패키지 등) 수집 및 준비
deck prepare

# 4. 현장 반입용 단일 번들 빌드
deck bundle build --out ./bundle.tar

# 5. 대상 장비에서 워크플로우 실행
deck apply
```

상세한 사용법은 [빠른 시작 가이드](docs/getting-started/quick-start.md)에서 확인하실 수 있습니다.

## 핵심 기능

- **타입 기반 워크플로우:** 모호한 셸 명령어 대신 파일, 패키지, 서비스 등 명확한 목적을 가진 '타입'으로 단계를 정의합니다. 이를 통해 작업 의도를 분명히 드러내고 리뷰 효율을 높일 수 있습니다.
- **철저한 사전 검증:** 데이터센터 현장에 진입하기 전, 워크플로우의 구조적 결함이나 설정 오류를 `lint` 기능을 통해 미리 파악하여 현장에서의 시행착오를 최소화합니다.
- **자기 완결형 번들 (Self-contained):** 워크플로우 정의서, 실행에 필요한 리소스, 그리고 `deck` 바이너리까지 하나의 아카이브로 패키징합니다. 현장에서 의존성 문제로 작업이 중단되는 상황을 방지합니다.
- **폐쇄망 환경 최적화:** SSH 접근이 제한되거나 인터넷 연결이 없는 환경, 로컬 운영자가 직접 개입해야 하는 특수한 운영 시나리오에 맞춰 설계되었습니다.

## 설치

**요구 사항:**
- **Go 1.25 이상** (모든 OS에서 빌드 및 `prepare` 실행 가능)
- **Linux 실행 환경** (RHEL, Ubuntu 계열 지원; `apply` 단계에서 필요)

```bash
# 바이너리 설치
go install github.com/taedi90/deck/cmd/deck@latest

# 설치 확인
deck version
```

### 셸 자동완성 설정

현재 세션에서 자동완성을 활성화하려면 다음 명령어를 실행하세요.

```bash
source <(deck completion bash) # bash 사용 시
source <(deck completion zsh)  # zsh 사용 시
deck completion fish | source  # fish 사용 시
```

영구적으로 적용하려면 위 명령어를 `~/.bashrc` 또는 `~/.zshrc` 파일 끝에 추가하세요.

## 상세 문서

- [시작하기](docs/getting-started/README.md)
- [핵심 개념](docs/core-concepts/README.md)
- [사용자 가이드](docs/user-guide/README.md)
- [레퍼런스](docs/reference/README.md)
- [기여하기](docs/contributing/README.md)

## 라이선스

Apache-2.0. 자세한 내용은 `LICENSE` 파일을 참고하세요.
