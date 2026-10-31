# Feature Specification: Migração do ShellVault para nova base desktop (Fase 1)

**Feature Branch**: `002-wails-migration`

**Created**: 2026-10-23

**Status**: Draft

**Input**: User description: "Migração do ShellVault de Electron para Go + Wails mantendo paridade total com o MVP 001, corrigindo bugs de processo, layout e terminal, com design novo em estilo glass, personalização do terminal e recursos para uso em NOC (keepalive, algoritmos legados por host, ssh-agent, atalhos)."

## Contexto

O ShellVault (feature 001) funciona, mas a base atual apresenta bugs recorrentes de processo,
layout e terminal, além de um visual pouco acabado. Esta feature reescreve o app sobre uma nova
base desktop, decidida pelo usuário e registrada como insumo do plano. O objetivo é que o
usuário de NOC (que acessa OLTs, roteadores e servidores de vários fabricantes no dia a dia)
não perca nenhuma capacidade, tenha os bugs conhecidos eliminados na raiz e ganhe um terminal
personalizável e uma interface com identidade visual própria.

**Regra de paridade:** todo requisito funcional de `specs/001-mvp-ssh-client/spec.md`
(FR-001 a FR-034) continua valendo nesta feature, salvo onde um requisito abaixo o substitui
de forma explícita. Nada que o app faz hoje pode deixar de existir.

## Clarifications

### Session 2026-10-23

- Q: Processo de desenvolvimento (Spec Kit ou GSD)? → A: Spec Kit, nesta feature 002.
- Q: Onde ficam os segredos: cofre com senha mestra ou keychain do sistema? → A: Manter o cofre com senha mestra; o keychain do sistema não é usado.
- Q: SFTP e port forwarding local entram na Fase 1 ou na Fase 2? → A: Fase 1 (paridade total).
- Q: O que acontece com o código atual? → A: Congelado como baseline no mesmo repositório (commit `7206c0c`); a migração ocorre nesta branch e o app atual serve de referência até a paridade.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Paridade do núcleo: cofre, hosts e sessões SSH sem regressões (Priority: P1)

O operador abre o app novo, cria ou desbloqueia o cofre e encontra os mesmos hosts, grupos e
credenciais que já usava. Conecta em um equipamento e recebe um único prompt, digita na hora e
fecha a aba sem deixar nada rodando em segundo plano.

**Why this priority**: sem o núcleo (cofre, hosts, SSH em abas) não existe produto; e os bugs
de prompt duplicado e de processos órfãos são os que mais atrapalham o uso diário.

**Independent Test**: com um servidor SSH de teste, criar o cofre, cadastrar um host, conectar,
verificar um único prompt e encerrar a aba; repetir 100 vezes e verificar que os recursos do
app voltam ao nível inicial.

**Acceptance Scenarios**:

1. **Given** um cofre existente criado pela versão anterior, **When** o usuário abre a versão nova e digita a senha mestra, **Then** todos os hosts, grupos, credenciais, host keys conhecidas e regras de encaminhamento aparecem iguais e utilizáveis.
2. **Given** um host cadastrado, **When** o usuário conecta, **Then** o banner e o prompt aparecem exatamente uma vez, sem repetições.
3. **Given** uma aba conectada, **When** o usuário fecha a aba, **Then** a conexão é encerrada, nenhuma atividade da sessão permanece em execução e o servidor registra a desconexão.
4. **Given** 100 ciclos de abrir e fechar abas, **When** o último ciclo termina, **Then** o consumo de recursos do app volta ao patamar de antes dos ciclos.
5. **Given** o app aberto, **When** o usuário tenta abrir uma segunda instância, **Then** nenhuma janela nova aparece e a janela existente vem para frente.
6. **Given** qualquer operação do app (abrir shell, conectar, transferir), **When** ela é executada, **Then** nenhuma janela extra ou vazia do próprio app é aberta.
7. **Given** a primeira conexão a um servidor, **When** a chave do servidor é apresentada, **Then** o fingerprint SHA-256 é exibido para confirmação; e se a chave mudar depois, a conexão é bloqueada com aviso claro.

---

### User Story 2 - Janela e layout estáveis em qualquer tamanho (Priority: P1)

O operador maximiza, restaura, encaixa a janela na lateral da tela e troca de monitor com escala
diferente. A interface sempre ocupa a janela inteira, a lista de hosts nunca é cortada e o
terminal nunca fica em branco ou com faixas vazias.

**Why this priority**: o layout quebrando ao redimensionar é um dos bugs atuais mais visíveis e
impede o uso em telas divididas, comuns em NOC.

**Independent Test**: com duas abas abertas, executar 20 ciclos de maximizar, restaurar e
redimensionar, além de encaixe lateral e escalas de 100%, 125% e 150%, verificando após cada
ciclo que não há área vazia, corte ou terminal em branco e que o servidor recebeu o novo tamanho.

