// Topo da lista no celular: os terminais abertos (tocar volta ao console em tela cheia) e o
// "Novo terminal" com o shell do aparelho.
import { t } from '../../i18n/pt-BR';
import { isLive, useSessions } from '../../stores/sessions';
import { useShells } from '../../stores/shells';
import { useUi } from '../../stores/ui';
import { useWorkspace } from '../../stores/workspace';
import { PlusIcon, TerminalIcon } from '../../ui/icons';
import { openLocal } from '../terminal/openLocal';
import s from './sidebar.module.css';

export function MobileTerminals() {
  const tabs = useSessions((st) => st.tabs);
  const activate = useSessions((st) => st.activate);
  const setMobilePane = useWorkspace((w) => w.setMobilePane);
  const defaultShell = useUi((u) => u.settings?.defaultShell);
  // Sem shells detectados a lista traz os de Windows de reserva: no celular, só os reais.
  const shells = useShells((st) => st.shells).filter(
    (sh) => sh.kind !== 'powershell' && sh.kind !== 'cmd',
  );
  const shell = shells.find((sh) => sh.id === defaultShell) ?? shells[0];

  return (
    <div className={s.mobileTerminals}>
      {shell && (
        <button type="button" className={s.mobileRow} onClick={() => void openLocal(shell.id)}>
          <PlusIcon />
          <span>{t.sidebar.newTerminal}</span>
        </button>
      )}
      {tabs.length > 0 && <h3 className={s.mobileTitle}>{t.sidebar.openTerminals}</h3>}
      {tabs.map((tab) => (
        <button
          key={tab.id}
          type="button"
          className={s.mobileRow}
          onClick={() => {
            activate(tab.id);
            setMobilePane('terminal');
          }}
        >
          <TerminalIcon />
          <span className={s.mobileLabel}>{tab.title}</span>
          <span className={s.mobileDot} data-live={isLive(tab.state) || undefined} />
        </button>
      ))}
    </div>
  );
}
