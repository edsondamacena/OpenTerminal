import { useId, useState, type InputHTMLAttributes, type TextareaHTMLAttributes } from 'react';
import { t } from '../i18n/pt-BR';
import { EyeIcon, EyeOffIcon } from './icons';
import s from './ui.module.css';

interface FieldShellProps {
  label: string;
  hint?: string | undefined;
  error?: string | null | undefined;
}

export interface TextFieldProps
  extends FieldShellProps, Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  type?: 'text' | 'password' | 'number';
  /** Campo de senha com botão mostrar/ocultar. */
  secret?: boolean;
}

export function TextField({
  label,
  hint,
  error,
  secret = false,
  type = 'text',
  id: idProp,
  ...rest
}: TextFieldProps) {
  const generatedId = useId();
  const id = idProp ?? generatedId;
  const [revealed, setRevealed] = useState(false);
  const inputType = secret ? (revealed ? 'text' : 'password') : type;
  return (
    <div className={s.field}>
      <label className={s.label} htmlFor={id}>
        {label}
      </label>
      <div className={s.inputWrap}>
        <input
          id={id}
          className={s.input}
          type={inputType}
          spellCheck={false}
          autoComplete="off"
          aria-invalid={error ? true : undefined}
          {...rest}
        />
        {secret && (
          <button
            type="button"
            className={`${s.button} ${s.ghost} ${s.icon}`}
            aria-label={revealed ? t.common.hide : t.common.show}
            onClick={() => {
              setRevealed((v) => !v);
            }}
          >
            {revealed ? <EyeOffIcon /> : <EyeIcon />}
          </button>
        )}
      </div>
      {error ? (
        <span className={s.error}>{error}</span>
      ) : (
        hint && <span className={s.hint}>{hint}</span>
      )}
    </div>
  );
}

export interface TextAreaFieldProps
  extends FieldShellProps, TextareaHTMLAttributes<HTMLTextAreaElement> {}

export function TextAreaField({ label, hint, error, ...rest }: TextAreaFieldProps) {
  const id = useId();
  return (
    <div className={s.field}>
      <label className={s.label} htmlFor={id}>
        {label}
      </label>
      <div className={s.inputWrap}>
        <textarea id={id} className={s.input} spellCheck={false} autoComplete="off" {...rest} />
      </div>
      {error ? (
        <span className={s.error}>{error}</span>
      ) : (
        hint && <span className={s.hint}>{hint}</span>
      )}
    </div>
  );
}
