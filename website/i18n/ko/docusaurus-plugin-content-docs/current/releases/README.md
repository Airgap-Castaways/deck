---
source: docs/releases/README.md
source_hash: 4eb20f2faf59b0cf45ef53e058a5ea83abad7460
---
# 릴리스 노트

이 디렉터리는 GitHub 릴리스 노트에 삽입되는 릴리스별 주요 변경 사항 파일을 보관합니다.

## 동작 방식

각 릴리스마다 `docs/releases/v<MAJOR>.<MINOR>.<PATCH>.md`(예: `docs/releases/v0.3.0.md`) 형식의 파일을 저장소에 추가할 수 있습니다. 이 파일에는 사용자 관점에서 작성된 3~6개의 주요 변경 사항 항목이 담깁니다.

릴리스 워크플로가 실행되면:

1. `docs/releases/<tag>.md`가 존재하면, 그 내용이 `DECK_RELEASE_HIGHLIGHTS` 환경 변수로 로드됩니다.
2. GoReleaser가 그 변수를 읽어, GitHub 릴리스 헤더에서 자동 생성된 영문 feat/fix 체인지로그 위에 렌더링합니다.
3. 결과 릴리스 노트에는 주요 변경 사항 블록, 수평선, 그리고 기본 릴리스 아티팩트 텍스트와 그 뒤에 생성된 체인지로그가 포함됩니다.

해당 태그에 대한 파일이 없으면, 헤더는 기본 텍스트("Release artifacts for `deck` are attached below...")로 폴백되며 주요 변경 사항 블록 없이 릴리스가 진행됩니다. 주요 변경 사항 파일 제공은 선택 사항입니다.

## 파일 형식

각 파일은 `## Highlights` 헤딩으로 시작하고 그 뒤에 항목 목록이 와야 합니다. `docs/releases/TEMPLATE.md`를 시작점으로 사용하세요.

## 오프라인 체인지로그

주요 변경 사항 파일은 코드와 함께 커밋되므로, 저장소와 함께 이동하며 GitHub 릴리스 페이지에 접근할 수 없는 에어갭(망분리) 사이트를 위한 사람이 읽을 수 있는 오프라인 체인지로그 역할을 합니다.
