import { useEffect, useRef, type ReactNode } from 'react';
import { useExitGhost } from './motion';
import { Popover, type PopoverAnchor } from './Popover';
import s from './ui.module.css';

export interface MenuItem {
  label: string;
  icon?: ReactNode;
  danger?: boolean;
  disabled?: boolean;
  onSelect: () => void;
}

export interface MenuProps {
  /** Botão que abriu o menu, ou o ponto do clique direito. */
  anchor: PopoverAnchor;
  items: MenuItem[];
  onClose: () => void;
}

/** Menu navegável por teclado (setas, Enter, Esc), ancorado e sempre inteiro na janela. */
export function Menu({ anchor, items, onClose }: MenuProps) {
  const ref = useRef<HTMLDivElement | null>(null);
  useExitGhost(ref);

  function focusFirst() {
    ref.current?.querySelector<HTMLElement>('button:not([disabled])')?.focus();
  }

  useEffect(() => {
    const close = (e: MouseEvent) => {
      const target = e.target as Node;
      // O botão que abriu alterna o menu no próprio clique.
      if (ref.current?.contains(target) || (anchor instanceof Element && anchor.contains(target)))
        return;
      onClose();
    };
    window.addEventListener('mousedown', close, true);
    window.addEventListener('blur', onClose);
    return () => {
      window.removeEventListener('mousedown', close, true);
      window.removeEventListener('blur', onClose);
    };
  }, [anchor, onClose]);

  function onKeyDown(e: React.KeyboardEvent) {
    const buttons = [
      ...(ref.current?.querySelectorAll<HTMLElement>('button:not([disabled])') ?? []),
    ];
    const i = buttons.indexOf(document.activeElement as HTMLElement);
    if (e.key === 'Escape') {
      onClose();
      if (anchor instanceof HTMLElement) anchor.focus();
    } else if (e.key === 'ArrowDown') buttons[(i + 1) % buttons.length]?.focus();
    else if (e.key === 'ArrowUp') buttons[(i - 1 + buttons.length) % buttons.length]?.focus();
    else if (e.key === 'Home') buttons[0]?.focus();
    else if (e.key === 'End') buttons[buttons.length - 1]?.focus();
    else if (e.key === 'Tab') onClose();
    else return;
    if (e.key !== 'Tab') e.preventDefault();
  }

  return (
    <Popover anchor={anchor} onPositioned={focusFirst}>
      {({ ref: setFloating, style }) => (
        <div
          ref={(el) => {
            ref.current = el;
            if (typeof setFloating === 'function') setFloating(el);
          }}
          role="menu"
          className={`${s.menu} raised glass-edge`}
          style={style}
          onKeyDown={onKeyDown}
        >
          {items.map((item) => (
            <button
              key={item.label}
              type="button"
              role="menuitem"
              disabled={item.disabled}
              className={`${s.menuItem} ${item.danger ? s.menuItemDanger : ''}`}
              onClick={() => {
                onClose();
                item.onSelect();
              }}
            >
              {item.icon}
              {item.label}
            </button>
          ))}
        </div>
      )}
    </Popover>
  );
}
