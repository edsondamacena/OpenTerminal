// Opções lado a lado (FR-158a, FR-166): uma escolha entre poucas, com indicador que desliza
// até a selecionada. Teclado: setas movem a seleção (padrão de grupo de rádio).
import { useId, useRef, type ReactNode } from 'react';
import { useSlidingIndicator } from './motion';
import s from './ui.module.css';

export interface SegmentedOption<V extends string | number> {
  value: V;
  label: ReactNode;
  /** Nome acessível quando o rótulo é só um desenho. */
  ariaLabel?: string;
}

export interface SegmentedProps<V extends string | number> {
  label: string;
  value: V;
  options: readonly SegmentedOption<V>[];
  onChange: (value: V) => void;
  hint?: string;
}

export function Segmented<V extends string | number>({
  label,
  value,
  options,
  onChange,
  hint,
}: SegmentedProps<V>) {
  const id = useId();
  const groupRef = useRef<HTMLDivElement>(null);
  const indicator = useSlidingIndicator(groupRef, value, 'x', '[aria-checked="true"]');

  function move(step: number) {
    const i = options.findIndex((o) => o.value === value);
    const next = options[(i + step + options.length) % options.length];
    if (!next) return;
    onChange(next.value);
    requestAnimationFrame(() => {
      groupRef.current?.querySelector<HTMLElement>('[aria-checked="true"]')?.focus();
    });
  }

  return (
    <div className={s.field}>
      <span className={s.label} id={`${id}-label`}>
        {label}
      </span>
      <div
        ref={groupRef}
        role="radiogroup"
        aria-labelledby={`${id}-label`}
        className={s.segmented}
        style={indicator ?? undefined}
        data-indicator={indicator ? '' : undefined}
        onKeyDown={(e) => {
          if (e.key === 'ArrowRight' || e.key === 'ArrowDown') move(1);
          else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') move(-1);
          else return;
          e.preventDefault();
        }}
      >
        {options.map((option) => {
          const checked = option.value === value;
          return (
            <button
              key={String(option.value)}
              type="button"
              role="radio"
              aria-checked={checked}
              aria-label={option.ariaLabel}
              tabIndex={checked ? 0 : -1}
              className={s.segment}
              onClick={() => {
                onChange(option.value);
              }}
            >
              {option.label}
            </button>
          );
        })}
      </div>
      {hint && <span className={s.hint}>{hint}</span>}
    </div>
  );
}
