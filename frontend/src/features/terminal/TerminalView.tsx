import { useEffect, useRef, type ReactNode } from 'react';
import { createHighlighter, createLineJoiner } from './highlight';
import { applyCtrl, setActiveWriter } from './mobileKeys';
import { FitAddon } from '@xterm/addon-fit';
import { SearchAddon } from '@xterm/addon-search';
import { Unicode11Addon } from '@xterm/addon-unicode11';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { WebglAddon } from '@xterm/addon-webgl';
import { Terminal } from '@xterm/xterm';
import '@xterm/xterm/css/xterm.css';
import type { Tab } from '../../stores/sessions';
import { effectiveProfile, useProfiles } from '../../stores/profiles';
import { useUi } from '../../stores/ui';
import { useVault } from '../../stores/vault';
import { fontSizeOf, rememberMobileZoom, useWorkspace } from '../../stores/workspace';
import { ringBell } from './bell';
import { attach } from './dataBus';
import { needsConfirm } from './paste';
import { applyProfile, cssFontFamily, schemeOf, schemeTheme, withAlpha } from './profile';
import { SearchBar } from './SearchBar';
import { registerTerminal } from './terminalRegistry';
import { countResize, registerProbe } from './testProbe';
import s from './terminal.module.css';

async function copySelection(term: Terminal): Promise<boolean> {
  const text = term.getSelection();
  if (!text) return false;
  await navigator.clipboard.writeText(text);
  term.clearSelection();
  return true;
}

/** Cola; várias linhas passam antes pela confirmação (FR-144). */
function pasteText(sessionId: string, term: Terminal, text: string): void {
  if (!text) return;
  if (needsConfirm(text)) useWorkspace.getState().setPendingPaste({ sessionId, text });
  else term.paste(text);
}

export interface TerminalViewProps {
  tab: Tab;
  active: boolean;
  /** Faixa sobre o terminal (desconectado, processo encerrado). */
  banner?: ReactNode;
}

