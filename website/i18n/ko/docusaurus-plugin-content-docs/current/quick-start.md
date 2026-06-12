---
source: docs/quick-start.md
source_hash: f2f6d8e88013f5f863d54d73488cf37a79357acf
---
# 빠른 시작

이 튜토리얼은 기본 `deck` 경로를 단계별로 안내합니다:

1. 워크스페이스 생성
2. 절차를 워크플로로 표현
3. 린트
4. 번들 빌드
5. 로컬에서 실행

## 1. 워크스페이스 생성

```bash
deck init --out ./demo
```

이 명령은 두 개의 진입 워크플로와 준비된 출력 트리 스캐폴드를 갖춘 시작 레이아웃을 생성합니다:

- `./demo/workflows/prepare.yaml`
- `./demo/workflows/scenarios/apply.yaml`
- `./demo/workflows/vars.yaml`
- `./demo/workflows/components/example-apply.yaml`
- `./demo/outputs/files/`
- `./demo/outputs/packages/`
- `./demo/outputs/images/`

## 2. 스텝 추가 또는 편집

`deck init`은 고정된 `workflows/prepare.yaml` 진입 워크플로, `workflows/scenarios/` 아래의 `apply.yaml` 시나리오, 그리고 `workflows/components/` 아래의 재사용 가능한 프래그먼트를 생성합니다. 준비 작업은 `workflows/prepare.yaml`에, 대상 머신에서 실행되는 작업은 `workflows/scenarios/apply.yaml`에 들어갑니다.

타입이 지정된 스텝을 우선 사용하세요. 절차가 커질수록 읽기와 린트가 더 쉬워집니다.

스텝을 선택할 때는 [Step Kinds](step-kinds.md)에서 시작하세요. `when`, `parallelGroup`, `register`, `metadata`, `retry`, `timeout` 같은 공통 스텝 필드는 [Step Envelope Contract](workflow-model.md#step-envelope-contract)를 참조하세요.

```yaml
version: v1alpha1
steps:
  - id: write-motd
    apiVersion: deck/v1alpha1
    kind: WriteFile
    spec:
      path: /etc/motd
      content: |
        deck maintenance session in progress
```

사이트별 값을 스텝 정의에서 분리하려면 `vars.yaml` 또는 인라인 `vars`를 사용하세요.

## 3. 패키징 전 검증

```bash
deck lint
deck lint --workflow ./demo/workflows/scenarios/apply.yaml
```

`deck lint`는 워크플로 구조와 각 타입 스텝의 스키마를 검사합니다. 여기서 실수를 잡는 것이 에어갭(망분리) 내부에서 발견하는 것보다 비용이 적게 듭니다.

## 4. 오프라인 번들 빌드

`workflows/prepare.yaml`을 포함하는 워크스페이스 디렉터리에서 `prepare`를 실행하세요. 이 단계에서 `workflows/vars.yaml`과 `workflows/scenarios/apply.yaml`은 선택 사항입니다.

```bash
cd ./demo
deck prepare
deck bundle build --out ./bundle.tar
```

`prepare`는 생성된 아티팩트를 `./demo/outputs/` 아래에 기록하고, 루트 `./demo/deck` 런처를 기록하며, `./demo/.deck/manifest.json`을 갱신합니다. `bundle build`는 현재 워크스페이스를 사이트로 가져갈 아카이브로 변환합니다.

## 5. 대상 사이트에서 로컬로 적용

```bash
deck apply
```

`apply`는 변경이 필요한 머신에서 워크플로를 로컬로 실행합니다. `workflows/`를 포함하는 워크스페이스 또는 압축 해제된 번들 루트에서 실행하세요. SSH, 컨트롤러, 외부 리치백(reach-back)이 필요 없습니다.

## 6. 선택 사항: 사이트 보조 추가

일부 사이트는 공유 번들 소스로서 에어갭(망분리) 내부의 임시 로컬 서버로부터 이점을 얻습니다. 실제 문제를 해결할 때 `deck server up`을 사용하세요.

일반적인 패턴:

```bash
deck server up --root ./bundle --addr :8080
deck server up --root ./bundle --addr :8443 --tls-self-signed
deck server up --root ./bundle --addr :8080 --daemon --unit deck-server
```

TLS 및 데몬 플래그는 [CLI Reference](cli.md)를, `.deck/logs/server-audit.log` 아래에 기록되는 현재 감사 레코드 형태는 [Server Audit Log](server-audit-log.md)를 참조하세요.

이 경로는 로컬 워크플로를 확장합니다. 대체하는 것이 아닙니다.

## 다음에 읽을 내용

- [Why deck?](core-concepts/why-deck.md)
- [Workflow model](workflow-model.md)
- [Apply State](apply-state.md)
- [Step Kinds](step-kinds.md)
- [Bundle layout](bundle-layout.md)
- [CLI Reference](cli.md)
- [Using deck ask](ask.md)
