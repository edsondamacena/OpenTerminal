import { describe, expect, it } from 'vitest';
import { indicatorFor, nextPhase } from './motion';

describe('ciclo de saída animada (FR-170)', () => {
  it('abre, anima a saída e só desmonta no fim da animação', () => {
    expect(nextPhase('closed', 'open')).toBe('open');
    expect(nextPhase('open', 'close')).toBe('closing');
    expect(nextPhase('closing', 'animationEnd')).toBe('closed');
  });

  it('reabrir durante a saída volta a aberto', () => {
    expect(nextPhase('closing', 'open')).toBe('open');
  });

  it('eventos fora de hora não mudam o estado', () => {
    expect(nextPhase('closed', 'close')).toBe('closed');
    expect(nextPhase('closed', 'animationEnd')).toBe('closed');
    expect(nextPhase('open', 'animationEnd')).toBe('open');
    expect(nextPhase('open', 'open')).toBe('open');
  });
});

describe('indicador deslizante (FR-166)', () => {
  const container = { left: 100, top: 40, width: 600, height: 300 };

  it('mede o item relativo ao contêiner no eixo pedido', () => {
    const item = { left: 180, top: 70, width: 120, height: 32 };
    expect(indicatorFor(container, item, 'x')).toEqual({ offset: 80, size: 120 });
    expect(indicatorFor(container, item, 'y')).toEqual({ offset: 30, size: 32 });
  });

  it('considera a rolagem do contêiner', () => {
    const item = { left: 100, top: 40, width: 600, height: 32 };
    expect(indicatorFor(container, item, 'y', 64)).toEqual({ offset: 64, size: 32 });
  });
});
