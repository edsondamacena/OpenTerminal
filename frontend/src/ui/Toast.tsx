// Erros e avisos na interface (FR-030: nada vai para arquivo de log). Erros dizem a causa, o
// código e oferecem "Copiar detalhes" (FR-174).
import { useRef, useState } from 'react';
import { create } from 'zustand';
import type { AppError } from '@shared/ipc/errors';
import { ERROR_CAUSES, errorDetails } from '../i18n/errors';
import { t } from '../i18n/pt-BR';
import { CheckIcon, CloseIcon, CopyIcon, WarningIcon } from './icons';
import { useExitGhost } from './motion';
import s from './ui.module.css';

interface ToastItem {
  id: number;
  kind: 'error' | 'info';
  text: string;
  error?: AppError;
  at: Date;
}

interface ToastState {
  items: ToastItem[];
  push: (kind: ToastItem['kind'], text: string, error?: AppError) => void;
  dismiss: (id: number) => void;
  /** Com o mouse em cima o toast não some (dá tempo de copiar os detalhes). */
  hold: (id: number, held: boolean) => void;
}

const DURATION = { error: 10_000, info: 4000 } as const;
const timers = new Map<number, ReturnType<typeof setTimeout>>();
let nextId = 1;

export const useToasts = create<ToastState>((set, get) => {
  const schedule = (id: number, ms: number) => {
    clearTimeout(timers.get(id));
    timers.set(
      id,
      setTimeout(() => {
        get().dismiss(id);
      }, ms),
    );
  };
  return {
    items: [],
    push(kind, text, error) {
      const id = nextId++;
      const item: ToastItem = { id, kind, text, at: new Date(), ...(error ? { error } : {}) };
      set({ items: [...get().items.slice(-3), item] });
      schedule(id, DURATION[kind]);
    },
    dismiss(id) {
      clearTimeout(timers.get(id));
      timers.delete(id);
      set({ items: get().items.filter((i) => i.id !== id) });
    },
    hold(id, held) {
      if (held) clearTimeout(timers.get(id));
      else schedule(id, DURATION.info);
    },
  };
});

export function toastError(error: AppError | string): void {
  if (typeof error === 'string') useToasts.getState().push('error', error);
  else useToasts.getState().push('error', ERROR_CAUSES[error.code], error);
}

export function toastInfo(text: string): void {
  useToasts.getState().push('info', text);
}

function ErrorExtra({ error, at }: { error: AppError; at: Date }) {
  const [copied, setCopied] = useState(false);
  return (
    <>
      {error.message !== ERROR_CAUSES[error.code] && (
        <span className={s.toastMessage}>{error.message}</span>
      )}
      <span className={s.toastFooter}>
        <code className={s.toastCode}>{error.code}</code>
        <button
          type="button"
          className={s.toastCopy}
          onClick={() => {
            void navigator.clipboard
              .writeText(errorDetails(error, at, __APP_VERSION__))
              .then(() => {
                setCopied(true);
              });
          }}
        >
          {copied ? <CheckIcon aria-hidden /> : <CopyIcon aria-hidden />}
          {copied ? t.errors.detailsCopied : t.errors.copyDetails}
        </button>
      </span>
    </>
  );
}

function ToastCard({ item }: { item: ToastItem }) {
  const { dismiss, hold } = useToasts();
  const ref = useRef<HTMLDivElement>(null);
  useExitGhost(ref);
  return (
    <div
      ref={ref}
      role={item.kind === 'error' ? 'alert' : undefined}
      className={`${s.toast} raised`}
      onMouseEnter={() => {
        hold(item.id, true);
      }}
      onMouseLeave={() => {
        hold(item.id, false);
      }}
    >
      {item.kind === 'error' && <WarningIcon className={s.toastErrorIcon} />}
      <span className={s.toastBody}>
        <span className={item.error ? s.toastTitle : undefined}>{item.text}</span>
        {item.error && <ErrorExtra error={item.error} at={item.at} />}
      </span>
      <button
        type="button"
        className={`${s.button} ${s.ghost} ${s.icon}`}
        aria-label={t.common.close}
        onClick={() => {
          dismiss(item.id);
        }}
      >
        <CloseIcon />
      </button>
    </div>
  );
}

export function Toasts() {
  const items = useToasts((st) => st.items);
  return (
    <div className={s.toasts} role="status" aria-live="polite">
      {items.map((item) => (
        <ToastCard key={item.id} item={item} />
      ))}
    </div>
  );
}
