---
source: docs/apply-runlogs.md
source_hash: 9cbed2a8b352d85dcab6499185812b3a3c3d12f2
---
# 적용 실행 로그

모든 `deck apply` 호출은 진단 및 감사 목적으로 실행 로그를 기록합니다. 실행 로그는 워크스페이스 외부의 XDG state 루트 아래에 기록되며 번들에는 절대 포함되지 않습니다.

## 위치

```text
$XDG_STATE_HOME/deck/runs/<run-id>/
~/.local/state/deck/runs/<run-id>/    (default when XDG_STATE_HOME is unset)
```

각 apply 호출은 자체 디렉터리를 갖습니다. 실행 ID는 `20060102T150405.000000000Z` 형식(Go 기준 시간, 나노초 정밀도)으로 포맷된 UTC 타임스탬프이므로, 디렉터리 이름은 시간순으로 정렬됩니다.

각 실행 디렉터리 안에는 두 개의 파일이 기록됩니다:

```text
<run-id>/
├── record.json     # structured summary of the run
└── events.jsonl    # per-step event stream (one JSON object per line)
```

두 파일 모두 비공개 권한(모드 `0600`/`0700`)으로 생성됩니다. 실행 중에 점진적으로 기록되므로, 중단된 apply에 대해서도 부분 레코드가 존재합니다.

## `record.json`

전체 apply 호출을 요약하는 단일 JSON 객체입니다. 이 파일은 매 스텝 이벤트 이후 다시 작성되므로, 항상 알려진 최신 상태를 반영합니다.

필드(`id`, `command`, `workflow_ref`을 제외하고 모두 `omitempty`):

| 필드 | 타입 | 설명 |
|---|---|---|
| `id` | string | 실행 ID; 디렉터리 이름과 일치. |
| `command` | string | 항상 `"apply"`. |
| `workflow_ref` | string | `deck apply`에 전달된 워크플로 경로 또는 URL. |
| `workflow_source` | string | 추론된 소스: `"local"` 또는 `"server"`. |
| `scenario` | string | `--scenario` 값(제공된 경우). |
| `bundle_root` | string | `--root` 값(제공된 경우). |
| `selected_phase` | string | `--phase` 값(제공된 경우). |
| `hostname` | string | apply 시작 시점의 머신 호스트명. |
| `started_at` | string | 실행이 시작된 RFC 3339 나노초 타임스탬프. |
| `ended_at` | string | 실행이 끝난 RFC 3339 나노초 타임스탬프. |
| `status` | string | 최종 실행 상태(예: `"running"`, `"success"`, `"failed"`). |
| `error` | string | 실행이 실패한 경우 최상위 에러 메시지. |
| `steps` | array | 스텝별 요약 레코드(아래 참조). |

`steps`의 각 항목은 다음 필드를 갖는 객체입니다:

| 필드 | 타입 | 설명 |
|---|---|---|
| `step_id` | string | 워크플로의 스텝 식별자. |
| `kind` | string | 스텝 종류(예: `"WriteFile"`). |
| `phase` | string | 스텝이 속한 단계. |
| `status` | string | 마지막으로 관측된 스텝 상태. |
| `reason` | string | 상태가 `"skipped"` 또는 유사한 경우의 짧은 사유 문자열. |
| `attempt` | int | 재시도 횟수. |
| `started_at` | string | RFC 3339 나노초 타임스탬프. |
| `ended_at` | string | RFC 3339 나노초 타임스탬프. |
| `error` | string | 스텝이 실패한 경우의 에러 메시지. |

짧은 예시:

```json
{
  "id": "20240315T120000.000000000Z",
  "command": "apply",
  "workflow_ref": "workflows/scenarios/apply.yaml",
  "workflow_source": "local",
  "hostname": "node-01",
  "started_at": "2024-03-15T12:00:00.000000000Z",
  "ended_at": "2024-03-15T12:00:05.123456789Z",
  "status": "success",
  "steps": [
    {
      "step_id": "write-config",
      "kind": "WriteFile",
      "phase": "configure",
      "status": "done",
      "started_at": "2024-03-15T12:00:01.000000000Z",
      "ended_at": "2024-03-15T12:00:01.500000000Z"
    }
  ]
}
```

