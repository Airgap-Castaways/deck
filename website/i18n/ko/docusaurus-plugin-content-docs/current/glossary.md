---
source: docs/glossary.md
source_hash: 0143acb68daaab4f0ef2b9a61ee6c2927aa9d540
---
# 용어집

deck 문서 전반에서 사용되는 용어를 알파벳순으로 정리한 참조 자료입니다. 각 항목은 권위 있는 출처로 연결됩니다.

---

**Air-gapped**
직접적인 인터넷 접속이 없어 모든 의존성을 사전에 반입해야 하는 배포 환경입니다. deck은 에어갭(망분리) 제약을 중심으로 설계되었습니다. `prepare`는 네트워크에 연결된 머신에서 모든 것을 가져오고, `bundle build`는 추가적인 외부 접근(reach-back) 없이 전송할 수 있는 아카이브로 패키징합니다. [bundle-layout.md](bundle-layout.md)를 참조하세요.

**Apply**
대상 머신에서 워크플로를 실행하는 명령이자 단계입니다. `deck apply`는 시나리오를 읽고, 번들 루트를 해석하고, 번들 매니페스트를 검증한 뒤 각 단계를 순서대로 실행합니다. 적용 상태가 추적되므로 중단된 실행을 재개할 수 있습니다. [apply-state.md](apply-state.md)와 [CLI 레퍼런스](cli.md)를 참조하세요.

**Bundle**
`deck bundle build`가 생성하는 자기 완결적 아카이브로, 워크플로 파일, 준비된 아티팩트(`outputs/`), deck 런처를 에어갭(망분리) 사이트로 반입합니다. 번들은 오프라인 인계의 단위입니다. [bundle-layout.md](bundle-layout.md)를 참조하세요.

**Bundle root**
`deck apply` 실행의 기준이 되는 디렉터리입니다. `workflows/` 트리와 `.deck/manifest.json`을 포함해야 합니다. 추출된 번들이나 준비된 워크스페이스에서 실행할 때, 번들 루트는 `--root`로 전달되거나 현재 디렉터리에서 해석되는 디렉터리입니다. [bundle-layout.md](bundle-layout.md)와 [workspace-layout.md](workspace-layout.md)를 참조하세요.