**Acceptance Scenarios**:

1. **Given** uma sessão aberta, **When** a janela é maximizada e restaurada 20 vezes seguidas, **Then** a interface preenche toda a janela em todos os ciclos e o terminal mostra o conteúdo correto.
2. **Given** escala de exibição de 125% ou 150%, **When** a janela é redimensionada, **Then** não aparecem faixas vazias, a barra lateral não é cortada e o texto do terminal continua nítido.
3. **Given** um redimensionamento contínuo (arrastar a borda), **When** o usuário solta a borda, **Then** o servidor recebe o tamanho final do terminal, e o terminal não recebe uma enxurrada de mudanças de tamanho intermediárias.
4. **Given** o botão "+" de nova aba, **When** o usuário o aciona, **Then** o menu de shells locais abre ancorado ao botão, inteiramente dentro da janela e por cima do terminal, mesmo perto das bordas.
5. **Given** qualquer menu de contexto (host, aba, arquivo), **When** é aberto perto de uma borda da janela, **Then** ele se reposiciona para ficar inteiro visível.

---

### User Story 3 - Uso em NOC: equipamentos legados, conexões longas e atalhos (Priority: P1)

O operador acessa OLTs e roteadores antigos que só aceitam algoritmos de criptografia antigos,
deixa sessões abertas por horas e trabalha quase só pelo teclado.

**Why this priority**: sem suporte a equipamentos legados, parte do parque do NOC fica
inacessível; sessões que caem por inatividade e a falta de atalhos atrasam a operação.

**Independent Test**: conectar em um servidor de teste que só aceita algoritmos legados (falha
com a opção desligada, funciona com a opção ligada); deixar uma sessão ociosa por 30 minutos
atrás de um equipamento que derruba conexões ociosas; operar abas e busca apenas por teclado.

**Acceptance Scenarios**:

1. **Given** um host que só aceita algoritmos legados, **When** a opção "algoritmos legados" do host está desligada, **Then** a conexão falha com mensagem que sugere ligar a opção; **When** a opção é ligada, **Then** a conexão funciona.
2. **Given** a opção "algoritmos legados" ligada em um host, **When** o usuário vê a lista ou a edição do host, **Then** há indicação visível de que o host usa algoritmos fracos.
3. **Given** uma sessão ociosa, **When** passam 30 minutos sem digitação, **Then** a sessão continua conectada graças a sinais periódicos de atividade.
4. **Given** a conexão caiu (rede ou servidor), **When** a perda é detectada, **Then** a aba mostra o estado "desconectado" com um botão de reconectar em até 90 segundos.
5. **Given** um agente SSH em execução no computador com chaves carregadas, **When** um host está configurado para usar o agente, **Then** a autenticação usa as chaves do agente sem exigir chave no cofre.
6. **Given** o foco no app, **When** o usuário usa Ctrl+Shift+T, Ctrl+Shift+W, Ctrl+Tab, Ctrl+Shift+Tab, Ctrl+Shift+F, Ctrl+Shift+P, Ctrl+=, Ctrl+- e Ctrl+0, **Then** respectivamente: abre nova aba local, fecha a aba atual (com confirmação se conectada), vai à próxima e à anterior aba, abre a busca no terminal, abre a paleta de comandos, aumenta, diminui e restaura o tamanho da fonte.
7. **Given** a paleta de comandos aberta, **When** o usuário digita parte do nome de um host ou de uma ação, **Then** pode conectar no host, abrir SFTP, abrir shell local, abrir configurações ou bloquear o cofre sem usar o mouse.
8. **Given** texto selecionado no terminal, **When** a seleção termina, **Then** o texto é copiado; **When** o usuário cola várias linhas (Ctrl+Shift+V ou botão direito), **Then** o app pede confirmação mostrando o número de linhas antes de enviar.
9. **Given** uma URL na saída do terminal, **When** o usuário a aciona com Ctrl+clique, **Then** ela é aberta no navegador padrão do sistema (o app em si continua sem fazer requisições de rede).

---

### User Story 4 - Personalizar o terminal com preview ao vivo (Priority: P2)

O operador ajusta fonte, cores e cursor do terminal vendo o resultado na hora, importa o esquema
de cores que já usa em outro terminal e marca equipamentos de produção com um fundo
avermelhado para não digitar comando no lugar errado.

**Why this priority**: melhora conforto e reduz erro humano, mas o app é utilizável sem isso.

**Independent Test**: abrir as configurações do terminal com duas abas conectadas, mudar fonte,
tamanho e esquema de cores, verificar o preview e as abas abertas sem reconectar; importar um
esquema do Windows Terminal e um do iTerm2; aplicar um perfil a um host e conferir que só ele muda.

