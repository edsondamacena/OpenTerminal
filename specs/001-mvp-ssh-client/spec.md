# Feature Specification: MVP do cliente SSH ShellVault

**Feature Branch**: `001-mvp-ssh-client`

**Created**: 2026-10-23

**Status**: Draft

**Input**: User description: "(vazio) — escopo do MVP definido em
`docs/reference/termius-analysis.md`: cadastro de hosts com grupos e busca, terminal SSH em abas
(senha e chave, verificação de host key), terminal local, cofre local criptografado com senha
mestra, SFTP básico e port forwarding local."

## Clarifications

### Session 2026-10-23

- Q: Se o usuário esquecer a senha mestra, deve haver forma de recuperar o acesso ao cofre? → A:
  Não. Sem chave de recuperação nem desbloqueio alternativo; a única saída é apagar o cofre.
- Q: Ao bloquear o cofre por inatividade, o que acontece com sessões, transferências e
  encaminhamentos abertos? → A: Continuam ativos e visíveis; só o acesso a hosts e credenciais
  fica bloqueado.
- Q: Quais shells o terminal local deve oferecer no MVP? → A: Só PowerShell (Windows PowerShell
  nativo) e Prompt de Comando; PowerShell 7, Git Bash, WSL e shells personalizados ficam fora.
- Q: O app deve gravar log de diagnóstico local? → A: Não; nenhum log em arquivo, erros aparecem
  apenas na interface.
- Q: Onde aplicar o visual de vidro translúcido (estilo "liquid glass")? → A: Na moldura (barra
  lateral, abas, barra de título, diálogos, menus); terminal e SFTP com fundo sólido; efeito
  desligado quando o Windows está com transparência desativada ou em alto contraste.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Cofre com senha mestra (Priority: P1)

Na primeira execução, o usuário cria uma senha mestra que protege todos os dados sensíveis do app.
Nas execuções seguintes, desbloqueia o cofre com essa senha antes de acessar hosts e credenciais.
O cofre se bloqueia sozinho após um período de inatividade e pode ser bloqueado manualmente.

**Why this priority**: todas as outras histórias guardam credenciais; sem o cofre não há onde
armazená-las com segurança (Princípio II da constituição).

**Independent Test**: criar o cofre, fechar o app, reabrir, desbloquear com a senha correta e
verificar que a senha errada é recusada e que o arquivo de dados não contém nenhum segredo legível.

**Acceptance Scenarios**:

1. **Given** primeira execução, **When** o usuário define e confirma uma senha mestra que atende
   ao requisito mínimo, **Then** o cofre é criado e fica desbloqueado.
2. **Given** cofre existente e bloqueado, **When** o usuário informa a senha correta, **Then** o
   cofre é desbloqueado e os hosts aparecem.
3. **Given** cofre bloqueado, **When** o usuário informa senha incorreta, **Then** o acesso é
   negado com mensagem genérica e nenhum dado é exibido.
4. **Given** cofre desbloqueado e sem interação por 15 minutos, **When** o tempo expira, **Then**
   o cofre é bloqueado e a senha mestra é exigida novamente; sessões abertas continuam ativas.
5. **Given** cofre desbloqueado, **When** o usuário troca a senha mestra informando a atual,
   **Then** a nova senha passa a valer e a antiga deixa de funcionar.

---

### User Story 2 - Cadastrar hosts e conectar via SSH em abas (Priority: P1)

O usuário cadastra servidores (nome, endereço, porta, usuário e método de autenticação: senha ou
chave privada) e abre uma sessão SSH interativa com um clique. Cada sessão abre em uma aba; várias
sessões podem ficar abertas ao mesmo tempo. Na primeira conexão a um servidor, o app mostra a
impressão digital da chave do servidor para o usuário confirmar.

**Why this priority**: é o propósito central do produto; junto com o cofre forma o menor produto
utilizável.

**Independent Test**: cadastrar um host de teste com senha e outro com chave, conectar a ambos em
abas separadas, executar comandos e encerrar as sessões.

**Acceptance Scenarios**:

1. **Given** cofre desbloqueado, **When** o usuário cadastra um host com senha e clica em
   conectar, **Then** abre uma aba com terminal interativo autenticado.
2. **Given** um host com chave privada (com ou sem passphrase) importada para o cofre, **When** o
   usuário conecta, **Then** a autenticação usa a chave sem pedir senha do servidor.
