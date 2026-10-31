// Caixa de seleção própria: o input nativo continua (teclado, leitor de tela, formulários),
// invisível sob a caixa desenhada com os tokens.
import type { InputHTMLAttributes, ReactNode } from 'react';
import { CheckIcon } from './icons';
import s from './ui.module.css';

export interface CheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label: ReactNode;
  /** Texto de apoio abaixo do rótulo. */
  hint?: string | undefined;
}

export function Checkbox({ label, hint, className, ...rest }: CheckboxProps) {
  return (
    <label className={`${s.checkbox} ${className ?? ''}`}>
      <span className={s.checkboxControl}>
        <input type="checkbox" className={s.checkboxInput} {...rest} />
        <span className={s.checkboxBox} aria-hidden>
          <CheckIcon className={s.checkboxMark} />
        </span>
      </span>
      <span className={s.checkboxText}>
        <span>{label}</span>
        {hint && <span className={s.hint}>{hint}</span>}
      </span>
    </label>
  );
}