**Acceptance Scenarios**:

1. **Given** a tela de personalização, **When** o usuário muda qualquer opção, **Then** o terminal de amostra reflete a mudança em menos de 200 ms.
2. **Given** abas conectadas, **When** o usuário salva uma mudança no perfil global, **Then** todas as abas abertas aplicam a mudança sem reconectar e sem perder o conteúdo.
3. **Given** um arquivo de esquema de cores do Windows Terminal (JSON) ou do iTerm2 (`.itermcolors`), **When** o usuário o importa, **Then** o esquema aparece na lista com suas 16 cores ANSI, fundo, texto, cursor e seleção; **When** o arquivo é inválido, **Then** uma mensagem diz o que está errado e nada é alterado.
4. **Given** um esquema de cores, **When** o usuário o exporta, **Then** recebe um arquivo JSON que pode ser importado de novo com resultado idêntico.
5. **Given** um host com perfil próprio (override), **When** o usuário conecta nele, **Then** a aba usa o perfil do host e as demais continuam com o perfil global.
6. **Given** os presets embutidos (Dracula, Nord, Solarized Dark, Solarized Light, One Dark, Tokyo Night, Catppuccin), **When** o usuário escolhe um, **Then** ele é aplicado por completo e pode ser duplicado e editado.
7. **Given** a opacidade do fundo do terminal, **When** o usuário a reduz abaixo de 100%, **Then** o app avisa que pode reduzir legibilidade e desempenho; o padrão é 100%.

---

### User Story 5 - Nova identidade visual glass (Priority: P2)

O operador usa um app com aparência moderna e consistente: superfícies translúcidas sobre o
fundo da janela, barra de título própria, barra lateral com grupos recolhíveis e busca, abas com
estado claro, em tema escuro ou claro, com alternativa opaca quando o vidro não está disponível.

**Why this priority**: o visual atual é percebido como pouco acabado; mas a identidade não
bloqueia o uso operacional.

**Independent Test**: comparar capturas de cada tela (cofre, lista de hosts, terminal, SFTP,
encaminhamento, configurações, diálogos e menus) nos temas escuro e claro, com vidro e sem
vidro, contra o guia de marca; verificar contraste mínimo de texto em todas.

**Acceptance Scenarios**:

1. **Given** um sistema que suporta efeito de vidro, **When** o app abre, **Then** barra lateral, abas, diálogos e menus aparecem translúcidos sobre o fundo da janela.
2. **Given** um sistema sem suporte ao vidro, com transparência desativada no sistema, em alto contraste ou com o vidro desligado nas configurações, **When** o app abre, **Then** todas as superfícies aparecem opacas com aparência equivalente, sem reiniciar.
3. **Given** qualquer tela, **When** comparada ao guia de marca, **Then** cores, tipografia, raios, sombras e ícones vêm todos do conjunto de tokens da marca, sem valores avulsos.
4. **Given** uma aba, **When** a sessão muda de estado (conectando, conectado, desconectado, erro), **Then** a aba mostra o estado por cor e ícone, legíveis também para quem não distingue cores.
5. **Given** a barra lateral, **When** o usuário recolhe ou expande um grupo ou busca um host, **Then** a árvore reflete a ação de imediato e mantém o estado recolhido entre execuções.
6. **Given** a barra lateral expandida, **When** o usuário a minimiza (botão ou Ctrl+Shift+B), **Then** ela vira um trilho estreito só com ícones (hosts, credenciais, SFTP, encaminhamentos, configurações), cada um com rótulo ao passar o mouse, o terminal ganha a largura liberada sem reconectar, e o estado é lembrado entre execuções.
7. **Given** o app aberto com sessões, **When** o usuário abre as configurações, **Then** elas ocupam a área principal como uma página com navegação por seções (Geral, Aparência, Terminal, Cofre e segurança, Atalhos, Sobre), as sessões continuam ativas e voltar ao terminal (botão, Esc ou clique numa aba) restaura a aba anterior.
8. **Given** botões, menus, campos e abas, **When** o usuário passa o mouse, foca pelo teclado, pressiona ou seleciona um item, **Then** cada estado (repouso, hover, foco, pressionado, selecionado, desabilitado) tem aparência distinta em vidro, com indicador de seleção que desliza entre itens.
9. **Given** menus, diálogos, abas, página de configurações e o recolhimento da barra lateral, **When** abrem, fecham ou mudam, **Then** a transição é animada em até 250 ms; com "reduzir movimento" ligado no Windows, as mudanças são instantâneas.
10. **Given** a página de Aparência, **When** o usuário escolhe uma cor de destaque (entre as da marca ou "seguir o Windows"), **Then** botões primários, seleção, foco e indicadores mudam em todas as telas de imediato, mantendo o contraste mínimo.
11. **Given** uma sessão ativa, **When** o usuário olha o rodapé, **Then** a barra de status mostra o host/shell da aba ativa, estado da conexão, latência (SSH), encaminhamentos ativos e estado do cofre, e cada item leva à ação correspondente ao ser clicado.

