# Roteiro pós-002: fluxo de NOC/SOC

Origem: revisão de 24/10/2026 comparando com o Termius. Os itens baratos de uso diário entraram
na 002 (FR-173–FR-177, tarefas T115–T125). O restante abaixo vira a especificação **003**
(`/speckit-specify`) quando a 002 fechar — nada aqui é implementado sem spec, plano e tarefas.

## Ordem sugerida

1. Atalhos Ctrl+Shift e erro com detalhes — **na 002** (T115–T118)
2. Fechar SFTP e encaminhamentos — **na 002** (US6)
3. Página Hosts com três visões e herança por grupo
4. Telnet, serial e jump host
5. Painel de tema dentro da sessão
6. Snippets, broadcast e log de sessão
7. Identidade visual, instalador NSIS e auditoria final — **na 002** (T077–T085, T105, T101)

## Página Hosts

A lateral continua como árvore de navegação; a área principal ganha a página "Hosts" (substitui
o vazio "Nenhuma aba aberta") com três modos:

| Modo | Uso | Detalhes |
| --- | --- | --- |
| Árvore | navegação rápida | pastas em cascata, contador, estado expandido lembrado, arrastar e soltar |
| Grade | visão geral | cartões com ícone do fabricante, nome, `user@host:porta`, tags e protocolo |
| Lista densa | NOC com centenas de hosts | tabela ordenável (nome, endereço, usuário, protocolo, tags, último acesso, grupo) com seleção múltipla |

- Trilha de navegação (Clientes › TJCE › Core); clicar numa pasta da árvore filtra a página.
- Herança por grupo: usuário, credencial, porta, jump host, algoritmos legados e perfil de
  terminal; o host só sobrescreve o que difere. Go: `ResolveEffectiveConfig(hostID)` percorre os
  pais. UI: valor herdado como placeholder "herdado de TJCE".
- Virtualização (TanStack Virtual) nas três visões (acima de 500 hosts).
- Favoritos e Recentes no topo; tag de ambiente com cor (prod em vermelho) refletida na borda da aba.
- Conexão rápida na busca: `admin@10.0.0.1:2222` + Enter conecta, com opção de salvar depois.
- Ícone do fabricante detectado pelo banner SSH ou prompt: Huawei VRP, RouterOS, ZTE, Datacom,
  Cisco, Linux (no lugar das iniciais).

## Prioridade alta

- Telnet e serial (`go.bug.st/serial`) — OLTs, switches antigos e console.
- Jump host / ProxyJump por host e herdado do grupo.
- Snippets com variáveis (`{{interface}}`), em uma ou várias abas.
- Broadcast de entrada para várias abas ou painéis.
- Painéis divididos / workspaces, Ctrl+Alt+setas para mover o foco.
- Log de sessão em arquivo com data/hora (auditoria e evidência para cliente).
- Página Known Hosts para ver e remover fingerprints (o backend já existe).
- Importar `~/.ssh/config`, CSV do Termius, sessões do PuTTY (registro) e MobaXterm (`.mxtsessions`).

## Prioridade média

- Painel de tema dentro da sessão: trocar esquema e fonte com a aba aberta.
- Confirmação de colagem multilinha com número de linhas e prévia.

## Decidido

- **Nome**: o app passa a se chamar **OpenTerminal** (24/10/2026). "OpenConnect" foi descartado por
  ser o cliente VPN open source (compatível com AnyConnect, par do ocserv), o que confundia busca e
  marca no público de redes.
