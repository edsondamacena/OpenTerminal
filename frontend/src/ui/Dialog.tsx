import { useEffect, useId, useLayoutEffect, useRef, type SubmitEvent, type ReactNode } from 'react';
import { createPortal } from 'react-dom';
import { useExitGhost } from './motion';
import s from './ui.module.css';

const FOCUSABLE =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/** Diálogos abertos, do mais antigo ao do topo: só o do topo reage a Esc e Tab. */
const stack: string[] = [];

export interface DialogProps {
  title: string;
  children: ReactNode;
  actions?: ReactNode;
  onClose: () => void;
  /** Quando definido, o conteúdo vira um <form> e Enter envia. */
  onSubmit?: () => void;
  /** Impede fechar com Esc/clique fora (ex.: decisão de host key). */
  modal?: boolean;
  width?: number;
  /** Para onde vai o foco ao fechar; padrão: o elemento focado ao abrir, se ainda existir. */
  returnFocus?: () => void;
}

export function Dialog({
  title,
  children,
  actions,
  onClose,
  onSubmit,
  modal = false,
  width,
  returnFocus,
}: DialogProps) {
  const titleId = useId();
  const ref = useRef<HTMLDivElement>(null);
  const backdropRef = useRef<HTMLDivElement>(null);
  useExitGhost(backdropRef);
  const latest = useRef({ onClose, modal, returnFocus });
  useLayoutEffect(() => {
    latest.current = { onClose, modal, returnFocus };
  });

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const root = ref.current;
    // `autoFocus` do React já focou um botão antes deste efeito: não roubar esse foco.
    if (!root?.contains(document.activeElement)) {
      const first = root?.querySelector<HTMLElement>('input, select, textarea') ?? root;
      first?.focus();
    }
    stack.push(titleId);

    // No document: Esc funciona mesmo se o elemento focado sumiu e o foco caiu no body (B2).
    const onKeyDown = (e: KeyboardEvent) => {
      if (stack[stack.length - 1] !== titleId || e.defaultPrevented || !ref.current) return;
      if (e.key === 'Escape') {
        if (latest.current.modal) return;
        e.preventDefault();
        latest.current.onClose();
        return;
      }
      if (e.key !== 'Tab') return;
      // Mantém o foco dentro do diálogo.
      const items = [...ref.current.querySelectorAll<HTMLElement>(FOCUSABLE)];
      const head = items[0];
      const tail = items[items.length - 1];
      if (!head || !tail) return;
      const inside = ref.current.contains(document.activeElement);
      if (!inside) {
        e.preventDefault();
        (e.shiftKey ? tail : head).focus();
      } else if (e.shiftKey && document.activeElement === head) {
        e.preventDefault();
        tail.focus();
      } else if (!e.shiftKey && document.activeElement === tail) {
        e.preventDefault();
        head.focus();
      }
    };
    document.addEventListener('keydown', onKeyDown);

    return () => {
      document.removeEventListener('keydown', onKeyDown);
      stack.splice(stack.indexOf(titleId), 1);
      const back = latest.current.returnFocus;
      if (back) back();
      else if (previous?.isConnected) previous.focus();
    };
  }, [titleId]);

  function submit(e: SubmitEvent) {
    e.preventDefault();
    onSubmit?.();
  }

  const body = (
    <>
      <h2 id={titleId} className={s.dialogTitle}>
        {title}
      </h2>
      {children}
      {actions && <div className={s.dialogActions}>{actions}</div>}
    </>
  );

  return createPortal(
    <div
      ref={backdropRef}
      className={s.backdrop}
      onMouseDown={(e) => {
        if (e.target === e.currentTarget && !modal) onClose();
      }}
    >
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className={`${s.dialog} raised glass-edge`}
        style={width ? { width: `min(${width}px, calc(100vw - 48px))` } : undefined}
      >
        {onSubmit ? (
          <form onSubmit={submit} autoComplete="off" style={{ display: 'contents' }}>
            {body}
          </form>
        ) : (
          body
        )}
      </div>
    </div>,
    document.body,
  );
}