---

### User Story 6 - Arquivos e túneis com a mesma confiabilidade (Priority: P3)

O operador continua usando o navegador SFTP e as regras de encaminhamento local exatamente como
na versão anterior, agora na base nova.

**Why this priority**: são recursos usados com menos frequência que o terminal, mas fazem parte
da paridade obrigatória.

**Independent Test**: executar os cenários de aceitação das histórias 5 e 6 da spec 001 (SFTP e
port forwarding) na versão nova, com o mesmo servidor de teste, e obter os mesmos resultados.

**Acceptance Scenarios**:

1. **Given** os cenários de aceitação de SFTP da spec 001 (navegar, enviar vários arquivos, baixar, criar pasta, renomear, excluir, conflito, progresso e cancelamento, arquivos parciais), **When** executados na versão nova, **Then** todos passam.
2. **Given** os cenários de aceitação de encaminhamento local da spec 001 (regras por host, loopback como padrão, aviso fora do loopback, porta em uso, indicador com cofre bloqueado), **When** executados na versão nova, **Then** todos passam.
3. **Given** um arquivo de 1 GB, **When** enviado e baixado, **Then** o conteúdo chega idêntico e a velocidade é pelo menos igual à da versão anterior no mesmo ambiente.

---

### Edge Cases

- **Cofre da versão anterior com versão de formato desconhecida ou corrompido**: o app não altera o arquivo original, mostra a tela de cofre ilegível e oferece apagar e recomeçar, como na 001.
- **Migração interrompida** (queda de energia durante a conversão): o cofre original permanece intacto e a migração recomeça do zero na próxima abertura.
- **Servidor que envia dados muito rápido** (ex.: `cat` de arquivo grande, `show running-config` longo): a interface continua responsiva e nenhum dado é perdido ou duplicado.
- **Fechar a aba no meio da autenticação** ou com diálogo de host key aberto: a tentativa é cancelada sem deixar conexão pendurada.
- **Redimensionar para tamanho mínimo** ou minimizar: nenhum tamanho zero ou negativo é enviado ao servidor.
- **Troca de monitor com escala diferente** com a janela aberta: o layout se ajusta sem área vazia.
- **Algoritmos legados ligados, mas o servidor aceita modernos**: o app prefere os algoritmos modernos e só usa os legados quando o servidor não oferece outra opção.
- **Agente SSH indisponível** quando o host está configurado para usá-lo: mensagem clara e opção de usar outra credencial.
- **Esquema de cores importado com cores ausentes**: as cores ausentes recebem o valor do preset padrão, e o app informa quais foram completadas.
- **Fonte configurada não está mais instalada**: o terminal usa a fonte embutida padrão e avisa nas configurações.
- **Colar texto com uma única linha terminada em quebra de linha**: tratado como uma linha, sem confirmação; colar com caracteres de controle exibe-os escapados na confirmação.
- **Atalho do app que coincide com uma tecla usada pelo programa remoto** (ex.: Ctrl+W no editor do servidor): os atalhos do app usam apenas as combinações listadas; as demais chegam ao servidor sem interceptação (ver Assumptions).

## Requirements *(mandatory)*

### Functional Requirements

**Paridade e migração**

- **FR-101**: O sistema MUST atender a todos os requisitos FR-001 a FR-034 de `specs/001-mvp-ssh-client/spec.md`, exceto onde um requisito desta spec os substitui explicitamente (FR-032 é substituído por FR-150).
- **FR-102**: O sistema MUST abrir um cofre criado pela versão anterior, mediante a senha mestra, e convertê-lo para o formato novo preservando hosts, grupos, credenciais, host keys conhecidas, regras de encaminhamento e configurações.
- **FR-103**: A conversão do cofre MUST preservar o arquivo original intacto até que o cofre novo esteja gravado e verificado; o formato novo MUST ser versionado.
- **FR-104**: Os segredos MUST continuar cifrados com chave derivada da senha mestra; nenhum segredo pode ficar legível no armazenamento, em arquivos temporários ou na memória após o bloqueio (como em FR-002, FR-005 e SC-004 da 001). O cofre de credenciais do sistema operacional MUST NOT ser usado.

**Processo e sessões**

