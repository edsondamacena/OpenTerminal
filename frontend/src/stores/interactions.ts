// Perguntas do processo principal aguardando resposta (host key, prompts do servidor).
import { create } from 'zustand';
import type { HostKeyEvent, PromptEvent } from '@shared/model/types';

interface InteractionsState {
  hostKeys: HostKeyEvent[];
  prompts: PromptEvent[];
  answerHostKey: (sessionId: string, accept: boolean) => void;
  answerPrompt: (sessionId: string, answers: string[] | null) => void;
}

export const useInteractions = create<InteractionsState>((set, get) => ({
  hostKeys: [],
  prompts: [],
  answerHostKey(sessionId, accept) {
    set({ hostKeys: get().hostKeys.filter((h) => h.sessionId !== sessionId) });
    void window.openterminal.sessions.answerHostKey(sessionId, accept);
  },
  answerPrompt(sessionId, answers) {
    set({ prompts: get().prompts.filter((p) => p.sessionId !== sessionId) });
    void window.openterminal.sessions.answerPrompt(sessionId, answers);
  },
}));

let subscribed = false;

export function subscribeInteractions(): void {
  if (subscribed) return;
  subscribed = true;
  const api = window.openterminal.sessions;
  api.onHostKey((e) => {
    const rest = useInteractions.getState().hostKeys.filter((h) => h.sessionId !== e.sessionId);
    useInteractions.setState({ hostKeys: [...rest, e] });
  });
  api.onPrompt((e) => {
    const rest = useInteractions.getState().prompts.filter((p) => p.sessionId !== e.sessionId);
    useInteractions.setState({ prompts: [...rest, e] });
  });
}

/** Aba fechada: descarta perguntas pendentes dela. */
export function dropInteractions(sessionId: string): void {
  const s = useInteractions.getState();
  useInteractions.setState({
    hostKeys: s.hostKeys.filter((h) => h.sessionId !== sessionId),
    prompts: s.prompts.filter((p) => p.sessionId !== sessionId),
  });
}
