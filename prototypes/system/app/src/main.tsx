import { createRoot } from "react-dom/client";
import { App } from "./App.tsx";
import type { SystemDesign } from "./system.ts";
import "./styles.css";

const root = createRoot(document.getElementById("root")!);

fetch("./system.json")
  .then((response) => {
    if (!response.ok) throw new Error(`system.json: ${response.status}`);
    return response.json() as Promise<SystemDesign>;
  })
  .then((design) => root.render(<App design={design} />))
  .catch((error: unknown) => {
    console.error("[system] failed to load system.json", error);
    root.render(<p className="empty">Could not load system.json: {String(error)}</p>);
  });