- **FR-110**: O sistema MUST rodar em instância única; uma segunda execução traz a janela existente para frente e encerra.
- **FR-111**: Nenhuma operação do app MUST iniciar uma nova cópia do próprio executável nem abrir janelas além da janela principal e dos diálogos do sistema (seleção de arquivo).
- **FR-112**: Cada sessão MUST ter uma única assinatura de dados de saída durante toda a sua vida, removida ao fechar; o mesmo dado nunca é exibido duas vezes.
- **FR-113**: O tamanho inicial do terminal MUST ser enviado ao servidor uma única vez, depois que a área do terminal tiver dimensões válidas (maiores que zero); mudanças seguintes MUST ser agrupadas para que um redimensionamento contínuo resulte em poucas notificações ao servidor.
- **FR-114**: Fechar uma aba MUST encerrar a conexão, o canal e toda atividade associada à sessão; o app MUST NOT acumular atividades de sessões encerradas.
- **FR-115**: A saída das sessões MUST ser entregue à interface em lotes para manter a interface responsiva sob saída intensa, sem perder nem reordenar dados.

**Conexão SSH (novos)**

- **FR-120**: O sistema MUST enviar sinais periódicos de atividade (a cada 30 segundos por padrão) em toda conexão SSH e marcar a aba como desconectada quando o servidor deixar de responder.
- **FR-121**: Cada host MUST ter a opção "algoritmos legados", desligada por padrão, que permite negociar algoritmos antigos (troca de chaves `diffie-hellman-group14-sha1` e `diffie-hellman-group1-sha1`, chave de servidor e assinatura `ssh-rsa` (SHA-1) e cifras `aes128-cbc` e `3des-cbc`) além dos modernos, que continuam preferidos.
- **FR-122**: Quando uma conexão falhar por falta de algoritmo em comum, a mensagem MUST indicar isso e sugerir ligar "algoritmos legados" no host.
- **FR-123**: Hosts com "algoritmos legados" ligados MUST exibir indicação visível na lista e na edição.
- **FR-124**: O sistema MUST permitir autenticação via agente SSH do sistema (agente do OpenSSH do Windows ou compatível), como método de autenticação do host.
- **FR-125**: A verificação de host key MUST exibir o fingerprint SHA-256 na primeira conexão e bloquear conexões cuja chave mudou, como em FR-013 da 001.

**Janela, layout e menus**

- **FR-130**: A interface MUST ocupar sempre toda a área da janela, em qualquer tamanho, ao maximizar, restaurar, encaixar na lateral e em escalas de 100%, 125% e 150%.
- **FR-131**: A área do terminal MUST se reajustar ao tamanho disponível depois de cada mudança de tamanho, sem exibir áreas vazias, conteúdo cortado ou terminal em branco.
- **FR-132**: Menus (nova aba, contexto de host, aba e arquivo) MUST abrir ancorados ao elemento que os originou, MUST ficar inteiros dentro da janela, reposicionando-se perto das bordas, e MUST aparecer acima do terminal.
- **FR-133**: A janela MUST ter barra de título própria com área de arrasto e botões de minimizar, maximizar/restaurar e fechar desenhados pelo app, com o mesmo comportamento dos do sistema; o encaixe do Windows por arrasto até a borda, por duplo clique na barra e por Win+setas MUST continuar funcionando. O menu de layouts de encaixe ao passar o mouse sobre o botão maximizar não é exigido (ver Assumptions).

**Teclado, cópia e busca**

- **FR-140**: O sistema MUST oferecer os atalhos: Ctrl+T (nova aba de shell local padrão), Ctrl+W (fechar aba, com confirmação se houver conexão ativa), Ctrl+Tab e Ctrl+Shift+Tab (próxima e anterior aba), Ctrl+Shift+F (buscar no terminal), Ctrl+K (paleta de comandos), Ctrl+= e Ctrl+- (aumentar e diminuir fonte) e Ctrl+0 (restaurar fonte), além do Ctrl+Shift+T da 001.
- **FR-141**: A paleta de comandos MUST permitir buscar e executar: conectar em host, abrir SFTP, abrir shell local, abrir configurações, bloquear o cofre, alternar tema e abrir a busca no terminal.
- **FR-142**: A busca no terminal MUST encontrar texto no histórico, destacar as ocorrências, navegar entre elas e indicar a contagem, com opções de diferenciar maiúsculas e de expressão regular.
- **FR-143**: Selecionar texto no terminal MUST copiá-lo para a área de transferência.
- **FR-144**: Ctrl+Shift+V e o clique com o botão direito MUST colar; colar mais de uma linha MUST pedir confirmação mostrando o número de linhas e o início do conteúdo.
- **FR-145**: URLs na saída do terminal MUST ser acionáveis com Ctrl+clique, abrindo no navegador padrão do sistema após o usuário acionar; o app não faz a requisição.

**Personalização do terminal**

