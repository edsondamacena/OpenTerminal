# Licenças de terceiros incluídas no app

Dependências de código são verificadas por `scripts/check-licenses.mjs`. Este arquivo registra
o que é embutido como **conteúdo** (ícones, fontes, esquemas de cores), com origem e licença.

| Item | Origem | Licença | Uso |
|---|---|---|---|
| Lucide (`lucide-react` 1.52.0) | https://lucide.dev | ISC | todos os ícones da interface (FR-169) |
| Simple Icons 16.36.0 (só os caminhos usados, em `frontend/src/ui/osIcons.gen.ts`) | https://simpleicons.org | CC0-1.0 | logos de sistema no avatar do host (FR-178); as marcas são dos respectivos donos e servem só para identificar o sistema |
| JetBrains Mono (Fontsource 5.3.0, `woff2` latim) | https://github.com/JetBrains/JetBrainsMono | OFL-1.1 | fonte embutida do terminal (`frontend/src/assets/fonts/`) |
| Cascadia Code (Fontsource 5.3.0, `woff2` latim e caixas) | https://github.com/microsoft/cascadia-code | OFL-1.1 | fonte embutida do terminal (`frontend/src/assets/fonts/`) |
| OpenTerminal Dark | tokens próprios em `frontend/src/theme/tokens.ts` | licença do projeto | preset de cores padrão |
| Dracula | https://github.com/dracula/windows-terminal | MIT | preset de cores embutido |
| Nord | https://github.com/nordtheme/terminal-app | MIT | preset de cores embutido |
| Solarized Dark | https://github.com/altercation/solarized | MIT | preset de cores embutido |
| Solarized Light | https://github.com/altercation/solarized | MIT | preset de cores embutido |
| One Dark | https://github.com/atom/one-dark-syntax | MIT | preset de cores embutido |
| Tokyo Night | https://github.com/enkia/tokyo-night-vscode-theme | MIT | preset de cores embutido |
| Catppuccin Mocha | https://github.com/catppuccin/catppuccin | MIT | preset de cores embutido |

## Dependências npm de produção do frontend

Versões e licenças conferidas nos manifestos instalados em `frontend/node_modules`.

| Item | Origem | Licença | Uso |
|---|---|---|---|
| `@floating-ui/react-dom` 2.1.9 | https://github.com/floating-ui/floating-ui | MIT | posicionamento de menus e popovers |
| `@xterm/addon-fit` 0.11.0 | https://github.com/xtermjs/xterm.js/tree/master/addons/addon-fit | MIT | ajuste do terminal ao contêiner |
| `@xterm/addon-search` 0.16.0 | https://github.com/xtermjs/xterm.js/tree/master/addons/addon-search | MIT | busca no conteúdo do terminal |
| `@xterm/addon-unicode11` 0.9.0 | https://github.com/xtermjs/xterm.js/tree/master/addons/addon-unicode11 | MIT | largura e segmentação de caracteres Unicode 11 |
| `@xterm/addon-web-links` 0.12.0 | https://github.com/xtermjs/xterm.js/tree/master/addons/addon-web-links | MIT | detecção de links na saída do terminal |
| `@xterm/addon-webgl` 0.19.0 | https://github.com/xtermjs/xterm.js/tree/master/addons/addon-webgl | MIT | renderização acelerada do terminal |
| `@xterm/xterm` 6.0.0 | https://github.com/xtermjs/xterm.js | MIT | emulador de terminal na interface |
| `lucide-react` 1.52.0 | https://github.com/lucide-icons/lucide | ISC | biblioteca de ícones React |
| `react` 19.3.1 | https://github.com/react/react | MIT | componentes da interface |
| `react-dom` 19.3.1 | https://github.com/react/react | MIT | renderização da interface no DOM |
| `zustand` 5.0.16 | https://github.com/pmndrs/zustand | MIT | gerenciamento de estado da interface |

## Dependências Go

Versões e licenças conferidas no `go.mod` e nos arquivos de licença dos módulos locais.