3. **Given** primeira conexão a um servidor, **When** a conexão é estabelecida, **Then** o app
   exibe a impressão digital da chave do servidor e só prossegue após confirmação; a chave aceita
   é memorizada.
4. **Given** um servidor cuja chave memorizada mudou, **When** o usuário conecta, **Then** a
   conexão é bloqueada com alerta explícito e só prossegue se o usuário aceitar a nova chave
   conscientemente.
5. **Given** duas sessões abertas, **When** o usuário alterna entre abas, **Then** cada aba mantém
   seu histórico de tela e sessão independentes.
6. **Given** uma sessão ativa, **When** a conexão cai, **Then** a aba indica a desconexão e
   oferece reconectar.
7. **Given** uma sessão ativa, **When** o usuário redimensiona a janela, **Then** o terminal
   remoto se ajusta ao novo tamanho.

---

### User Story 3 - Organizar hosts em grupos e buscar (Priority: P2)

O usuário organiza hosts em grupos (ex.: "Produção", "Clientes/ACME"), edita e remove hosts e
grupos, e encontra rapidamente um host digitando parte do nome, endereço ou usuário.

**Why this priority**: torna o app prático com dezenas de servidores, mas não é necessário para a
primeira conexão.

**Independent Test**: cadastrar 50 hosts em grupos diferentes, buscar por fragmento de nome e de
endereço e mover hosts entre grupos.

**Acceptance Scenarios**:

1. **Given** hosts cadastrados, **When** o usuário cria um grupo e move hosts para ele, **Then** a
   lista mostra os hosts agrupados.
2. **Given** grupos podem conter subgrupos, **When** o usuário cria um subgrupo, **Then** ele
   aparece aninhado sob o grupo pai.
3. **Given** a busca, **When** o usuário digita parte do nome, endereço ou usuário, **Then** a
   lista filtra os hosts correspondentes enquanto digita, sem distinguir maiúsculas/minúsculas.
4. **Given** um grupo com hosts, **When** o usuário exclui o grupo, **Then** o app pede
   confirmação e os hosts passam para o nível raiz (não são excluídos).
5. **Given** um host, **When** o usuário o duplica, **Then** é criada uma cópia editável.

---

### User Story 4 - Terminal local (Priority: P2)

O usuário abre, em uma nova aba, um terminal do próprio computador (PowerShell ou Prompt de
Comando) sem sair do app.

**Why this priority**: conveniência frequente para quem trabalha com servidores, mas independente
do núcleo SSH.

**Independent Test**: abrir uma aba de PowerShell e uma de Prompt de Comando, executar comandos e
fechar as abas.

**Acceptance Scenarios**:

1. **Given** o app aberto, **When** o usuário escolhe "novo terminal local" e um shell, **Then**
   abre uma aba com o shell no diretório pessoal do usuário.
2. **Given** um terminal local aberto, **When** o usuário digita `exit`, **Then** a aba indica que
   o processo terminou.
3. **Given** o terminal local, **When** o cofre está bloqueado, **Then** o terminal local continua
   disponível (não depende do cofre).

---

### User Story 5 - Transferir arquivos via SFTP (Priority: P3)

A partir de um host cadastrado, o usuário abre um navegador de arquivos remoto, navega por pastas,
envia arquivos do computador para o servidor e baixa arquivos do servidor.

**Why this priority**: complementa o SSH; é útil, mas o usuário tem alternativas externas.

**Independent Test**: abrir SFTP em um host de teste, navegar, enviar e baixar um arquivo de
100 MB e comparar o conteúdo.

**Acceptance Scenarios**:

1. **Given** um host cadastrado, **When** o usuário abre o SFTP, **Then** vê o conteúdo do
   diretório inicial remoto (nome, tamanho, data de modificação, tipo).
2. **Given** o navegador remoto, **When** o usuário entra e sai de pastas, **Then** a listagem e o
   caminho atual são atualizados.
3. **Given** o navegador remoto, **When** o usuário envia um arquivo local, **Then** o progresso é
   exibido e o arquivo aparece na pasta remota com o mesmo conteúdo.
4. **Given** um arquivo remoto, **When** o usuário o baixa e escolhe o destino, **Then** o
   progresso é exibido e o arquivo local é idêntico ao remoto.