- **FR-150**: O fundo do terminal e do SFTP MUST ser opaco por padrão (100%); o usuário pode reduzir a opacidade do fundo do terminal, com aviso de impacto em legibilidade e desempenho. (Substitui FR-032 da 001.)
- **FR-151**: A tela de personalização MUST ter um terminal de amostra que reflete cada mudança imediatamente.
- **FR-152**: O usuário MUST poder configurar: família da fonte (fontes monoespaçadas instaladas e fontes embutidas no app), tamanho, peso normal e negrito, altura de linha, espaçamento entre letras e ligaduras; estilo do cursor (bloco, barra, sublinhado), piscar e cor; linhas de histórico (100 a 100.000); opacidade do fundo; intensidade do desfoque; espaçamento interno.
- **FR-153**: O usuário MUST poder editar um esquema de cores completo (texto, fundo, cursor, seleção e as 16 cores ANSI) com seletor de cor.
- **FR-154**: O sistema MUST incluir os presets embutidos Dracula, Nord, Solarized Dark, Solarized Light, One Dark, Tokyo Night e Catppuccin, além do esquema padrão da marca.
- **FR-155**: O sistema MUST importar esquemas de cores no formato JSON do Windows Terminal e no formato `.itermcolors` do iTerm2, e exportar esquemas em JSON reimportável.
- **FR-156**: Configurações de terminal MUST existir como perfil global padrão e como perfil opcional por host, que sobrescreve o global apenas nas sessões desse host.
- **FR-157**: Mudanças de perfil MUST ser aplicadas às abas abertas sem reconectar e sem perder o conteúdo exibido.
- **FR-158a**: As configurações MUST ser feitas por seleção, sem digitar valores livres: esquemas de cores numa galeria em que cada opção mostra uma amostra de terminal com suas cores; fontes numa lista em que cada opção é desenhada na própria fonte; tamanho, peso, altura de linha, espaçamento, histórico, opacidade, desfoque e espaçamento interno por opções predefinidas ou controles deslizantes com o valor visível; cursor por opções ilustradas. Texto livre só onde não há lista possível (nomes de esquemas, hosts).
- **FR-158**: Perfis de terminal e esquemas de cores MUST ser persistidos localmente e sobreviver a reinícios.

**Identidade visual**

