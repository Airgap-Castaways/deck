# Translation Terminology (EN → KO)

Rules for translating `docs/` (English source) into `website/i18n/ko/`:

- **Keep in English (do NOT translate):** code identifiers, CLI commands and flags (`deck apply`, `--root`, `--dry-run`), step-kind names (`CopyFile`, `DownloadImage`), file paths, YAML keys, environment variables, and error codes (`E_*`).
- **Keep code blocks and inline code verbatim.** Translate only prose and headings.
- **Preserve Markdown structure** — heading levels, tables, links, anchors — exactly.
- **Tone:** technical and concise.

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
| step / step kind | 스텝 / 스텝 종류 | step-kind names stay English |
| workspace | 워크스페이스 | |
| component fragment | 컴포넌트 프래그먼트 | |
| manifest | 매니페스트 | |
| source locator | 소스 로케이터 | |
| air-gapped | 에어갭(망분리) | |
| offline | 오프라인 | |
| content server | 콘텐츠 서버 | |
| registry | 레지스트리 | |
| artifact | 아티팩트 | |
| layer | 레이어 | OCI/container image layer |
| image | 이미지 | container image context |
| highlights | 주요 변경 사항 | release notes context |
