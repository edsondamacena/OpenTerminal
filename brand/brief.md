---
meta:
  lang: pt-BR
---
# Briefing — OpenTerminal

- **Produto**: cliente SSH desktop para Windows (Go + Wails + React), local-first, com cofre
  criptografado, SFTP, encaminhamento de portas e terminais locais.
- **Público**: NOC, infraestrutura e redes — quem passa o dia em terminais, inclusive com
  equipamentos legados (OLTs, roteadores).
- **Personalidade**: preciso, calmo, nativo do Windows 11; nada de visual "de jogo".
- **Clima visual**: preto piano — superfícies escuras e lisas, sem matiz. Roxo e azul descartados
  (lembravam o Termius).
- **Nome**: OpenTerminal (OpenConnect foi descartado por ser nome de VPN). Identificadores internos
  seguem `OpenTerminal`.
- **Logo** (estágios 1–2): escolhido pelo usuário em 2026-10-25 — `logo-source.png`: "O" aberto
  branco com o chevron do prompt em laranja `#FD7B17` sobre grafite `#1E2026`. Não houve rodadas
  de conceito no `brand/explore/`: o usuário trouxe a direção pronta. A marca mestra com geometria
  real é gerada por `build_mark.py`; o kit por `make_kit.py kit.json`.
- **Idioma das peças**: pt-BR.
- **Stack**: frontend React + CSS Modules com tokens em `frontend/src/theme/`; Windows
  (WebView2), modos glass-dark, glass-light, solid-dark, solid-light e high-contrast.