- **FR-160**: A interface MUST seguir o guia de marca gerado nesta feature (logo, paleta, tipografia, raios, sombras, ícones e movimento), com tema escuro e claro.
- **FR-161**: Todas as cores, fontes, raios, sombras e espaçamentos da interface MUST vir do conjunto de tokens da marca; valores avulsos na interface não são permitidos.
- **FR-162**: Barra lateral, abas, diálogos e menus MUST usar superfícies translúcidas com desfoque quando o sistema suportar, e alternativa opaca equivalente quando não suportar, quando a transparência estiver desligada no sistema, em alto contraste ou quando o usuário desligar o vidro nas configurações (mantém FR-031, FR-033 e FR-034 da 001).
- **FR-163**: As abas MUST mostrar o estado da sessão (conectando, conectado, desconectado, erro) por cor e ícone.
- **FR-164**: Grupos da barra lateral MUST ser recolhíveis, com o estado lembrado entre execuções.
- **FR-165**: O app MUST ter ícone próprio (barra de tarefas, atalho, instalador e janela) derivado do logo da marca.
- **FR-166**: Botões, menus, campos, abas e itens de lista MUST usar superfícies de vidro com estados visuais distintos para repouso, hover, foco visível pelo teclado, pressionado, selecionado e desabilitado; a seleção em listas, abas e navegação MUST ter um indicador que se move até o item escolhido.
- **FR-167**: A barra lateral MUST poder ser minimizada para um trilho de ícones e expandida de novo (botão na própria barra e Ctrl+B), com rótulo ao passar o mouse em cada ícone e o estado lembrado entre execuções; minimizar ou expandir MUST reajustar o terminal sem reconectar.
- **FR-168**: As configurações MUST ser uma página na área principal, com navegação por seções (Geral, Aparência, Terminal, Cofre e segurança, Atalhos, Sobre), substituindo o diálogo atual; sessões continuam ativas enquanto a página está aberta.
- **FR-169**: Toda a interface MUST usar um único conjunto de ícones em traço, com tamanhos padronizados pelos tokens (16, 20 e 24 px); ícones avulsos de outras origens não são permitidos.
- **FR-170**: Menus, diálogos, abas, troca de seção, toasts e o recolhimento da barra lateral MUST ter transições de no máximo 250 ms usando as curvas de movimento da marca; com a preferência "reduzir movimento" do sistema, MUST ficar sem animação.
- **FR-171**: O usuário MUST poder escolher a cor de destaque da interface entre as opções da marca ou seguir a cor de destaque do Windows; a troca vale de imediato em todas as telas, e opções que não atinjam o contraste mínimo no tema atual MUST ser ajustadas automaticamente.
- **FR-172**: A janela MUST ter uma barra de status no rodapé com host ou shell da aba ativa, estado da conexão, latência das sessões SSH, número de encaminhamentos ativos e estado do cofre; cada item abre a ação correspondente (reconectar, lista de encaminhamentos, bloquear/desbloquear).
- **FR-173**: Os atalhos do app MUST usar Ctrl+Shift por padrão, para não roubar teclas do shell remoto (Ctrl+W apaga palavra e Ctrl+K apaga até o fim da linha no readline, Ctrl+T transpõe caracteres, Ctrl+B é o prefixo do tmux): Ctrl+Shift+T (nova aba do shell padrão), Ctrl+Shift+W (fechar aba), Ctrl+Shift+P (paleta), Ctrl+Shift+B (barra lateral), Ctrl+Shift+F (buscar), Ctrl+Tab e Ctrl+Shift+Tab (trocar de aba), Ctrl+= / Ctrl+- / Ctrl+0 (tamanho da fonte). Na seção Atalhos, cada ação MUST poder ser remapeada gravando a combinação ou marcada como "Desativado"; combinação já usada por outra ação é recusada com o nome dela; Ctrl+letra sem Shift e combinações com AltGr só podem ser escolhidas com aviso de que deixam de chegar ao terminal. Substitui as teclas de FR-140, FR-167 e da suposição "Atalhos Ctrl+W e Ctrl+T".
- **FR-174**: Toda mensagem de erro MUST dizer a causa em linguagem simples (pelo código), mostrar o código e oferecer "Copiar detalhes" (código, mensagem técnica, operação, data/hora e versão do app, nunca segredos); "Ocorreu um erro inesperado" só aparece quando não houver código conhecido, e ainda assim com os detalhes copiáveis.
- **FR-175**: As configurações MUST ter a seção "Conexão (SSH)" com: intervalo de keepalive, tempo limite de conexão, algoritmos legados como padrão para hosts novos e o `$TERM` enviado ao servidor (padrão `xterm-256color`; opções `xterm-256color`, `xterm`, `vt100`, `linux`); esses itens saem de "Cofre e segurança".
- **FR-176**: A lista de shells locais (nova aba e shell padrão) MUST detectar Windows PowerShell, PowerShell 7 (`pwsh`), Prompt de Comando, Git Bash e cada distribuição WSL instalada, mostrando só os presentes na máquina.
- **FR-177**: As opções do terminal MUST incluir, por seleção: copiar ao selecionar, botão direito cola, sino (desligado, som ou visual), negrito em cores claras e reconexão automática como opção global.
- **FR-178**: O avatar do host MUST mostrar o logo do sistema do servidor quando ele for conhecido. O sistema é detectado na primeira conexão bem-sucedida e atualizado nas seguintes: pela versão SSH anunciada pelo servidor (ex.: `OpenSSH_9.6p1 Ubuntu`, `ROSSSH` do MikroTik, `Cisco-1.25`, `HUAWEI`, `Comware`) e, só em servidores OpenSSH sem distribuição na versão, por uma leitura de `/etc/os-release` num canal à parte, com tempo limite curto, sem mexer na aba do usuário e sem nada no histórico do shell. Equipamentos de rede nunca recebem comando. No formulário do host, o usuário MUST poder trocar o ícone por outro da lista ou voltar a "Automático"; sem sistema conhecido, o avatar continua com as iniciais. Os logos são embutidos no app (funciona offline) e servem só para identificar o sistema.

### Key Entities

