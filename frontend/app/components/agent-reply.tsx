import { Fragment, type ReactNode } from 'react';

type ReplyBlock =
  | { type: 'heading'; text: string }
  | { type: 'paragraph'; text: string }
  | { type: 'list'; ordered: boolean; items: string[] };

function appendListItem(blocks: ReplyBlock[], ordered: boolean, text: string) {
  const previous = blocks.at(-1);
  if (previous?.type === 'list' && previous.ordered === ordered) {
    previous.items.push(text);
    return;
  }
  blocks.push({ type: 'list', ordered, items: [text] });
}

function parseReply(reply: string): ReplyBlock[] {
  const blocks: ReplyBlock[] = [];

  for (const rawLine of reply.split('\n')) {
    const line = rawLine.trim();
    if (!line) continue;

    const heading = line.match(/^#{1,3}\s+(.+)$/);
    if (heading) {
      blocks.push({ type: 'heading', text: heading[1] });
      continue;
    }

    const numbered = line.match(/^\d+[.)]\s+(.+)$/);
    if (numbered) {
      appendListItem(blocks, true, numbered[1]);
      continue;
    }

    const bullet = line.match(/^[-*]\s+(.+)$/);
    if (bullet) {
      appendListItem(blocks, false, bullet[1]);
      continue;
    }

    blocks.push({ type: 'paragraph', text: line });
  }

  return blocks;
}

function renderInline(text: string): ReactNode[] {
  return text.split(/(\*\*[^*]+\*\*)/g).map((part, index) => {
    if (part.startsWith('**') && part.endsWith('**')) {
      return <strong key={`${part}-${index}`}>{part.slice(2, -2)}</strong>;
    }
    return <Fragment key={`${part}-${index}`}>{part}</Fragment>;
  });
}

export function AgentReply({ reply }: { reply: string }) {
  const blocks = parseReply(reply);

  return (
    <section className="agent-reply" aria-label="Explicación de Hausy">
      {blocks.map((block, index) => {
        if (block.type === 'heading') {
          return (
            <h3 key={`${block.text}-${index}`}>{renderInline(block.text)}</h3>
          );
        }
        if (block.type === 'list') {
          const List = block.ordered ? 'ol' : 'ul';
          return (
            <List key={`list-${index}`}>
              {block.items.map((item, itemIndex) => (
                <li key={`${item}-${itemIndex}`}>{renderInline(item)}</li>
              ))}
            </List>
          );
        }
        return <p key={`${block.text}-${index}`}>{renderInline(block.text)}</p>;
      })}
    </section>
  );
}
