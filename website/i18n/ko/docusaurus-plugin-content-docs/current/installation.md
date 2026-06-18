---
source: docs/installation.md
source_hash: 0a34c7ac643f338ca0acb507cb577da2764fc6d0
---
# 설치

## deck가 실행되는 환경

`deck`은 런타임 의존성이 없는 단일 정적 바이너리로 배포됩니다.

| 단계 | 플랫폼 |
|---|---|
| 작성, lint, prepare, bundle | macOS, Linux, Windows |
| Apply (타깃) | Linux — RHEL / Rocky / CentOS 또는 Debian / Ubuntu |

타깃 머신에 Go를 설치할 필요는 **없습니다**. `deck` 바이너리(그리고 워크플로에 필요한 그 외 모든 것)는 번들 안에 포함되어 이동합니다. 워크플로를 작성하고 준비하는 머신에만 PATH에 `deck` 바이너리가 있으면 됩니다.

릴리스 아티팩트는 다음 대상으로 빌드됩니다:

- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`

Windows 바이너리는 아직 릴리스에 게시되지 않았습니다. Windows 사용자는 소스에서 빌드할 수 있습니다(아래 참고).

## Homebrew (권장)

Airgap Castaways tap이 최신 릴리스를 게시합니다:

```bash
brew install Airgap-Castaways/tap/deck
```

이 명령은 `Airgap-Castaways/homebrew-tap`을 tap으로 추가하고 `deck` formula를 설치합니다. 업그레이드는 일반적인 `brew upgrade` 경로를 따릅니다.

## GitHub Releases (바이너리 tarball)

[릴리스 페이지](https://github.com/Airgap-Castaways/deck/releases)에서 사용하는 OS와 아키텍처에 맞는 아카이브를 내려받으세요.

아카이브 이름은 `deck_<version>_<os>_<arch>.tar.gz` 패턴을 따릅니다. 예:

- `deck_1.2.3_linux_amd64.tar.gz`
- `deck_1.2.3_darwin_arm64.tar.gz`

압축을 풀고 바이너리를 PATH에 배치하세요:

```bash
# Linux amd64 예시 — 버전과 아키텍처에 맞게 파일명을 조정하세요
tar -xzf deck_<version>_linux_amd64.tar.gz
sudo mv deck /usr/local/bin/deck
```

바이너리를 실행하기 전에 **체크섬을 검증**하세요. 각 릴리스는 아카이브와 함께 `checksums.txt` 파일을 게시합니다:

```bash
sha256sum --check --ignore-missing checksums.txt
```

### Debian 및 RHEL 패키지

릴리스 페이지에서는 동일한 바이너리로 빌드된 `.deb` 및 `.rpm` 패키지도 게시합니다. 이 패키지는 `deck`을 `/usr/bin/deck`에 설치합니다:

```bash
# Debian / Ubuntu
sudo dpkg -i deck_<version>_linux_amd64.deb

# RHEL / Rocky / CentOS
sudo rpm -i deck_<version>_linux_amd64.rpm
```

## 소스에서 빌드

Go 1.26 이상이 필요합니다.

```bash
go install github.com/Airgap-Castaways/deck/cmd/deck@latest
```

이 방식으로 설치한 바이너리에는 버전 스탬프 메타데이터가 포함되지 않습니다(`deck version` 출력의 commit과 date 필드가 비어 있습니다). 버전 정보를 포함해 빌드하려면 저장소를 클론한 뒤 Makefile을 사용하세요:

```bash
git clone https://github.com/Airgap-Castaways/deck.git
cd deck
make build
```

`make build`의 결과물은 저장소 루트에 `deck`로 생성됩니다.

## 검증

바이너리가 설치되어 접근 가능한지 확인하세요:

```bash
deck version
```

version, commit, build date가 표시되어야 합니다. ldflags 없이 `go install`로 바이너리를 설치한 경우 버전 필드가 비어 있는데, 이는 버전 정보가 없는 소스 빌드에서는 예상되는 동작입니다.

## 셸 자동 완성

`deck`은 `deck completion <shell>` 명령으로 bash, zsh, fish, PowerShell용 자동 완성 스크립트를 생성합니다.

### 즉시 적용 (현재 세션에만)

```bash
source <(deck completion bash)   # bash
source <(deck completion zsh)    # zsh
deck completion fish | source    # fish
```

PowerShell:

```powershell
deck completion powershell | Out-String | Invoke-Expression
```

### 영구 설정

셸 초기화 파일에 sourcing 명령을 추가하면 새 세션마다 자동 완성이 자동으로 로드됩니다.

**bash** — `~/.bashrc`에 추가:

```bash
source <(deck completion bash)
```

**zsh** — `~/.zshrc`에 추가:

```bash
source <(deck completion zsh)
```

**fish** — fish 자동 완성 디렉터리에 작성:

```bash
deck completion fish > ~/.config/fish/completions/deck.fish
```

**PowerShell** — `$PROFILE`에 추가:

```powershell
deck completion powershell | Out-String | Invoke-Expression
```

## 다음 단계

[Quick Start](quick-start.md)를 따라 첫 워크스페이스를 만들고, lint하고, 번들을 빌드한 뒤 로컬에서 적용해 보세요.