## `events.jsonl`

개행으로 구분된 JSON 스트림입니다. 각 줄은 스텝이 시작, 완료, 또는 건너뛰어질 때 apply 엔진이 방출하는 하나의 이벤트입니다. `record.json`과 달리 이벤트는 절대 다시 작성되지 않으며, 각 줄은 즉시 append되고 fsync되므로 `events.jsonl`은 타이밍 및 순서 데이터에 대해 가장 신뢰할 수 있는 소스입니다.

각 이벤트 줄의 필드:

| 필드 | 타입 | 설명 |
|---|---|---|
| `ts` | string | 이벤트가 기록된 RFC 3339 나노초 타임스탬프. |
| `event` | string | 이벤트 타입 이름. |
| `step_id` | string | 스텝 식별자. |
| `kind` | string | 스텝 종류. |
| `phase` | string | 스텝이 속한 단계. |
| `status` | string | 이 이벤트 시점의 스텝 상태. |
| `reason` | string | 짧은 사유 문자열. |
| `attempt` | int | 재시도 횟수. |
| `started_at` | string | 스텝 시작 시간(RFC 3339 나노초). |
| `ended_at` | string | 스텝 종료 시간(RFC 3339 나노초). |
| `error` | string | 스텝이 실패한 경우의 에러 메시지. |
| `batch_id` | string | 스텝이 병렬 배치의 일부일 때의 배치 식별자. |
| `parallel_group` | string | 워크플로의 `parallelGroup` 레이블. |
| `parallel` | bool | 스텝이 병렬로 실행된 경우 `true`. |
| `batch_size` | int | 배치 내 스텝 수. |
| `max_parallelism` | int | 배치에 적용된 동시성 제한. |
| `failed_step` | string | 배치가 중단될 때 첫 번째로 실패한 스텝의 ID. |

예시 줄(병렬 배치의 두 이벤트):

```jsonl
{"ts":"2024-03-15T12:00:01.000000000Z","event":"step_started","step_id":"pull-image","kind":"LoadImage","phase":"load","status":"running","batch_id":"b1","parallel_group":"images","parallel":true,"batch_size":3,"max_parallelism":3}
{"ts":"2024-03-15T12:00:03.500000000Z","event":"step_done","step_id":"pull-image","kind":"LoadImage","phase":"load","status":"done","started_at":"2024-03-15T12:00:01.000000000Z","ended_at":"2024-03-15T12:00:03.500000000Z","batch_id":"b1","parallel_group":"images","parallel":true}
```

## 적용 상태와의 관계

실행 로그와 [적용 상태](apply-state.md)는 서로 다른 목적을 갖습니다:

- **적용 상태**(`<workspace>/.deck/state/apply/<key>.json` 또는 `$XDG_STATE_HOME/deck/state/apply/<key>.json`)는 `deck apply`가 어느 단계를 건너뛸지 결정할 때 참조하는 재개 가능한 체크포인트입니다. 워크플로 핑거프린트로 키가 지정되며, 제자리에서 갱신됩니다.
- **실행 로그**는 append 전용 진단 기록입니다. 각 apply 호출은 정확히 하나의 실행 디렉터리를 생성합니다. 실행 로그는 `deck apply` 자체에서 참조하지 않으며, 자동으로 정리되지 않습니다.

실행 로그는 어떤 스텝이 어떤 순서로, 어떤 타이밍으로 실행되었고, 어떤 에러 메시지가 발생했는지와 같은 질문에 답하는 데 사용하세요. 적용 상태는 단계가 완료되었는지 여부를 확인하고 중단된 apply를 재개하는 데 사용하세요.