**Component fragment**
`workflows/components/` 아래에 있는 재사용 가능한 YAML 파일로, `steps:` 목록만 포함합니다. 프래그먼트는 `phases[].imports`를 통해 시나리오 단계로 가져옵니다. 최상위 `phases`나 `vars`를 포함할 수 없으며, 전체 워크플로 스키마가 아닌 컴포넌트 프래그먼트 스키마를 따릅니다. [workspace-layout.md](workspace-layout.md#component-fragment-contract)와 [workflow-model.md](workflow-model.md)를 참조하세요.

**Context (runtime context)**
CEL `when` 표현식과 Go 템플릿 양쪽에서 `context.*`를 통해 스텝에 제공되는 deck 실행 메타데이터 집합입니다. 필드에는 `context.command`, `context.workflow.source`, `context.workflow.path`, `context.workflow.scenario`, `context.paths.bundleRoot`, `context.paths.outputRoot`, `context.paths.stateFile`가 포함됩니다. 컨텍스트 필드는 적용 상태 키 지문(fingerprint)의 일부입니다. [workflow-model.md](workflow-model.md#execution-context-fields)를 참조하세요.

**deck**
오프라인 Kubernetes 및 배포 워크플로를 작성, 패키징, 실행하기 위한 CLI 도구입니다. `init` → `lint` → `prepare` → `bundle build` → `apply`로 이어지는 운영자 흐름 전체를 다룹니다. [quick-start.md](quick-start.md)와 [CLI 레퍼런스](cli.md)를 참조하세요.

**Manifest**
`deck prepare`가 작성하는 `.deck/manifest.json` 파일로, `outputs/`에 있는 모든 아티팩트의 SHA-256 다이제스트, 크기, 경로를 기록합니다. 매니페스트는 `deck bundle verify`와 적용 시작 시점의 검증 검사를 위한 무결성 기준선입니다. 매니페스트가 없거나 비어 있으면 `E_MANIFEST_MISSING` 또는 `E_MANIFEST_EMPTY`가 발생합니다. [bundle-layout.md](bundle-layout.md#apply-time-manifest-verification)와 [diagnostics/error-codes.md](diagnostics/error-codes.md)를 참조하세요.

**ParallelGroup**
한 단계 내에서 동시 실행이 허용되는 연속된 워크플로 스텝 집합에 공유되는 레이블입니다. 동일한 `parallelGroup` 값을 가진 스텝은 같은 단계 내에서 인접해 있어야 하며, 서로의 `register` 출력을 읽거나 동일한 대상 경로에 기록해서는 안 됩니다. 병렬 배치에서 등록된 출력은 전체 배치가 성공한 후에만 보이게 됩니다. [workflow-model.md](workflow-model.md#parallel-batches)와 [apply-state.md](apply-state.md#parallel-batches-inside-a-phase)를 참조하세요.

**Phase**
논리적으로 관련된 스텝을 묶는 워크플로의 명명된 섹션으로, `deck apply`의 재개 경계 역할을 합니다. 완료된 단계는 재개 시 건너뛰며, 실패한 단계는 첫 스텝부터 다시 실행됩니다. 워크플로는 구조화된 실행을 위해 명명된 `phases:`를 사용하거나, 평면적인 `steps:` 목록(이는 `default`라는 암시적 단계로 실행됨)을 사용할 수 있습니다. [workflow-model.md](workflow-model.md#phases)와 [apply-state.md](apply-state.md#phase-based-resume)를 참조하세요.

**Prepare**
네트워크에서 아티팩트를 가져오고, 컨테이너 기반 패키지를 빌드하며, 준비된 모든 출력을 `outputs/` 아래에 작성하는 명령이자 단계입니다. 네트워크에 연결된 머신에서 `deck prepare`를 실행하는 것이 번들을 아카이브하여 에어갭(망분리)으로 반입하기 전의 첫 단계입니다. [workflow-model.md](workflow-model.md#prepare-semantics), [bundle-layout.md](bundle-layout.md), [CLI 레퍼런스](cli.md)를 참조하세요.

**Register**
스텝의 선언된 출력 키를 런타임 변수 이름에 매핑하는 스텝 엔벨로프 필드입니다. `register:` 블록은 출력을 `runtime.<name>`으로 내보내어, 이후 스텝에서 CEL(`runtime.name`)이나 템플릿(`.runtime.name`)을 통해 사용할 수 있게 합니다. 출력을 명시적으로 선언하는 스텝 종류만 `register`를 지원합니다. [workflow-model.md](workflow-model.md#register--capture-step-output)를 참조하세요.

**Scenario**
`workflows/scenarios/` 아래에 있는 완전한 워크플로 파일로, `deck apply`의 진입점 역할을 합니다. 시나리오는 `version` 필드와 `phases` 또는 `steps` 중 하나를 가져야 합니다. 컴포넌트 프래그먼트를 가져올 수 있으며, 공유 기본값을 확장하거나 재정의하기 위해 자체 `vars:` 블록을 정의할 수 있습니다. 시나리오 이름은 `.yaml` 확장자를 제외한 파일명입니다. [workspace-layout.md](workspace-layout.md#scenarios-workflowsscenarios)와 [workflow-model.md](workflow-model.md)를 참조하세요.

**Source locator**
`plan`, `apply`, `state` 명령에 대해 워크플로 소스와 진입점을 식별하기 위해 사용되는 플래그 조합입니다. 두 가지 주요 소스 선택자는 `--root <path>`(로컬 워크플로 트리 또는 번들 루트)와 `--server <url>`(원격 워크플로 서버)이며, `--scenario <name>`은 선택된 소스 아래의 시나리오 파일을 선택합니다. `--workflow <path-or-url>`은 명시적인 파일 탈출구로 사용할 수 있습니다. [CLI 레퍼런스](cli.md#workflow-source-locators)를 참조하세요.

**State key**
deck이 저장된 적용 상태를 특정 워크플로 실행에 매칭하기 위해 사용하는 안정적인 식별자입니다. import가 확장된 후 해석된 워크플로 바이트, 실행에 적용되는 유효 vars, 적용 실행 컨텍스트 지문(fingerprint)에서 도출됩니다. 워크플로, vars, 컨텍스트를 변경하면 다른 상태 키가 생성되어 재개 대신 새로운 실행이 일어납니다. [apply-state.md](apply-state.md#what-identifies-saved-state)를 참조하세요.

**Step envelope**
종류별 `spec` 검증이 실행되기 전에 모든 워크플로 스텝에 존재하는 공유 외부 래퍼입니다. 필수 필드인 `id`, `kind`, `spec`과 함께 `apiVersion`, `when`, `parallelGroup`, `retry`, `timeout`, `register`, `metadata`를 포함한 선택적 공유 필드를 담습니다. [workflow-model.md](workflow-model.md#step-envelope-contract)를 참조하세요.

**Step kind**
스텝이 무엇을 하는지 식별하는 타입화된 이름(`kind:`)입니다. 각 스텝 종류는 자체 스키마와 허용되는 역할의 정의된 집합(prepare 전용, apply 전용, 또는 둘 다)을 가집니다. 예로는 `WriteFile`, `DownloadFile`, `InstallPackage`, `InitKubeadm`, `Command`가 있습니다. 전체 스텝 종류 목록은 [step-kinds.md](step-kinds.md)에서 단계 및 작업 그룹별로 정리되어 있습니다.

**Vars**
`workflows/vars.yaml`, 시나리오의 `vars:` 블록, 또는 커맨드라인에서 `--var`나 `-f`/`--vars-file`을 통해 공급되는 정적 입력 변수입니다. vars는 실행 전에 해석되며 적용 상태 키 지문(fingerprint)의 일부입니다. Go 템플릿에서는 `.vars.NAME`으로, CEL 표현식에서는 `vars.NAME`으로 사용할 수 있습니다. [workflow-model.md](workflow-model.md#variables)를 참조하세요.

**Workspace**
`deck init`이 생성하는 디렉터리 트리로, 워크플로 파일, 준비된 출력, deck 메타데이터를 담습니다. 세 가지 주요 영역은 `workflows/`(운영 로직), `outputs/`(준비된 아티팩트), `.deck/`(체크섬, 매니페스트, 상태)입니다. [workspace-layout.md](workspace-layout.md)를 참조하세요.

**XDG state root**
deck이 워크스페이스 외부에 존재하는 사용자 범위의 영속 상태를 위해 사용하는 기본 디렉터리입니다. XDG Base Directory Specification을 따릅니다: 기본값은 `$XDG_STATE_HOME/deck/` 또는 `~/.local/state/deck/`입니다. 원격 워크플로 적용 상태, 적용 실행 로그, 캐시, ask 세션 데이터는 워크스페이스가 아니라 여기에 저장됩니다. [apply-state.md](apply-state.md#where-state-is-stored)와 [apply-runlogs.md](apply-runlogs.md)를 참조하세요.
