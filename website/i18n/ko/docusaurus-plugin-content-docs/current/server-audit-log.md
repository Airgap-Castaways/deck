---
source: docs/server-audit-log.md
source_hash: 5e01e9b395b71b006683b1373b3ed35076101859
---
# 서버 감사 로그

`deck server up`은 번들 루트 아래의 JSONL 로그 파일에 감사 레코드를 기록합니다.

## 위치

기본 파일 위치:

```text
<root>/.deck/logs/server-audit.log
```

## 현재 생성되는 레코드 형태

`deck server up`은 현재 구조화된 최상위 필드를 갖는 감사 스키마 버전 `2` 레코드를 생성합니다.

공통 필드:

- `ts`: UTC 기준 RFC3339Nano 타임스탬프
- `schema_version`: 현재 `2`
- `component`: 로그 생산자, 현재 `server`
- `event`: `request`와 같은 정규화된 이벤트 이름
- `level`: `info`, `warn`, 또는 `error`
- `message`: 짧은 사람이 읽을 수 있는 설명

요청 레코드에는 다음과 같은 최상위 요청 속성도 포함됩니다:

- `method`
- `path`
- `proto`
- `status`
- `bytes`
- `remote_addr`
- `duration_ms`

## 일반적인 예시

- 사이트 API, 레지스트리, 정적 파일, 헬스 체크를 포함한 라우팅된 서버 응답에 대해 레코드가 기록됩니다
- 현재 라이터는 요청 속성을 중첩된 `extra` 객체 아래가 아닌 최상위에 유지합니다

요청 레코드 예시:

```json
{"ts":"2026-04-03T05:00:00Z","schema_version":2,"component":"server","event":"request","level":"info","message":"http request handled","method":"GET","path":"/healthz","proto":"HTTP/1.1","status":200,"bytes":0,"remote_addr":"127.0.0.1:53422","duration_ms":1}
```

## 호환성 참고

- 현재 `deck server up`은 위의 구조화된 버전 2 형태를 기록합니다
- `deck server logs`는 `source`, `event_type`, 또는 중첩된 `extra`와 같은 필드를 사용했던 이전 레거시 레코드도 여전히 정규화할 수 있습니다
- 원시 감사 파일을 읽는 다운스트림 소비자는 앞으로 버전 2 최상위 구조를 기대해야 합니다

## 로테이션

- `deck server up`은 감사 로그가 구성된 크기 제한을 초과하면 로테이션합니다
- 기본값: 최대 크기 `50` MB, 보관 파일 `10`개
- 관련 플래그: `--audit-max-size-mb`, `--audit-max-files`

## 로그 보기

```bash
deck server logs --source file --path <root>/.deck/logs/server-audit.log
```
