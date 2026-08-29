import type { Monaco, OnMount } from "@monaco-editor/react";

export type CodeEditor = Parameters<OnMount>[0];

/**
 * Replaces the editor's contents with `next`, touching only the span that
 * actually changed.
 *
 * Edits arrive as whole documents, so the naive implementation is
 * `model.setValue(next)`. That resets the local cursor and selection on every
 * keystroke the other person makes, which makes the editor unusable for the
 * person not typing. Narrowing to the changed range keeps the cursor put in
 * the common case, where someone is editing a different part of the file.
 */
export function applyRemoteText(
  editor: CodeEditor,
  monaco: Monaco,
  next: string,
): void {
  const model = editor.getModel();
  if (!model) return;

  const current = model.getValue();
  if (current === next) return;

  // Longest common prefix.
  let start = 0;
  const limit = Math.min(current.length, next.length);
  while (start < limit && current[start] === next[start]) start += 1;

  // Longest common suffix, without crossing back over the prefix.
  let endCurrent = current.length;
  let endNext = next.length;
  while (
    endCurrent > start &&
    endNext > start &&
    current[endCurrent - 1] === next[endNext - 1]
  ) {
    endCurrent -= 1;
    endNext -= 1;
  }

  const range = monaco.Range.fromPositions(
    model.getPositionAt(start),
    model.getPositionAt(endCurrent),
  );

  // pushEditOperations, not setValue: it keeps the undo stack intact and lets
  // Monaco shift the local selection around the edit.
  model.pushEditOperations(
    [],
    [{ range, text: next.slice(start, endNext) }],
    () => null,
  );
}
