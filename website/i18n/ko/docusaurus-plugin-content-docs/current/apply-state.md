---
source: docs/apply-state.md
source_hash: 2c8f3738a05c03fc44693d1ff08e7c6eb40c0159
---
# 적용 상태(Apply state)

`deck apply`은 워크플로의 `StateKey`에서 파생한 상태 파일에 진행 상황을 저장합니다.

## 상태가 저장되는 위치 {#where-state-is-stored}

로컬 워크플로의 상태는 워크스페이스에 저장됩니다.

```text
<workspace>/.deck/state/apply/<state-key>.json
```

원격 워크플로의 상태는 사용자의 로컬 XDG 상태 루트에 저장됩니다.

```text
$XDG_STATE_HOME/deck/state/apply/<state-key>.json
~/.local/state/deck/state/apply/<state-key>.json
```

디렉터리를 직접 지정하려면 `--state-dir`을 사용합니다.

```bash
deck apply --state-dir /var/lib/deck/state/apply --server https://example.invalid --scenario apply
deck plan --state-dir /var/lib/deck/state/apply --server https://example.invalid --scenario apply
```

`--state-dir`을 지정하면 deck은 `<dir>/<state-key>.json`만 사용합니다. 워크스페이스 로컬 상태를 추론하지 않고, 예전 기본 경로를 읽지 않으며, 자동 마이그레이션도 실행하지 않습니다.

`.deck/state/` 아래의 상태는 런타임이 만드는 로컬 메타데이터입니다. 커밋해서는 안 되며 번들에도 포함되지 않습니다. 새로 클론한 저장소나 새로 추출한 번들은 상태 디렉터리를 복사하거나 직접 지정하지 않는 한 저장된 적용 상태 없이 시작합니다.

원격 워크플로는 현재 디렉터리, 번들 루트, `/tmp/deck` 같은 고정 임시 경로에 묶이지 않습니다. 이런 위치는 예기치 않거나, 공유되거나, 휘발성일 수 있기 때문입니다.

## 사용자 권한과 범위

상태는 기본적으로 사용자 범위(user-scoped)로 관리됩니다.

- `deck apply`과 `sudo deck apply`은 원격 워크플로에 대해 서로 다른 기본 XDG 루트를 사용합니다.
- `sudo -E deck apply`은 사용자 XDG 변수를 보존하므로, 사용자 소유의 상태 디렉터리 안에 root 소유 파일이 생길 수 있습니다.
- 권한을 높인 원격 워크플로를 반복해서 실행할 때는 `/var/lib/deck/state/apply` 같은 시스템 상태 디렉터리를 명시적으로 사용해야 합니다.
- 권한을 높인 명령과 그렇지 않은 명령이 모두 상태를 확인해야 한다면, 디렉터리 권한을 의도적으로 설정하십시오.

## 저장된 상태를 식별하는 기준 {#what-identifies-saved-state}

상태 키는 세 가지 지문(fingerprint)으로 이루어집니다. import를 확장한 뒤 해석된 워크플로 바이트, 해당 실행에 적용된 유효 vars, 그리고 적용 실행 컨텍스트입니다. 따라서 워크플로 파일을 바꾸거나, `--var` 오버라이드나 `-f` vars 파일을 다르게 지정하거나, 번들 루트를 바꾸면 **새로운** 상태 키가 만들어집니다. 이렇게 되면 이전 키 아래에 저장된 재개 진행 상황은 재개되지 않고 고아 상태가 됩니다. 예전 상태 파일은 삭제되거나 손상된 것으로 취급되지 않으며, 다만 다음에 deck이 다른 키를 계산할 때 선택되지 않을 뿐입니다.

특정 실행이 어떤 상태 파일을 사용할지 확실하지 않다면, `deck apply`에 넘길 플래그를 그대로 붙여 `deck state show`를 실행하십시오. 아무것도 실행하지 않고 해석된 상태 키와 파일 경로만 출력합니다. 저장된 상태가 예상대로 선택되지 않는다면 [troubleshooting.md](troubleshooting.md)를 참고하십시오.