5. **Given** já existe arquivo com o mesmo nome no destino, **When** a transferência começa,
   **Then** o app pergunta se deve substituir, pular ou cancelar.
6. **Given** uma transferência em andamento, **When** o usuário cancela, **Then** ela é
   interrompida e nenhum arquivo parcial é deixado com o nome final.
7. **Given** o navegador remoto, **When** o usuário cria pasta, renomeia ou exclui um item (com
   confirmação), **Then** a alteração é refletida no servidor.

---

### User Story 6 - Port forwarding local (Priority: P3)

O usuário define regras de encaminhamento local de porta associadas a um host (porta local →
destino acessível a partir do servidor), as ativa e desativa, e acessa serviços remotos pela
porta local.

**Why this priority**: recurso avançado usado por um subconjunto de usuários.

**Independent Test**: criar uma regra 127.0.0.1:15432 → banco:5432 via host de teste, ativá-la e
conectar a um serviço pela porta local.

**Acceptance Scenarios**:

1. **Given** um host, **When** o usuário cria uma regra com porta local, host de destino e porta
   de destino, **Then** a regra fica salva associada ao host.
2. **Given** uma regra salva, **When** o usuário a ativa, **Then** conexões à porta local são
   encaminhadas ao destino pelo servidor e o status aparece como "ativa".
3. **Given** a porta local já está em uso, **When** o usuário ativa a regra, **Then** o app
   informa o conflito e a regra não é ativada.
4. **Given** uma regra ativa, **When** o usuário a desativa ou a conexão com o servidor cai,
   **Then** a porta local é liberada e o status é atualizado.

---

### Edge Cases

- Host inalcançável ou porta fechada: erro claro em até 15 s (tempo limite padrão), sem travar a
  interface.
- Usuário ou senha inválidos, ou chave recusada: mensagem explicando que a autenticação falhou,
  sem revelar qual credencial estava errada além do que o servidor informar.
- Chave privada em formato não suportado ou passphrase incorreta: rejeitada na importação com
  mensagem clara; nada é salvo.
- Servidor que exige autenticação interativa (keyboard-interactive, ex.: código 2FA): o app exibe
  os prompts do servidor ao usuário.
- Senha mestra esquecida: não há recuperação; o usuário pode apagar o cofre e começar do zero
  após confirmação dupla.
- Arquivo do cofre corrompido ou de versão desconhecida: o app recusa abrir, não sobrescreve o
  arquivo e informa o problema.
- Sessão que cai enquanto o cofre está bloqueado: a aba continua visível, mas reconectar exige
  desbloquear o cofre primeiro.
- Fechar o app com sessões ou transferências ativas: o app pede confirmação antes de encerrar.
- Muitas abas abertas (20+): o app continua responsivo.
- Nomes de arquivo remotos com caracteres não ASCII ou espaços: exibidos e transferidos
  corretamente.
- Dois hosts com o mesmo nome: permitido; a lista mostra o endereço para diferenciá-los.

## Requirements *(mandatory)*

### Functional Requirements

**Cofre**

- **FR-001**: O sistema MUST exigir a criação de uma senha mestra (mínimo de 8 caracteres,
  confirmada por digitação dupla) na primeira execução.
- **FR-002**: O sistema MUST armazenar hosts, senhas, chaves privadas, passphrases, chaves de
  servidor conhecidas e regras de encaminhamento apenas de forma criptografada e protegida pela
  senha mestra.
- **FR-003**: O sistema MUST bloquear o cofre após 15 minutos sem interação (valor configurável
  entre 1 e 120 minutos, ou desativado) e permitir bloqueio manual a qualquer momento. O bloqueio
  MUST NOT encerrar nem ocultar sessões, transferências ou encaminhamentos já abertos; MUST
  impedir abrir novas conexões, ver ou editar hosts e credenciais até o desbloqueio.
- **FR-004**: O sistema MUST permitir trocar a senha mestra mediante a senha atual.
- **FR-005**: O sistema MUST NOT gravar segredos em logs, mensagens de erro ou arquivos não
  criptografados.
- **FR-006**: O sistema MUST permitir apagar o cofre inteiro após confirmação dupla, para o caso
  de senha mestra esquecida. O sistema MUST NOT oferecer chave de recuperação nem outro meio de
  desbloqueio além da senha mestra, e MUST avisar isso ao usuário na criação do cofre.

