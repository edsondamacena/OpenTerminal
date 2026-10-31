// Gerado por brand/export_app_tokens.py a partir de brand/brand.json. Não edite à mão.

export const BRAND = {
  pretoPiano: '#050505',
  laca: '#0b0b0b',
  superficie: '#171717',
  grafite: '#1e2026',
  nevoa: '#f5f5f5',
  cinza: '#a6a6a6',
  papel: '#f3f3f1',
  branco: '#ffffff',
  tinta: '#111111',
  chumbo: '#4a4a4a',
  laranjaOpenterminal: '#fd7b17',
  laranjaProfundo: '#b04e06',
  perigo: '#ff7b7b',
  perigoProfundo: '#c4322b',
  aviso: '#f0b955',
  avisoProfundo: '#9a6400',
  sucesso: '#4fd18b',
  sucessoProfundo: '#1f7a46',
} as const;

export const BRAND_ACCENTS = {
  laranja: { label: 'Laranja', dark: '#fd7b17', light: '#b04e06' },
  grafite: { label: 'Grafite', dark: '#f2f2f2', light: '#111111' },
  azul: { label: 'Azul', dark: '#4c9aff', light: '#0b5cc4' },
  ciano: { label: 'Ciano', dark: '#2cc6e0', light: '#00748a' },
  verde: { label: 'Verde', dark: '#3ddc84', light: '#13753f' },
  roxo: { label: 'Roxo', dark: '#a78bfa', light: '#6d3fd6' },
  rosa: { label: 'Rosa', dark: '#f472b6', light: '#b4236f' },
  vermelho: { label: 'Vermelho', dark: '#ff6b6b', light: '#c0282b' },
  amarelo: { label: 'Amarelo', dark: '#f5c542', light: '#8a6400' },
} as const;

export type BrandAccentId = keyof typeof BRAND_ACCENTS;