| Item | Origem | Licença | Uso |
|---|---|---|---|
| `git.sr.ht/~jackmordaunt/go-toast/v2` 2.0.3 | https://git.sr.ht/~jackmordaunt/go-toast | Unlicense OR MIT | notificações nativas |
| `github.com/Microsoft/go-winio` 0.6.4 | https://github.com/microsoft/go-winio | MIT | comunicação IPC no Windows |
| `github.com/aymanbagabas/go-pty` 0.2.3 | https://github.com/aymanbagabas/go-pty | MIT | pseudoterminal |
| `github.com/bep/debounce` 1.2.1 | https://github.com/bep/debounce | MIT | debounce de eventos |
| `github.com/creack/pty` 1.1.24 | https://github.com/creack/pty | MIT | pseudoterminal |
| `github.com/dustin/go-humanize` 1.1.0 | https://github.com/dustin/go-humanize | MIT | formatação legível |
| `github.com/gabriel-vasile/mimetype` 1.4.16 | https://github.com/gabriel-vasile/mimetype | MIT | detecção de MIME |
| `github.com/go-ole/go-ole` 1.3.0 | https://github.com/go-ole/go-ole | MIT | integração OLE no Windows |
| `github.com/godbus/dbus/v5` 5.1.0 | https://github.com/godbus/dbus | BSD-2-Clause | integração D-Bus |
| `github.com/google/uuid` 1.6.0 | https://github.com/google/uuid | BSD-3-Clause | identificadores UUID |
| `github.com/gorilla/websocket` 1.5.3 | https://github.com/gorilla/websocket | BSD-2-Clause | comunicação WebSocket |
| `github.com/jchv/go-winloader` 2021-07-11 | https://github.com/jchv/go-winloader | ISC | carregamento de DLL no Windows |
| `github.com/labstack/echo/v4` 4.15.3 | https://github.com/labstack/echo | MIT | servidor HTTP interno |
| `github.com/labstack/gommon` 0.5.0 | https://github.com/labstack/gommon | MIT | utilitários do Echo |
| `github.com/leaanthony/go-ansi-parser` 1.6.1 | https://github.com/leaanthony/go-ansi-parser | MIT | análise de sequências ANSI |
| `github.com/leaanthony/gosod` 1.0.4 | https://github.com/leaanthony/gosod | MIT | utilitário do Wails |
| `github.com/leaanthony/slicer` 1.6.0 | https://github.com/leaanthony/slicer | MIT | utilitário do Wails |
| `github.com/leaanthony/u` 1.1.1 | https://github.com/leaanthony/u | MIT | utilitário do Wails |
| `github.com/mattn/go-colorable` 0.1.14 | https://github.com/mattn/go-colorable | MIT | saída colorida no Windows |
| `github.com/mattn/go-isatty` 0.0.24 | https://github.com/mattn/go-isatty | MIT | detecção de terminal |
| `github.com/ncruces/go-strftime` 1.1.0 | https://github.com/ncruces/go-strftime | MIT | formatação de datas no SQLite |
| `github.com/pkg/browser` 2024-01-02 | https://github.com/pkg/browser | BSD-2-Clause | abertura do navegador padrão |
| `github.com/pkg/errors` 0.9.1 | https://github.com/pkg/errors | BSD-2-Clause | encadeamento de erros |
| `github.com/remyoudompheng/bigfft` 2023-01-29 | https://github.com/remyoudompheng/bigfft | BSD-3-Clause | operações FFT do SQLite |
| `github.com/rivo/uniseg` 0.4.7 | https://github.com/rivo/uniseg | MIT | segmentação Unicode |
| `github.com/samber/lo` 1.49.1 | https://github.com/samber/lo | MIT | utilitários de coleções |
| `github.com/tkrajina/go-reflector` 0.5.8 | https://github.com/tkrajina/go-reflector | Apache-2.0 | reflexão de tipos |
| `github.com/u-root/u-root` 0.16.0 | https://github.com/u-root/u-root | BSD-3-Clause | suporte a pseudoterminal |
| `github.com/valyala/bytebufferpool` 1.0.0 | https://github.com/valyala/bytebufferpool | MIT | buffers do Echo |
| `github.com/valyala/fasttemplate` 1.2.2 | https://github.com/valyala/fasttemplate | MIT | templates do Echo |
| `github.com/wailsapp/go-webview2` 1.0.22 (cópia local) | https://github.com/wailsapp/go-webview2 | MIT | WebView2 |
| `github.com/wailsapp/wails/v2` 2.17.0 | https://github.com/wailsapp/wails | MIT | framework desktop |
| `go.uber.org/goleak` 1.3.0 | https://github.com/uber-go/goleak | MIT | detecção de goroutines vazadas em testes |
| `golang.org/x/crypto` 0.58.0 | https://cs.opensource.google/go/x/crypto | BSD-3-Clause | criptografia e SSH |
| `golang.org/x/image` 0.41.0 | https://cs.opensource.google/go/x/image | BSD-3-Clause | leitura de metadados de fontes |
| `golang.org/x/net` 0.60.0 | https://cs.opensource.google/go/x/net | BSD-3-Clause | protocolos de rede |
| `golang.org/x/sys` 0.49.0 | https://cs.opensource.google/go/x/sys | BSD-3-Clause | chamadas de sistema |
| `golang.org/x/text` 0.43.0 | https://cs.opensource.google/go/x/text | BSD-3-Clause | suporte Unicode |
| `modernc.org/libc` 1.79.0 | https://gitlab.com/cznic/libc | BSD-3-Clause | runtime C em Go |
| `modernc.org/mathutil` 1.7.1 | https://gitlab.com/cznic/mathutil | BSD-3-Clause | utilitários matemáticos |
| `modernc.org/memory` 1.12.1 | https://gitlab.com/cznic/memory | BSD-3-Clause | alocação de memória |
| `modernc.org/sqlite` 1.61.0 | https://gitlab.com/cznic/sqlite | BSD-3-Clause | banco de dados SQLite |
