// Atualização automática no celular: ao abrir o app, procura a versão nova no GitHub; se houver e
// nenhuma sessão estiver conectada, baixa, confere o SHA256SUMS e instala (o app reinicia). Com
// sessão aberta não interrompe: fica para a próxima abertura ou para o botão Atualizar.
import { t } from '../i18n/pt-BR';
import { useSessions } from '../stores/sessions';
import { toastInfo } from '../ui/Toast';

const DELAY_MS = 5000;

function busy(): boolean {
  return useSessions
    .getState()
    .tabs.some((tab) => !['disconnected', 'error', 'closed'].includes(tab.state));
}

/** Liga a verificação uma vez por processo, só no app Android. Devolve o cancelamento. */
export function startAutoUpdate(): () => void {
  if (window.OpenTerminalAndroid === undefined) return () => undefined;
  const timer = setTimeout(() => {
    void (async () => {
      if (busy()) return;
      const check = await window.openterminal.app.checkUpdate(__APP_VERSION__);
      if (!check.ok || !check.value.available || busy()) return;
      toastInfo(t.settings.autoUpdating(check.value.latest));
      await window.openterminal.app.installUpdate(__APP_VERSION__);
    })();
  }, DELAY_MS);
  return () => {
    clearTimeout(timer);
  };
}
