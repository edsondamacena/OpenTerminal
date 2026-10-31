// Campo de busca da barra lateral: filtra enquanto digita (FR-011).
import { t } from '../../i18n/pt-BR';
import { CloseIcon, SearchIcon } from '../../ui/icons';
import s from './hosts.module.css';

export interface SearchBoxProps {
  value: string;
  onChange: (value: string) => void;
  /** Seta para baixo leva o foco aos resultados. */
  onFocusResults: () => void;
}

export function SearchBox({ value, onChange, onFocusResults }: SearchBoxProps) {
  return (
    <div className={s.search}>
      <SearchIcon className={s.searchIcon} width={14} height={14} />
      <input
        type="search"
        className={s.searchInput}
        placeholder={t.hosts.searchHint}
        aria-label={t.hosts.search}
        spellCheck={false}
        autoComplete="off"
        value={value}
        onChange={(e) => {
          onChange(e.target.value);
        }}
        onKeyDown={(e) => {
          if (e.key === 'Escape' && value) {
            e.preventDefault();
            onChange('');
          } else if (e.key === 'ArrowDown') {
            e.preventDefault();
            onFocusResults();
          }
        }}
      />
      {value && (
        <button
          type="button"
          className={s.searchClear}
          aria-label={t.common.close}
          onClick={() => {
            onChange('');
          }}
        >
          <CloseIcon width={12} height={12} />
        </button>
      )}
    </div>
  );
}
