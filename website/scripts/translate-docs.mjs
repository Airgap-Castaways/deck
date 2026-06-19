#!/usr/bin/env node
// Translate English docs/*.md into the website/i18n/ko mirror with source-hash frontmatter.
//
// Usage:
//   node website/scripts/translate-docs.mjs [--force] [--no-polish] [docs/a.md ...]
//   node website/scripts/translate-docs.mjs --normalize-only        # fix particles in-place, no LLM
//
// Pipeline per file: translate (pass 1) -> polish/윤문 (pass 2) -> frontmatter merge ->
// deterministic Korean particle normalization.
//
// Engine: $DECK_TRANSLATE_CMD (stdin->stdout) if set, else `claude -p` (Claude Code, headless).
// When DECK_TRANSLATE_CMD is set the polish pass is skipped (keeps tests deterministic).
import {execFileSync, spawnSync} from 'node:child_process';
import {readFileSync, writeFileSync, mkdirSync, existsSync, readdirSync, statSync} from 'node:fs';
import {dirname, join, relative} from 'node:path';

const REPO = execFileSync('git', ['rev-parse', '--show-toplevel']).toString().trim();
const KO = join(REPO, 'website/i18n/ko/docusaurus-plugin-content-docs/current');
const GLOSSARY = join(REPO, 'website/i18n/TERMINOLOGY.md');

const CORE = [
  'docs/introduction.md', 'docs/quick-start.md', 'docs/workflow-model.md',
  'docs/cli.md', 'docs/apply-state.md', 'docs/offline-kubernetes.md',
  'docs/core-concepts/why-deck.md', 'docs/core-concepts/architecture.md',
];

const args = process.argv.slice(2);
const force = args.includes('--force');
const noPolish = args.includes('--no-polish');
const normalizeOnly = args.includes('--normalize-only');
const targets = args.filter((a) => !a.startsWith('--'));
const CUSTOM = process.env.DECK_TRANSLATE_CMD;

// ── Deterministic Korean particle normalization (A) ───────────────────────────
// Latin terms kept in English still take Korean particles, and an LLM flip-flops
// on whether a word carries a final consonant (받침). We pin each fixed term to
// one reading and rewrite the wrong particle form. `deck` is read 덱 (받침 ㄱ),
// so it always takes 은/이/을/과/으로 — never 는/가/를/와/로.
const BATCHIM_FIX = {'는': '은', '가': '이', '를': '을', '와': '과', '로': '으로'};
// term -> true when the term ends in a 받침 (consonant). Extend as needed.
const FIXED_TERMS = {deck: true};

function normalizeParticles(text) {
  for (const [term, hasBatchim] of Object.entries(FIXED_TERMS)) {
    if (!hasBatchim) continue; // only the 받침 case is wired up today
    // term, optional closing md delimiters (` * _), then a vowel-form particle
    // that is NOT followed by another Hangul syllable (avoids 로그/가능/와이 etc.).
    const re = new RegExp(
      `(?<![A-Za-z0-9_-])(${term})([\`*_]*)(는|가|를|와|로)(?![가-힣])`,
      'g',
    );
    text = text.replace(re, (_m, w, delim, p) => w + delim + BATCHIM_FIX[p]);
  }
  return text;
}

// ── Engine plumbing ───────────────────────────────────────────────────────────
function gitHash(path) {
  return execFileSync('git', ['hash-object', path], {cwd: REPO}).toString().trim();
}

