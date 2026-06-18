---
source: docs/apply-state.md
source_hash: 5fbda5f57dd93041a2028c5d3962b52138a0d5d5
---
# 적용 상태

`deck apply`는 워크플로 `StateKey`에서 파생된 상태 파일에 진행 상황을 저장합니다.

## 상태가 저장되는 위치 {#where-state-is-stored}

로컬 워크플로 상태는 워크스페이스에 저장됩니다:

```text
<workspace>/.deck/state/apply/<state-key>.json
```

원격 워크플로 상태는 사용자 로컬 XDG state 루트에 저장됩니다:

```text
$XDG_STATE_HOME/deck/state/apply/<state-key>.json
~/.local/state/deck/state/apply/<state-key>.json
```

명시적인 디렉터리를 선택하려면 `--state-dir`를 사용하세요:

```bash
deck apply --state-dir /var/lib/deck/state/apply --server https://example.invalid --scenario apply
deck plan --state-dir /var/lib/deck/state/apply --server https://example.invalid --scenario apply
```

`--state-dir`가 지정되면 deck은 `<dir>/<state-key>.json`만 사용합니다. 워크스페이스 로컬 상태를 추론하거나, 이전 기본 경로를 읽거나, 자동 마이그레이션을 수행하지 않습니다.

`.deck/state/` 아래의 상태는 런타임 로컬 메타데이터입니다. 커밋해서는 안 되며 번들에도 포함되지 않습니다. 새로 클론한 저장소와 새로 추출한 번들은 상태 디렉터리를 복사하거나 명시적으로 지정하지 않는 한 저장된 적용 상태 없이 시작합니다.

원격 워크플로는 현재 디렉터리, 번들 루트, 또는 `/tmp/deck` 같은 고정된 임시 경로에 고정되지 않습니다. 이러한 위치는 예상치 못하거나, 공유되거나, 휘발성일 수 있습니다.

## 권한 및 사용자 범위

기본 상태는 사용자 범위(user-scoped)입니다.

- `deck apply`와 `sudo deck apply`는 원격 워크플로에 대해 서로 다른 기본 XDG 루트를 사용합니다.
- `sudo -E deck apply`는 사용자 XDG 변수를 보존하고 사용자 소유 상태 디렉터리에 root 소유 파일을 생성할 수 있습니다.
- 반복적인 권한 상승 원격 워크플로 실행은 `/var/lib/deck/state/apply` 같은 명시적인 시스템 상태 디렉터리를 사용해야 합니다.
- 권한 상승 명령과 비권한 명령 모두 상태를 검사해야 한다면, 명시적인 디렉터리 권한을 신중하게 선택하세요.

## 무엇이 저장된 상태를 식별하는가 {#what-identifies-saved-state}

상태 키는 세 가지의 지문(fingerprint)입니다: import가 확장된 후 해석된 워크플로 바이트, 실행에 대한 유효 vars, 그리고 적용 실행 컨텍스트. 즉, 워크플로 파일을 변경하거나, 다른 `--var` 오버라이드 또는 `-f` vars 파일을 제공하거나, 번들 루트를 변경하면 **새로운** 상태 키가 생성되며 — 이전 키 아래 저장된 이전 재개 진행 상황은 재개되지 않고 고아(orphan)가 됩니다. 이전 상태 파일은 삭제되거나 손상된 것으로 취급되지 않으며, deck이 다음에 다른 키를 계산할 때 단지 선택되지 않을 뿐입니다.

특정 호출이 어떤 상태 파일을 사용할지 확실하지 않다면, `deck apply`에 전달하려는 동일한 플래그로 `deck state show`를 실행하세요 — 아무것도 실행하지 않고 해석된 상태 키와 파일 경로를 출력합니다. 저장된 상태가 예상대로 선택되지 않는 경우 [troubleshooting.md](troubleshooting.md)를 참조하세요.

## 마이그레이션

기본 경로 실행은 새 대상 파일이 아직 존재하지 않을 때 기존 상태를 앞으로 마이그레이션합니다.

마이그레이션 소스:

```text
$XDG_STATE_HOME/deck/state/<state-key>.json
~/.local/state/deck/state/<state-key>.json
~/.deck/state/<state-key>.json
```