**Hosts e grupos**

- **FR-007**: Usuários MUST poder criar, editar, duplicar e excluir hosts com: rótulo, endereço
  (nome ou IP, IPv4/IPv6), porta (padrão 22), usuário e método de autenticação (senha, chave
  privada ou perguntar ao conectar).
- **FR-008**: Usuários MUST poder importar chaves privadas (formatos OpenSSH e PEM, tipos
  Ed25519, ECDSA e RSA) para o cofre e reutilizar a mesma chave em vários hosts.
- **FR-009**: Usuários MUST poder gerar um novo par de chaves (Ed25519) no app e copiar a chave
  pública.
- **FR-010**: Usuários MUST poder criar, renomear, aninhar e excluir grupos e mover hosts entre
  grupos.
- **FR-011**: O sistema MUST filtrar a lista de hosts por texto (rótulo, endereço, usuário),
  ignorando maiúsculas/minúsculas, enquanto o usuário digita.

**Sessões de terminal**

- **FR-012**: O sistema MUST abrir cada sessão (SSH ou local) em uma aba própria, permitindo
  várias simultâneas, renomear, reordenar e fechar abas.
- **FR-013**: O sistema MUST verificar a chave do servidor em toda conexão SSH: na primeira vez,
  exibir a impressão digital (SHA-256) para confirmação; em caso de divergência com a chave
  memorizada, bloquear a conexão com alerta até aceitação explícita.
- **FR-014**: O sistema MUST suportar autenticação por senha, chave privada (com passphrase) e
  keyboard-interactive.
- **FR-015**: O terminal MUST suportar cores (256 e true color), redimensionamento, copiar/colar,
  rolagem de pelo menos 10.000 linhas e caracteres Unicode.
- **FR-016**: O sistema MUST indicar desconexão na aba e oferecer reconexão com um clique.
- **FR-017**: O sistema MUST abrir terminais locais com PowerShell e Prompt de Comando, iniciando
  no diretório pessoal do usuário, sem exigir o cofre desbloqueado. Outros shells (PowerShell 7,
  Git Bash, WSL, personalizados) estão fora do MVP.
- **FR-018**: Usuários MUST poder ajustar fonte, tamanho da fonte e tema claro/escuro do terminal.

**SFTP**

- **FR-019**: O sistema MUST abrir um navegador de arquivos remoto para um host, listando nome,
  tamanho, data de modificação e tipo, com navegação entre pastas.
- **FR-020**: Usuários MUST poder enviar e baixar arquivos (incluindo vários de uma vez) com
  indicação de progresso e cancelamento.
- **FR-021**: O sistema MUST perguntar o que fazer em conflito de nome (substituir, pular,
  cancelar) e MUST NOT deixar arquivo parcial com o nome final após falha ou cancelamento.
- **FR-022**: Usuários MUST poder criar pastas, renomear e excluir itens remotos, com confirmação
  para exclusão.

**Port forwarding**

- **FR-023**: Usuários MUST poder criar, editar e excluir regras de encaminhamento local
  (endereço e porta locais, host e porta de destino) associadas a um host.
- **FR-024**: O sistema MUST permitir ativar e desativar regras individualmente, exibindo o
  status de cada uma (inativa, ativa, erro).
- **FR-025**: O sistema MUST, por padrão, escutar apenas em 127.0.0.1 e exigir escolha explícita
  para escutar em outras interfaces.
- **FR-026**: O sistema MUST informar quando a porta local está em uso e liberar a porta ao
  desativar a regra ou perder a conexão.

**Geral**

- **FR-027**: Todas as funcionalidades acima MUST funcionar sem conta e sem serviço online do
  projeto; o app MUST NOT enviar dados do usuário para fora da máquina.
- **FR-028**: O sistema MUST pedir confirmação ao fechar com sessões, transferências ou
  encaminhamentos ativos.
- **FR-029**: A interface MUST estar em português do Brasil.
- **FR-030**: O sistema MUST NOT gravar logs de diagnóstico em arquivo no MVP; erros MUST ser
  apresentados ao usuário na interface, com mensagem compreensível.

**Aparência**

- **FR-031**: A moldura do app (barra lateral de hosts, abas, barra de título, diálogos e menus)
  MUST usar superfícies de vidro translúcido: o conteúdo por trás aparece desfocado, com brilho
  sutil nas bordas, em tema claro e escuro. O visual MUST ser criação própria, sem assets,
  ícones ou componentes de terceiros (Princípio I).