function koPathFor(src) {
  return join(KO, src.replace(/^docs\//, ''));
}

function recordedHash(outPath) {
  if (!existsSync(outPath)) return null;
  const m = readFileSync(outPath, 'utf8').match(/^---\n([\s\S]*?)\n---/);
  if (!m) return null;
  const line = m[1].split('\n').find((l) => l.startsWith('source_hash:'));
  return line ? line.slice('source_hash:'.length).trim() : null;
}

function claude(instruction, input) {
  const r = spawnSync('claude', ['-p', instruction, '--output-format', 'text'],
    {input, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024});
  if (r.status !== 0) throw new Error(`claude failed: ${r.stderr || r.error}`);
  return r.stdout;
}

// Shared style rules for both passes (B). Keep terse — it is prepended to prompts.
const STYLE_RULES =
  '한국어 기술 문서 작성 규칙:\n' +
  '- 문어체 설명문. 종결어미는 "~합니다/~입니다"로 통일하고 구어체("~해요")는 쓰지 않습니다.\n' +
  '- 번역투/직역투를 피하고 자연스럽게 읽히도록 문장을 재구성합니다. 영어 어순을 그대로 옮기지 않습니다.\n' +
  '- 불필요한 피동("~되어집니다")과 군더더기를 제거하고 능동적이고 간결하게 씁니다.\n' +
  '- 조사는 앞말의 실제 발음 받침에 맞춥니다. 특히 "deck"은 "덱"으로 읽어 받침이 있으므로 ' +
  '항상 은/이/을/과/으로(는/가/를/와/로 금지)를 사용합니다.\n' +
  '- 코드 블록, 인라인 코드, CLI 명령/플래그, 스텝 종류 이름, 파일 경로, YAML 키, ' +
  '에러 코드(E_*), 헤딩 앵커({#...})는 영어 원문 그대로 둡니다.\n' +
  '- 마크다운 구조(헤딩 레벨, 표, 링크, 앵커, 프런트매터 --- 블록)를 정확히 보존합니다.';

function translate(content) {
  if (CUSTOM) {
    const r = spawnSync('sh', ['-c', CUSTOM], {input: content, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024});
    if (r.status !== 0) throw new Error(`DECK_TRANSLATE_CMD failed: ${r.stderr}`);
    return r.stdout;
  }
  const glossary = existsSync(GLOSSARY) ? readFileSync(GLOSSARY, 'utf8') : '';
  const instruction =
    '다음 영어 마크다운 문서를 한국어로 번역하세요. 번역된 마크다운만 출력하고 ' +
    '설명이나 머리말은 붙이지 마세요.\n\n' + STYLE_RULES + '\n\n용어집:\n\n' + glossary;
  return claude(instruction, content);
}

// Pass 2 — 윤문/polish (C). Refines naturalness without changing meaning or structure.
function polish(content) {
  if (CUSTOM || noPolish) return content;
  const instruction =
    '다음 한국어 기술 문서를 의미를 바꾸지 않고 더 자연스럽고 매끄러운 한국어로 윤문하세요. ' +
    '윤문된 마크다운만 출력하고 설명은 붙이지 마세요. 직역투를 없애고 문장을 매끄럽게 다듬되, ' +
    '내용을 추가하거나 빼지 않습니다.\n\n' + STYLE_RULES;
  return claude(instruction, content);
}

// Strip accidental ```markdown ... ``` fences the model sometimes wraps output in.
function stripFence(text) {
  const m = text.match(/^\s*```(?:markdown)?\n([\s\S]*?)\n```\s*$/);
  return m ? m[1] : text;
}

// Merge our provenance keys into a SINGLE frontmatter block. Docusaurus only
// parses the first block, so stacking would silently drop source keys (slug,
// sidebar_label). For frontmatter-less sources the model sometimes prepends a
// stray `---`; strip that lone artifact instead of swallowing body content.
function mergeFrontmatter(raw, meta, srcHadFm) {
  if (srcHadFm) {
    const m = raw.match(/^---\n([\s\S]*?)\n---\n?/);
    return m
      ? `---\n${meta}\n${m[1]}\n---\n${raw.slice(m[0].length)}`
      : `---\n${meta}\n---\n${raw}`;
  }
  return `---\n${meta}\n---\n${raw.replace(/^---\n/, '')}`;
}

function walk(dir) {
  const out = [];
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) out.push(...walk(p));
    else if (name.endsWith('.md')) out.push(p);
  }
  return out;
}

// ── --normalize-only: deterministic particle fix over existing ko files ────────
if (normalizeOnly) {
  const files = targets.length ? targets.map((t) => existsSync(t) ? t : koPathFor(t)) : walk(KO);
  let fixed = 0;
  for (const f of files) {
    if (!existsSync(f)) { console.error(`skip (missing): ${f}`); continue; }
    const before = readFileSync(f, 'utf8');
    const after = normalizeParticles(before);
    if (after !== before) { writeFileSync(f, after); fixed++; console.log(`normalized: ${relative(REPO, f)}`); }
  }
  console.log(`\nDone: ${fixed} file(s) normalized.`);
  process.exit(0);
}

// ── Translation run ───────────────────────────────────────────────────────────
const sources = targets.length ? targets : CORE;
let translated = 0, skipped = 0;
for (const src of sources) {
  const abs = join(REPO, src);
  if (!existsSync(abs)) { console.error(`skip (missing): ${src}`); continue; }
  const hash = gitHash(src);
  const out = koPathFor(src);
  if (!force && recordedHash(out) === hash) { skipped++; console.log(`unchanged: ${src}`); continue; }
  const srcText = readFileSync(abs, 'utf8');
  const srcHadFm = /^---\n[\s\S]*?\n---/.test(srcText);
  const meta = `source: ${src}\nsource_hash: ${hash}`;
  const raw = stripFence(polish(translate(srcText)));
  const content = normalizeParticles(mergeFrontmatter(raw, meta, srcHadFm));
  mkdirSync(dirname(out), {recursive: true});
  writeFileSync(out, content);
  translated++;
  console.log(`translated: ${src} -> ${relative(REPO, out)}`);
}
console.log(`\nDone: ${translated} translated, ${skipped} unchanged.`);
