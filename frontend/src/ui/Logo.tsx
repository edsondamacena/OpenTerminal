// Marca do OpenTerminal (build/appicon.png): o prompt ">_" azul num quadrado branco. Em tamanhos
// pequenos (barra de título) a variante "mark" dispensa o quadrado para o prompt ocupar o espaço.
export function Logo({
  size = 20,
  variant = 'tile',
}: {
  size?: number;
  variant?: 'tile' | 'mark';
}) {
  const mark = variant === 'mark';
  return (
    <svg
      width={size}
      height={size}
      viewBox={mark ? '4.6 4.4 15.6 15.6' : '0 0 24 24'}
      aria-hidden="true"
      focusable="false"
    >
      {!mark && (
        <rect
          x="1"
          y="1"
          width="22"
          height="22"
          rx="5.6"
          fill="#ffffff"
          stroke="rgba(0, 0, 0, 0.08)"
          strokeWidth="0.4"
        />
      )}
      <path
        d="M6.2 7.3L12 10.9L6.2 14.5"
        fill="none"
        stroke="#008cf8"
        strokeWidth="2.1"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
      <path
        d="M11.9 15.5H18.5"
        fill="none"
        stroke="#008cf8"
        strokeWidth="2.1"
        strokeLinecap="round"
      />
    </svg>
  );
}
