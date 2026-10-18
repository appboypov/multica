/** The sheet state in the URL hash: `#/<zoom>?journey=<id>&step=<n>&model=<name>`. */
export interface Route {
  zoom: string;
  journey: string | null;
  step: number;
  model: string | null;
}

export function readRoute(hash: string): Route {
  const [path, query = ""] = hash.replace(/^#\/?/, "").split("?");
  const params = new URLSearchParams(query);
  return {
    zoom: decodeURIComponent(path) || "map",
    journey: params.get("journey"),
    step: Math.max(0, Number(params.get("step") ?? 0) || 0),
    model: params.get("model"),
  };
}

export function routeHash(route: Route): string {
  const params = new URLSearchParams();
  if (route.journey) {
    params.set("journey", route.journey);
    params.set("step", String(route.step));
  }
  if (route.model) params.set("model", route.model);
  const query = params.toString();
  return `#/${encodeURIComponent(route.zoom)}${query ? `?${query}` : ""}`;
}
