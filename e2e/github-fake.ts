import { createServer, type Server } from "node:http";

export interface FakeGitHubRepository {
  id: number;
  full_name: string;
  private: boolean;
  default_branch: string;
  archived: boolean;
  html_url: string;
  clone_url: string;
}

export function githubRepository(fullName: string, id: number, isPrivate = true): FakeGitHubRepository {
  return {
    id, full_name: fullName, private: isPrivate, default_branch: "main", archived: false,
    html_url: `https://github.com/${fullName}`, clone_url: `https://github.com/${fullName}.git`,
  };
}

export type FakeGitHub = { server: Server; repositories: FakeGitHubRepository[] };

// The API must inherit this URL before startup. JWT signing uses a test RSA
// key; the fake deliberately ignores auth and never contacts real GitHub.
export async function startGitHubFake(): Promise<FakeGitHub> {
  const configured = process.env.GITHUB_API_URL;
  if (!configured) {
    throw new Error(
      "Repository E2E requires the API to be started with GITHUB_API_URL pointing at this fake, " +
      "GITHUB_APP_ID and an RSA GITHUB_APP_PRIVATE_KEY; set the same GITHUB_API_URL for Playwright.",
    );
  }
  const base = new URL(configured);
  if (base.protocol !== "http:" || !base.port || base.pathname !== "/" || base.search || base.hash) {
    throw new Error("GITHUB_API_URL must be an http://host:port origin for the repository E2E fake");
  }
  const repositories: FakeGitHub["repositories"] = [];
  const server = createServer((request, response) => {
    request.resume();
    const url = new URL(request.url ?? "/", base);
    const json = (status: number, body: unknown) => {
      response.writeHead(status, { "Content-Type": "application/json" });
      response.end(JSON.stringify(body));
    };
    if (request.method === "POST" && /^\/app\/installations\/\d+\/access_tokens$/.test(url.pathname)) {
      json(201, { token: "e2e-installation-token" });
    } else if (request.method === "GET" && url.pathname === "/installation/repositories") {
      const page = Number(url.searchParams.get("page") ?? "1");
      const perPage = Number(url.searchParams.get("per_page") ?? "100");
      json(200, { total_count: repositories.length, repositories: repositories.slice((page - 1) * perPage, page * perPage) });
    } else if (request.method === "GET" && url.pathname.startsWith("/repos/")) {
      const repository = repositories.find((item) => `/repos/${item.full_name}` === url.pathname);
      json(repository ? 200 : 404, repository ?? { message: "Not Found" });
    } else if (request.method === "DELETE" && url.pathname === "/installation/token") {
      response.writeHead(204);
      response.end();
    } else if (request.method === "GET" && /^\/app\/installations\/\d+$/.test(url.pathname)) {
      json(200, { account: { login: "appboypov", type: "User", avatar_url: "" } });
    } else {
      json(500, { message: `Unexpected fake GitHub request: ${request.method} ${url.pathname}` });
    }
  });
  const { promise, resolve, reject } = Promise.withResolvers<void>();
  server.once("error", reject);
  server.listen(Number(base.port), base.hostname, () => {
    server.removeListener("error", reject);
    resolve();
  });
  await promise;
  return { server, repositories };
}

export async function stopGitHubFake(github: FakeGitHub | undefined): Promise<void> {
  if (!github) return;
  const { promise, resolve, reject } = Promise.withResolvers<void>();
  github.server.close((error) => error ? reject(error) : resolve());
  github.server.closeIdleConnections();
  await promise;
}
