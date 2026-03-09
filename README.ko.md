# deck

[English README](./README.md) | [문서 홈](./docs/README.md)

`deck`은 no SSH, no PXE, no BMC, 그리고 인터넷 연결 자체를 전제할 수 없는 극단적인 air-gapped 환경에서 manual-first maintenance session을 수행하기 위한 단일 바이너리 도구입니다.

운영자는 self-contained bundle을 준비해 사이트로 반입하고, 변경이 필요한 대상 장비에서 작업을 로컬로 직접 실행할 수 있습니다.

## Visuals

![deck terminal demo](docs/assets/deck-cli.gif)

## deck이 적합한 문제

- **수동 우선 운영**: 기본 경로는 대상 사이트에서 운영자가 직접 로컬 실행하는 방식입니다.
- **극단적 air-gap 환경 집중**: 연결된 제어 루프를 사용할 수 없거나 신뢰할 수 없는 환경을 전제로 합니다.
- **자가 포함 번들**: `bundle.tar`는 오프라인 작업에 필요한 워크플로, 아티팩트, `deck` 바이너리를 함께 담습니다.
- **YAML 기반 워크플로**: 더 큰 제어 시스템 없이도 운영자가 읽고, 검토하고, 조정할 수 있습니다.
- **명시적 site assistance**: 사이트 내부에서 임시 공유 서버나 조정 지점을 두고 싶다면 추가적으로 사용할 수 있지만, 그것이 기본 모드는 아닙니다.

## deck을 써야 할 때

- 아티팩트를 사전에 준비해서 승인된 경로로 반입한 뒤, 현장에서 로컬 실행해야 할 때
- `validate -> pack -> apply`처럼 작고 명확한 운영 흐름이 필요할 때
- 연결이 끊긴 호스트, 클러스터, 어플라이언스에 반복 가능한 유지보수 절차가 필요할 때
- 나중에 선택적으로 site-local assistance를 추가할 수는 있어도, 각 노드의 실행 모델은 계속 로컬 실행으로 유지하고 싶을 때

## deck을 쓰지 말아야 할 때

- 원격 오케스트레이션 플랫폼, 장기 실행 control plane, agent 기반 rollout 시스템이 필요할 때
- 항상 연결된 환경, 실시간 cloud API, SSH 기반 자동화가 기본 전제일 때
- Terraform, Pulumi, Ansible 같은 범용 인프라 플랫폼을 대체하려고 할 때

## Install

요구 사항:

- Go 1.22+
- Linux 타깃 환경

```bash
# 소스에서 바로 실행
go run ./cmd/deck --help

# 바이너리 설치
go install ./cmd/deck

# 확인
deck --help
```

## Quick Start

1. 시작용 작업 공간을 만듭니다.

```bash
deck init --out ./demo
```

2. `./demo/workflows/pack.yaml`, `./demo/workflows/apply.yaml`, `./demo/workflows/vars.yaml`을 유지보수 세션에 맞게 수정합니다.

3. 패키징이나 적용 전에 워크플로를 검증합니다.

```bash
deck validate --file ./demo/workflows/apply.yaml
```

4. 자체 포함형 오프라인 번들을 만듭니다.

```bash
deck pack --out ./bundle.tar
```

5. 번들을 오프라인 사이트로 반입한 뒤, 현장에서 로컬로 실행합니다.

```bash
deck apply
```

6. site-assisted execution은 사이트 내부에서 임시 공유 번들 소스나 로컬 coordination point가 정말 필요할 때만 추가합니다. 이 경로는 기본 로컬 흐름에 대한 보조적 선택지입니다.

단계별 가이드는 `docs/tutorials/quick-start.md`부터 시작하시면 됩니다.

## deck이 동작하는 방식

1. 유지보수 작업에 맞는 YAML 워크플로를 작성하거나 조정합니다.
2. `pack`이 패키지, 이미지, 파일, 워크플로, `deck` 바이너리를 번들에 모읍니다.
3. 승인된 오프라인 반입 경로를 통해 번들을 전달합니다.
4. `apply`는 SSH나 원격 실행 의존성 없이 대상 사이트에서 로컬로 실행됩니다.
5. 선택적인 site-assisted workflow는 임시 로컬 서버나 공유 가시성을 추가할 수 있지만, 운영자는 여전히 각 대상에서 `deck`을 직접 실행합니다.

## Workflow Model

워크플로 DSL은 YAML 기반이며 step 실행을 중심으로 구성됩니다. 워크플로는 `role`(`pack` 또는 `apply`)을 선언하고, top-level `steps` 또는 이름 있는 `phases`를 사용합니다.

일반적인 호스트 변경에는 typed primitive를 우선 사용하세요. 적절한 step kind가 없을 때만 `RunCommand`를 마지막 수단으로 남겨두는 것이 좋습니다.

```yaml
role: apply
version: v1alpha1
steps:
  - id: write-repo-config
    apiVersion: deck/v1alpha1
    kind: WriteFile
    spec:
      path: /etc/example.repo
      content: |
        [offline-base]
        name=offline-base
        baseurl=file:///srv/offline-repo
        enabled=1
        gpgcheck=0
```

공통 step 기능:

- `when` 조건 실행
- `retry`, `timeout` 제어
- step 출력값을 다음 step에 전달하는 `register`
- 워크플로 및 각 지원 step kind에 대한 JSON Schema 검증

## Bundle Contract

설계상 준비된 번들에는 다음이 포함될 수 있습니다.

- 오프라인 실행 입력을 담는 `workflows/`
- 수집된 아티팩트를 담는 `packages/`, `images/`, `files/`
- 자체 실행을 위한 `deck` 및 `files/deck`
- 아티팩트 체크섬 메타데이터를 담는 `.deck/manifest.json`

## Command Surface

- 핵심 로컬 흐름: `init`, `validate`, `pack`, `apply`
- 로컬 계획 및 진단: `diff`, `doctor`
- 선택적 site-local helper: `serve`, `list`, `health`, `logs`
- 번들 수명주기 및 캐시 관리: `bundle`, `cache`
- 호환성과 고급 step은 계속 지원되지만, 새 워크플로에서는 `RunCommand`를 마지막 수단으로 두는 것이 좋습니다.

## Documentation Map

- 문서 홈: `docs/README.md`
- Quick start 튜토리얼: `docs/tutorials/quick-start.md`
- Offline Kubernetes 튜토리얼: `docs/tutorials/offline-kubernetes.md`
- CLI 레퍼런스: `docs/reference/cli.md`
- 워크플로 모델: `docs/reference/workflow-model.md`
- 번들 구조: `docs/reference/bundle-layout.md`
- 스키마 레퍼런스: `docs/reference/schema-reference.md`
- 서버 감사 로그 레퍼런스: `docs/reference/server-audit-log.md`
- 예제 워크플로: `docs/examples/README.md`
- 원본 JSON Schema: `docs/schemas/README.md`

## Scope and Non-goals

- `deck`은 air-gapped 준비, 패키징, 오프라인 설치, 결정적 로컬 실행에 집중합니다.
- site-assisted 사용은 명시적이고 부가적인 선택지입니다. 로컬 운영자 경로를 대체하지 않습니다.
- `deck`은 원격 오케스트레이션 프레임워크가 아닙니다.
- 항상 연결된 cloud-first 워크플로를 최적화 대상으로 삼지 않습니다.

## Contributing and Validation

변경 전에는 작업에 맞는 검증을 실행합니다.

```bash
go test ./...
go run ./cmd/deck validate --file <workflow.yaml>
```

## License

Apache-2.0 라이선스를 따릅니다. 자세한 내용은 `LICENSE`를 참고하세요.
