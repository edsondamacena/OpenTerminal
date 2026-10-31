// Camada flutuante ancorada (FR-132): fica inteira na janela, vira de lado perto das bordas e
// sai do fluxo do layout via portal no body; a camada (z-index) vem da classe de quem usa.
import {
  autoUpdate,
  flip,
  offset,
  shift,
  size,
  useFloating,
  type Placement,
  type VirtualElement,
} from '@floating-ui/react-dom';
import { useEffect, useMemo, useRef, type CSSProperties, type ReactNode } from 'react';
import { createPortal } from 'react-dom';

/** Elemento que originou a camada, ou um ponto (menu de contexto). */
export type PopoverAnchor = Element | { x: number; y: number };

const EDGE = 8;

function toReference(anchor: PopoverAnchor): Element | VirtualElement {
  if (anchor instanceof Element) return anchor;
  const { x, y } = anchor;
  return { getBoundingClientRect: () => DOMRect.fromRect({ x, y, width: 0, height: 0 }) };
}

export interface PopoverRenderProps {
  ref: (node: HTMLElement | null) => void;
  style: CSSProperties;
}

export interface PopoverProps {
  anchor: PopoverAnchor;
  placement?: Placement;
  /** Distância do âncora em px (ponto: 0). */
  gap?: number;
  /** Chamado uma vez quando a camada fica visível (antes disso ela não recebe foco). */
  onPositioned?: () => void;
  children: (props: PopoverRenderProps) => ReactNode;
}

export function Popover({
  anchor,
  placement = 'bottom-start',
  gap,
  onPositioned,
  children,
}: PopoverProps) {
  const reference = useMemo(() => toReference(anchor), [anchor]);
  const { refs, floatingStyles, isPositioned } = useFloating({
    strategy: 'fixed',
    placement,
    // Posição por left/top: o `transform` fica livre para a animação de entrada.
    transform: false,
    elements: { reference },
    whileElementsMounted: autoUpdate,
    middleware: [
      offset(gap ?? (anchor instanceof Element ? 4 : 0)),
      flip({ padding: EDGE }),
      shift({ padding: EDGE }),
      size({
        padding: EDGE,
        apply({ availableHeight, elements }) {
          elements.floating.style.maxHeight = `${String(Math.max(0, availableHeight))}px`;
        },
      }),
    ],
  });

  const notified = useRef(false);
  useEffect(() => {
    if (!isPositioned || notified.current) return;
    notified.current = true;
    onPositioned?.();
  }, [isPositioned, onPositioned]);

  return createPortal(
    children({
      ref: refs.setFloating,
      style: {
        ...floatingStyles,
        // Antes do primeiro cálculo a posição é (0,0): esconder evita o salto.
        visibility: isPositioned ? 'visible' : 'hidden',
      },
    }),
    document.body,
  );
}
