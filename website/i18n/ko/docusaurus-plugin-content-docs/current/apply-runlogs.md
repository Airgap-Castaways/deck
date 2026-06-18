---
source: docs/apply-runlogs.md
source_hash: e24818e7cf5e07b205de678a42f3df012ccce63f
---
# Apply 실행 로그

`deck apply`을 실행할 때마다 진단과 감사를 위한 실행 로그를 남깁니다. 실행 로그는 워크스페이스 바깥의 XDG 상태 루트 아래에 기록하며, 번들에는 절대 포함하지 않습니다.

## 위치

```text
$XDG_STATE_HOME/deck/runs/<run-id>/
~/.local/state/deck/runs/<run-id>/    (default when XDG_STATE_HOME is unset)
```

apply을 실행할 때마다 고유한 디렉터리를 하나씩 만듭니다. 실행 ID는 `20060102T150405.000000000Z` 형식의 UTC 타임스탬프(Go 기준 시각, 나노초 정밀도)이므로 디렉터리 이름이 시간순으로 정렬됩니다.

각 실행 디렉터리 안에는 다음 두 파일을 기록합니다:

```text
<run-id>/
├── record.json     # structured summary of the run
└── events.jsonl    # per-step event stream (one JSON object per line)
```

두 파일 모두 비공개 권한(모드 `0600`/`0700`)으로 생성합니다. 실행 도중 조금씩 이어서 기록하므로 중단된 apply에 대해서도 부분 기록이 남습니다.

## `record.json`

apply 실행 전체를 요약한 단일 JSON 객체입니다. 스텝 이벤트가 발생할 때마다 이 파일을 다시 쓰므로 항상 가장 최근에 파악한 상태를 반영합니다.

필드(`id`, `command`, `workflow_ref`을 제외하면 모두 `omitempty`):

| 필드 | 타입 | 설명 |
|---|---|---|
| `id` | string | 실행 ID. 디렉터리 이름과 일치합니다. |
| `command` | string | 항상 `"apply"`. |
| `workflow_ref` | string | `deck apply`에 전달한 워크플로 경로 또는 URL. |
| `workflow_source` | string | 추론한 소스: `"local"` 또는 `"server"`. |
| `scenario` | string | 지정한 경우 `--scenario` 값. |
| `bundle_root` | string | 지정한 경우 `--root` 값. |
| `selected_phase` | string | 지정한 경우 `--phase` 값. |
| `hostname` | string | apply 시작 시점의 머신 호스트명. |
| `started_at` | string | 실행을 시작한 RFC 3339 나노초 타임스탬프. |
| `ended_at` | string | 실행을 마친 RFC 3339 나노초 타임스탬프. |
| `status` | string | 최종 실행 상태. 예: `"running"`, `"success"`, `"failed"`. |
| `error` | string | 실행이 실패한 경우 최상위 에러 메시지. |
| `steps` | array | 스텝별 요약 레코드(아래 참고). |

`steps`의 각 항목은 다음 필드를 갖는 객체입니다:

| 필드 | 타입 | 설명 |
|---|---|---|
| `step_id` | string | 워크플로에 정의된 스텝 식별자. |
| `kind` | string | 스텝 종류. 예: `"WriteFile"`. |
| `phase` | string | 스텝이 속한 단계. |
| `status` | string | 마지막으로 관측한 스텝 상태. |
| `reason` | string | 상태가 `"skipped"` 등일 때의 짧은 사유 문자열. |
| `attempt` | int | 재시도 횟수. |
| `started_at` | string | RFC 3339 나노초 타임스탬프. |
| `ended_at` | string | RFC 3339 나노초 타임스탬프. |
| `error` | string | 스텝이 실패한 경우의 에러 메시지. |

간단한 예시:

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

줄 단위로 구분된 JSON 스트림입니다. 각 줄은 apply 엔진이 스텝의 시작·종료·건너뜀에 맞춰 내보내는 이벤트 하나입니다. `record.json`과 달리 이벤트는 절대 덮어쓰지 않습니다. 각 줄을 곧바로 append하고 fsync하므로, 타이밍과 순서를 파악하기에는 `events.jsonl`이 가장 믿을 만한 자료입니다.

각 이벤트 줄의 필드:

| 필드 | 타입 | 설명 |
|---|---|---|
| `ts` | string | 이벤트를 기록한 RFC 3339 나노초 타임스탬프. |
| `event` | string | 이벤트 유형 이름. |
| `step_id` | string | 스텝 식별자. |
| `kind` | string | 스텝 종류. |
| `phase` | string | 스텝이 속한 단계. |
| `status` | string | 이 이벤트 시점의 스텝 상태. |
| `reason` | string | 짧은 사유 문자열. |
| `attempt` | int | 재시도 횟수. |
| `started_at` | string | 스텝 시작 시각(RFC 3339 나노초). |
| `ended_at` | string | 스텝 종료 시각(RFC 3339 나노초). |
| `error` | string | 스텝이 실패한 경우의 에러 메시지. |
| `batch_id` | string | 스텝이 병렬 배치에 속할 때의 배치 식별자. |
| `parallel_group` | string | 워크플로의 `parallelGroup` 레이블. |
| `parallel` | bool | 스텝이 병렬로 실행되면 `true`. |
| `batch_size` | int | 배치에 포함된 스텝 수. |
| `max_parallelism` | int | 배치에 적용된 동시성 한도. |
| `failed_step` | string | 배치가 중단될 때 처음 실패한 스텝의 ID. |

예시 줄(병렬 배치에서 발생한 이벤트 두 개):

```jsonl
{"ts":"2024-03-15T12:00:01.000000000Z","event":"step_started","step_id":"pull-image","kind":"LoadImage","phase":"load","status":"running","batch_id":"b1","parallel_group":"images","parallel":true,"batch_size":3,"max_parallelism":3}
{"ts":"2024-03-15T12:00:03.500000000Z","event":"step_done","step_id":"pull-image","kind":"LoadImage","phase":"load","status":"done","started_at":"2024-03-15T12:00:01.000000000Z","ended_at":"2024-03-15T12:00:03.500000000Z","batch_id":"b1","parallel_group":"images","parallel":true}
```

## apply 상태와의 관계

실행 로그와 [apply 상태](apply-state.md)는 목적이 서로 다릅니다:

- **apply 상태**(`<workspace>/.deck/state/apply/<key>.json` 또는 `$XDG_STATE_HOME/deck/state/apply/<key>.json`)는 `deck apply`이 어떤 단계를 건너뛸지 판단할 때 참조하는 재개 가능한 체크포인트입니다. 워크플로 핑거프린트를 키로 삼아 제자리에서 갱신합니다.
- **실행 로그**는 append 전용 진단 기록입니다. apply을 실행할 때마다 실행 디렉터리를 정확히 하나씩 만듭니다. `deck apply` 자체는 실행 로그를 참조하지 않으며, 자동으로 정리하지도 않습니다.

실행 로그는 어떤 스텝이 어떤 순서로, 어떤 타이밍에 실행되었고 어떤 에러 메시지가 나왔는지 같은 질문에 답할 때 사용합니다. apply 상태는 특정 단계가 완료되었는지 확인하고 중단된 apply을 재개할 때 사용합니다.
