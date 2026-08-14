---
source: docs/apply-runlogs.md
source_hash: af18193baf5519091a90989c13ae34f2d4c0743e
---
# 적용 실행 로그

`deck apply`를 실행할 때마다 진단과 감사를 위한 실행 로그를 남깁니다. 실행 로그는 워크스페이스 바깥의 XDG 상태 루트 아래에 쌓이며, 번들에는 포함되지 않습니다.

## 위치

```text
$XDG_STATE_HOME/deck/runs/<run-id>/
~/.local/state/deck/runs/<run-id>/    (default when XDG_STATE_HOME is unset)
```

적용을 실행할 때마다 고유한 디렉터리를 하나씩 만듭니다. 실행 ID는 `20060102T150405.000000000Z` 형식(Go 기준 시각, 나노초 정밀도)의 UTC 타임스탬프라서 디렉터리 이름이 자연히 시간순으로 정렬됩니다.

실행 디렉터리마다 그 안에 파일 두 개를 기록합니다.

```text
<run-id>/
├── record.json     # structured summary of the run
└── events.jsonl    # per-step event stream (one JSON object per line)
```

두 파일 모두 소유자 전용 권한(모드 `0600`/`0700`)으로 만듭니다. 실행 도중 점진적으로 기록하므로 중간에 멈춘 적용이라도 부분 기록이 남습니다.

## `record.json`

적용 실행 전체를 요약한 단일 JSON 객체입니다. 스텝 이벤트가 발생할 때마다 파일을 다시 쓰기 때문에 언제 보아도 최신 상태를 반영합니다.

필드는 다음과 같습니다(`id`, `command`, `workflow_ref`을 뺀 나머지는 모두 `omitempty`).

| Field | Type | Description |
|---|---|---|
| `id` | string | 실행 ID이며 디렉터리 이름과 같습니다. |
| `command` | string | 항상 `"apply"`입니다. |
| `workflow_ref` | string | `deck apply`에 넘긴 워크플로 경로 또는 URL입니다. |
| `workflow_source` | string | 추론된 소스로 `"local"` 또는 `"server"`입니다. |
| `scenario` | string | `--scenario` 값(지정한 경우). |
| `bundle_root` | string | `--root` 값(지정한 경우). |
| `selected_phase` | string | `--phase` 값(지정한 경우). |
| `hostname` | string | 적용을 시작한 시점의 머신 호스트명입니다. |
| `started_at` | string | 실행이 시작된 RFC 3339 나노초 타임스탬프입니다. |
| `ended_at` | string | 실행이 끝난 RFC 3339 나노초 타임스탬프입니다. |
| `status` | string | 최종 실행 상태입니다. 예: `"running"`, `"success"`, `"failed"`. |
| `error` | string | 실행이 실패했을 때의 최상위 오류 메시지입니다. |
| `steps` | array | 스텝별 요약 레코드입니다(아래 참조). |

`steps`의 각 항목은 다음 필드를 가진 객체입니다.

| Field | Type | Description |
|---|---|---|
| `step_id` | string | 워크플로 안의 스텝 식별자입니다. |
| `kind` | string | 스텝 종류입니다. 예: `"WriteFile"`. |
| `phase` | string | 스텝이 속한 단계입니다. |
| `status` | string | 마지막으로 관측한 스텝 상태입니다. |
| `reason` | string | 상태가 `"skipped"` 등일 때의 짧은 사유 문자열입니다. |
| `attempt` | int | 재시도 횟수입니다. |
| `started_at` | string | RFC 3339 나노초 타임스탬프입니다. |
| `ended_at` | string | RFC 3339 나노초 타임스탬프입니다. |
| `error` | string | 스텝이 실패했을 때의 오류 메시지입니다. |

짧은 예시입니다.

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

줄바꿈으로 구분한 JSON 스트림입니다. 각 줄은 적용 엔진이 스텝을 시작하거나 끝내거나 건너뛸 때 내보내는 이벤트 하나입니다. `record.json`과 달리 이벤트는 절대 다시 쓰지 않고, 각 줄을 덧붙인 뒤 곧바로 fsync합니다. 그래서 타이밍과 순서 데이터는 `events.jsonl`이 가장 믿을 만한 출처입니다.

각 이벤트 줄의 필드는 다음과 같습니다.

| Field | Type | Description |
|---|---|---|
| `ts` | string | 이벤트를 기록한 RFC 3339 나노초 타임스탬프입니다. |
| `event` | string | 이벤트 유형 이름입니다. |
| `step_id` | string | 스텝 식별자입니다. |
| `kind` | string | 스텝 종류입니다. |
| `phase` | string | 스텝이 속한 단계입니다. |
| `status` | string | 이 이벤트 시점의 스텝 상태입니다. |
| `reason` | string | 짧은 사유 문자열입니다. |
| `attempt` | int | 재시도 횟수입니다. |
| `started_at` | string | 스텝 시작 시각(RFC 3339 나노초)입니다. |
| `ended_at` | string | 스텝 종료 시각(RFC 3339 나노초)입니다. |
| `error` | string | 스텝이 실패했을 때의 오류 메시지입니다. |
| `batch_id` | string | 스텝이 병렬 배치에 속할 때의 배치 식별자입니다. |
| `parallel_group` | string | 워크플로의 `parallelGroup` 라벨입니다. |
| `parallel` | bool | 스텝이 병렬로 실행되면 `true`입니다. |
| `batch_size` | int | 배치에 포함된 스텝 수입니다. |
| `max_parallelism` | int | 배치에 적용한 동시성 한도입니다. |
| `failed_step` | string | 배치가 중단될 때 처음 실패한 스텝의 ID입니다. |

다음은 예시 줄입니다(병렬 배치에서 나온 이벤트 두 개).

```jsonl
{"ts":"2024-03-15T12:00:01.000000000Z","event":"step_started","step_id":"pull-image","kind":"LoadImage","phase":"load","status":"running","batch_id":"b1","parallel_group":"images","parallel":true,"batch_size":3,"max_parallelism":3}
{"ts":"2024-03-15T12:00:03.500000000Z","event":"step_done","step_id":"pull-image","kind":"LoadImage","phase":"load","status":"done","started_at":"2024-03-15T12:00:01.000000000Z","ended_at":"2024-03-15T12:00:03.500000000Z","batch_id":"b1","parallel_group":"images","parallel":true}
```

## 적용 상태와의 관계

실행 로그와 [적용 상태](apply-state.md)는 목적이 서로 다릅니다.

- **적용 상태**(`<workspace>/.deck/state/apply/<key>.json` 또는 `$XDG_STATE_HOME/deck/state/apply/<key>.json`)는 `deck apply`이 어떤 단계를 건너뛸지 정할 때 참조하는 재개 가능한 체크포인트입니다. 워크플로 지문을 키로 삼아 제자리에서 갱신합니다.
- **실행 로그**는 덧붙이기 전용 진단 이력입니다. 적용을 실행할 때마다 실행 디렉터리가 정확히 하나씩 생깁니다. `deck apply` 자체는 실행 로그를 참조하지 않으며, 자동으로 정리하지도 않습니다.

실행 로그는 어떤 스텝이 어떤 순서로 어떤 타이밍에 실행되었는지, 어떤 오류 메시지가 나왔는지 같은 질문에 답할 때 씁니다. 적용 상태는 어떤 단계가 완료되었는지 판단하고 중단된 적용을 재개할 때 씁니다.
