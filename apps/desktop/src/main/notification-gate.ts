/**
 * Main-process authority for native notification eligibility. Renderer focus
 * is window-local, so only main can answer whether any Multica window is in
 * the foreground and collapse the same realtime event from N renderers.
 */
export class NotificationGate {
  private readonly seenItemIds = new Set<string>();

  constructor(private readonly maxRememberedItems = 1_000) {}

  shouldShow(itemId: string, anyWindowFocused: boolean): boolean {
    const normalizedItemId = itemId.trim();
    if (!normalizedItemId) return false;
    if (this.seenItemIds.has(normalizedItemId)) return false;

    this.seenItemIds.add(normalizedItemId);
    if (this.seenItemIds.size > this.maxRememberedItems) {
      const oldest = this.seenItemIds.values().next().value;
      if (typeof oldest === "string") this.seenItemIds.delete(oldest);
    }
    return !anyWindowFocused;
  }
}

export type NativeNotificationDestination = "inbox" | "updates";

export interface NativeNotificationPayload {
  slug: string;
  itemId: string;
  issueKey: string;
  title: string;
  body: string;
  /** Inbox for a to-do, Updates for a notice; the click routes on it. */
  destination: NativeNotificationDestination;
}

type NativeNotificationTextField = Exclude<keyof NativeNotificationPayload, "destination">;

export function parseNativeNotificationPayload(
  value: unknown,
): NativeNotificationPayload | null {
  if (!value || typeof value !== "object") return null;
  const candidate = value as Record<string, unknown>;
  const limits: Record<NativeNotificationTextField, number> = {
    slug: 256,
    itemId: 256,
    issueKey: 256,
    title: 512,
    body: 2_000,
  };
  const text = {} as Pick<NativeNotificationPayload, NativeNotificationTextField>;
  for (const key of Object.keys(limits) as NativeNotificationTextField[]) {
    const field = candidate[key];
    const mayBeEmpty = key === "slug" || key === "body";
    if (
      typeof field !== "string" ||
      (!mayBeEmpty && !field.trim()) ||
      field.length > limits[key]
    ) {
      return null;
    }
    text[key] = field;
  }
  const destination = candidate.destination;
  if (destination !== "inbox" && destination !== "updates") return null;
  return { ...text, destination };
}
