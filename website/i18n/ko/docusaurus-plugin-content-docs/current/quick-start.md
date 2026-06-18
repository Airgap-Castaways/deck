---
source: docs/quick-start.md
source_hash: acc02d4dea6bf8b93cd509442a52ab253c6919e0
---
# 빠른 시작

이 튜토리얼은 기본 `deck` 흐름을 처음부터 끝까지 따라갑니다.

1. 워크스페이스를 만듭니다
2. 절차를 워크플로로 표현합니다
3. 린트합니다
4. 번들을 빌드합니다
5. 로컬에서 실행합니다

## 1. 워크스페이스 만들기

```bash
deck init --out ./demo
```

이 명령은 진입 워크플로 두 개와 미리 준비된 출력 트리 골격으로 구성된 시작용 레이아웃을 만듭니다.

- `./demo/workflows/prepare.yaml`
- `./demo/workflows/scenarios/apply.yaml`
- `./demo/workflows/vars.yaml`
- `./demo/workflows/components/example-apply.yaml`
- `./demo/outputs/files/`
- `./demo/outputs/packages/`
- `./demo/outputs/images/`

## 2. 스텝 추가하거나 수정하기

`deck init`은 역할이 서로 다른 진입 워크플로 두 개를 만듭니다. `workflows/prepare.yaml`은 연결된 환경에서 실행되어 아티팩트를 가져오고, `workflows/scenarios/apply.yaml`은 대상 머신에서 실행되어 이를 적용합니다. 먼저 `apply.yaml`을 편집해 대상 노드가 수행할 작업을 정의하고, `prepare.yaml`을 편집해 번들에 내려받을 파일을 지정합니다. 여러 시나리오에서 함께 쓰는 재사용 가능한 조각은 `workflows/components/` 아래에 둡니다.

타입이 지정된 스텝을 권장합니다. 절차가 커질수록 읽고 린트하기가 더 쉬워집니다.

스텝을 고를 때는 [스텝 종류](step-kinds.md)부터 살펴봅니다. `when`, `parallelGroup`, `register`, `metadata`, `retry`, `timeout` 같은 공통 스텝 필드는 [스텝 엔벨로프 계약](workflow-model.md#step-envelope-contract)을 참고합니다.

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

사이트별 값은 `vars.yaml`이나 인라인 `vars`에 두어 스텝 정의와 분리합니다.

## 3. 패키징 전에 검증하기

```bash
deck lint
deck lint --workflow ./demo/workflows/scenarios/apply.yaml
```

`deck lint`은 워크플로 구조와 타입이 지정된 각 스텝의 스키마를 검사합니다. 실수는 에어갭(망분리) 안에서 발견할 때보다 이 단계에서 잡는 편이 비용이 적습니다.

## 4. 오프라인 번들 빌드하기

`workflows/prepare.yaml`이 있는 워크스페이스 디렉터리에서 `prepare`를 실행합니다. 이 단계에서 `workflows/vars.yaml`과 `workflows/scenarios/apply.yaml`은 선택 사항입니다.

```bash
cd ./demo
deck prepare
deck bundle build --out ./bundle.tar
```

`prepare`는 생성된 아티팩트를 `./demo/outputs/` 아래에 쓰고, 루트에 `./demo/deck` 런처를 작성하며, `./demo/.deck/manifest.json`을 갱신합니다. `bundle build`는 현재 워크스페이스를 사이트로 가져갈 아카이브로 묶습니다.

## 5. 대상 사이트에서 로컬로 적용하기

```bash
deck apply
```

`apply`는 변경이 필요한 머신에서 시나리오를 로컬로 실행합니다. `workflows/`가 있는 워크스페이스나 압축을 푼 번들 루트에서 실행합니다. SSH도, 컨트롤러도, 외부로 나가는 역방향 연결도 필요하지 않습니다.

성공하면 stderr에 단계와 스텝의 진행 상황이 표시되고 마지막에 완료 요약이 나옵니다. 깔끔하게 끝난 실행은 종료 코드 0으로 종료하며, 적용 상태를 `.deck/state/apply/` 아래에 저장합니다. 이 상태는 무엇이 실행되었는지 감사하거나, 실행이 중단된 경우 단계 경계에서 재개하는 데 유용합니다. 무언가 실패하면 `deck apply`는 0이 아닌 코드로 종료하고 실패한 스텝과 그 오류를 출력합니다. 실행이 예상대로 진행되지 않으면 [문제 해결](troubleshooting.md)을 참고합니다.

## 6. 선택 사항: 사이트 보조 수단 추가하기

일부 사이트에서는 에어갭(망분리) 안에 임시 로컬 서버를 두어 공유 번들 소스로 활용하면 도움이 됩니다. 실제 문제를 해결해야 할 때 `deck server up`을 사용합니다.

전형적인 패턴은 다음과 같습니다.

```bash
deck server up --root ./bundle --addr :8080
deck server up --root ./bundle --addr :8443 --tls-self-signed
deck server up --root ./bundle --addr :8080 --daemon --unit deck-server
```

TLS와 데몬 플래그는 [CLI 레퍼런스](cli.md)를, `.deck/logs/server-audit.log` 아래에 기록되는 현재 감사 레코드 형식은 [서버 감사 로그](server-audit-log.md)를 참고합니다.

이 경로는 로컬 워크플로를 대체하지 않고 확장할 뿐입니다.

## 다음에 읽을 내용

- [deck이 필요한 이유](core-concepts/why-deck.md)
- [워크플로 모델](workflow-model.md)
- [적용 상태](apply-state.md)
- [스텝 종류](step-kinds.md)
- [번들 레이아웃](bundle-layout.md)
- [CLI 레퍼런스](cli.md)
- [deck ask 사용하기](ask.md)
