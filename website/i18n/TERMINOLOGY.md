# Translation Terminology (EN → KO)

Rules for translating `docs/` (English source) into `website/i18n/ko/`.

## Hard rules (do not break)

- **Keep in English (do NOT translate):** code identifiers, CLI commands and flags (`deck apply`, `--root`, `--dry-run`), step-kind names (`CopyFile`, `DownloadImage`), file paths, YAML keys, environment variables, error codes (`E_*`), and heading anchors (`{#id}`).
- **Keep code blocks and inline code verbatim.** Translate only prose and headings.
- **Preserve Markdown structure** — heading levels, tables, links, anchors, the frontmatter `---` block — exactly. Emit a single frontmatter block.

## Style (write like a Korean tech doc, not a translation)

- 문어체 설명문. 종결어미는 **~합니다/~입니다**로 통일하고 구어체(~해요)는 쓰지 않습니다.
- 번역투/직역투를 피하고 자연스럽게 읽히도록 문장을 재구성합니다. 영어 어순을 그대로 옮기지 않습니다.
- 불필요한 피동(~되어집니다)과 군더더기를 제거하고 능동적이고 간결하게 씁니다.
- 한 문장이 길면 끊어 씁니다. 명사 나열보다 서술형 연결을 선호합니다.

## 조사 규칙 (particles)

영문 단어를 그대로 둘 때 조사는 **그 단어의 실제 발음 받침**에 맞춥니다. 고정 표기는 다음과 같으며, 빌드 후처리(`translate-docs.mjs`의 particle normalizer)가 이를 강제합니다:

| 단어 | 읽기 | 받침 | 사용 조사 | 금지 |
|---|---|---|---|---|
| `deck` | 덱 | 있음(ㄱ) | 은 / 이 / 을 / 과 / 으로 | 는 / 가 / 를 / 와 / 로 |

예) `deck은`, `deck이`, `deck을`, `deck과`, `deck으로`.

## Term mapping

| English | Korean | Notes |
|---|---|---|
| workflow | 워크플로 | |
| scenario | 시나리오 | |
| bundle | 번들 | |
| bundle root | 번들 루트 | |
| prepare (phase) | 준비 (단계) | |
| apply (phase) | 적용 (단계) | |
| phase | 단계 | |
| parallelism / parallel group | 병렬 처리 / 병렬 그룹 | |
| step / step kind | 스텝 / 스텝 종류 | step-kind names stay English |
| step envelope | 스텝 엔벨로프 | |
| register (capture output) | register | field key; keep English, gloss "출력 캡처" |
| condition | 조건 | CEL `when` context |
| template / templating | 템플릿 / 템플릿화 | |
| precedence | 우선순위 | |
| overlay (vars file) | 오버레이 | |
| workspace | 워크스페이스 | |
| component fragment | 컴포넌트 프래그먼트 | |
| manifest | 매니페스트 | |
| source locator | 소스 로케이터 | |
| air-gapped | 에어갭(망분리) | |
| offline | 오프라인 | |
| content server | 콘텐츠 서버 | |
| daemon | 데몬 | |
| audit log | 감사 로그 | |
| registry | 레지스트리 | |
| artifact | 아티팩트 | |
| checksum | 체크섬 | |
| layer | 레이어 | OCI/container image layer |
| image | 이미지 | container image context |
| runtime (facts) | 런타임 | `runtime.host` context |
| host / node | 호스트 / 노드 | |
| control-plane / worker | 컨트롤 플레인 / 워커 | Kubernetes roles |
| resumable / resume | 재개 가능 / 재개 | apply state context |
| highlights | 주요 변경 사항 | release notes context |
