// Busca no histórico do terminal (FR-142): destaque, anterior/próxima, contagem, maiúsculas e regex.
import { useEffect, useRef, useState } from 'react';
import type { ISearchOptions } from '@xterm/addon-search';
import { t } from '../../i18n/pt-BR';
import { useUi } from '../../stores/ui';
import { useWorkspace } from '../../stores/workspace';
import { Button } from '../../ui/Button';
import {
  CaseSensitiveIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  CloseIcon,
  RegexIcon,
} from '../../ui/icons';
import { focusTerminal, terminalOf } from './terminalRegistry';
import s from './terminal.module.css';

function validRegex(source: string): boolean {
  try {
    new RegExp(source);
    return true;
  } catch {
    return false;
  }
}

export function SearchBar({ sessionId }: { sessionId: string }) {
  const handle = terminalOf(sessionId);
  const tokens = useUi((u) => u.tokens);
  const close = useWorkspace((w) => w.setSearchFor);
  const inputRef = useRef<HTMLInputElement>(null);
  // Texto selecionado no terminal vira a busca inicial.
  const [query, setQuery] = useState(() => handle?.term.getSelection().split('\n')[0] ?? '');
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [regex, setRegex] = useState(false);
  const [result, setResult] = useState<{ index: number; count: number } | null>(null);
  const invalid = regex && query !== '' && !validRegex(query);

  const options: ISearchOptions = {
    caseSensitive,
    regex,
    decorations: {
      matchBackground: tokens.terminalMatch,
      matchOverviewRuler: tokens.terminalMatch,
      activeMatchBackground: tokens.terminalMatchActive,
      activeMatchColorOverviewRuler: tokens.terminalMatchActive,
    },
  };

  useEffect(() => {
    inputRef.current?.focus();
    inputRef.current?.select();
  }, []);

  useEffect(() => {
    const sub = handle?.search.onDidChangeResults(({ resultIndex, resultCount }) => {
      setResult({ index: resultIndex, count: resultCount });
    });
    return () => {
      sub?.dispose();
      handle?.search.clearDecorations();
    };
  }, [handle]);

  // Busca incremental enquanto digita.
  useEffect(() => {
    if (!handle) return;
    if (query === '' || invalid) {
      handle.search.clearDecorations();
      return;
    }
    handle.search.findNext(query, { ...options, incremental: true });
    // `options` é derivado destes mesmos valores.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [handle, query, invalid]);

  // Trocar maiúsculas/regex/tema: o addon guarda o resultado anterior para o mesmo texto, então
  // limpa antes de buscar de novo (senão a contagem fica presa em "Nenhum resultado").
  useEffect(() => {
    if (!handle || query === '' || invalid) return;
    handle.search.clearDecorations();
    handle.search.findNext(query, { ...options, incremental: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [caseSensitive, regex, tokens]);

  function find(direction: 'next' | 'previous') {
    if (!handle || query === '' || invalid) return;
    if (direction === 'next') handle.search.findNext(query, options);
    else handle.search.findPrevious(query, options);
  }

  function dismiss() {
    close(null);
    focusTerminal(sessionId);
  }

  const status = invalid
    ? t.terminal.search.invalidRegex
    : result && query !== ''
      ? t.terminal.search.count(result.index, result.count)
      : '';

  return (
    <div
      className={`${s.searchBar} raised glass-edge`}
      role="search"
      aria-label={t.terminal.search.label}
      data-search-for={sessionId}
    >
      <input
        ref={inputRef}
        className={s.searchInput}
        type="text"
        spellCheck={false}
        placeholder={t.terminal.search.placeholder}
        aria-label={t.terminal.search.label}
        aria-invalid={invalid}
        value={query}
        onChange={(e) => {
          setQuery(e.target.value);
        }}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault();
            find(e.shiftKey ? 'previous' : 'next');
          } else if (e.key === 'Escape') {
            e.preventDefault();
            dismiss();
          }
        }}
      />
      <span
        className={`${s.searchCount} ${invalid ? s.searchInvalid : ''}`}
        role="status"
        aria-live="polite"
      >
        {status}
      </span>
      <Button
        variant="ghost"
        iconOnly
        className={s.searchToggle}
        aria-pressed={caseSensitive}
        aria-label={t.terminal.search.caseSensitive}
        title={t.terminal.search.caseSensitive}
        onClick={() => {
          setCaseSensitive((v) => !v);
        }}
      >
        <CaseSensitiveIcon />
      </Button>
      <Button
        variant="ghost"
        iconOnly
        className={s.searchToggle}
        aria-pressed={regex}
        aria-label={t.terminal.search.regex}
        title={t.terminal.search.regex}
        onClick={() => {
          setRegex((v) => !v);
        }}
      >
        <RegexIcon />
      </Button>
      <span className={s.searchDivider} />
      <Button
        variant="ghost"
        iconOnly
        aria-label={t.terminal.search.previous}
        title={t.terminal.search.previous}
        disabled={!result?.count}
        onClick={() => {
          find('previous');
        }}
      >
        <ChevronUpIcon />
      </Button>
      <Button
        variant="ghost"
        iconOnly
        aria-label={t.terminal.search.next}
        title={t.terminal.search.next}
        disabled={!result?.count}
        onClick={() => {
          find('next');
        }}
      >
        <ChevronDownIcon />
      </Button>
      <Button
        variant="ghost"
        iconOnly
        aria-label={t.terminal.search.close}
        title={t.terminal.search.close}
        onClick={dismiss}
      >
        <CloseIcon />
      </Button>
    </div>
  );
}
