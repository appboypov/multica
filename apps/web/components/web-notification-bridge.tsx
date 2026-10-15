"use client";

import { useEffect, useRef } from "react";
import {
  registerSystemNotificationClickHandler,
  systemNotificationPath,
} from "@multica/core/platform";
import { useNavigation } from "@multica/views/navigation";

/**
 * Routes browser notification clicks to the source workspace: a to-do banner
 * opens Inbox at that to-do, a notice banner opens Updates at its row. The web
 * counterpart of the desktop `DesktopInboxBridge`: desktop receives the click
 * via Electron IPC, web wires it through the Notification API's `onclick`
 * (registered here into the core singleton).
 *
 * The route uses the `slug` the notification was emitted with — the SOURCE
 * workspace — not the active one, so a click always opens the right page even
 * after the user switches workspaces (#3766). An empty slug (unresolved
 * source) is ignored. Marking a notice read is handled by UpdatesPage's
 * selected-item effect, which covers the `?issue=` URL-param path.
 */
export function WebNotificationBridge() {
  const { push } = useNavigation();
  // The adapter identity changes with the current route; the ref keeps the
  // registered click handler stable while always calling the latest push.
  const pushRef = useRef(push);
  useEffect(() => {
    pushRef.current = push;
  }, [push]);

  useEffect(() => {
    registerSystemNotificationClickHandler((payload) => {
      const path = systemNotificationPath(payload);
      if (path) pushRef.current(path);
    });
    return () => registerSystemNotificationClickHandler(null);
  }, []);

  return null;
}
