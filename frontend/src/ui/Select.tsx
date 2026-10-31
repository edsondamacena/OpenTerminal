// Seleção própria (FR-166): botão com lista flutuante, navegável por teclado e com busca pela
// primeira letra. Mesmo visual dos outros campos; a lista usa o Popover (fica inteira na janela).
import { useEffect, useId, useRef, useState, type ReactNode } from 'react';
import { CheckIcon, ChevronDownIcon } from './icons';
import { Popover } from './Popover';
import s from './ui.module.css';

export interface SelectOption<V extends string> {
  value: V;
  label: string;
  /** Ícone à esquerda do rótulo, na lista e no valor escolhido. */
  icon?: ReactNode;
}

export interface SelectProps<V extends string> {
  label: string;
  value: V;
  options: readonly SelectOption<V>[];
  onChange: (value: V) => void;
  disabled?: boolean;
  hint?: string;
}

export function Select<V extends string>({
  label,
  value,
  options,
  onChange,
  disabled,
  hint,
}: SelectProps<V>) {
  const id = useId();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const listRef = useRef<HTMLUListElement | null>(null);
  // Lista aberta = há âncora (o botão, com a largura medida ao abrir).
  const [anchor, setAnchor] = useState<{ el: HTMLElement; width: number } | null>(null);
  const open = anchor !== null;
  const setClosed = () => {
    setAnchor(null);
  };
  const selectedIndex = Math.max(
    0,
    options.findIndex((o) => o.value === value),
  );
  const [active, setActive] = useState(selectedIndex);
  const current = options.find((o) => o.value === value);

  useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent) => {
      const target = e.target as Node;
      if (listRef.current?.contains(target) || buttonRef.current?.contains(target)) return;
      setAnchor(null);
    };
    const onBlur = () => {
      setAnchor(null);
    };
    window.addEventListener('mousedown', close, true);
    window.addEventListener('blur', onBlur);
    return () => {
      window.removeEventListener('mousedown', close, true);
      window.removeEventListener('blur', onBlur);
    };
  }, [open]);

  useEffect(() => {
    if (open)
      listRef.current
        ?.querySelector(`[data-index="${String(active)}"]`)
        ?.scrollIntoView({ block: 'nearest' });
  }, [open, active]);

  function show() {
    const el = buttonRef.current;
    if (!el) return;
    setActive(selectedIndex);
    setAnchor({ el, width: el.offsetWidth });
  }

  function choose(index: number) {
    const option = options[index];
    setClosed();
    buttonRef.current?.focus();
    if (option && option.value !== value) onChange(option.value);
  }

  function onKeyDown(e: React.KeyboardEvent) {
    const n = options.length;
    if (!open) {
      if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) {
        e.preventDefault();
        show();
      }
      return;
    }
    if (e.key === 'ArrowDown') setActive((active + 1) % n);
    else if (e.key === 'ArrowUp') setActive((active - 1 + n) % n);
    else if (e.key === 'Home') setActive(0);
    else if (e.key === 'End') setActive(n - 1);
    else if (e.key === 'Enter' || e.key === ' ') choose(active);
    else if (e.key === 'Escape') setClosed();
    else if (e.key === 'Tab') {
      setClosed();
      return;
    } else if (e.key.length === 1) {
      // Primeira letra: vai ao próximo item que começa com ela.
      const letter = e.key.toLocaleLowerCase('pt-BR');
      for (let step = 1; step <= n; step++) {
        const i = (active + step) % n;
        if (options[i]?.label.toLocaleLowerCase('pt-BR').startsWith(letter)) {
          setActive(i);
          break;
        }
      }
    } else return;
    e.preventDefault();
  }

  return (
    <div className={s.field}>
      <span className={s.label} id={`${id}-label`}>
        {label}
      </span>
      <button
        ref={buttonRef}
        type="button"
        role="combobox"
        aria-labelledby={`${id}-label`}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={open ? `${id}-list` : undefined}
        aria-activedescendant={open ? `${id}-${String(active)}` : undefined}
        disabled={disabled}
        className={s.selectButton}
        onClick={() => {
          if (open) setClosed();
          else show();
        }}
        onKeyDown={onKeyDown}
      >
        <span className={s.selectValue}>
          {current?.icon}
          {current?.label ?? ''}
        </span>
        <ChevronDownIcon className={s.selectChevron} />
      </button>
      {hint && <span className={s.hint}>{hint}</span>}
      {anchor && (
        <Popover anchor={anchor.el}>
          {({ ref, style }) => (
            <ul
              ref={(el) => {
                listRef.current = el;
                if (typeof ref === 'function') ref(el);
              }}
              id={`${id}-list`}
              role="listbox"
              aria-labelledby={`${id}-label`}
              className={`${s.menu} ${s.selectList} raised glass-edge`}
              style={{ ...style, minWidth: anchor.width }}
            >
              {options.map((option, i) => (
                <li
                  key={option.value}
                  id={`${id}-${String(i)}`}
                  data-index={i}
                  role="option"
                  aria-selected={option.value === value}
                  data-active={i === active || undefined}
                  className={s.selectOption}
                  onMouseMove={() => {
                    if (i !== active) setActive(i);
                  }}
                  onMouseDown={(e) => {
                    e.preventDefault();
                    choose(i);
                  }}
                >
                  <span className={s.selectCheck}>{option.value === value && <CheckIcon />}</span>
                  {option.icon}
                  {option.label}
                </li>
              ))}
            </ul>
          )}
        </Popover>
      )}
    </div>
  );
}