export function TerminalView({ tab, active, banner }: TerminalViewProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const termRef = useRef<Terminal | null>(null);
  const fontBaseRef = useRef(14);
  const fitRef = useRef<FitAddon | null>(null);
  const fitNowRef = useRef<() => void>(() => undefined);
  const activeRef = useRef(active);
  const profile = useProfiles((p) => effectiveProfile(tab.hostId, p.global, p.overrides));
  const scheme = useProfiles((p) => schemeOf(p.schemes, profile.colorSchemeId));
  const unlocked = useVault((v) => v.status?.state === 'unlocked');
  const zoom = useWorkspace((w) => w.zoom);
  const searching = useWorkspace((w) => w.searchFor === tab.id);
  useEffect(() => {
    activeRef.current = active;
  }, [active]);

  // Cria o terminal uma vez por aba.
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    // Valores iniciais do perfil; o efeito abaixo reaplica tudo quando a fonte carregar.
    const profiles = useProfiles.getState();
    const initial = effectiveProfile(tab.hostId, profiles.global, profiles.overrides);
    const term = new Terminal({
      allowProposedApi: true,
      scrollback: initial.scrollback,
      cursorBlink: initial.cursorBlink,
      cursorStyle: initial.cursorStyle,
      fontFamily: cssFontFamily(initial.fontFamily),
      fontSize: fontSizeOf(tab.id, initial.fontSize, useWorkspace.getState().zoom),
      lineHeight: initial.lineHeight,
      allowTransparency: initial.backgroundOpacity < 1,
      theme: schemeTheme(schemeOf(profiles.schemes, initial.colorSchemeId), initial),
      rightClickSelectsWord: false,
      drawBoldTextInBrightColors: useUi.getState().settings?.boldBright ?? true,
    });
    const fit = new FitAddon();
    term.loadAddon(fit);
    const search = new SearchAddon();
    term.loadAddon(search);
    // Ctrl+clique abre no navegador padrão; o app não faz a requisição (FR-145).
    term.loadAddon(
      new WebLinksAddon((event, uri) => {
        if (event.ctrlKey) void window.openterminal.app.openExternalUrl(uri);
      }),
    );
    term.loadAddon(new Unicode11Addon());
    term.unicode.activeVersion = '11';
    term.open(container);
    termRef.current = term;
    fitRef.current = fit;

    const sessionId = tab.id;
    const api = window.openterminal.sessions;
    let lastSize: { cols: number; rows: number } | null = null;
    const fitAndResize = () => {
      if (!activeRef.current || container.clientWidth <= 0 || container.clientHeight <= 0) return;
      fit.fit();
      const size = { cols: term.cols, rows: term.rows };
      if (size.cols <= 0 || size.rows <= 0) return;
      if (lastSize?.cols === size.cols && lastSize.rows === size.rows) return;
      lastSize = size;
      countResize(sessionId);
      void api.resize(sessionId, size);
    };
    fitNowRef.current = fitAndResize;
    const unregisterProbe = registerProbe(sessionId, term);
    const unregisterTerminal = registerTerminal(sessionId, {
      term,
      search,
      focus: () => {
        if (activeRef.current) term.focus();
      },
    });
    // Realce de palavras de rede (lido a cada trecho: ligar/desligar vale na hora).
    const highlight = createHighlighter();
    // A OLT quebra a linha em pedaços ("interface " | "gpon 2/7"): junta antes de pintar.
    const joiner = createLineJoiner((data) => {
      term.write(highlight(data));
    });
    const detach = attach(sessionId, (data) => {
      if (useUi.getState().settings?.highlight ?? true) joiner.push(data);
      else {
        joiner.flush();
        term.write(data);
      }
    });
    const onData = term.onData((data) => {
      void api.write(sessionId, applyCtrl(data));
    });
    const onBell = term.onBell(() => {
      ringBell(behavior()?.bell ?? 'visual', container.parentElement ?? container);
    });

    term.attachCustomKeyEventHandler((e) => {
      if (e.type !== 'keydown' || !e.ctrlKey || !e.shiftKey) return true;
      if (e.code === 'KeyC') {
        // No Chromium do Linux Ctrl+Shift+C também abre o DevTools (inspecionar): cancela depois de copiar.
        e.preventDefault();
        void copySelection(term);
        return false;
      }
      // Ctrl+Shift+V: o xterm não trata a tecla; a colagem chega pelo evento `paste` abaixo.
      if (e.code === 'KeyV') return false;
      return true;
    });
    // Opções de FR-177 lidas na hora do evento: mudar nas configurações vale sem recriar o xterm.
    const behavior = () => useUi.getState().settings;
    // Selecionar copia (FR-143), se ligado; o clique direito cola (FR-144), se ligado.
    let copyTimer: ReturnType<typeof setTimeout> | null = null;
    const onSelection = term.onSelectionChange(() => {
      if (copyTimer) clearTimeout(copyTimer);
      if (!(behavior()?.copyOnSelect ?? false)) return;
      copyTimer = setTimeout(() => {
        const text = term.getSelection();
        if (text) void navigator.clipboard.writeText(text);
      }, 120);
    });
    const onContextMenu = (e: MouseEvent) => {
      e.preventDefault();
      if (!(behavior()?.rightClickPaste ?? true)) return;
      void navigator.clipboard.readText().then((text) => {
        pasteText(sessionId, term, text);
      });
    };
    container.addEventListener('contextmenu', onContextMenu);
    // Toda colagem nativa (Ctrl+Shift+V, menu do sistema) passa pela confirmação antes do xterm.
    const onPaste = (e: ClipboardEvent) => {
      e.preventDefault();
      e.stopPropagation();
      pasteText(sessionId, term, e.clipboardData?.getData('text/plain') ?? '');
    };
    container.addEventListener('paste', onPaste, true);
    // Pinça com dois dedos: muda a fonte desta aba; no celular o tamanho vale para as próximas.
    let pinch: { dist: number; steps: number } | null = null;
    const distance = (e: TouchEvent) => {
      const [a, b] = [e.touches[0], e.touches[1]];
      return a && b ? Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY) : 0;
    };
    const onTouchStart = (e: TouchEvent) => {
      pinch = e.touches.length === 2 ? { dist: distance(e), steps: 0 } : null;
    };
    const onTouchMove = (e: TouchEvent) => {
      if (!pinch || e.touches.length !== 2 || pinch.dist === 0) return;
      e.preventDefault();
      const steps = Math.round(Math.log(distance(e) / pinch.dist) / Math.log(1.12));
      if (steps === pinch.steps) return;
      const workspace = useWorkspace.getState();
      workspace.zoomBy(sessionId, steps - pinch.steps, fontBaseRef.current);
      pinch.steps = steps;
      const offset = useWorkspace.getState().zoom[sessionId];
      if (offset !== undefined) rememberMobileZoom(offset);
    };
    const onTouchEnd = () => {
      pinch = null;
    };
    container.addEventListener('touchstart', onTouchStart, { capture: true, passive: true });
    container.addEventListener('touchmove', onTouchMove, { capture: true, passive: false });
    container.addEventListener('touchend', onTouchEnd, true);

    let timer: ReturnType<typeof setTimeout> | null = null;
    const observer = new ResizeObserver(() => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => {
        // Aba oculta tem tamanho 0: o fit só roda quando visível.
        fitAndResize();
      }, 50);
    });
    observer.observe(container);

    // Troca de monitor com outra escala: o tamanho em CSS pode não mudar, mas a célula muda.
    let stopDpr: () => void = () => undefined;
    const watchDpr = () => {
      const query = matchMedia(`(resolution: ${String(window.devicePixelRatio)}dppx)`);
      const onChange = () => {
        fitAndResize();
        watchDpr();
      };
      query.addEventListener('change', onChange, { once: true });
      stopDpr = () => {
        query.removeEventListener('change', onChange);
      };
    };
    watchDpr();

    return () => {
      if (timer) clearTimeout(timer);
      observer.disconnect();
      stopDpr();
      if (copyTimer) clearTimeout(copyTimer);
      onSelection.dispose();
      unregisterTerminal();
      container.removeEventListener('contextmenu', onContextMenu);
      container.removeEventListener('paste', onPaste, true);
      container.removeEventListener('touchstart', onTouchStart, true);
      container.removeEventListener('touchmove', onTouchMove, true);
      container.removeEventListener('touchend', onTouchEnd, true);
      onData.dispose();
      onBell.dispose();
      detach();
      joiner.flush(); // o resto da última linha não fica preso no timer
      unregisterProbe();
      term.dispose();
      termRef.current = null;
      fitNowRef.current = () => undefined;
    };
  }, [tab.id, tab.hostId]);

  // WebGL só na aba visível (limite de contextos do Chromium; research R4).
  // Barra de teclas do Android escreve no terminal visível.
  useEffect(() => {
    if (!active) return;
    return setActiveWriter((data) => void window.openterminal.sessions.write(tab.id, data));
  }, [active, tab.id]);

  useEffect(() => {
    const term = termRef.current;
    if (!term || !active) return;
    let webgl: WebglAddon | null = null;
    try {
      webgl = new WebglAddon();
      webgl.onContextLoss(() => {
        webgl?.dispose();
        webgl = null;
      });
      term.loadAddon(webgl);
    } catch {
      webgl = null; // Sem WebGL: fica o renderer DOM.
    }
    requestAnimationFrame(() => {
      fitNowRef.current();
      term.focus();
    });
    return () => {
      webgl?.dispose();
    };
  }, [active]);

  // Negrito em cores claras (FR-177) muda ao vivo, como o perfil.
  const boldBright = useUi((u) => u.settings?.boldBright ?? true);
  useEffect(() => {
    const term = termRef.current;
    if (term) term.options.drawBoldTextInBrightColors = boldBright;
  }, [boldBright]);

  // Override do host (FR-156): consultado com o cofre aberto.
  useEffect(() => {
    if (tab.hostId && unlocked) void useProfiles.getState().loadOverride(tab.hostId);
  }, [tab.hostId, unlocked]);

  // Perfil aplicado ao vivo, sem reconectar (FR-157); o zoom da aba soma ao tamanho do perfil.
  useEffect(() => {
    const term = termRef.current;
    if (!term) return;
    let alive = true;
    fontBaseRef.current = profile.fontSize;
    void applyProfile(term, profile, scheme, fontSizeOf(tab.id, profile.fontSize, zoom)).then(
      () => {
        if (alive && activeRef.current) fitNowRef.current();
      },
    );
    return () => {
      alive = false;
    };
  }, [profile, scheme, zoom, tab.id]);

  const background =
    profile.backgroundOpacity < 1
      ? withAlpha(scheme.background, profile.backgroundOpacity)
      : scheme.background;

  return (
    <div
      className={`${s.view} ${active ? '' : s.hidden}`}
      role="tabpanel"
      aria-label={tab.title}
      data-session-id={tab.id}
    >
      {banner}
      {searching && <SearchBar sessionId={tab.id} />}
      {/* Espaçamento no quadro, não no contêiner do xterm: o FitAddon mede a altura do pai
          sem descontar o padding e cortaria a última linha. */}
      <div
        className={`${s.frame} solid`}
        style={{
          padding: profile.padding,
          background,
          backdropFilter:
            profile.backgroundOpacity < 1 && profile.blur > 0
              ? `blur(${String(profile.blur)}px)`
              : undefined,
        }}
      >
        <div ref={containerRef} className={s.xterm} />
      </div>
    </div>
  );
}
