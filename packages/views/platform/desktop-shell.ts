/** True when the renderer runs inside the Electron desktop shell, as evidenced
 *  by the preload-exposed `desktopAPI` bridge. Avoids hard-coding
 *  navigator/process checks — those vary across electron-vite + jsdom tests. */
export function isDesktopShell(): boolean {
  if (typeof window === "undefined") return false;
  return "desktopAPI" in window && typeof window.desktopAPI === "object" && window.desktopAPI !== null;
}
