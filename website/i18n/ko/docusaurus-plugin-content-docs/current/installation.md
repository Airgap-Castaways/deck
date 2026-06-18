---
source: docs/installation.md
source_hash: 0a34c7ac643f338ca0acb507cb577da2764fc6d0
---
# 설치

## deck이 실행되는 환경

`deck`은 런타임 의존성이 없는 단일 정적 바이너리로 배포됩니다.

| 단계 | 플랫폼 |
|---|---|
| 작성, 린트, prepare, 번들링 | macOS, Linux, Windows |
| 적용(대상) | Linux — RHEL / Rocky / CentOS 또는 Debian / Ubuntu |

대상 머신에 Go를 설치할 필요는 **없습니다**. `deck` 바이너리뿐 아니라 워크플로 실행에 필요한 모든 것이 번들 하나에 담겨 함께 이동합니다. 워크플로를 작성하고 prepare하는 머신에만 PATH에 `deck` 바이너리가 있으면 됩니다.

릴리스 아티팩트는 다음 환경을 대상으로 빌드합니다.

- `linux/amd64`, `linux/arm64`
- `darwin/amd64`, `darwin/arm64`

Windows 바이너리는 아직 릴리스로 공개하지 않았습니다. Windows 사용자는 소스에서 직접 빌드할 수 있습니다(아래 참고).

## Homebrew (권장)

Airgap Castaways tap에서 최신 릴리스를 배포합니다.

```bash
brew install Airgap-Castaways/tap/deck
```

이 명령은 `Airgap-Castaways/homebrew-tap`을 tap으로 추가하고 `deck` 포뮬러를 설치합니다. 업그레이드는 평소의 `brew upgrade` 절차를 그대로 따릅니다.

## GitHub Releases (바이너리 tarball)

[릴리스 페이지](https://github.com/Airgap-Castaways/deck/releases)에서 사용 중인 OS와 아키텍처에 맞는 아카이브를 내려받습니다.

아카이브 이름은 `deck_<version>_<os>_<arch>.tar.gz` 형식을 따릅니다. 예를 들면 다음과 같습니다.

- `deck_1.2.3_linux_amd64.tar.gz`
- `deck_1.2.3_darwin_arm64.tar.gz`

압축을 풀고 바이너리를 PATH에 둡니다.

```bash
# Example for Linux amd64 — adjust the filename for your version and arch
tar -xzf deck_<version>_linux_amd64.tar.gz
sudo mv deck /usr/local/bin/deck
```

바이너리를 실행하기 전에 **체크섬을 검증합니다**. 각 릴리스는 아카이브와 함께 `checksums.txt` 파일을 배포합니다.

```bash
sha256sum --check --ignore-missing checksums.txt
```

### Debian 및 RHEL 패키지

릴리스 페이지에서는 동일한 바이너리로 빌드한 `.deb` 및 `.rpm` 패키지도 배포합니다. 이 패키지는 `deck`을 `/usr/bin/deck`에 설치합니다.

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

이 방식으로 설치한 바이너리에는 버전 스탬프 메타데이터가 들어가지 않습니다(`deck version` 출력에서 commit과 date 필드가 비어 있습니다). 버전 정보까지 함께 빌드하려면 저장소를 클론한 뒤 Makefile을 사용합니다.

```bash
git clone https://github.com/Airgap-Castaways/deck.git
cd deck
make build
```

`make build`의 결과물은 저장소 루트에 `deck`이라는 이름으로 생성됩니다.

## 검증

바이너리가 설치되어 정상적으로 실행되는지 확인합니다.

```bash
deck version
```

버전, commit, 빌드 날짜가 표시됩니다. ldflags 없이 `go install`로 설치했다면 버전 필드가 비어 있는데, 이는 버전 정보가 없는 소스 빌드에서는 정상적인 현상입니다.

## 셸 자동 완성

`deck`은 `deck completion <shell>` 명령으로 bash, zsh, fish, PowerShell용 자동 완성 스크립트를 생성합니다.

### 즉시 적용(현재 세션에만 해당)

```bash
source <(deck completion bash)   # bash
source <(deck completion zsh)    # zsh
deck completion fish | source    # fish
```

PowerShell의 경우:

```powershell
deck completion powershell | Out-String | Invoke-Expression
```

### 영구 설정

셸 초기화 파일에 sourcing 명령을 추가해 두면 새 세션을 열 때마다 자동 완성이 자동으로 로드됩니다.

**bash** — `~/.bashrc`에 추가합니다.

```bash
source <(deck completion bash)
```

**zsh** — `~/.zshrc`에 추가합니다.

```bash
source <(deck completion zsh)
```

**fish** — fish 자동 완성 디렉터리에 기록합니다.

```bash
deck completion fish > ~/.config/fish/completions/deck.fish
```

**PowerShell** — `$PROFILE`에 추가합니다.

```powershell
deck completion powershell | Out-String | Invoke-Expression
```

## 다음 단계

[빠른 시작](quick-start.md)을 따라 첫 워크스페이스를 만들고, 린트하고, 번들을 빌드한 뒤 로컬에 적용해 봅니다.
