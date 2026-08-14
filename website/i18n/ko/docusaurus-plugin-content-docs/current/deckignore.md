---
source: docs/deckignore.md
source_hash: 9c58b38d8d30d723ebbd94d3b1cd19d22df2670c
---
# .deckignore

`.deckignore`는 번들을 빌드할 때와 `deck server up`으로 콘텐츠를 서빙할 때 어떤 파일이 제외될지를 제어합니다. 워크스페이스(또는 번들 루트)의 루트에 위치하며 gitignore 스타일의 패턴 구문을 사용합니다.

## 위치

`.deckignore`는 워크스페이스 루트 — `workflows/`, `outputs/`, `.deck/`를 포함하는 동일한 디렉터리 — 에 배치하세요. `deck init`이 자동으로 시작용 파일을 그곳에 생성합니다.

## 구문

패턴은 `github.com/sabhiram/go-gitignore` 라이브러리에 의해 Go `regexp`로 컴파일되며, 이 라이브러리는 gitignore 규칙의 일부를 구현합니다. 변환이 전체 gitignore 엔진이 아닌 Go regexp를 거치기 때문에, 일부 문자는 표준 gitignore와 다르게 동작합니다. 지원되는 기능은 다음과 같습니다:

| 기능 | 지원 여부 | 비고 |
|---------|-----------|-------|
| 빈 줄 | 예 | 무시됨 (구분자) |
| `#` 주석 | 예 | `#`로 시작하는 줄은 무시됨 |
| 후행 공백 | 예 | `\`로 이스케이프하지 않는 한 제거됨 |
| 부정 `!` | 예 | `!pattern`은 이전에 매칭된 경로를 무시 해제함 |
| 후행 `/` (디렉터리 전용) | 예 | `foo/`는 `foo`라는 이름의 디렉터리만 매칭함 |
| 슬래시 없는 패턴 (앵커 없음) | 예 | `*.log`는 트리 어디에서나 매칭됨 |
| 선행 `/` (앵커됨) | 예 | `/foo`는 루트에서만 매칭됨 |
| 단일 `*` 와일드카드 | 예 | 디렉터리 경계를 넘지 않음 |
| `**` 더블 스타 | 예 | 디렉터리를 넘어 매칭됨 (`a/**/b`는 `a/b`, `a/x/b` 등을 매칭) |
| `?` 와일드카드 | 아니오 | `?`는 **리터럴** `?` 문자로 처리됨 (라이브러리가 regexp에서 `\?`로 이스케이프함); 임의의 단일 문자와 **매칭되지 않음** |
| 문자 클래스 `[…]` | 부분적 | 브래킷 클래스는 Go regexp 문자 클래스로 **동작함** (예: `[abc]`, `[0-9]`). 그러나 gitignore의 부정 형태 `[!abc]`는 **부정하지 않음** — Go regexp는 `[^abc]`를 요구하며, `[!…]` 패턴은 예상대로 동작하지 않음 |
| 백슬래시 이스케이프 `\#`, `\!` | 예 | `#` 또는 `!`를 리터럴 문자로 처리함 |

> **참고:** `.deckignore`는 `go-gitignore`를 사용하며, 이는 전체 gitignore 사양을 구현하는 대신 패턴을 Go `regexp`로 컴파일합니다. 두 가지 주목할 만한 차이점: `?`는 리터럴 문자로 처리되며(단일 문자 와일드카드가 아님), 문자 클래스 부정은 Go regexp 구문을 사용합니다 — `[!abc]`는 **부정하지 않습니다**(부정이 필요하면 `[^abc]`를 사용하되, 이는 표준 gitignore 구문이 아닌 원시 regexp 구문입니다).

### `Matches`가 적용하는 규칙

`deck`은 워크스페이스 루트를 기준으로 한 슬래시(`/`) 경로로 `Matcher.Matches(rel, isDir)`를 호출합니다. 패턴 평가 전에:

- 선행 `./`는 제거됩니다.
- 경로는 모든 플랫폼에서 슬래시(`/`)로 변환됩니다.
- 루트 경로 자체(`.` 또는 빈 문자열)는 절대 매칭되지 않습니다.

디렉터리의 경우, `deck`은 `rel/`(후행 슬래시 형태)와 `rel` 양쪽을 모두 테스트하므로, `outputs/`와 같은 후행 슬래시 패턴이 디렉터리 순회를 올바르게 가지치기합니다.

## 파일이 없을 때의 동작

`.deckignore`가 존재하지 않으면, `Load`는 빈 매처를 반환하고 `Matches`는 항상 `false`를 반환합니다 — 아무것도 제외되지 않습니다. 이는 오류가 아니라 no-op입니다.

## 적용되는 위치

`.deckignore`는 네 곳에서 적용됩니다:

**번들 빌드 (`deck bundle build`)**
`internal/bundle/collect.go`는 아카이브 트리를 순회하기 전에 번들 루트에서 `deckignore.Load`를 한 번 호출합니다. 매칭된 파일은 건너뛰고, 매칭된 디렉터리는 전체 서브트리를 가지치기합니다(`filepath.SkipDir`).

**정적 파일 및 워크플로 서빙 (`deck server up`)**
`internal/server/http_static.go`는 `resolveCategoryPath`와 `buildWorkflowIndex` 내부에서 `.deckignore`를 로드합니다. 무시 규칙에 매칭되는 경로에 대한 요청은 HTTP 404를 받습니다.

**Browse UI (`deck server up`)**
`internal/server/http_browse.go`(`listBrowseEntries`)는 `.deckignore`를 로드하고 브라우저에 표시되는 디렉터리 목록에서 매칭된 항목을 생략합니다.

**OCI 레지스트리 카탈로그 (`deck server up`)**
`internal/server/http_registry.go`(`scanRegistryCatalog`)는 `.tar` 파일을 찾기 위해 `outputs/images/`와 `images/`를 스캔하기 전에 `.deckignore`를 로드합니다. 번들 루트를 기준으로 한 경로가 매칭되는 `.tar`는 레지스트리 카탈로그에서 제외됩니다 — `/v2/_catalog`에 나타나지 않으며 풀(pull)할 수 없습니다.

## 기본 내용

`deck init`은 다음과 같은 시작용 `.deckignore`를 작성합니다:

```
.git/
.gitignore
.deckignore
/*.tar
```

이는 버전 관리 내부 파일, 무시 파일 자체, 그리고 워크스페이스 루트에 있는 임의의 `.tar` 아카이브를 번들과 서버에서 숨깁니다.

## 예시

```gitignore
# Exclude version-control internals
.git/

# Exclude the ignore files themselves from the bundle
.gitignore
.deckignore

# Exclude all loose tar archives directly under the workspace root
/*.tar

# Exclude a specific large image tarball from the server registry
outputs/images/dev-tools.tar

# Exclude an entire scenario subdirectory
workflows/scenarios/internal/

# Re-include one file inside a broader excluded directory
!outputs/files/README.txt
```

## 관련 문서

- [Workspace Layout](workspace-layout.md) — 워크스페이스 디렉터리 구조
- [Bundle Layout](bundle-layout.md) — 번들 아카이브에 들어가는 내용
- [Server Registry](server/registry.md) — `deck server up`이 서빙하는 OCI 레지스트리
