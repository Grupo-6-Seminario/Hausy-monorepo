import type { AgentResponse } from './types';

type TurnEvent =
  | ({ type: 'results' | 'done' } & AgentResponse)
  | { type: 'reply'; delta: string }
  | { type: 'error'; error: string };

export interface TurnHandlers {
  // The ranking is ready; the reply is still being written.
  onResults(partial: AgentResponse): void;
  onReply(delta: string): void;
}

const unavailable = 'El agente local no pudo responder.';

// readTurn reads the backend's answer to one message. A streamed answer
// (NDJSON) reports the ranking, then the reply as it is written, and ends with
// "done": the whole turn, whose reply replaces the streamed text. A plain JSON
// answer is the whole turn at once.
export async function readTurn(
  response: Response,
  handlers: TurnHandlers,
): Promise<AgentResponse> {
  const type = response.headers.get('Content-Type') ?? '';
  if (!type.includes('application/x-ndjson') || !response.body) {
    return (await response.json()) as AgentResponse;
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffered = '';
  for (;;) {
    const { value, done } = await reader.read();
    if (done) return { error: unavailable };
    buffered += decoder.decode(value, { stream: true });
    const lines = buffered.split('\n');
    buffered = lines.pop() ?? '';
    for (const line of lines) {
      if (!line.trim()) continue;
      const event = JSON.parse(line) as TurnEvent;
      if (event.type === 'results') handlers.onResults(event);
      else if (event.type === 'reply') handlers.onReply(event.delta);
      else if (event.type === 'done') return event;
      else return { error: event.error || unavailable };
    }
  }
}