마이그레이션 대상:

```text
local workflow:  <workspace>/.deck/state/apply/<state-key>.json
remote workflow: $XDG_STATE_HOME/deck/state/apply/<state-key>.json
remote workflow: ~/.local/state/deck/state/apply/<state-key>.json
```

마이그레이션은 이전 파일을 삭제하는 대신 상태를 복사합니다. 동일한 키에 대해 이전 XDG 상태와 레거시 `~/.deck/state/`가 모두 존재하면 이전 XDG 상태가 우선합니다. 기존 대상 파일은 절대 덮어쓰지 않습니다.

`--v>=1`에서 deck은 마이그레이션을 `event=state_migrated source=<old> target=<new>`로 보고합니다.

## 단계 기반 재개 {#phase-based-resume}

이제 apply는 스텝 경계가 아니라 단계 경계에서 재개합니다.

- 완료된 단계는 이후 비-fresh 실행에서 건너뜁니다
- 실패한 단계는 다음 비-fresh 실행에서 첫 번째 스텝부터 다시 실행됩니다
- 실패한 단계 내부의 부분 진행 상황은 재사용되지 않습니다

## 무엇이 저장되는가

- 포맷 버전 및 종류
- 상태 키
- 가능한 경우 워크플로 경로, 소스, 해시
- 상태 및 현재 단계
- 완료된 단계 이름
- 실행이 중단될 때의 실패 단계 오류
- 완전히 완료된 단계가 내보낸 런타임 vars
- 비밀 값이 없는 런타임 비밀 메타데이터

새 상태 파일은 버전이 지정된 v2 JSON 형태를 사용합니다. 이전 v1 파일은 계속 읽을 수 있으며 내부적으로 정규화됩니다.

## 단계 내 병렬 배치 {#parallel-batches-inside-a-phase}

단계가 명시적인 `parallelGroup` 배치를 사용할 때:

- 동일한 배치의 스텝은 동일한 `runtime` 스냅샷에서 시작합니다
- 해당 배치의 `register` 출력은 전체 배치가 성공한 후에만 표시됩니다
- 배치 내 스텝 중 하나라도 실패하면 전체 단계는 미완료 상태로 남습니다

배치가 실행되기 전에 작성 및 검증 규칙이 여전히 적용됩니다:

- `parallelGroup` 값은 단계 내에서 연속적이어야 합니다
- 적용 시점 병렬 배치는 제한된 안전 종류 허용 목록(allowlist)만 지원합니다
- 동일 배치 스텝은 동일한 리터럴 경로를 대상으로 하거나, 동일한 준비된 출력 루트를 공유하거나, 서로의 `runtime.*` 출력을 소비할 수 없습니다

현재 배치 규칙 및 제약은 [Workflow Model](workflow-model.md#parallel-batches)을 사용하세요.

## `--fresh`

실행 전에 선택된 저장된 적용 상태를 지우려면 `deck apply --fresh`를 사용하세요.

- `deck apply --fresh`는 모든 단계를 다시 실행하고 정상 경로에 새 상태를 다시 씁니다.
- 선택된 상태 키만 지워집니다. 동일한 디렉터리의 다른 상태 파일은 보존됩니다.
- `deck apply --dry-run --fresh`는 `--fresh`가 상태를 지우기 때문에 거부됩니다.

`deck plan`은 읽기 전용이며 `--fresh`를 지원하지 않습니다. apply를 실행하지 않고 명시적인 상태 관리를 하려면 `deck state clear`를 사용하세요.

## 상태 관리

적용 상태를 검사하고 삭제하려면 `deck state`를 사용하세요:

```bash
deck state show --root . --scenario apply
deck state show --server https://example.invalid --scenario apply
deck state list
deck state clear --root . --scenario apply --yes
deck state clear --all --yes
```

`deck state show`는 `deck plan` 및 `deck apply`와 동일한 워크플로, vars, 상태 키 로직을 사용합니다. apply/plan이 명시적인 상태 디렉터리를 사용한 경우 `deck state`와 함께 `--state-dir`를 사용하세요.
