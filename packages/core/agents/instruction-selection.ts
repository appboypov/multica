export const SELECTION_PLACEHOLDER = "{selection}";
export const SELECTION_BLOCK = `<SystemIdentity>\n${SELECTION_PLACEHOLDER}\n</SystemIdentity>`;

export function appendSelection(current: string[], added: string[]): string[] {
  const selection = [...current];
  const selectedPaths = new Set(current);
  for (const path of added) {
    if (!selectedPaths.has(path)) {
      selectedPaths.add(path);
      selection.push(path);
    }
  }
  return selection;
}

export function insertSelectionPlaceholder(
  text: string,
  cursor: number | null,
): { text: string; cursor: number } {
  if (cursor !== null) {
    return {
      text: text.slice(0, cursor) + SELECTION_BLOCK + text.slice(cursor),
      cursor: cursor + SELECTION_BLOCK.length,
    };
  }
  const nextText = (text ? `${text}\n\n` : "") + SELECTION_BLOCK;
  return { text: nextText, cursor: nextText.length };
}

export function selectionWarning(
  instructions: string,
  selection: string[],
): "not_loaded" | "empty_placeholder" | null {
  const hasPlaceholder = instructions.includes(SELECTION_PLACEHOLDER);
  if (selection.length > 0 && !hasPlaceholder) return "not_loaded";
  if (selection.length === 0 && hasPlaceholder) return "empty_placeholder";
  return null;
}
