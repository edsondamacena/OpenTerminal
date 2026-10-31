"""Gera brand/brand.json (fonte da verdade da marca) e confere os contrastes.

Rodar da raiz: python brand/build_brand.py
"""
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
OUT = ROOT / 'brand' / 'brand.json'


def lum(hex_: str) -> float:
    h = hex_.lstrip('#')
    rgb = [int(h[i:i + 2], 16) / 255 for i in (0, 2, 4)]
    lin = [c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4 for c in rgb]
    return 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2]


def ratio(a: str, b: str) -> float:
    la, lb = sorted((lum(a), lum(b)), reverse=True)
    return (la + 0.05) / (lb + 0.05)


C = {
    # Neutros: preto piano, sem matiz.
    'Preto piano': '#050505',
    'Laca': '#0B0B0B',
    'Grafite': '#1E2026',
    'Superfície': '#171717',
    'Névoa': '#F5F5F5',
    'Cinza': '#A6A6A6',
    'Papel': '#F3F3F1',
    'Branco': '#FFFFFF',
    'Tinta': '#111111',
    'Chumbo': '#4A4A4A',
    # Destaque: o laranja do chevron.
    'Laranja OpenTerminal': '#FD7B17',
    'Laranja profundo': '#B04E06',
    # Retorno.
    'Perigo': '#FF7B7B',
    'Perigo profundo': '#C4322B',
    'Aviso': '#F0B955',
    'Aviso profundo': '#9A6400',
    'Sucesso': '#4FD18B',
    'Sucesso profundo': '#1F7A46',
}

CONTRAST = [
    ('Névoa', 'Laca', 'Texto principal no escuro'),
    ('Cinza', 'Laca', 'Texto secundário no escuro'),
    ('Laranja OpenTerminal', 'Laca', 'Destaque e foco no escuro'),
    ('Laca', 'Laranja OpenTerminal', 'Texto de botão primário no escuro'),
    ('Tinta', 'Papel', 'Texto principal no claro'),
    ('Chumbo', 'Papel', 'Texto secundário no claro'),
    ('Laranja profundo', 'Branco', 'Destaque e foco no claro'),
    ('Branco', 'Laranja profundo', 'Texto de botão primário no claro'),
    ('Perigo', 'Laca', 'Erro no escuro'),
    ('Perigo profundo', 'Branco', 'Erro no claro'),
]

for fg, bg, use in CONTRAST:
    r = ratio(C[fg], C[bg])
    flag = 'ok' if r >= 4.5 else 'FALHA'
    print(f'{r:5.2f}:1 {flag:5} {use}')
    assert r >= 4.5, use


def group(name, items):
    return {'name': name, 'items': [{'name': n, 'hex': C[n], 'token': t, 'role': r} for n, t, r in items]}


