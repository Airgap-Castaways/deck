#!/usr/bin/env node
// Translate English docs/*.md into website/i18n/ko mirror with source-hash frontmatter.
// Usage: node website/scripts/translate-docs.mjs [--force] [docs/a.md docs/b.md ...]
// Engine: $DECK_TRANSLATE_CMD (stdin->stdout) if set, else `claude -p` (Claude Code, headless).
import {execFileSync, spawnSync} from 'node:child_process';
import {readFileSync, writeFileSync, mkdirSync, existsSync} from 'node:fs';
import {dirname, join, relative} from 'node:path';

const REPO = execFileSync('git', ['rev-parse', '--show-toplevel']).toString().trim();
const KO = join(REPO, 'website/i18n/ko/docusaurus-plugin-content-docs/current');
const GLOSSARY = join(REPO, 'website/i18n/TERMINOLOGY.md');

const CORE = [
  'docs/README.md', 'docs/quick-start.md', 'docs/workflow-model.md',
  'docs/cli.md', 'docs/apply-state.md', 'docs/offline-kubernetes.md',
  'docs/core-concepts/why-deck.md', 'docs/core-concepts/architecture.md',
];

const args = process.argv.slice(2);
const force = args.includes('--force');
const targets = args.filter((a) => a !== '--force');
const sources = targets.length ? targets : CORE;

function gitHash(path) {
  return execFileSync('git', ['hash-object', path], {cwd: REPO}).toString().trim();
}

function koPathFor(src) {
  const rel = src.replace(/^docs\//, '');
  return join(KO, rel);
}

function recordedHash(outPath) {
  if (!existsSync(outPath)) return null;
  const s = readFileSync(outPath, 'utf8');
  const m = s.match(/^---\n([\s\S]*?)\n---/);
  if (!m) return null;
  const line = m[1].split('\n').find((l) => l.startsWith('source_hash:'));
  return line ? line.slice('source_hash:'.length).trim() : null;
}

function translate(content) {
  const glossary = existsSync(GLOSSARY) ? readFileSync(GLOSSARY, 'utf8') : '';
  const custom = process.env.DECK_TRANSLATE_CMD;
  if (custom) {
    const r = spawnSync('sh', ['-c', custom], {input: content, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024});
    if (r.status !== 0) throw new Error(`DECK_TRANSLATE_CMD failed: ${r.stderr}`);
    return r.stdout;
  }
  const instruction =
    'Translate the following English Markdown document into Korean. ' +
    'Output ONLY the translated Markdown, with no preamble or commentary. ' +
    'Preserve all Markdown structure, links, anchors, and tables. ' +
    'Keep code blocks, inline code, CLI commands/flags, step-kind names, file paths, ' +
    'YAML keys, and error codes (E_*) verbatim in English. Follow this glossary:\n\n' +
    glossary;
  const r = spawnSync('claude', ['-p', instruction, '--output-format', 'text'],
    {input: content, encoding: 'utf8', maxBuffer: 64 * 1024 * 1024});
  if (r.status !== 0) throw new Error(`claude failed: ${r.stderr || r.error}`);
  return r.stdout;
}

let translated = 0, skipped = 0;
for (const src of sources) {
  const abs = join(REPO, src);
  if (!existsSync(abs)) { console.error(`skip (missing): ${src}`); continue; }
  const hash = gitHash(src);
  const out = koPathFor(src);
  if (!force && recordedHash(out) === hash) { skipped++; console.log(`unchanged: ${src}`); continue; }
  const body = translate(readFileSync(abs, 'utf8'));
  const fm = `---\nsource: ${src}\nsource_hash: ${hash}\n---\n`;
  mkdirSync(dirname(out), {recursive: true});
  writeFileSync(out, fm + body);
  translated++;
  console.log(`translated: ${src} -> ${relative(REPO, out)}`);
}
console.log(`\nDone: ${translated} translated, ${skipped} unchanged.`);
