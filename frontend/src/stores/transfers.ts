import { create } from 'zustand';
import type { ConflictChoice, ConflictEvent, Transfer } from '@shared/model/types';
import { toastError } from '../ui/Toast';

export interface TransferView extends Transfer {
  /** Bytes por segundo, estimado entre eventos de progresso. */
  speed: number;
  updatedAt: number;
}

interface TransfersState {
  /** Mais recentes primeiro. */
  transfers: TransferView[];
  conflicts: ConflictEvent[];
  answerConflict: (transferId: string, choice: ConflictChoice) => void;
  cancel: (transferId: string) => void;
  clearFinished: (sessionId: string) => void;
}

const FINAL = new Set<Transfer['state']>(['completed', 'cancelled', 'failed', 'skipped']);

export const isFinished = (t: Transfer): boolean => FINAL.has(t.state);

export const useTransfers = create<TransfersState>((set, get) => ({
  transfers: [],
  conflicts: [],
  answerConflict(transferId, choice) {
    set({ conflicts: get().conflicts.filter((c) => c.transferId !== transferId) });
    void window.openterminal.sftp.resolveConflict(transferId, choice).then((res) => {
      if (!res.ok) toastError(res.error);
    });
  },
  cancel(transferId) {
    void window.openterminal.sftp.cancel(transferId).then((res) => {
      if (!res.ok) toastError(res.error);
    });
  },
  clearFinished(sessionId) {
    set({
      transfers: get().transfers.filter((t) => t.sessionId !== sessionId || !isFinished(t)),
    });
  },
}));

function upsert(t: Transfer): void {
  const now = performance.now();
  const { transfers, conflicts } = useTransfers.getState();
  const previous = transfers.find((x) => x.id === t.id);
  let speed = previous?.speed ?? 0;
  if (previous && t.state === 'running' && now > previous.updatedAt) {
    const instant = ((t.bytesDone - previous.bytesDone) * 1000) / (now - previous.updatedAt);
    // Média móvel para a velocidade não oscilar a cada evento.
    speed = previous.speed ? previous.speed * 0.7 + instant * 0.3 : instant;
  }
  const view: TransferView = { ...t, speed: isFinished(t) ? 0 : speed, updatedAt: now };
  useTransfers.setState({
    transfers: previous ? transfers.map((x) => (x.id === t.id ? view : x)) : [view, ...transfers],
    // Conflito resolvido em outro lugar (ex.: "cancelar o restante") sai da fila.
    conflicts: isFinished(t) ? conflicts.filter((c) => c.transferId !== t.id) : conflicts,
  });
}

let subscribed = false;

export function subscribeTransfers(): void {
  if (subscribed) return;
  subscribed = true;
  window.openterminal.sftp.onTransfer(upsert);
  window.openterminal.sftp.onConflict((c) => {
    useTransfers.setState({ conflicts: [...useTransfers.getState().conflicts, c] });
  });
}
