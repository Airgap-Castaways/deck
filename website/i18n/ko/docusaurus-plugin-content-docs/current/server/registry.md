---
source: docs/server/registry.md
source_hash: 0f3a11818eb64151a9aeaf6f8578cde96feb4476
---
# Server registry

`deck server up`은 `/v2` 접두사에서 읽기 전용 OCI Distribution v2 레지스트리를 제공합니다. 이 레지스트리는 `deck prepare`가 번들에 내려받아 둔 컨테이너 이미지 아카이브를 서빙하므로, `containerd`, `docker`, `crane` 같은 런타임이 외부 네트워크 없이 로컬 서버에서 곧바로 이미지를 받을 수 있습니다.

## 개요

레지스트리는 pull 전용입니다. push를 비롯한 쓰기 작업은 지원하지 않으며, `GET`이나 `HEAD`가 아닌 요청에는 모두 `405 Method Not Allowed`를 반환합니다.

이미지는 번들 루트 아래의 `outputs/images/`(및 레거시 경로인 `images/`)에서 `.tar` 파일을 훑어 찾아냅니다. 각 tar은 Docker tarball 형식으로 읽으며, `.deckignore`로 제외된 파일은 건너뜁니다.

`outputs/images/`가 채워지는 방식은 [Bundle Layout](../bundle-layout.md)을 참고하세요.

## Endpoints

`GET`과 `HEAD` 응답에는 `Docker-Distribution-API-Version: registry/2.0` 헤더가 붙습니다. `GET`/`HEAD`가 아니어서 거부된 요청에는 이 헤더 없이 `405 Method Not Allowed`만 반환됩니다.

### Ping

```
GET  /v2
HEAD /v2
GET  /v2/
HEAD /v2/
```

빈 본문과 함께 `200 OK`를 반환합니다. 표준 OCI/Docker 레지스트리 탐색 확인 절차입니다.

### Catalog

```
GET  /v2/_catalog
HEAD /v2/_catalog
```

서버가 서빙할 수 있는 모든 리포지터리 이름을 담은 JSON 객체를 반환합니다.

```json
{"repositories": ["calico/node-driver-registrar", "quay.io/calico/node-driver-registrar"]}
```

### Tags list

```
GET  /v2/<name>/tags/list
HEAD /v2/<name>/tags/list
```

지정한 리포지터리의 태그를 모두 담은 JSON 객체를 반환합니다. 리포지터리를 찾지 못하면 `404`를 반환합니다.

```json
{"name": "calico/node-driver-registrar", "tags": ["v3.28.0"]}
```

### Manifest

```
GET  /v2/<name>/manifests/<ref>
HEAD /v2/<name>/manifests/<ref>
```

`<ref>`에는 태그(예: `3.18`) 또는 digest(예: `sha256:abc123…`)를 쓸 수 있습니다. 알맞은 `Content-Type`과 `Docker-Content-Digest` 헤더를 붙여 원본 이미지 매니페스트를 반환하며, 이미지를 찾지 못하면 `404`를 반환합니다.

### Blob

```
GET  /v2/<name>/blobs/<digest>
HEAD /v2/<name>/blobs/<digest>
```

`<digest>`로 식별되는 이미지 config나 압축된 레이어를 반환합니다. 지정한 리포지터리 안에서 해당 digest를 찾지 못하면 `404`를 반환합니다.

## Repository name aliases

이미지 tarball에는 `RepoTags` 필드가 들어 있습니다. 서버는 각 태그를 느슨하게 검증하며 파싱해 정규 리포지터리 이름을 도출합니다. 예를 들어 `quay.io/calico/node-driver-registrar:v3.28.0` 태그의 정규 리포지터리는 `quay.io/calico/node-driver-registrar`입니다.

정규 이름의 첫 경로 구성 요소가 레지스트리 도메인처럼 보이면(`.`이나 `:`을 포함하거나 `localhost`이면), 서버는 도메인 접두사를 떼어낸 두 번째 alias도 함께 등록합니다. 위 예시에서는 다음 두 이름이 같은 이미지를 가리키며 서로 바꿔 쓸 수 있습니다.

- `quay.io/calico/node-driver-registrar` (정규, 도메인 접두사 포함)
- `calico/node-driver-registrar` (alias, 도메인 제거)

덕분에 클라이언트는 완전히 정규화된 이름이든 더 짧은 형태든 어느 쪽으로도 이미지를 받을 수 있습니다.

> **Docker Hub 참고:** go-containerregistry는 `docker.io`를 `index.docker.io`로 정규화하므로, `docker.io/library/alpine:3.18`로 저장된 태그는 정규 이름 `index.docker.io/library/alpine`으로 등록됩니다. 클라이언트는 `index.docker.io/library/alpine`이나 도메인을 떼어낸 alias인 `library/alpine`으로 받아야 합니다.

### Alias collisions {#alias-collisions}

서로 다른 레지스트리의 두 이미지가 같은 alias로 축약될 수 있습니다. 예를 들어 `quay.io/calico/node`와 `registry.example.com/calico/node`는 둘 다 `calico/node`라는 alias를 만들어냅니다. 이런 alias는 더 이상 이미지 하나를 특정하지 못하므로, 서버는 이를 모호한(ambiguous) 것으로 취급합니다.

- 모호한 alias는 **카탈로그(`/v2/_catalog`)에서 제외됩니다.**
- 모호한 alias에 대한 태그, 매니페스트, blob 요청에는 충돌하는 이미지 중 하나를 임의로 서빙하지 않고 **`404 Not Found`**를 반환합니다. 거부할 때마다 충돌한 정규 리포지터리들을 나열한 `registry_alias_collision` 이벤트를 [서버 감사 로그](../server-audit-log.md)에 남깁니다.
- **정규(도메인 접두사 포함) 이름은 언제나 정상적으로 동작합니다.** 모호함을 해소하려면 `quay.io/calico/node`나 `registry.example.com/calico/node`를 명시적으로 받으면 됩니다.

요청한 이름이 어떤 이미지의 정규 리포지터리이면서 동시에 다른 이미지의 alias이기도 하면, 정규 리포지터리가 우선합니다.

## Read-only enforcement

`GET`이나 `HEAD`가 아닌 메서드(`POST`, `PUT`, `DELETE`, `PATCH` 포함)는 모두 경로 디스패치 이전 단계에서 `405 Method Not Allowed`를 반환합니다. push, delete, upload 엔드포인트는 아예 존재하지 않습니다.

## `.deckignore`

서버는 이미지 tarball을 스캔하기 전에 번들 루트에서 `.deckignore`를 읽습니다. 번들 루트 기준 경로가 ignore 규칙에 매칭되는 `.tar` 파일은 카탈로그에서 제외되며 서빙되지 않습니다.

## Usage example

기본 주소로 서버를 시작합니다.

```sh
deck server up --root /path/to/bundle
```

레지스트리는 서버의 나머지 부분과 같은 주소에서 수신 대기합니다(기본값 `:8080`, `--addr`로 설정 가능). `crane`으로 이미지를 받습니다.

```sh
crane pull localhost:8080/calico/node-driver-registrar:v3.28.0 node-driver-registrar.tar
```

또는 `containerd`가 이 서버를 레지스트리 미러로 사용하도록 설정하고 `http://localhost:8080`을 가리키게 하면 됩니다.

TLS 옵션과 데몬 모드는 [deck server up](../cli/deck_server_up.md)을 참고하세요.
