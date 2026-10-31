/** Faixa de cor da latência na barra de status: até 150 ms boa, até 400 ms lenta, acima ruim. */
export function latencyLevel(ms: number): 'good' | 'slow' | 'bad' {
  if (ms <= 150) return 'good';
  if (ms <= 400) return 'slow';
  return 'bad';
}
