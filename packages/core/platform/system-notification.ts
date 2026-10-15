"use client";

// Native system notification bridge for the WEB app.
//
// The desktop app renders OS banners through its Electron main process
// (`window.desktopAPI.showNotification`); the web app has no such bridge, so it
// uses the browser Notification API here. `handleInboxNew` (realtime sync) is
// the single decision point — it already gates on focus + the source
// workspace's mute preference — and calls `showWebNotification` as the web path
// when no `desktopAPI` is present.
//
// Lives in `packages/core` (not `packages/views`) because the caller
// (`handleInboxNew`) lives in core and `views` cannot be imported from core
// (dependency direction is views → core). The click-routing decision, which
// needs the app router, is injected by the host app via
// `registerSystemNotificationClickHandler` instead — core stays headless.

import { paths } from "../paths/paths";

/** Where a clicked banner lands: a to-do in Inbox, or a notice in Updates. */
export type SystemNotificationDestination = "inbox" | "updates";

export interface SystemNotificationPayload {
  /**
   * Source workspace slug. Empty when it couldn't be resolved — the click is
   * then a no-op rather than routing to the wrong workspace (#3766).
   */
  slug: string;
  /** The to-do's or notice's own id; collapses repeat banners for one row. */
  itemId: string;
  /**
   * The destination page's selector: Inbox's `?todo=` to-do id, or Updates'
   * `?issue=` key (issue id, else the notice id).
   */
  issueKey: string;
  title: string;
  body: string;
  destination: SystemNotificationDestination;
}

/**
 * The page a clicked banner opens, in the workspace it was emitted for. Shared
 * by the web and desktop click bridges so both route a to-do to Inbox and a
 * notice to Updates. Null when the source workspace is unknown: the click then
 * does nothing.
 */
export function systemNotificationPath(
  target: Pick<SystemNotificationPayload, "slug" | "issueKey" | "destination">,
): string | null {
  if (!target.slug) return null;
  const ws = paths.workspace(target.slug);
  return target.destination === "inbox"
    ? ws.inboxTodo(target.issueKey)
    : ws.updatesItem(target.issueKey);
}

type ClickHandler = (payload: SystemNotificationPayload) => void;

// Module-level singleton — mirrors how the desktop preload registers its
// behavior once at boot. The web shell registers a router-aware handler; while
// unregistered (SSR, tests, pre-mount) a click is a silent no-op.
let clickHandler: ClickHandler | null = null;

/**
 * Register how a clicked web notification routes (focus + navigate to the
 * source workspace's inbox, focused on the item). Called once by the web app
 * shell; pass `null` to unregister. Desktop does NOT use this — it routes
 * through its own Electron IPC bridge (`onInboxOpen`).
 */
export function registerSystemNotificationClickHandler(
  handler: ClickHandler | null,
): void {
  clickHandler = handler;
}

// Read the Notification constructor off `window` (rather than the bare global)
// so it's both SSR-safe and injectable from the core Node test environment,
// where `window`/`Notification` don't exist by default.
function getNotificationCtor(): typeof Notification | null {
  if (typeof window === "undefined") return null;
  const ctor = (window as { Notification?: typeof Notification }).Notification;
  return typeof ctor === "function" ? ctor : null;
}

/** True when the browser exposes the Notification API (false on SSR / old engines). */
export function isWebNotificationSupported(): boolean {
  return getNotificationCtor() !== null;
}

export type WebNotificationPermission = NotificationPermission | "unsupported";

/** Current permission, or "unsupported" when the API is unavailable. */
export function getWebNotificationPermission(): WebNotificationPermission {
  const ctor = getNotificationCtor();
  return ctor ? ctor.permission : "unsupported";
}

/**
 * Prompt for notification permission. Resolves to the resulting permission, or
 * "unsupported". Only "default" triggers the browser prompt — an
 * already-decided permission (granted/denied) is returned as-is.
 */
export async function requestWebNotificationPermission(): Promise<WebNotificationPermission> {
  const ctor = getNotificationCtor();
  if (!ctor) return "unsupported";
  if (ctor.permission !== "default") return ctor.permission;
  try {
    return await ctor.requestPermission();
  } catch {
    // Older Safari used a callback signature and can reject the promise form.
    return ctor.permission;
  }
}

/**
 * Show a native browser notification for a new inbox item. No-op unless the
 * Notification API is supported AND permission is "granted" — the caller
 * (`handleInboxNew`) owns the WHETHER (focus + mute gating); this owns only the
 * rendering. Clicking the banner focuses the tab and routes via the registered
 * click handler.
 */
export function showWebNotification(payload: SystemNotificationPayload): void {
  const ctor = getNotificationCtor();
  if (!ctor || ctor.permission !== "granted") return;
  let notification: Notification;
  try {
    notification = new ctor(payload.title, {
      body: payload.body,
      // Collapse repeat banners for the same inbox row (e.g. a reconnect
      // replays the `inbox:new` event).
      tag: payload.itemId,
    });
  } catch {
    // Some engines require an active ServiceWorkerRegistration to construct a
    // Notification (notably Chrome on Android). Degrade silently — the in-app
    // inbox and unread badge still surface the new item.
    return;
  }
  notification.onclick = () => {
    try {
      window.focus();
    } catch {
      // Best-effort; some browsers disallow programmatic focus.
    }
    notification.close();
    clickHandler?.(payload);
  };
}
