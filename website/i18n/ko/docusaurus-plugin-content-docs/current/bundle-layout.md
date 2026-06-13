---
source: docs/bundle-layout.md
source_hash: 604f2348405f8e4a53814b5502c8ba7a70bc483a
---
# 번들 레이아웃

`deck prepare`는 현재 디렉터리 아래에 자체 완결형 워크스페이스를 작성합니다. `deck bundle build`는 그 워크스페이스를 사이트로 가져갈 단일 tarball로 아카이브합니다.

번들은 오프라인 인계의 단위입니다. 워크플로가 대상 머신에서 실행되는 데 필요한 모든 것이 그 안에 들어 있어야 합니다 — 실행 시점의 암묵적 페치도, 외부 서비스로의 역참조도 없어야 합니다.

## 표준 번들 입력

`deck bundle build`는 다음 워크스페이스 경로를 아카이브합니다:

- `deck`: `prepare` 중 워크스페이스 루트에 작성되는 런처 스크립트
- `workflows/`: 사이트에서 사용되는 시나리오, 컴포넌트, 변수 파일
- `outputs/bin/`: `prepare` 중 선택된 플랫폼별 런타임 바이너리
- `outputs/packages/`: `prepare` 중 페치된 OS 또는 Kubernetes 패키지
- `outputs/images/`: `prepare` 중 페치된 컨테이너 이미지 아카이브
- `outputs/files/`: `prepare` 중 복사 또는 다운로드된 지원 파일
- `.deck/manifest.json`: `bundle verify`가 사용하는 무결성 매니페스트

`bundle build`는 기본적으로 임의의 추가 루트 레벨 경로를 아카이브하지 않습니다. 워크플로가 사이트에서 추가 콘텐츠를 필요로 한다면, 표준 번들과 함께 이동하도록 `workflows/` 아래에 두거나 `prepare` 중 `outputs/` 아래에 생성하십시오.

## 번들 내용 예시

일반적인 Kubernetes 컨트롤 플레인 번들에는 다음이 포함될 수 있습니다:

```
deck
.deck/manifest.json
workflows/scenarios/apply.yaml
workflows/prepare.yaml
workflows/vars.yaml
outputs/bin/linux/amd64/deck
outputs/packages/kubernetes-1.29.tar.gz
outputs/images/pause-3.9.tar
outputs/images/coredns-1.11.tar
outputs/files/kubeadm.conf
```

운영자는 이를 대상 노드에서 풀고 `./deck apply`를 실행합니다. 런처는 해당 플랫폼이 번들에 포함되어 있을 때 `outputs/bin/<os>/<arch>/deck`에서 일치하는 런타임 바이너리를 선택합니다.

## Prepare 재사용 무결성

`deck prepare`는 실행 간에 이전에 페치된 아티팩트를 재사용할 수 있지만, 재사용 규칙은 아티팩트 계열별로 다릅니다:

- `DownloadFile`은 로컬 SHA256 상태를 다시 확인하며, URL 소스의 경우 `ETag` 및 `Last-Modified` 같은 원격 검증자도 참조할 수 있습니다.
- `DownloadPackage`는 이제 게시된 패키지 출력과 내보낸 패키지 캐시 페이로드에 대해 SHA256 메타데이터를 기록한 후, 재사용 전에 해당 체크섬을 재검증합니다.
- `DownloadImage`는 이제 저장된 이미지 아카이브에 대해 SHA256 메타데이터를 기록하고, 재사용 전에 해당 체크섬을 재검증합니다.

`DownloadImage` 재사용에는 이전의 성공한 `prepare` 실행이 작성한 저장된 tar 파일과 `outputs/images/.deck-cache-images.json` 메타데이터가 필요합니다. 이 메타데이터는 동일한 출력 디렉터리에서 여러 이미지/플랫폼 세트를 추적할 수 있습니다. 해당 메타데이터가 없는 일치하는 tar 파일은 캐시 미스로 취급됩니다. `deck prepare --refresh`는 이미지 재사용을 우회하고 아카이브를 다시 다운로드합니다.

남아 있는 드리프트 갭:

- 패키지 재사용은 여전히 업스트림 리포지토리 드리프트를 자체적으로 감지하지 못합니다. 이 갭을 해소하려면 repodata/release 핑거프린트 같은 리포지토리 스냅샷 메타데이터나 명시적 미러 버전 계약이 필요할 가능성이 큽니다.
- 이미지 재사용은 이제 페치된 소스 다이제스트를 메타데이터에 보존하지만, 변경 가능한 태그 드리프트는 재사용 시 아직 탐지되지 않습니다. 후속 작업으로 원격 접근이 허용될 때 저장된 다이제스트를 현재 레지스트리 매니페스트와 비교할 수 있습니다.

