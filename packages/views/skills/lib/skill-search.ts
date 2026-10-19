/**
 * Keeps the items whose shown fields contain the search text, ignoring case.
 * Callers pass only fields the row renders, so every match can be
 * highlighted. An empty search keeps every item.
 */
export function filterBySkillSearch<T>(
  items: readonly T[],
  query: string,
  fields: (item: T) => Array<string | null | undefined>,
): readonly T[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return items;
  return items.filter((item) =>
    fields(item).some((value) => value?.toLowerCase().includes(needle) ?? false),
  );
}
