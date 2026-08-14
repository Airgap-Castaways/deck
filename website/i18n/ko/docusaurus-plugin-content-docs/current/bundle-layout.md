---
source: docs/bundle-layout.md
source_hash: 78017be71214f8505fa7a2c0ad5c68cbad0361cc
---
title: Bundle layout
---

# 번들 레이아웃

`deck prepare`은 현재 디렉터리 아래에 자체 완결적인 워크스페이스를 만듭니다. `deck bundle build`은 이 워크스페이스를 단일 tarball로 묶어 현장으로 가져갈 수 있게 합니다.

번들은 오프라인 인계의 단위입니다. 워크플로가 대상 장비에서 실행되는 데 필요한 모든 것은 번들 안에 들어 있어야 합니다. 실행 시점에 무언가를 암묵적으로 내려받거나 외부 서비스로 다시 연결하는 일이 없어야 합니다.

## 표준 번들 입력

`deck bundle build`은 다음 워크스페이스 경로를 묶습니다.

- `deck`: `prepare` 단계에서 워크스페이스 루트에 생성하는 런처 스크립트
- `workflows/`: 현장에서 사용하는 시나리오, 컴포넌트, 변수 파일
- `outputs/bin/`: `prepare` 단계에서 선택한 플랫폼별 런타임 바이너리
- `outputs/packages/`: `prepare` 단계에서 가져온 OS 또는 Kubernetes 패키지
- `outputs/images/`: `prepare` 단계에서 가져온 컨테이너 이미지 아카이브
- `outputs/files/`: `prepare` 단계에서 복사하거나 내려받은 보조 파일
- `.deck/manifest.json`: `bundle verify`가 사용하는 무결성 매니페스트

`bundle build`은 기본적으로 루트 수준의 임의 경로를 추가로 묶지 않습니다. 워크플로가 현장에서 별도의 콘텐츠를 필요로 한다면, 표준 번들과 함께 옮겨지도록 `workflows/` 아래에 두거나 `prepare` 단계에서 `outputs/` 아래에 생성하십시오.

## 번들 콘텐츠 예시

일반적인 Kubernetes 컨트롤 플레인 번들은 다음과 같은 내용을 담을 수 있습니다.

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

운영자는 대상 노드에서 이를 풀어 놓은 뒤 `./deck apply`을 실행합니다. 해당 플랫폼이 번들에 포함되어 있으면, 런처는 `outputs/bin/<os>/<arch>/deck`에서 일치하는 런타임 바이너리를 선택합니다.

## Prepare 재사용 무결성

`deck prepare`은 실행과 실행 사이에 이전에 가져온 아티팩트를 재사용할 수 있지만, 재사용 규칙은 아티팩트 계열마다 다릅니다.

- `DownloadFile`은 로컬 SHA256 상태를 다시 확인하며, URL 소스의 경우 `ETag`나 `Last-Modified` 같은 원격 검증자도 함께 참고할 수 있습니다.
- `DownloadPackage`은 이제 게시된 패키지 출력과 내보낸 패키지 캐시 페이로드에 대해 SHA256 메타데이터를 기록하고, 재사용 전에 그 체크섬을 다시 검증합니다.
- `DownloadImage`은 이제 저장된 이미지 아카이브에 대해 SHA256 메타데이터를 기록하고, 재사용 전에 그 체크섬을 다시 검증합니다.

`DownloadImage` 재사용에는 저장된 tar 파일과 더불어, 이전에 성공한 `prepare` 실행이 작성한 `outputs/images/.deck-cache-images.json` 메타데이터가 필요합니다. 이 메타데이터는 같은 출력 디렉터리 안에서 여러 이미지/플랫폼 집합을 추적할 수 있습니다. 메타데이터 없이 tar 파일만 있는 경우는 캐시 미스로 처리합니다. `deck prepare --refresh`은 이미지 재사용을 건너뛰고 아카이브를 다시 내려받습니다.

남아 있는 드리프트 격차는 다음과 같습니다.

