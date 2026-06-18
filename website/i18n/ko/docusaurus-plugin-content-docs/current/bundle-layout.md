---
source: docs/bundle-layout.md
source_hash: f1e7d2655d3aa901d6dba4c5cd45f6b7b990827f
---
# 번들 레이아웃

`deck prepare`는 현재 디렉터리 아래에 자체 완결적인 워크스페이스를 작성합니다. `deck bundle build`는 그 워크스페이스를 현장으로 가져갈 단일 tarball로 아카이브합니다.

번들은 오프라인 인수인계의 단위입니다. 워크플로가 대상 머신에서 실행되는 데 필요한 모든 것이 그 안에 들어 있어야 합니다 — 실행 시점에 암묵적으로 가져오거나 외부 서비스로 되돌아가 접근하는 일이 없어야 합니다.

## 표준 번들 입력

`deck bundle build`는 다음 워크스페이스 경로를 아카이브합니다:

- `deck`: `prepare` 중에 워크스페이스 루트에 작성되는 런처 스크립트
- `workflows/`: 현장에서 사용되는 시나리오, 컴포넌트, 변수 파일
- `outputs/bin/`: `prepare` 중에 선택된 플랫폼별 런타임 바이너리
- `outputs/packages/`: `prepare` 중에 가져온 OS 또는 Kubernetes 패키지
- `outputs/images/`: `prepare` 중에 가져온 컨테이너 이미지 아카이브
- `outputs/files/`: `prepare` 중에 복사하거나 다운로드한 지원 파일
- `.deck/manifest.json`: `bundle verify`가 사용하는 무결성 매니페스트

`bundle build`는 기본적으로 임의의 추가 루트 레벨 경로를 아카이브하지 않습니다. 워크플로가 현장에서 추가 콘텐츠를 필요로 한다면, 표준 번들과 함께 이동하도록 `workflows/` 아래에 두거나 `prepare` 중에 `outputs/` 아래에 생성하십시오.

## 예시 번들 내용

전형적인 Kubernetes 컨트롤 플레인 번들은 다음을 포함할 수 있습니다:

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

운영자는 대상 노드에서 이것을 풀고, `./deck apply`를 실행합니다. 런처는 해당 플랫폼이 번들에 포함되어 있을 때 `outputs/bin/<os>/<arch>/deck`에서 일치하는 런타임 바이너리를 선택합니다.

## Prepare 재사용 무결성

`deck prepare`는 실행 사이에 이전에 가져온 아티팩트를 재사용할 수 있지만, 재사용 규칙은 아티팩트 계열별로 다릅니다:

- `DownloadFile`은 로컬 SHA256 상태를 재확인하며, URL 소스의 경우 `ETag` 및 `Last-Modified` 같은 원격 검증자도 참조할 수 있습니다.
- `DownloadPackage`는 이제 게시된 패키지 출력 및 내보낸 패키지 캐시 페이로드에 대해 SHA256 메타데이터를 기록한 다음, 재사용 전에 해당 체크섬을 재검증합니다.
- `DownloadImage`는 이제 저장된 이미지 아카이브에 대해 SHA256 메타데이터를 기록하고, 재사용 전에 해당 체크섬을 재검증합니다.

`DownloadImage` 재사용에는 저장된 tar 파일과 이전의 성공적인 `prepare` 실행이 작성한 `outputs/images/.deck-cache-images.json` 메타데이터가 필요합니다. 이 메타데이터는 동일한 출력 디렉터리에서 여러 이미지/플랫폼 세트를 추적할 수 있습니다. 해당 메타데이터가 없는 일치하는 tar 파일은 캐시 미스로 처리됩니다. `deck prepare --refresh`는 이미지 재사용을 우회하고 아카이브를 다시 다운로드합니다.

남아 있는 드리프트 갭:

