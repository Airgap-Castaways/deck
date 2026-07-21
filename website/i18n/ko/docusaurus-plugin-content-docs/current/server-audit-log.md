---
source: docs/server-audit-log.md
source_hash: 854ba70599bece6d677ff6ad54c2c3c4dc765d9a
---
# 서버 감사 로그

`deck server up`은 번들 루트 아래 JSONL 로그 파일에 감사 기록을 남깁니다.

## 위치

기본 파일 위치는 다음과 같습니다.

```text
<root>/.deck/logs/server-audit.log
```

## 현재 기록되는 레코드 형태

`deck server up`은 감사 스키마 버전 `2` 레코드를 최상위 필드로 구성한 구조화된 형태로 남깁니다.

공통 필드는 다음과 같습니다.

- `ts`: UTC 기준 RFC3339Nano 타임스탬프
- `schema_version`: 현재 값은 `2`
- `component`: 로그를 생성한 주체이며, 현재는 `server`
- `event`: 정규화된 이벤트 이름으로, 예를 들면 `request`
- `level`: `info`, `warn`, `error` 중 하나
- `message`: 사람이 읽을 수 있는 짧은 설명

요청 레코드에는 다음과 같은 최상위 요청 속성도 함께 담깁니다.

- `method`
- `path`
- `proto`
- `status`
- `bytes`
- `remote_addr`
- `duration_ms`

## 대표적인 예시

- 사이트 API, 레지스트리, 정적 파일, 헬스 체크 등 라우팅된 서버 응답마다 레코드를 남깁니다
- 현재 작성기는 요청 속성을 중첩된 `extra` 객체가 아니라 최상위에 둡니다

요청 레코드 예시:

```json
{"ts":"2026-04-03T05:00:00Z","schema_version":2,"component":"server","event":"request","level":"info","message":"http request handled","method":"GET","path":"/healthz","proto":"HTTP/1.1","status":200,"bytes":0,"remote_addr":"127.0.0.1:53422","duration_ms":1}
```

레지스트리는 도메인이 제거된 별칭(alias)이 둘 이상의 정식(canonical) 저장소로 매핑되는 요청을 받으면 `registry_alias_collision` 이벤트(`level: warn`)를 남깁니다. 이때 요청은 `404`로 거부되며, 레코드의 `canonical_repos`에 충돌한 저장소 목록이 담깁니다. 자세한 내용은 [별칭 충돌](server/registry.md#alias-collisions)을 참고하세요.

```json
{"ts":"2026-04-03T05:00:00Z","schema_version":2,"component":"server","event":"registry_alias_collision","level":"warn","message":"ambiguous registry alias rejected","alias":"calico/node","canonical_repos":["quay.io/calico/node","registry.example.com/calico/node"],"method":"GET","path":"/v2/calico/node/manifests/v1"}
```

## 호환성 참고

- 현재 `deck server up`은 위에서 설명한 구조화된 버전 2 형태로 기록합니다
- `deck server logs`는 `source`, `event_type`, 중첩된 `extra` 같은 필드를 쓰던 기존 레거시 레코드도 여전히 정규화할 수 있습니다
- 원본 감사 파일을 직접 읽는 다운스트림 소비자는 앞으로 버전 2의 최상위 구조를 기준으로 삼아야 합니다

## 로테이션

- `deck server up`은 감사 로그가 설정된 크기 한도를 넘으면 로테이션합니다
- 기본값은 최대 크기 `50` MB, 보관 파일 `10`개입니다
- 관련 플래그: `--audit-max-size-mb`, `--audit-max-files`

## 로그 보기

```bash
deck server logs --source file --path <root>/.deck/logs/server-audit.log
```