- 패키지 재사용은 아직 업스트림 저장소의 드리프트를 자체적으로 감지하지 못합니다. 이 격차를 해소하려면 repodata/release 지문 같은 저장소 스냅샷 메타데이터나 명시적인 미러 버전 계약이 필요할 것입니다.
- 이미지 재사용은 이제 가져온 소스 다이제스트를 메타데이터에 보존하지만, 가변 태그의 드리프트는 아직 재사용 시점에 탐지하지 않습니다. 원격 접근이 허용될 때 저장된 다이제스트를 현재 레지스트리 매니페스트와 비교하는 방식은 후속 작업으로 다룰 수 있습니다.

## 적용 시점 매니페스트 검증 {#apply-time-manifest-verification}

`deck apply`은 번들 루트를 해석할 때마다 워크플로 단계를 실행하기 전에 번들 매니페스트를 검증합니다. 검증에 실패하면 어떤 단계도 시작하지 않고 실행을 즉시 중단합니다.

### 검증을 유발하는 호출

검증은 다음 세 가지 경우에 자동으로 실행됩니다.

- **워크스페이스에서의 단순 `deck apply`**: 명시적 경로를 주지 않으면, 현재 디렉터리에 `workflows/` 트리가 있을 경우 deck은 그 디렉터리를 번들 루트로 사용하고 그에 대해 검증을 실행합니다.
- **`deck apply --root <dir>`**: 명시한 루트를 번들 루트로 사용하고 검증을 실행합니다.
- **`deck apply <bundle-path>`**: 위치 인자로 준 디렉터리나 `.tar` 아카이브를 번들 루트로 사용하고 검증을 실행합니다. `.tar` 아카이브를 준 경우, deck은 먼저 이를 키 기반 캐시 디렉터리로 추출한 다음 추출한 콘텐츠를 검증합니다.

검증은 번들 루트가 전혀 해석되지 않을 때만 건너뜁니다. 예를 들어 위치 인자 번들 없이 `--workflow <path>`을 주거나, 위치 인자 번들 없이 `--scenario <name> --source server`을 주는 경우입니다.

전송하거나 적용하기 전에 번들을 명시적으로 검증하려면 다음을 실행하십시오.

```bash
deck bundle verify --file ./bundle.tar
```

### 무엇을 검증하는가

검증은 `.deck/manifest.json`을 읽고, 각 항목을 `outputs/{files,packages,images,bin}`에 있는 해당 아티팩트와 대조합니다(`outputs/` 접두사는 선택 사항이며, 접두사 없이 `files/`, `packages/`, `images/`, `bin/` 경로를 쓰는 레거시 번들도 추적합니다). 각 항목에 대해 다음을 확인합니다.

- 아티팩트가 디스크(또는 tar 아카이브 내부)에 존재하는지,
- SHA-256 다이제스트가 기록된 값과 일치하는지,
- 0이 아닌 크기가 기록되어 있을 때 파일 크기가 일치하는지.

항목별 확인을 마친 뒤에는, 번들에 존재하는 모든 패키지 저장소 인덱스 파일(`Release`, `Packages.gz`, `repomd.xml`)과 모든 이미지 `.tar`이 매니페스트 항목으로 포함되어 있는지도 교차 확인합니다.

매니페스트가 추적하는 전체 경로 목록은 [workspace-layout.md](workspace-layout.md)를 참고하십시오.

### 실패 모드

| Error code | 조건 |
|---|---|
| `E_MANIFEST_MISSING` | 번들 디렉터리나 tar 아카이브에 `.deck/manifest.json`이 없음 |
| `E_MANIFEST_EMPTY` | 매니페스트 파일은 있으나 `entries` 배열이 비어 있음 |
| `E_BUNDLE_INTEGRITY` | 아티팩트가 없거나, 그 크기나 SHA-256 다이제스트가 매니페스트와 일치하지 않거나, 매니페스트 항목 경로가 구조적으로 유효하지 않거나, 필수 오프라인 아티팩트가 번들에는 있으나 매니페스트에는 없음 |

전체 에러 코드 목록은 [diagnostics/error-codes.md](diagnostics/error-codes.md)를, 해결 절차는 [troubleshooting.md](troubleshooting.md)를 참고하십시오.

## 핵심 규칙

현장에서 워크플로를 실행하는 데 필요한 것이라면, 대상 장비에 이미 존재한다고 가정하지 말고 표준 번들 입력에 넣으십시오.