- **Cofre**: contêiner protegido pela senha mestra; agora com versão de formato nova e registro de migração da versão anterior. Contém hosts, grupos, credenciais, host keys conhecidas, regras de encaminhamento e perfis por host.
- **Host**: como na 001, mais: opção "algoritmos legados", método de autenticação "agente SSH", referência opcional a um perfil de terminal próprio, sistema detectado e ícone escolhido pelo usuário (FR-178).
- **Perfil de terminal**: conjunto de opções de fonte, cursor, histórico, opacidade, desfoque, espaçamento e esquema de cores; existe um perfil global e zero ou mais perfis associados a hosts.
- **Esquema de cores**: nome, origem (embutido, importado, personalizado), texto, fundo, cursor, seleção e 16 cores ANSI.
- **Sessão**: aba ativa (SSH, local ou SFTP), com estado (conectando, conectado, desconectado, erro, encerrado), tamanho do terminal e host de origem.
- **Guia de marca**: logo, paleta, tipografia, tokens e regras de uso; fonte única dos valores visuais da interface.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-101**: 100% dos cenários de aceitação da spec 001 passam na versão nova (paridade).
- **SC-102**: Ao conectar em uma OLT Datacom DmOS, em um roteador Huawei, em um Mikrotik e em um servidor Linux, o banner e o prompt aparecem exatamente uma vez em 100% de 20 conexões a cada um.
- **SC-103**: Em 20 ciclos seguidos de maximizar, restaurar e redimensionar, nas escalas de 100%, 125% e 150%, nenhum ciclo apresenta área vazia, corte da barra lateral ou terminal em branco.
- **SC-104**: O menu de nova aba aparece ancorado ao botão "+" e inteiro dentro da janela em 100% das aberturas, inclusive com a janela no tamanho mínimo.
- **SC-105**: Após 100 aberturas e fechamentos de abas, a quantidade de atividades internas e a memória do app voltam a, no máximo, 5% acima do patamar inicial.
- **SC-106**: Mudanças de fonte, tamanho e esquema de cores aparecem em todas as abas abertas em menos de 1 segundo, sem reconexão.
- **SC-107**: Um esquema exportado do Windows Terminal e um `.itermcolors` são importados com todas as cores correspondentes ao arquivo de origem.
- **SC-108**: Uma varredura do diretório de dados após um fluxo completo encontra 0 ocorrências de senhas, passphrases ou trechos de chaves privadas em texto legível (equivalente a SC-004 da 001).
- **SC-109**: O instalador do app para Windows tem menos de 30 MB, e o app ocioso com uma aba aberta usa menos de 150 MB de memória.
- **SC-110**: Uma sessão ociosa atrás de um equipamento que derruba conexões ociosas após 5 minutos permanece conectada por pelo menos 30 minutos.
- **SC-111**: Hosts que só aceitam algoritmos legados conectam em 100% das tentativas com a opção ligada.
- **SC-112**: Com 20 sessões abertas e uma delas despejando saída contínua, a troca de aba continua abaixo de 100 ms (mantém SC-007 da 001).
- **SC-113**: 100% das telas e estados revisados usam apenas tokens da marca e mantêm contraste mínimo de 4,5:1 nos temas escuro e claro, com e sem vidro.
- **SC-114**: Um usuário da versão anterior abre a versão nova e encontra 100% dos seus hosts, grupos, credenciais e regras sem recadastrar nada.
- **SC-115**: Em revisão das capturas, 100% dos componentes interativos mostram os seis estados de FR-166, e nenhuma transição passa de 250 ms nem ocorre com "reduzir movimento" ligado.

## Assumptions

- **Base técnica decidida pelo usuário** (insumo do plano, não requisito de negócio): reescrita
  com backend em Go e janela via Wails v2 (v2.17, a estável; o v3 ainda está em beta em
  outubro de 2026), interface reaproveitando o React e o xterm.js da 001, armazenamento local
  em SQLite sem CGO e criptografia Argon2id + XChaCha20-Poly1305. O plano MUST revisar a
  constituição, cujos exemplos de bibliotecas citam o ecossistema Node.
- **Identidade visual**: o logo, a paleta, a tipografia e os tokens vêm da execução do
  skill `brand-kit` (estágios 0 a 5) dentro desta feature; o briefing já foi levantado
  (terminal + cofre, clima moderno vibrante, pt-BR).
- **Nome visível**: o app aparece como **OpenHost** (janela, instalador, textos); "ShellVault"
  continua só como nome interno do repositório e do executável.
- **Atalhos**: o app usa Ctrl+Shift por padrão (FR-173); Ctrl+letra sem Shift chega ao servidor.
  O menu da aba "Enviar combinação de teclas" continua para casos raros.
- **Limites da base técnica** (levantados no planejamento): a cifra `aes256-cbc` não é suportada
  pela biblioteca SSH escolhida e fica fora da opção "algoritmos legados"; a janela sem moldura
  da versão estável do Wails não oferece o menu de layouts de encaixe ao passar o mouse sobre o
  maximizar (o encaixe por arrasto, duplo clique e Win+setas continua).
- **Shell local padrão** do Ctrl+T é o PowerShell; configurável nas preferências.
- **Fontes embutidas**: ao menos duas fontes monoespaçadas com licença SIL OFL (uma delas com
  ligaduras), escolhidas no plano.
- **Detecção de queda**: a aba vai para "desconectado" após 3 sinais de atividade sem resposta
  (90 s com o padrão de 30 s).
- **Plataforma**: Windows 10/11 é o alvo de entrega desta feature; o código não deve impedir
  Linux e macOS depois, mas builds e testes dessas plataformas ficam fora.
- **Fora desta feature** (Fase 2 e 3): encaminhamento remoto, divisão de painéis, snippets por
  fabricante, log de sessão em arquivo, ProxyJump, importação de `~/.ssh/config` e
  atualização automática.
- **Ambiente de teste** de equipamentos reais (OLT DmOS, Huawei, Mikrotik) é fornecido pelo
  usuário para SC-102; os demais critérios são verificados com servidores SSH em contêiner,
  incluindo um configurado só com algoritmos legados.