- 패키지 재사용은 여전히 자체적으로 업스트림 리포지토리 드리프트를 감지하지 못합니다. 이 갭을 닫으려면 repodata/release 핑거프린트 같은 리포지토리 스냅샷 메타데이터나 명시적인 미러 버전 계약이 필요할 가능성이 높습니다.
- 이미지 재사용은 이제 가져온 소스 다이제스트를 메타데이터에 보존하지만, 가변 태그 드리프트는 아직 재사용 시 탐지되지 않습니다. 후속 작업으로 원격 접근이 허용될 때 저장된 다이제스트를 현재 레지스트리 매니페스트와 비교할 수 있습니다.

## Apply 시점 매니페스트 검증 {#apply-time-manifest-verification}

`deck apply`가 번들 루트를 해석할 때마다, 어떤 워크플로 단계든 실행하기 전에 번들 매니페스트를 검증합니다. 검증 실패는 실행을 즉시 중단시킵니다 — 어떤 단계도 시작되지 않습니다.

### 검증을 유발하는 호출

검증은 세 가지 경우에 자동으로 실행됩니다:

- **워크스페이스에서의 일반 `deck apply`** — 명시적 경로가 주어지지 않으면, deck은 현재 디렉터리에 `workflows/` 트리가 있을 경우 이를 번들 루트로 사용하며, 그에 대해 검증이 실행됩니다.
- **`deck apply --root <dir>`** — 명시적 루트가 번들 루트로 사용되며, 검증이 실행됩니다.
- **`deck apply <bundle-path>`** — 위치 인자 디렉터리 또는 `.tar` 아카이브가 번들 루트로 사용되며, 검증이 실행됩니다. `.tar` 아카이브가 주어지면, deck은 먼저 이를 키 기반 캐시 디렉터리에 추출한 다음 추출된 내용을 검증합니다.

검증은 번들 루트가 해석되지 않을 때만 건너뜁니다 — 예를 들어, 위치 인자 번들 없이 `--workflow <path>`가 제공되거나, 위치 인자 번들 없이 `--scenario <name> --source server`가 제공되는 경우입니다.

전송 또는 적용 전에 번들을 명시적으로 검증하려면 다음을 실행하십시오:

```bash
deck bundle verify --file ./bundle.tar
```

### 검증되는 항목

검증은 `.deck/manifest.json`을 읽고 모든 항목을 `outputs/{files,packages,images,bin}`의 해당 아티팩트와 대조합니다(`outputs/` 접두사는 선택 사항입니다 — 맨 `files/`, `packages/`, `images/`, `bin/` 경로를 사용하는 레거시 번들도 추적됩니다). 각 항목에 대해 다음을 확인합니다:

- 아티팩트가 디스크에(또는 tar 아카이브 내부에) 존재하는지,
- SHA-256 다이제스트가 기록된 값과 일치하는지,
- 0이 아닌 크기가 기록된 경우 파일 크기가 일치하는지.

항목별 확인 후, 이 함수는 번들에 존재하는 모든 패키지 리포지토리 인덱스 파일(`Release`, `Packages.gz`, `repomd.xml`)과 모든 이미지 `.tar`가 매니페스트 항목으로 커버되는지도 교차 확인합니다.

매니페스트가 추적하는 전체 경로 목록은 [workspace-layout.md](workspace-layout.md)를 참조하십시오.

### 실패 모드

| Error code | 조건 |
|---|---|
| `E_MANIFEST_MISSING` | `.deck/manifest.json`이 번들 디렉터리 또는 tar 아카이브에 없음 |
| `E_MANIFEST_EMPTY` | 매니페스트 파일은 존재하지만 `entries` 배열이 비어 있음 |
| `E_BUNDLE_INTEGRITY` | 아티팩트가 없거나, 크기 또는 SHA-256 다이제스트가 매니페스트와 일치하지 않거나, 매니페스트 항목 경로가 구조적으로 유효하지 않거나, 필수 오프라인 아티팩트가 번들에는 있지만 매니페스트에는 없음 |

전체 에러 코드 카탈로그는 [diagnostics/error-codes.md](diagnostics/error-codes.md)를, 해결 단계는 [troubleshooting.md](troubleshooting.md)를 참조하십시오.

## 핵심 규칙

워크플로를 실행하기 위해 현장에서 필요한 것이라면, 대상 머신에 이미 존재한다고 가정하지 말고 표준 번들 입력에 포함하십시오.
