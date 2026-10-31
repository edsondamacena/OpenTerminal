# Quickstart: validação do MVP ShellVault

**Feature**: `001-mvp-ssh-client` | **Date**: 2026-10-23

Guia para provar, de ponta a ponta, que o MVP atende à spec. Detalhes de API em
[contracts/ipc-bridge.md](contracts/ipc-bridge.md); entidades em [data-model.md](data-model.md).

## Pré-requisitos

- Windows 10/11 x64; Node.js 24 LTS; npm 11.
- Docker Desktop em execução (servidor OpenSSH de teste para integração).
- Opcional: `ssh`/`sftp` do Windows (OpenSSH Client) para a comparação de velocidade de SC-006.

## Preparação

```powershell
npm ci
npm run test:sshd:up      # sobe o contêiner OpenSSH de teste em 127.0.0.1:2222
```

O contêiner (`tests/fixtures/sshd/`) expõe os usuários:

| Usuário | Autenticação |
|---|---|
| `pwuser` | senha `pw-test-123` |
| `keyuser` | chave Ed25519 de teste (`tests/fixtures/keys/`, com e sem passphrase); RSA e ECDSA também |
| `kbuser` | keyboard-interactive (prompt do servidor via PAM, resposta `kb-test-123`) |

E um serviço TCP de eco em `echo:7000` acessível só de dentro da rede do contêiner (para port
forwarding).

## Verificações automáticas

```powershell
npm run lint
npm run typecheck
npm run test:unit          # Vitest — cofre, cripto, hosts/grupos, busca, regras, tokens de contraste
npm run test:integration   # Vitest — SSH, SFTP e forwarding contra o contêiner
npm run test:e2e           # Playwright + Electron — fluxos da interface
```

Esperado: tudo verde. Nenhuma tarefa é concluída com teste falhando (Princípio IV).

## Cenários manuais (por história)

Rodar `npm run dev`.

1. **US1 — Cofre**: criar senha mestra `senha-teste-1` (recusar `1234567`); fechar e reabrir;
   senha errada → mensagem genérica; senha certa → hosts visíveis. Em Configurações, ajustar
   bloqueio para 1 min, esperar → tela de bloqueio, abas abertas continuam. Trocar a senha;
   a antiga deixa de funcionar.
2. **US2 — SSH**: cadastrar `127.0.0.1:2222` / `pwuser` (senha) e `keyuser` (chave importada
   com passphrase). Primeira conexão mostra `SHA256:…`; aceitar. Rodar `top` e redimensionar a
   janela. `npm run test:sshd:rotate-key` e reconectar → bloqueio com alerta. Parar o
   contêiner → aba mostra desconexão e botão Reconectar. Testar `kbuser` (prompts aparecem).
3. **US3 — Grupos e busca**: `npm run seed:hosts -- 1000` (popula cofre de desenvolvimento);
   criar `Clientes/ACME`, mover hosts, buscar por fragmento de endereço em maiúsculas;
   excluir grupo → hosts vão para a raiz.
4. **US4 — Terminal local**: abrir PowerShell e Prompt de Comando; `cd` mostra a pasta do
   usuário; `exit` → aba indica processo encerrado; bloquear cofre → novo terminal local
   ainda abre.
5. **US5 — SFTP**: abrir SFTP em `pwuser`; criar pasta `teste ç ã`; enviar arquivo de 100 MB,
   baixar de volta e comparar com `Get-FileHash`; repetir envio → diálogo
   substituir/pular/cancelar; cancelar um envio grande no meio → nenhum arquivo com o nome
   final no destino.
6. **US6 — Forwarding**: regra `127.0.0.1:17000 → echo:7000` em `pwuser`; ativar; testar com
   `Test-NetConnection 127.0.0.1 -Port 17000` e um cliente de eco; ocupar a porta antes →
   erro "porta em uso"; desativar → porta liberada.

## Critérios de sucesso medidos

| SC | Como medir |
|---|---|
| SC-002 | teste de integração mede clique→prompt pronto contra o contêiner local (< 3 s) |
| SC-003 | teste unitário de busca com 1.000 hosts (< 200 ms por consulta, mais E2E de digitação) |
| SC-004 | `npm run test:secrets-scan` — após fluxo E2E completo, busca as senhas/chaves de teste (texto, base64 e UTF-16) em todo `%APPDATA%\ShellVault` de teste; esperado 0 ocorrências |
| SC-005 | teste de integração de troca de host key: conexão nunca chega a `authenticating` sem aceite |
| SC-006 | `npm run bench:sftp` — 1 GB upload/download vs. `sftp.exe` no mesmo contêiner; ≥ 80% e hash igual |
| SC-007 / SC-009 | E2E abre 20 abas e mede troca de aba (< 100 ms) com vidro ligado e desligado |
| SC-008 | executar a suíte de integração com o adaptador de rede externo desativado |
| FR-033 | Windows: Configurações → Personalização → Cores → desligar "Efeitos de transparência" e ativar alto contraste; moldura fica sólida sem reiniciar |
