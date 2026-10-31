# Quickstart / validação: ShellVault em Go + Wails

**Feature**: `002-wails-migration`

Guia para rodar e validar a feature de ponta a ponta. Detalhes de entidades em
[data-model.md](data-model.md) e da ponte em [contracts/bridge.md](contracts/bridge.md).

## Pré-requisitos

- Windows 11 (validação principal) e Windows 10 (fallback sólido).
- Go 1.26+ (`go version`), Wails CLI v2.17 (`go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0`,
  depois `wails doctor` sem erros), Node 24 LTS, WebView2 Runtime (já presente no Windows 11).
- Docker Desktop (servidores SSH de teste), NSIS (instalador).
- OpenSSH Client do Windows com `ssh-agent` (para o cenário de agente).

## Rodar

```text
wails dev                       # app com recarga do frontend
wails build -clean -nsis        # binário e instalador em build/bin
```

## Verificações automáticas

| Comando | Cobre |
|---|---|
| `go vet ./... && go test ./internal/...` | domínio, cofre, cripto, migração 001→002, importadores de esquema, algoritmos, agregador |
| `go test -run Leak ./internal/session/...` | SC-105 (100 aberturas/fechamentos, goroutines estáveis) |
| `node scripts/sshd.mjs up && go test -tags integration ./internal/...` | SSH, host key, legados (`sshd-legacy`), keepalive (`idle-killer`), agente, SFTP, forwarding |
| `npm --prefix frontend run lint && npm --prefix frontend run typecheck && npm --prefix frontend test` | frontend (lógica pura, lint de tokens) |
| `npm --prefix frontend run test:e2e` | E2E via CDP no build `e2e`: paridade 001, layout, menus, atalhos, personalização |
| `npm --prefix frontend run test:secrets-scan` | SC-108 |
| `go run ./scripts/bench-sftp` | SC-006 da 001 / US6-3 |

## Cenários manuais

1. **Migração (SC-114)**: copiar um `%APPDATA%\ShellVault` da versão Electron, abrir a versão
   nova, desbloquear → mesmos hosts, grupos, credenciais e regras; `vault.svlt.migrated`
   presente; conectar num host já conhecido sem novo pedido de host key.
2. **Prompt único (SC-102)**: com acesso fornecido pelo usuário, conectar 20 vezes em cada
   um: OLT Datacom DmOS, roteador Huawei, Mikrotik, servidor Linux → banner e prompt uma vez só.
3. **Layout (SC-103)**: com duas abas, maximizar/restaurar 20 vezes, encaixar à esquerda e à
   direita, Win+setas, mudar a escala para 125% e 150% e arrastar a janela entre monitores →
   nada cortado, nada em branco; `stty size` no servidor confere com o terminal.
4. **Menus (SC-104)**: com a janela no tamanho mínimo, abrir o "+" e menus de contexto perto
   das quatro bordas → sempre inteiros e ancorados.
5. **Algoritmos legados (SC-111)**: host no `sshd-legacy` falha com a opção desligada e
   mensagem sugerindo ligá-la; conecta com a opção ligada; ícone de aviso aparece na lista.
6. **Keepalive (SC-110)**: conexão via `idle-killer` ociosa por 30 minutos continua viva;
   derrubar a rede → aba "desconectado" em até 90 s com botão de reconectar.
7. **Personalização (SC-106, SC-107)**: com três abas, mudar fonte, tamanho e esquema →
   abas atualizam em menos de 1 s; importar um esquema do Windows Terminal e um
   `.itermcolors`; exportar e reimportar; override vermelho num host de "produção".
8. **Atalhos e paleta**: Ctrl+T/W/Tab/Shift+Tab, Ctrl+Shift+F com regex, Ctrl+K conectando num
   host só pelo teclado, Ctrl+=/−/0; colar 5 linhas pede confirmação.
9. **Identidade (SC-113)**: percorrer todas as telas nos temas escuro e claro, com vidro e com
   vidro desligado, e comparar com o brand book.
10. **Tamanho e memória (SC-109)**: instalador < 30 MB; após 2 minutos ocioso com uma aba,
    soma do *private working set* do app + processos WebView2 < 150 MB.
11. **Processos (FR-110/111)**: abrir shells, SFTP e transferências; no Gerenciador de Tarefas,
    nenhuma janela extra do app; segunda execução só traz a janela para frente.

Resultados MUST ser registrados em `specs/002-wails-migration/validation.md`.