## 마이그레이션

기본 경로로 실행하면, 새 대상 파일이 아직 없을 때 기존 상태를 앞으로 마이그레이션합니다.

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

마이그레이션은 상태를 복사할 뿐 예전 파일을 삭제하지 않습니다. 같은 키에 대해 예전 XDG 상태와 레거시 `~/.deck/state/`가 모두 존재하면 예전 XDG 상태를 우선합니다. 기존 대상 파일은 절대 덮어쓰지 않습니다.

`--v>=1`에서 deck은 마이그레이션을 `event=state_migrated source=<old> target=<new>` 형식으로 보고합니다.

## 단계 기반 재개 {#phase-based-resume}

이제 적용은 스텝 경계가 아니라 단계 경계에서 재개합니다.

- 완료된 단계는 이후 fresh가 아닌 실행에서 건너뜁니다
- 실패한 단계는 다음 fresh가 아닌 실행에서 첫 스텝부터 다시 실행합니다
- 실패한 단계 안에서의 부분 진행 상황은 재사용하지 않습니다

## 저장되는 내용

- 포맷 버전과 종류(kind)
- 상태 키
- 사용할 수 있는 경우 워크플로 경로, 소스, 해시
- 상태와 현재 단계
- 완료된 단계 이름
- 실행이 멈췄을 때의 실패 단계 오류
- 완전히 완료된 단계가 내보낸 런타임 vars
- 비밀 값을 제외한 런타임 시크릿 메타데이터

새 상태 파일은 버전이 지정된 v2 JSON 형태를 사용합니다. 예전 v1 파일도 계속 읽을 수 있으며 내부적으로 정규화됩니다.

## 단계 내부의 병렬 배치 {#parallel-batches-inside-a-phase}

단계가 명시적인 `parallelGroup` 배치를 사용할 때는 다음이 적용됩니다.

- 같은 배치의 스텝은 동일한 `runtime` 스냅샷에서 시작합니다
- 해당 배치의 `register` 출력은 배치 전체가 성공한 뒤에야 보입니다
- 배치 안의 스텝이 하나라도 실패하면 단계 전체가 미완료로 남습니다

배치를 실행하기 전에 작성 규칙과 검증 규칙도 그대로 적용됩니다.

- `parallelGroup` 값은 단계 안에서 연속적이어야 합니다
- 적용 시점의 병렬 배치는 제한된 안전 종류 허용 목록만 지원합니다
- 같은 배치의 스텝은 동일한 리터럴 경로를 대상으로 삼거나, 동일한 준비된 출력 루트를 공유하거나, 서로의 `runtime.*` 출력을 소비할 수 없습니다

현재의 배치 규칙과 제약은 [워크플로 모델](workflow-model.md#parallel-batches)을 참고하십시오.

## `--fresh`

실행 전에 선택된 저장 적용 상태를 지우려면 `deck apply --fresh`를 사용합니다.

- `deck apply --fresh`은 모든 단계를 다시 실행하고 정상 경로에 새 상태를 다시 씁니다.
- 선택된 상태 키만 지워지며, 같은 디렉터리의 다른 상태 파일은 보존됩니다.
- `deck apply --dry-run --fresh`은 `--fresh`가 상태를 지우므로 거부됩니다.

`deck plan`은 읽기 전용이며 `--fresh`를 지원하지 않습니다. 적용을 실행하지 않고 상태를 명시적으로 관리하려면 `deck state clear`를 사용하십시오.

## 상태 관리

적용 상태를 확인하고 삭제하려면 `deck state`를 사용합니다.

```bash
deck state show --root . --scenario apply
deck state show --server https://example.invalid --scenario apply
deck state list
deck state clear --root . --scenario apply --yes
deck state clear --all --yes
```

`deck state show`은 `deck plan` 및 `deck apply`과 동일한 워크플로, vars, 상태 키 로직을 사용합니다. 적용이나 계획에서 명시적 상태 디렉터리를 사용했다면 `deck state`에도 `--state-dir`을 함께 지정하십시오.
