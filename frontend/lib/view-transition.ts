// Animated view changes.
//
// Sending a message rebuilds most of the page at once: the welcome view turns
// into the search workspace, or a follow-up joins the conversation. `morph`
// applies such a change as one browser view transition, so elements that exist
// on both sides (the composer, the headline, the message being sent) glide to
// their new place instead of jumping. The choreography itself is CSS, keyed on
// `data-morphing="<kind>"` on `<html>` while the transition runs (see
// `app/globals.css`, "View morphs").
//
// Where the browser cannot animate views, or the reader asked for reduced
// motion, the update is applied at once and nothing else changes.

import { flushSync } from 'react-dom';

/**
 * Which change is running. `workspace`: the welcome view becomes the search
 * workspace, so most of the page is replaced. `turn`: a follow-up joins the
 * conversation, so most of the page stays as it is.
 */
export type MorphKind = 'workspace' | 'turn';

let running: ViewTransition | undefined;

/**
 * Applies `update` (React state setters) as one animated view change.
 * Resolves once the DOM shows the new view; the animation may still be
 * running. Anything that renders into the new view, like a request's
 * results, should wait for this.
 */
export function morph(kind: MorphKind, update: () => void): Promise<void> {
  const root = document.documentElement;
  if (
    typeof document.startViewTransition !== 'function' ||
    window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
  ) {
    update();
    return Promise.resolve();
  }

  root.dataset.morphing = kind;
  // The browser captures the old view first, then calls back; flushSync makes
  // React commit inside that callback so the new view is captured complete.
  const transition = document.startViewTransition(() => flushSync(update));
  running = transition;
  const settle = () => {
    if (running !== transition) return;
    running = undefined;
    delete root.dataset.morphing;
  };
  transition.finished.then(settle, settle);
  return transition.updateCallbackDone;
}
