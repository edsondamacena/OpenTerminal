// Barra de título própria (FR-133): área de arrasto, ações do app e botões da janela.
import { useEffect, useState, type ReactNode } from 'react';
import { t } from '../i18n/pt-BR';
import { CloseIcon, MaximiseIcon, MinimiseIcon, RestoreIcon } from '../ui/icons';
import { Logo } from '../ui/Logo';
import s from './titlebar.module.css';

function useMaximised(): boolean {
  const [maximised, setMaximised] = useState(false);
  useEffect(() => {
    let alive = true;
    const sync = () => {
      void window.openterminal.app.windowIsMaximised().then((res) => {
        if (alive && res.ok) setMaximised(res.value);
      });
    };
    sync();
    window.addEventListener('resize', sync);
    return () => {
      alive = false;
      window.removeEventListener('resize', sync);
    };
  }, []);
  return maximised;
}

export interface TitleBarProps {
  /** Antes da marca (alternar a barra lateral). */
  leading?: ReactNode;
  /** Ações do app à direita (encaminhamentos, bloquear, configurações). */
  actions?: ReactNode;
}

export function TitleBar({ leading, actions }: TitleBarProps) {
  const maximised = useMaximised();
  const app = window.openterminal.app;

  return (
    <header
      className={`${s.titleBar} drag`}
      onDoubleClick={(e) => {
        // Só na área de arrasto, não nos botões.
        if (
          e.target === e.currentTarget ||
          (e.target as HTMLElement).dataset.dragArea !== undefined
        )
          void app.windowToggleMaximise();
      }}
    >
      {leading && <div className={`${s.leading} no-drag`}>{leading}</div>}
      <span className={s.brand} data-drag-area>
        <Logo size={16} variant="mark" />
        <span className={s.appName} data-drag-area>
          {t.common.appName}
        </span>
      </span>
      <span className={s.spacer} data-drag-area />
      {actions && <div className={`${s.actions} no-drag`}>{actions}</div>}
      <div className={`${s.windowControls} no-drag`}>
        <button
          type="button"
          className={s.control}
          aria-label={t.window.minimise}
          title={t.window.minimise}
          onClick={() => void app.windowMinimise()}
        >
          <MinimiseIcon />
        </button>
        <button
          type="button"
          className={s.control}
          aria-label={maximised ? t.window.restore : t.window.maximise}
          title={maximised ? t.window.restore : t.window.maximise}
          onClick={() => void app.windowToggleMaximise()}
        >
          {maximised ? (
            <RestoreIcon width={14} height={14} />
          ) : (
            <MaximiseIcon width={13} height={13} />
          )}
        </button>
        <button
          type="button"
          className={`${s.control} ${s.close}`}
          aria-label={t.window.close}
          title={t.window.close}
          onClick={() => void app.windowClose()}
        >
          <CloseIcon />
        </button>
      </div>
    </header>
  );
}
