---
source: docs/quick-start.md
source_hash: 70161f9cba42e6541586728a6ae0bff77e4cbd89
---
# 빠른 시작

이 튜토리얼은 기본 `deck` 경로를 따라 진행합니다.

1. 워크스페이스를 만듭니다
2. 절차를 워크플로로 표현합니다
3. 린트합니다
4. 번들을 빌드합니다
5. 로컬에서 실행합니다

## 1. 워크스페이스 만들기

```bash
deck init --out ./demo
```

이 명령은 진입 워크플로 두 개와 미리 준비된 출력 트리 뼈대를 갖춘 시작 레이아웃을 생성합니다.

- `./demo/workflows/prepare.yaml`
- `./demo/workflows/scenarios/apply.yaml`
- `./demo/workflows/vars.yaml`
- `./demo/workflows/components/example-apply.yaml`
- `./demo/outputs/files/`
- `./demo/outputs/packages/`
- `./demo/outputs/images/`

## 2. 스텝 추가 또는 편집

`deck init`은 역할이 서로 다른 진입 워크플로 두 개를 생성합니다. `workflows/prepare.yaml`은 연결된 환경에서 실행되어 아티팩트를 가져오고, `workflows/scenarios/apply.yaml`은 대상 머신에서 실행되어 그 아티팩트를 적용합니다. 먼저 대상 노드가 수행할 작업을 `apply.yaml`에 작성하고, 번들에 무엇을 내려받을지는 `prepare.yaml`에서 제어합니다. 여러 시나리오가 공유하는 재사용 가능한 조각은 `workflows/components/` 아래에 둡니다.

가능하면 타입이 지정된 스텝을 사용하십시오. 절차가 커질수록 읽고 린트하기가 더 쉬워집니다.

스텝을 고를 때는 [Step Kinds](step-kinds.md)부터 살펴보십시오. `when`, `parallelGroup`, `register`, `metadata`, `retry`, `timeout` 같은 공통 스텝 필드는 [Step Envelope Contract](workflow-model.md#step-envelope-contract)를 참고하십시오.

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

사이트별 값은 `vars.yaml`이나 인라인 `vars`에 두어 스텝 정의와 분리하십시오.

## 3. 패키징 전 검증

```bash
deck lint
deck lint --workflow ./demo/workflows/scenarios/apply.yaml
```

`deck lint`은 워크플로 구조와 타입이 지정된 각 스텝의 스키마를 검사합니다. 여기서 실수를 잡는 편이 에어갭(망분리) 안에서 발견하는 것보다 비용이 적습니다.

## 4. 오프라인 번들 빌드

`workflows/prepare.yaml`이 있는 워크스페이스 디렉터리에서 `prepare`를 실행하십시오. 이 단계에서 `workflows/vars.yaml`과 `workflows/scenarios/apply.yaml`은 선택 사항입니다.

```bash
cd ./demo
deck prepare
deck bundle build --out ./bundle.tar
```

`prepare`는 생성한 아티팩트를 `./demo/outputs/` 아래에 쓰고, 루트에 `./demo/deck` 런처를 만들며, `./demo/.deck/manifest.json`을 갱신합니다. `bundle build`는 현재 워크스페이스를 사이트로 가져갈 아카이브로 묶습니다.

## 5. 대상 사이트에서 로컬로 적용

```bash
deck apply
```

`apply`는 변경이 필요한 머신에서 시나리오를 로컬로 실행합니다. `workflows/`가 있는 워크스페이스나 압축을 푼 번들 루트에서 실행하십시오. SSH도, 컨트롤러도, 외부로 나가는 회신 연결도 필요하지 않습니다.

성공하면 stderr에 단계와 스텝 진행 상황이 나타나고 마지막에 완료 요약이 표시됩니다. 정상적으로 끝난 실행은 0으로 종료하며, 적용 상태를 `.deck/state/apply/` 아래에 저장합니다. 이 상태는 무엇이 실행됐는지 감사하거나, 실행이 중단됐을 때 단계 경계에서 재개하는 데 유용합니다. 무언가 실패하면 `deck apply`는 0이 아닌 값으로 종료하고 실패한 스텝과 그 오류를 출력합니다. 실행이 예상대로 진행되지 않으면 [Troubleshooting](troubleshooting.md)을 참고하십시오.

## 6. 선택 사항: 사이트 보조 추가

일부 사이트에서는 에어갭(망분리) 안에 공유 번들 소스로 임시 로컬 서버를 두는 것이 도움이 됩니다. 실제로 해결할 문제가 있을 때 `deck server up`을 사용하십시오.

일반적인 패턴은 다음과 같습니다.

```bash
deck server up --root ./bundle --addr :8080
deck server up --root ./bundle --addr :8443 --tls-self-signed
deck server up --root ./bundle --addr :8080 --daemon --unit deck-server
```

TLS와 데몬 플래그는 [CLI Reference](cli.md)를, `.deck/logs/server-audit.log` 아래에 기록되는 현재 감사 레코드 형태는 [Server Audit Log](server-audit-log.md)를 참고하십시오.

이 경로는 로컬 워크플로를 확장할 뿐 대체하지는 않습니다.

## 다음에 읽을 문서

- [Why deck?](core-concepts/why-deck.md)
- [Workflow model](workflow-model.md)
- [Apply State](apply-state.md)
- [Step Kinds](step-kinds.md)
- [Bundle layout](bundle-layout.md)
- [CLI Reference](cli.md)
- [Using deck ask](ask.md)