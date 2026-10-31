"""Exporta brand/brand.json para o app (T081):

- frontend/src/theme/brand.gen.ts  paleta e opções de destaque, lidas por tokens.ts e accent.ts
- frontend/src/theme/tokens.css    variáveis --brand-* (marca no Logo e em peças fixas)
- brand/tokens/                    formatos genéricos do brand-kit (DTCG e CSS), para outras ferramentas

Rodar da raiz depois de build_brand.py: python brand/export_app_tokens.py
"""
import json
import re
import subprocess
import sys
import unicodedata
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
BRAND = json.loads((ROOT / 'brand' / 'brand.json').read_text(encoding='utf-8'))
THEME = ROOT / 'frontend' / 'src' / 'theme'
HEADER = 'Gerado por brand/export_app_tokens.py a partir de brand/brand.json. Não edite à mão.'


def key(name: str) -> str:
    """"Laranja OpenTerminal" -> "laranjaOpenterminal"."""
    plain = unicodedata.normalize('NFKD', name).encode('ascii', 'ignore').decode()
    words = re.findall(r'[A-Za-z0-9]+', plain)
    return words[0].lower() + ''.join(w.capitalize() for w in words[1:])


def kebab(name: str) -> str:
    plain = unicodedata.normalize('NFKD', name).encode('ascii', 'ignore').decode()
    return '-'.join(re.findall(r'[a-z0-9]+', plain.lower()))


def main() -> None:
    colors = {item['name']: item['hex'].lower() for g in BRAND['colors']['groups'] for item in g['items']}
    accents = BRAND['accents']

    lines = [f'// {HEADER}', '', 'export const BRAND = {']
    lines += [f"  {key(n)}: '{h}'," for n, h in colors.items()]
    lines += ['} as const;', '', 'export const BRAND_ACCENTS = {']
    for a in accents:
        lines.append(f"  {a['id']}: {{ label: '{a['label']}', dark: '{a['dark'].lower()}', light: '{a['light'].lower()}' }},")
    lines += ['} as const;', '', 'export type BrandAccentId = keyof typeof BRAND_ACCENTS;', '']
    (THEME / 'brand.gen.ts').write_text('\n'.join(lines), encoding='utf-8')

    css = [f'/* {HEADER} */', ':root {']
    css += [f'  --brand-{kebab(n)}: {h};' for n, h in colors.items()]
    css += [
        '  /* Logo: quadrado grafite, anel branco e chevron laranja. */',
        f"  --brand-tile: {colors['Grafite']};",
        f"  --brand-ring: {colors['Branco']};",
        f"  --brand-chevron: {colors['Laranja OpenTerminal']};",
        '}',
        '',
    ]
    (THEME / 'tokens.css').write_text('\n'.join(css), encoding='utf-8')

    # Mesmo formato do resto do frontend, para regenerar sem diff de estilo.
    subprocess.run('npx prettier --write src/theme/brand.gen.ts src/theme/tokens.css', shell=True,
                   cwd=ROOT / 'frontend', check=True, stdout=subprocess.DEVNULL)

    script = ROOT / '.claude' / 'skills' / 'brand-kit' / 'scripts' / 'export_tokens.py'
    if script.exists():
        subprocess.run([sys.executable, '-I', str(script), str(ROOT / 'brand' / 'brand.json'),
                        '-o', str(ROOT / 'brand' / 'tokens'), '--only', 'dtcg', 'css'], check=True)
    print(f'tokens do app em {THEME}')


if __name__ == '__main__':
    main()