brand = {
    'meta': {
        'name': 'OpenTerminal',
        'title': 'OpenTerminal — guia da marca',
        'tagline': 'Seus hosts, um prompt de distância.',
        'system': 'Preto piano',
        'lang': 'pt-BR',
        'updated': '2026-10-25',
        'summary': (
            'OpenTerminal é um cliente SSH para Windows feito para quem passa o dia em terminais: NOC, '
            'infraestrutura e redes. A marca é preto piano — superfícies escuras e lisas, sem matiz — '
            'com um único ponto de cor: o laranja do chevron, que marca onde a ação acontece.'
        ),
        'labels': {
            's_overview': 'Visão geral', 's_logo': 'Logo', 's_color': 'Cores', 's_type': 'Tipografia',
            's_shape': 'Forma e elevação', 's_motion': 'Movimento', 's_voice': 'Voz e texto',
            's_components': 'Componentes',
            'l_logo': 'Uma ideia, poucas formas. Use o arquivo certo para o tamanho e o fundo.',
            'l_color': 'Tokens de cor e como usá-los.', 'l_type': 'Duas famílias, cada uma com uma função.',
            'l_shape': 'Raios intencionais, bordas firmes, sombras nítidas.', 'l_motion': 'Movimento com motivo.',
            'l_voice': 'Como a marca fala.',
            'l_components': 'Blocos de interface prontos, todos feitos dos tokens acima.',
            'logo_concept': 'Conceito', 'logo_lockups': 'Assinaturas', 'logo_marks': 'Símbolo e variações',
            'logo_space': 'Área de proteção e tamanho', 'logo_misuse': 'Usos incorretos',
            'color_shell': 'Fundo e superfície', 'color_contrast': 'Contraste', 'rules': 'Regras',
            'shell_light': 'Fundo claro', 'shell_dark': 'Fundo escuro', 'shell_card': 'Cartão',
            'shell_card_note': 'sempre a cor da superfície', 'verdict_aa': 'AA', 'verdict_large': 'AA grande',
            'verdict_fail': 'Falha', 'type_families': 'Famílias', 'type_scale': 'Escala', 'shape_radius': 'Raio',
            'shape_elevation': 'Elevação', 'shape_blob': 'Forma orgânica',
            'elevation_lead': 'Passe o mouse ou pressione um nível para sentir a interação.', 'never': 'Nunca',
            'motion_principles': 'Princípios', 'motion_tokens': 'Tokens', 'motion_never': 'Nunca anima',
            'motion_easing': 'Curvas', 'motion_duration': 'Duração', 'motion_examples': 'Exemplos',
            'motion_reduced': 'Todo exemplo para quando o sistema pede movimento reduzido (trecho abaixo).',
            'mx_fade-up': 'Surgir', 'mx_fade-up_use': 'Seções e cartões entrando', 'mx_pop': 'Aparecer',
            'mx_pop_use': 'Selos e confirmações', 'mx_slide-in': 'Deslizar',
            'mx_slide-in_use': 'Toasts e painéis laterais', 'mx_stagger': 'Em sequência',
            'mx_stagger_use': 'Listas carregando', 'mx_press': 'Pressionar', 'mx_press_use': 'Botões',
            'mx_exit': 'Sair', 'mx_exit_use': 'Fechar e dispensar', 'mx_shimmer': 'Esqueleto',
            'mx_shimmer_use': 'Conteúdo ainda carregando', 'voice_traits': 'Personalidade',
            'voice_terms': 'Terminologia', 'voice_examples': 'Exemplos', 'voice_microcopy': 'Microtexto',
            'th_concept': 'Conceito', 'th_use': 'Use', 'th_avoid': 'Evite', 'st_default': 'Padrão',
            'st_hover': 'Hover', 'st_press': 'Pressionado', 'st_loading': 'Carregando', 'st_disabled': 'Desativado',
            'c_button': 'Botão', 'c_input': 'Campo', 'c_badge': 'Selo', 'c_card': 'Cartões',
            'c_stat': 'Números e esqueleto', 'c_avatar': 'Avatar', 'c_toast': 'Toast', 'c_empty': 'Estado vazio',
            'c_table': 'Abas e tabela', 'c_dialog': 'Diálogo', 'theme_light': 'Claro', 'theme_dark': 'Escuro',
            'code': 'Código', 'copy': 'Copiar', 'copied': 'Copiado', 'download': 'Baixar', 'do': 'Faça',
            'dont': 'Não faça',
        },
    },
    'logo': {
        'dir': 'kit',
        'idea': (
            'Um "O" aberto — o host — com o chevron do prompt dentro. A abertura à direita é a porta: '
            'o terminal saindo para a rede. O chevron é a única cor da marca.'
        ),
        'primary': {'light': 'openterminal-horizontal.svg', 'dark': 'openterminal-horizontal-reversed.svg'},
        'lockups': [
            {'label': 'Horizontal', 'file': 'openterminal-horizontal.svg', 'bg': 'light', 'use': 'Uso principal em fundo claro'},
            {'label': 'Horizontal invertida', 'file': 'openterminal-horizontal-reversed.svg', 'bg': 'dark', 'use': 'Uso principal em fundo escuro'},
            {'label': 'Empilhada', 'file': 'openterminal-stacked.svg', 'bg': 'light', 'use': 'Espaços estreitos e quadrados'},
            {'label': 'Empilhada invertida', 'file': 'openterminal-stacked-reversed.svg', 'bg': 'dark', 'use': 'Espaços estreitos no escuro'},
        ],
        'marks': [
            {'label': 'Ícone do app', 'file': 'openterminal-app-icon.svg', 'bg': 'white', 'use': 'Barra de tarefas, atalhos e instalador'},
            {'label': 'Símbolo', 'file': 'openterminal-mark.svg', 'bg': 'light', 'use': 'Barra de título e avatares'},
            {'label': 'Símbolo invertido', 'file': 'openterminal-mark-reversed.svg', 'bg': 'dark', 'use': 'Sobre preto piano'},
            {'label': 'Uma cor', 'file': 'openterminal-mark-black.svg', 'bg': 'white', 'use': 'Impressão e carimbos'},
        ],
        'clearspace': 'Em volta do logo, deixe livre a largura do chevron.',
        'clearspace_ratio': 0.25,
        'minsize': [
            {'file': 'openterminal-mark.svg', 'px': 16, 'label': 'Símbolo: 16 px'},
            {'file': 'openterminal-horizontal.svg', 'px': 24, 'label': 'Assinatura: 24 px de altura'},
        ],
        'donts': [
            {'effect': 'stretch', 'label': 'Não distorça'},
            {'effect': 'rotate', 'label': 'Não gire'},
            {'effect': 'recolor', 'label': 'Não troque as cores'},
            {'effect': 'shadow', 'label': 'Não aplique sombra'},
            {'effect': 'outline', 'label': 'Não contorne'},
            {'effect': 'busy', 'label': 'Não use sobre fundo carregado'},
        ],
    },
    'colors': {
        'groups': [
            group('Preto piano', [
                ('Preto piano', '--terminal-background', 'Fundo do terminal no escuro'),
                ('Laca', '--surface-glass', 'Moldura do app no escuro'),
                ('Superfície', '--surface-raised', 'Diálogos, menus e toasts no escuro'),
                ('Grafite', '--brand-tile', 'Quadrado do ícone'),
                ('Névoa', '--text-primary', 'Texto principal no escuro'),
                ('Cinza', '--text-secondary', 'Texto secundário no escuro'),
            ]),
            group('Claro', [
                ('Papel', '--surface-glass', 'Moldura do app no claro'),
                ('Branco', '--surface-raised', 'Diálogos e menus no claro'),
                ('Tinta', '--text-primary', 'Texto principal no claro'),
                ('Chumbo', '--text-secondary', 'Texto secundário no claro'),
            ]),
            group('Destaque', [
                ('Laranja OpenTerminal', '--accent', 'Chevron, foco, seleção e botão primário no escuro'),
                ('Laranja profundo', '--accent', 'Destaque no claro (5,3:1 sobre branco)'),
            ]),
            group('Retorno', [
                ('Perigo', '--danger', 'Erro no escuro'), ('Perigo profundo', '--danger', 'Erro no claro'),
                ('Aviso', '--warning', 'Aviso no escuro'), ('Aviso profundo', '--warning', 'Aviso no claro'),
                ('Sucesso', '--success', 'Conectado no escuro'),
                ('Sucesso profundo', '--success', 'Conectado no claro'),
            ]),
        ],
        'contrast': [{'fg': fg, 'bg': bg, 'use': use} for fg, bg, use in CONTRAST],
        'rules': [
            'Uma cor só: o laranja aparece onde o usuário age (foco, seleção, botão primário) e no chevron.',
            'Nada de roxo ou azul na interface; a paleta ANSI do terminal é do usuário e fica de fora.',
            'Superfícies sem matiz: os neutros são cinzas puros, como laca de piano.',
            'No claro use o Laranja profundo; o Laranja OpenTerminal não passa de 3:1 sobre branco.',
            'Estado nunca depende só de cor: conectado, erro e aviso levam também ícone ou texto.',
        ],
    },
    # Opções do seletor de destaque (T112); 'system' usa a cor do Windows ajustada para 4,5:1.
    'accents': [
        {'id': 'laranja', 'label': 'Laranja', 'dark': C['Laranja OpenTerminal'], 'light': C['Laranja profundo']},
        {'id': 'grafite', 'label': 'Grafite', 'dark': '#F2F2F2', 'light': C['Tinta']},
    ],
    'theme': {
        'light': {'bg': 'Papel', 'fg': 'Tinta', 'muted': 'Chumbo'},
        'dark': {'bg': 'Laca', 'fg': 'Névoa', 'muted': 'Cinza'},
        'card': 'Superfície', 'card_fg': 'Névoa',
        'primary': 'Laranja OpenTerminal', 'primary_fg': 'Laca',
        'secondary': 'Grafite', 'secondary_fg': 'Névoa',
        'destructive': 'Perigo', 'destructive_fg': 'Laca',
        'accent': 'Laranja OpenTerminal', 'success': 'Sucesso', 'pending': 'Aviso',
    },
    'fonts': {
        'css': 'https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;700&display=swap',
        'display': {'family': 'Segoe UI Variable Display', 'fallback': "'Segoe UI', system-ui, sans-serif",
                    'use': 'Títulos. Fonte do sistema do Windows: não é distribuída com o app.',
                    'sample': 'Crie sua senha mestra'},
        'body': {'family': 'Segoe UI Variable Text', 'fallback': "'Segoe UI', system-ui, sans-serif",
                 'use': 'Interface: rótulos, campos, menus.', 'sample': 'noc@10.20.0.1 · Conectado'},
        'mono': {'family': 'JetBrains Mono', 'fallback': "'Cascadia Mono', Consolas, monospace",
                 'use': 'Terminal, logotipo, códigos de erro. SIL OFL, embutida no app.',
                 'sample': 'ssh noc@core-router-01'},
    },
    'type_scale': [
        {'role': 'Título de tela', 'font': 'display', 'size': 22, 'weight': 650, 'sample': 'Crie sua senha mestra', 'note': 'Telas cheias (cofre)'},
        {'role': 'Título de diálogo', 'font': 'display', 'size': 16, 'weight': 600, 'sample': 'Editar host', 'note': 'Cabeçalho de diálogos'},
        {'role': 'Corpo', 'font': 'body', 'size': 13, 'weight': 400, 'sample': 'As alterações são salvas automaticamente.', 'note': 'Texto corrido e descrições'},
        {'role': 'Rótulo', 'font': 'body', 'size': 12, 'weight': 600, 'sample': 'Autenticação', 'note': 'Rótulos de campo e grupos'},
        {'role': 'Terminal', 'font': 'mono', 'size': 14, 'weight': 400, 'sample': 'noc@core-router-01:~$ show ip route', 'note': 'Padrão do terminal; o usuário escolhe tamanho'},
        {'role': 'Código de erro', 'font': 'mono', 'size': 11, 'weight': 400, 'sample': 'SCHEME_INVALID', 'upper': True, 'note': 'Códigos em toasts de erro'},
    ],
    'type_rules': [
        'Interface na fonte do sistema: o app parece nativo do Windows 11.',
        'Monoespaçada só para o que é do terminal: comandos, endereços em contexto técnico e códigos.',
    ],
    'shape': {
        'radius': [
            {'name': 'Controle', 'px': 8, 'use': 'Botões, campos, itens de lista'},
            {'name': 'Cartão', 'px': 10, 'use': 'Avisos, cartões de esquema'},
            {'name': 'Elevado', 'px': 14, 'use': 'Diálogos, menus, toasts, painéis'},
        ],
        'elevation': {
            'style': 'soft',
            'levels': [
                {'name': 'Plano', 'radius': 8, 'border': True, 'offset': 0, 'use': 'Campos e listas'},
                {'name': 'Elevado', 'radius': 14, 'border': False, 'offset': 18, 'use': 'Diálogos, menus e toasts'},
            ],
            'never': ['Contorno claro em volta de toasts e avisos', 'Sombras coloridas'],
        },
    },
    'motion': {
        'durations': {'fast': 120, 'base': 180, 'slow': 240},
        'easings': {'standard': [0.2, 0, 0, 1], 'enter': [0.2, 0.8, 0.2, 1], 'exit': [0.4, 0, 1, 1]},
        'examples': [
            {'preset': 'slide-in', 'duration': 'base', 'easing': 'enter'},
            {'preset': 'pop', 'duration': 'fast', 'easing': 'enter'},
            {'preset': 'press', 'duration': 'fast', 'easing': 'standard'},
            {'preset': 'exit', 'duration': 'fast', 'easing': 'exit'},
        ],
        'principles': [
            {'title': 'Rápido', 'body': 'Quem está num incidente não espera animação: 120 a 240 ms.'},
            {'title': 'Com origem', 'body': 'Movimento mostra de onde algo veio: o menu sai do botão, o toast do canto.'},
        ],
        'never': ['O terminal e o texto que chega nele', 'Redimensionar painéis'],
    },
    'voice': {
        'traits': [
            {'trait': 'Direta', 'body': 'Diz o que aconteceu e o que fazer, sem rodeio.'},
            {'trait': 'Técnica', 'body': 'Usa os termos do NOC (host, porta, algoritmo) sem jargão de marketing.'},
            {'trait': 'Calma', 'body': 'Erro é informação: causa, código e como resolver.'},
        ],
        'terms': [
            {'concept': 'Cofre', 'use': 'cofre, senha mestra', 'avoid': 'banco de dados, keychain'},
            {'concept': 'Equipamento', 'use': 'host', 'avoid': 'máquina, nó'},
            {'concept': 'Algoritmos antigos', 'use': 'algoritmos legados', 'avoid': 'modo compatível'},
        ],
        'pairs': [
            {'do': 'A conexão caiu. Reconectando em 4 s.', 'dont': 'Ops! Algo deu errado :('},
            {'do': 'O arquivo de esquema de cores não é válido.', 'dont': 'Erro 0x80004005'},
        ],
        'microcopy': [
            {'type': 'Estado', 'text': 'Conectado'},
            {'type': 'Reconexão', 'text': 'Reconectando em 8 s (tentativa 3 de 6)'},
            {'type': 'Cofre', 'text': 'Cofre desbloqueado'},
            {'type': 'Erro', 'text': 'Copiar detalhes'},
        ],
    },
}

OUT.write_text(json.dumps(brand, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
print(f'brand.json em {OUT}')