## 적용 시점 매니페스트 검증

`deck apply`가 번들 루트를 해석할 때마다, 워크플로 단계를 실행하기 전에 `bundle.VerifyManifest`를 실행합니다. 검증 실패는 실행을 즉시 중단시킵니다 — 어떤 단계도 시작되지 않습니다.

### 검증을 트리거하는 호출

`RunOptions`에서 `opts.BundleRoot`가 비어 있지 않을 때마다 검증이 실행됩니다(`internal/install/runner.go:114`). CLI는 세 가지 방식으로 `BundleRoot`를 채웁니다:

- **워크스페이스에서의 일반 `deck apply`** — 명시적 경로가 주어지지 않으면 `ResolveBundleRoot`가 현재 디렉터리(`.`)로 폴백합니다. 디렉터리에 `workflows/` 트리가 있으면 번들 루트로 사용되고 검증이 실행됩니다.
- **`deck apply --root <dir>`** — 명시적 루트가 해석되어 `BundleRoot`로 전달되고 검증이 실행됩니다.
- **`deck apply <bundle-path>`** — 위치 인자 번들 경로(디렉터리 또는 `.tar` 아카이브)가 해석되어 `BundleRoot`로 전달되고 검증이 실행됩니다.

검증은 번들 루트가 해석되지 않을 때만 건너뜁니다: 이는 위치 인자 번들이 없는 `deck apply --workflow <path>`와, 위치 인자 번들이 없는 `deck apply --scenario <name> --source server`에서 발생합니다. `--root`도 위치 인자 경로도 없는 로컬 `--scenario`(또는 일반 `deck apply`)의 경우, deck은 현재 디렉터리를 번들 루트로 해석합니다 — `workflows/` 트리가 있으면 검증이 실행되고, 그렇지 않으면 명령이 오류를 냅니다.

위치 인자 `.tar` 번들이 주어지면, deck은 번들 루트를 해석하기 전에 이를 캐시 디렉터리(아카이브의 SHA-256으로 키 지정)로 추출합니다. 그런 다음 추출된 디렉터리에 대해 검증이 실행됩니다.

### 검증 대상

`bundle.VerifyManifest`(`internal/bundle/verify.go:37`)는 `.deck/manifest.json`을 읽고 그 안의 모든 항목을 `outputs/{files,packages,images,bin}`의 해당 아티팩트와 대조합니다(`outputs/` 접두사는 선택 사항입니다 — bare `files/`, `packages/`, `images/`, `bin/`을 사용하는 레거시 번들도 추적됩니다). 각 항목에 대해 다음을 확인합니다:

- 아티팩트가 디스크에(또는 tar 아카이브 내에) 존재하는지,
- SHA-256 다이제스트가 기록된 값과 일치하는지,
- 0이 아닌 크기가 기록된 경우 파일 크기가 일치하는지.

항목별 확인 후, 이 함수는 번들에 존재하는 모든 패키지 리포지토리 인덱스 파일(`Release`, `Packages.gz`, `repomd.xml`)과 모든 이미지 `.tar`가 매니페스트 항목으로 커버되는지도 교차 확인합니다.

매니페스트가 추적하는 경로의 전체 목록은 [workspace-layout.md](workspace-layout.md)를 참조하십시오.

### 실패 모드

| Error code | 조건 |
|---|---|
| `E_MANIFEST_MISSING` | `.deck/manifest.json`이 번들 디렉터리 또는 tar 아카이브에 없음 |
| `E_MANIFEST_EMPTY` | 매니페스트 파일은 존재하지만 그 `entries` 배열이 비어 있음 |
| `E_BUNDLE_INTEGRITY` | 아티팩트가 누락되었거나, 크기 또는 SHA-256 다이제스트가 매니페스트와 일치하지 않거나, 매니페스트 항목 경로가 구조적으로 유효하지 않거나, 필수 오프라인 아티팩트가 번들에는 존재하지만 매니페스트에는 없음 |

전체 에러 코드 카탈로그는 [diagnostics/error-codes.md](diagnostics/error-codes.md)를 참조하십시오.

## 핵심 규칙

사이트가 워크플로를 실행하는 데 필요한 것이라면, 대상 머신에 이미 존재한다고 가정하지 말고 표준 번들 입력에 두십시오.
