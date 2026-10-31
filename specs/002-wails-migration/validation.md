# Validação — migração Wails

## SC-109 — tamanho e memória

Execute `scripts/measure.ps1` com uma única aba local aberta e o app ocioso por 120 segundos.
O script mede o instalador (ou o executável quando ainda não houver instalador) e soma o
*private working set* do processo principal com seus descendentes `msedgewebview2.exe`.

**Resultado (2026-10-25, Windows 11 Pro 26200, WebView2 155.0.4283.62)**: build de release
(`wails build -clean -nsis -trimpath -ldflags "-s -w"`, `check-release.mjs` ok).

| Medida | Valor | Meta |
|---|---:|---:|
| Instalador `OpenHost-amd64-installer.exe` | 9,22 MiB | < 15 MB |
| Executável `shellvault.exe` | 18,5 MiB | — |
| App + 6 processos WebView2, 1 aba local, 120 s ocioso | 107,13 MiB | ≤ 150 MB |

A primeira medição deu 402,66 MiB: a listagem de fontes monoespaçadas (`internal/fonts`) lia
cada arquivo de `C:\Windows\Fonts` inteiro (322 MB alocados, vistos num perfil de heap) e o GC
não devolvia a memória com o app ocioso. A leitura passou a ser sob demanda
(`sfnt.ParseCollectionReaderAt`); as mitigações de R19 no WebView2 não foram necessárias.

## T105 — instalação e desinstalação

Validação automatizada em 2026-10-25 no Windows 11 build 26200.7623, usando
`scripts/check-installer.ps1` elevado e o instalador de 9,41 MiB gerado pelo build atual.

| Verificação | Resultado |
|---|---|
| Instalação silenciosa e registro | OpenHost 0.2.0 instalado; `DisplayName`, `DisplayVersion`, `DisplayIcon` e desinstalador válidos |
| Arquivos e atalhos | `OpenHost.exe`, `uninstall.exe`, Menu Iniciar e Área de Trabalho encontrados |
| Inicialização | processo permaneceu ativo por 10 s com 6 descendentes WebView2 |
| Segunda instância | segundo processo saiu com código 0; primeira janela permaneceu ativa |
| Desinstalação silenciosa | diretório, atalhos e chave de registro removidos após o `Au_.exe` terminar |
| Dados do usuário | hash da árvore `%APPDATA%\ShellVault` idêntico antes e depois |

O fallback visual e o ciclo de instalação no Windows 10 permanecem pendentes de validação manual
do usuário, pois essa versão do sistema não está disponível nesta máquina. Por isso T105 ainda
não foi marcada como concluída em `tasks.md`.

## SC-006 / T096 — desempenho SFTP de 1 GB

Medição em 2026-10-25 no mesmo host Windows e no mesmo contêiner OpenSSH local, com arquivo
aleatório de 1.024 MB. Upload e download tiveram o SHA-256 conferido após cada execução.

| Implementação | Upload | Download |
|---|---:|---:|
| 001 (Electron/ssh2) | 115,3 MB/s | 154,2 MB/s |
| 002 (Go/pkg/sftp) | 118,9 MB/s | 160,1 MB/s |
| Go ÷ 001 | 103% | 104% |

Na execução final da 002, o `sftp.exe` do Windows mediu 126,3 MB/s no upload e 151,1 MB/s no
download; o Go atingiu respectivamente 94% e 106% dessa referência, acima do mínimo de 80% de
SC-006. O primeiro ensaio revelou que o wrapper de progresso escondia o tamanho do leitor e
forçava uploads sequenciais; após preservar `Size()`, o `pkg/sftp` voltou a usar escritas paralelas.

Resultado: hashes idênticos, desempenho da 002 não inferior ao da 001 e SC-006 atendido.
