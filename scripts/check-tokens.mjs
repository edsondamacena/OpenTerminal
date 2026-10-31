#!/usr/bin/env node

import { readdirSync, readFileSync } from 'node:fs';
import { relative, resolve, sep } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const SOURCE_EXTENSIONS = new Set(['.css', '.ts', '.tsx', '.js', '.jsx']);
const COLOR_LITERAL = /#[0-9a-f]{3,8}\b|\brgba?\s*\(|\bhsla?\s*\(/gi;

function extensionOf(path) {
  const dot = path.lastIndexOf('.');
  return dot < 0 ? '' : path.slice(dot);
}

function slash(path) {
  return path.split(sep).join('/');
}

function lineOf(source, offset) {
  let line = 1;
  for (let index = 0; index < offset; index++) {
    if (source.charCodeAt(index) === 10) line++;
  }
  return line;
}

function withoutBlockComments(source) {
  return source.replace(/\/\*[\s\S]*?\*\//g, (comment) => comment.replace(/[^\r\n]/g, ' '));
}

function allowedTokenValue(value) {
  const normalized = value.trim().replace(/!important\s*$/i, '').trim();
  return (
    /^var\(\s*--[\w-]+\s*\)$/i.test(normalized) ||
    /^(?:currentColor|transparent|inherit|none)$/i.test(normalized)
  );
}

function pushMatches(violations, source, pattern, reason, baseOffset = 0) {
  pattern.lastIndex = 0;
  for (const match of source.matchAll(pattern)) {
    violations.push({ line: lineOf(source, match.index ?? 0) + lineOf(source, baseOffset) - 1, reason });
  }
}

function scanCSS(source) {
  const clean = withoutBlockComments(source);
  const violations = [];
  pushMatches(violations, clean, COLOR_LITERAL, 'cor literal; use um token do tema');

  const declaration = /(^|[;{]\s*)(font-family|box-shadow)\s*:\s*([^;}]+)/gim;
  for (const match of clean.matchAll(declaration)) {
    const property = match[2].toLowerCase();
    const value = match[3].trim();
    if (allowedTokenValue(value)) continue;
    const valueOffset = (match.index ?? 0) + match[0].lastIndexOf(match[3]);
    violations.push({
      line: lineOf(clean, valueOffset),
      reason: `${property} literal; use var(--token)`,
    });
  }
  return violations;
}

function scanInlineStyles(source) {
  const violations = [];
  const styleObject = /style\s*=\s*\{\{([\s\S]*?)\}\}/g;
  for (const style of source.matchAll(styleObject)) {
    const body = style[1];
    const bodyOffset = (style.index ?? 0) + style[0].indexOf(body);
    COLOR_LITERAL.lastIndex = 0;
    for (const color of body.matchAll(COLOR_LITERAL)) {
      violations.push({
        line: lineOf(source, bodyOffset + (color.index ?? 0)),
        reason: 'cor literal em style; use um token do tema',
      });
    }
    const property = /\b(fontFamily|boxShadow)\s*:\s*(['"`])([\s\S]*?)\2/g;
    for (const match of body.matchAll(property)) {
      if (allowedTokenValue(match[3])) continue;
      violations.push({
        line: lineOf(source, bodyOffset + (match.index ?? 0)),
        reason: `${match[1]} literal em style; use var(--token)`,
      });
    }
  }
  return violations;
}

export function scanText(file, source) {
  const normalized = slash(file);
  const inTheme = normalized.includes('/frontend/src/theme/') || normalized.startsWith('frontend/src/theme/');
  const generatedFonts = normalized.endsWith('/frontend/src/styles/fonts.css') || normalized === 'frontend/src/styles/fonts.css';
  const inUI = normalized.includes('/frontend/src/ui/') || normalized.startsWith('frontend/src/ui/');
  const extension = extensionOf(normalized);
  const violations = [];

  if (extension === '.css' && !inTheme && !generatedFonts) {
    violations.push(...scanCSS(source));
  }
  if (['.tsx', '.jsx'].includes(extension) && !inTheme) {
    violations.push(...scanInlineStyles(source));
  }
  if (['.tsx', '.jsx'].includes(extension) && !inUI) {
    for (const match of source.matchAll(/<svg\b/g)) {
      violations.push({ line: lineOf(source, match.index ?? 0), reason: 'SVG inline fora de frontend/src/ui' });
    }
  }
  return violations;
}

function sourceFiles(directory) {
  const files = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) files.push(...sourceFiles(path));
    else if (SOURCE_EXTENSIONS.has(extensionOf(entry.name))) files.push(path);
  }
  return files;
}

export function scanProject(root) {
  const sourceRoot = resolve(root, 'frontend', 'src');
  const violations = [];
  for (const path of sourceFiles(sourceRoot)) {
    const file = slash(relative(root, path));
    for (const violation of scanText(file, readFileSync(path, 'utf8'))) {
      violations.push({ file, ...violation });
    }
  }
  return violations.sort((a, b) => a.file.localeCompare(b.file) || a.line - b.line || a.reason.localeCompare(b.reason));
}

function main() {
  const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
  const violations = scanProject(root);
  for (const violation of violations) {
    console.error(`${violation.file}:${violation.line}: ${violation.reason}`);
  }
  if (violations.length > 0) {
    console.error(`\n${violations.length} violação(ões) de tokens encontrada(s).`);
    process.exitCode = 1;
  } else {
    console.log('Tokens visuais: nenhum literal fora do tema.');
  }
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? '').href) main();
