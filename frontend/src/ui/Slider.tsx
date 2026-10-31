// Controle deslizante (FR-158a): escolhe um valor da faixa sem digitar; o valor fica visível.
import { useId, type CSSProperties } from 'react';
import s from './ui.module.css';

export interface SliderProps {
  label: string;
  value: number;
  min: number;
  max: number;
  step?: number;
  onChange: (value: number) => void;
  /** Texto do valor mostrado ao lado do rótulo (ex.: "14 px", "80%"). */
  format?: (value: number) => string;
  hint?: string | undefined;
  disabled?: boolean;
}

export function Slider({
  label,
  value,
  min,
  max,
  step = 1,
  onChange,
  format = String,
  hint,
  disabled,
}: SliderProps) {
  const id = useId();
  const fill = ((value - min) / (max - min)) * 100;
  return (
    <div className={s.field}>
      <div className={s.sliderHead}>
        <label className={s.label} htmlFor={id}>
          {label}
        </label>
        <output className={s.sliderValue} htmlFor={id}>
          {format(value)}
        </output>
      </div>
      <input
        id={id}
        type="range"
        className={s.slider}
        min={min}
        max={max}
        step={step}
        value={value}
        disabled={disabled}
        aria-valuetext={format(value)}
        style={{ '--fill': `${String(fill)}%` } as CSSProperties}
        onChange={(e) => {
          onChange(Number(e.target.value));
        }}
      />
      {hint && <span className={s.hint}>{hint}</span>}
    </div>
  );
}
