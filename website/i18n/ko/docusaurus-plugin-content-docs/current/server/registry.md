---
source: docs/server/registry.md
source_hash: 9da4bae85f5c67817284a541d68b507b95015265
---
# 서버 레지스트리

`deck server up`은 `/v2` 접두사에서 읽기 전용 OCI Distribution v2 레지스트리를 노출합니다. `deck prepare`가 번들로 다운로드한 컨테이너 이미지 아카이브를 제공하므로, `containerd`, `docker`, `crane` 같은 런타임이 외부 네트워크 접근 없이 로컬 서버에서 직접 이미지를 풀할 수 있습니다.

## 개요

이 레지스트리는 풀 전용입니다. 푸시나 그 밖의 쓰기 작업은 받지 않습니다. `GET` 또는 `HEAD`가 아닌 모든 요청은 `405 Method Not Allowed`를 반환합니다.

레지스트리는 번들 루트 아래의 `outputs/images/`(및 레거시 `images/` 경로)에서 `.tar` 파일을 스캔하여 이미지를 검색합니다. 각 tar는 Docker tarball 형식으로 읽습니다. `.deckignore`에 의해 제외된 파일은 건너뜁니다.

`outputs/images/`가 어떻게 채워지는지는 [Bundle Layout](../bundle-layout.md)을 참고하세요.

## 엔드포인트

`GET` 및 `HEAD` 응답은 `Docker-Distribution-API-Version: registry/2.0` 응답 헤더를 설정합니다. 거부된 비-`GET`/`HEAD` 요청은 이 헤더 없이 `405 Method Not Allowed`를 받습니다.

### Ping

```
GET  /v2
HEAD /v2
GET  /v2/
HEAD /v2/
```

빈 본문과 함께 `200 OK`를 반환합니다. 표준 OCI/Docker 레지스트리 디스커버리 검사입니다.

### Catalog

```
GET  /v2/_catalog
HEAD /v2/_catalog
```

서버가 제공할 수 있는 모든 리포지토리 이름을 나열하는 JSON 객체를 반환합니다.

```json
{"repositories": ["calico/node-driver-registrar", "quay.io/calico/node-driver-registrar"]}
```

### Tags list

```
GET  /v2/<name>/tags/list
HEAD /v2/<name>/tags/list
```

명명된 리포지토리의 모든 태그를 나열하는 JSON 객체를 반환합니다. 리포지토리를 찾을 수 없으면 `404`를 반환합니다.

```json
{"name": "calico/node-driver-registrar", "tags": ["v3.28.0"]}
```

### Manifest

```
GET  /v2/<name>/manifests/<ref>
HEAD /v2/<name>/manifests/<ref>
```

`<ref>`는 태그(예: `3.18`) 또는 다이제스트(예: `sha256:abc123…`)일 수 있습니다. 적절한 `Content-Type`과 `Docker-Content-Digest` 헤더와 함께 원본 이미지 매니페스트를 반환합니다. 이미지를 찾을 수 없으면 `404`를 반환합니다.

### Blob

```
GET  /v2/<name>/blobs/<digest>
HEAD /v2/<name>/blobs/<digest>
```

`<digest>`로 식별되는 이미지 구성(config) 또는 압축된 레이어를 반환합니다. 명명된 리포지토리 내에서 다이제스트를 찾을 수 없으면 `404`를 반환합니다.

## 리포지토리 이름 별칭

이미지 tarball은 `RepoTags` 필드를 포함합니다. 서버는 약한 검증으로 각 태그를 파싱하여 정규 리포지토리 이름을 도출합니다. 예를 들어 `quay.io/calico/node-driver-registrar:v3.28.0` 태그의 정규 리포지토리는 `quay.io/calico/node-driver-registrar`입니다.

정규 이름의 첫 번째 경로 구성 요소가 레지스트리 도메인처럼 보이면(`.` 또는 `:`를 포함하거나 `localhost`와 같으면), 서버는 도메인 접두사를 제거한 두 번째 별칭을 등록합니다. 위 예시를 사용하면 다음 두 이름은 모두 같은 이미지를 가리키며 서로 바꿔 쓸 수 있습니다.

- `quay.io/calico/node-driver-registrar` (정규, 도메인 접두사 포함)
- `calico/node-driver-registrar` (별칭, 도메인 제거됨)

이를 통해 클라이언트는 정규화된 전체 이름 또는 더 짧은 형식 중 하나로 풀할 수 있습니다.

> **Docker Hub 참고:** go-containerregistry는 `docker.io`를 `index.docker.io`로 정규화하므로, `docker.io/library/alpine:3.18`로 저장된 태그는 정규 이름 `index.docker.io/library/alpine` 아래에 등록됩니다. 클라이언트는 `index.docker.io/library/alpine` 또는 도메인이 제거된 별칭 `library/alpine`을 통해 풀해야 합니다.

## 읽기 전용 강제

`GET` 또는 `HEAD` 이외의 모든 메서드(`POST`, `PUT`, `DELETE`, `PATCH` 포함)는 어떤 경로 디스패치도 일어나기 전에 `405 Method Not Allowed`를 반환합니다. 푸시, 삭제, 업로드 엔드포인트는 없습니다.

## `.deckignore`

서버는 이미지 tarball을 스캔하기 전에 번들 루트에서 `.deckignore`를 로드합니다. 경로(번들 루트 기준)가 무시 규칙과 일치하는 모든 `.tar` 파일은 카탈로그에서 제외되며 제공될 수 없습니다.

## 사용 예시

기본 주소로 서버를 시작합니다.

```sh
deck server up --root /path/to/bundle
```

레지스트리는 나머지 서버와 동일한 주소(기본값 `:8080`, `--addr`로 구성 가능)에서 수신 대기합니다. `crane`을 사용하여 이미지를 풀합니다.

```sh
crane pull localhost:8080/calico/node-driver-registrar:v3.28.0 node-driver-registrar.tar
```

또는 `containerd`를 레지스트리의 미러로 서버를 사용하도록 구성하고 `http://localhost:8080`을 가리키게 합니다.

TLS 옵션 및 데몬 모드는 [deck server up](../cli/deck_server_up.md)을 참고하세요.
