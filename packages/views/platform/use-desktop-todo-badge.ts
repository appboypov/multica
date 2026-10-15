import { useEffect } from "react";
import { useTodoCount } from "@multica/core/todos/queries";

type BadgeCapableAPI = {
  setUnreadBadge?: (count: number) => void;
};

function getDesktopAPI(): BadgeCapableAPI | undefined {
  if (typeof window === "undefined") return undefined;
  return (window as unknown as { desktopAPI?: BadgeCapableAPI }).desktopAPI;
}

/**
 * Mirror the workspace's open to-do count (the Inbox number) onto the OS
 * dock/taskbar badge. Unread Updates never count. No-op on web (no
 * `desktopAPI`) and on the login screen (no workspace ⇒ count defaults to 0,
 * which clears any stale badge from a previous session).
 */
export function useDesktopTodoBadge(wsId: string | null | undefined): void {
  const count = useTodoCount(wsId);
  useEffect(() => {
    getDesktopAPI()?.setUnreadBadge?.(count);
  }, [count]);
}