- **FR-032**: A área do terminal e o navegador SFTP MUST ter fundo sólido, sem transparência.
- **FR-033**: O sistema MUST substituir o vidro por superfícies sólidas equivalentes quando o
  Windows estiver com "Efeitos de transparência" desativado, em modo de alto contraste, ou quando
  o sistema não suportar o efeito, mantendo o app totalmente utilizável.
- **FR-034**: Todo texto sobre superfícies de vidro MUST manter contraste mínimo de 4,5:1 em
  relação ao fundo, qualquer que seja o conteúdo por trás da janela.

### Key Entities

- **Cofre**: contêiner criptografado de todos os dados do usuário; tem versão de formato,
  configuração de bloqueio automático e é aberto pela senha mestra.
- **Host**: servidor cadastrado; rótulo, endereço, porta, usuário, método de autenticação,
  referência a credencial, grupo (opcional) e regras de encaminhamento.
- **Grupo**: agrupamento nomeado de hosts; pode ter grupo pai (aninhamento).
- **Credencial**: senha ou chave privada (com passphrase opcional e chave pública derivada);
  pode ser referenciada por vários hosts.
- **Chave de servidor conhecida**: associação endereço:porta → tipo e impressão digital da chave
  do servidor aceita pelo usuário.
- **Regra de encaminhamento**: endereço e porta locais, host e porta de destino, host associado.
- **Sessão**: aba aberta (SSH, local ou SFTP) com estado (conectando, conectada, desconectada);
  existe apenas em memória.
- **Transferência**: envio ou download em andamento com origem, destino, progresso e estado.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Um usuário novo cria o cofre, cadastra um host e abre a primeira sessão SSH em
  menos de 3 minutos, sem consultar documentação.
- **SC-002**: Com o cofre desbloqueado, uma conexão a um servidor na rede local fica pronta para
  uso em menos de 3 segundos após o clique.
- **SC-003**: A busca filtra uma lista de 1.000 hosts com resultado visível em menos de 200 ms
  por tecla digitada.
- **SC-004**: Nenhuma senha, chave privada ou passphrase é encontrada em texto legível em
  qualquer arquivo gravado pelo app (verificado por busca em todos os arquivos que o app cria).
- **SC-005**: 100% das conexões a servidores com chave alterada são bloqueadas até confirmação
  explícita.
- **SC-006**: Arquivos de 1 GB enviados e baixados via SFTP chegam com conteúdo idêntico, a uma
  velocidade de pelo menos 80% da obtida com ferramentas de linha de comando na mesma rede.
- **SC-007**: O app permanece responsivo (troca de aba em menos de 100 ms) com 20 sessões abertas
  simultaneamente.
- **SC-008**: Todas as funcionalidades do MVP funcionam com o computador sem acesso à internet
  (usando servidores na rede local).
- **SC-009**: Com o vidro ativo, a troca de aba e a rolagem do terminal continuam abaixo de
  100 ms, e mover ou redimensionar a janela por 5 s mantém menos de 5% dos quadros acima de
  50 ms.

## Assumptions

- Usuário único por instalação do Windows; não há contas nem múltiplos perfis no MVP.
- Plataforma alvo: Windows 10 e 11 (64 bits); macOS e Linux ficam para versões futuras.
- O visual de vidro é pensado primeiro para o Windows 11; no Windows 10 o efeito pode ser mais
  simples ou cair no fallback sólido (FR-033).
- Não há recuperação de senha mestra: é a garantia de que só o usuário acessa os dados.
- Tempo de bloqueio padrão de 15 minutos e tempo limite de conexão de 15 s são padrões de mercado
  e configuráveis.
- Importação de `~/.ssh/config` ou de outros clientes, sincronização em nuvem, equipes, FIDO2,
  Mosh, serial, Azure, autocomplete, encaminhamento remoto/dinâmico (SOCKS), jump hosts, agente
  SSH e planos pagos estão fora do escopo (Princípio V); serão specs próprias.
- Os servidores de destino rodam um servidor SSH padrão com SFTP habilitado.
- Exportação/backup do cofre fica fora do MVP; o arquivo criptografado pode ser copiado
  manualmente.
