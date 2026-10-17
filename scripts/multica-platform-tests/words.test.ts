/** Run: bun test scripts/multica-platform-tests/words.test.ts */
import { afterEach, beforeEach, describe, expect, test } from "bun:test";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import {
	collectWords, parseArguments, readMulticaAnswers, readMulticaComments,
} from "../../server/internal/service/builtin_skills/multica-platform/scripts/words.ts";

let temp: string;
const window = { start: "2026-10-14T02:00:00.000Z", end: "2026-10-15T02:00:00.000Z" };
const inside = "2026-10-14T12:00:00.000Z";
const before = new Date(Date.parse(window.start) - 1).toISOString();
const last = new Date(Date.parse(window.end) - 1).toISOString();

beforeEach(() => { temp = mkdtempSync(path.join(os.tmpdir(), "multica-words-")); });
afterEach(() => { rmSync(temp, { recursive: true, force: true }); });

function profile(): string {
	const file = path.join(temp, "profile.json");
	writeFileSync(file, JSON.stringify({ server_url: "http://multica.test", token: "fixture-token", workspace_id: "fixture-workspace" }));
	return file;
}

type Transport = (url: string, init?: RequestInit) => Promise<Response>;
function multicaTransport(handle: (url: URL) => unknown): Transport {
	return async (url, init) => {
		const headers = new Headers(init?.headers);
		expect(headers.get("Authorization")).toBe("Bearer fixture-token");
		expect(headers.get("X-Workspace-ID")).toBe("fixture-workspace");
		const parsed = new URL(url);
		return Response.json(parsed.pathname === "/api/me" ? { id: "brian" } : handle(parsed));
	};
}

describe("arguments", () => {
	test("--start and --end are both required ISO times", () => {
		expect(parseArguments(["--start", window.start, "--end", window.end])).toEqual(window);
		expect(() => parseArguments(["--start", window.start])).toThrow("Usage");
		expect(() => parseArguments(["--start", "yesterday", "--end", window.end])).toThrow("Usage");
	});
});

describe("network readers", () => {
	test("Multica comments page beyond 100 issues, skip old issues and filter author and window", async () => {
		const requests: string[] = [];
		const issue = (id: number, at = inside) => ({ id: String(id), identifier: `WORK-${id}`, title: `Issue ${id}`, last_activity_at: at });
		const comment = (id: string, at: string, author_type = "member", author_id = "brian") => ({ id, content: id, created_at: at, author_type, author_id });
		const transport = multicaTransport(url => {
			requests.push(url.pathname + url.search);
			if (url.pathname === "/api/issues") {
				expect(url.searchParams.get("limit")).toBe("100");
				if (url.searchParams.get("offset") === "0") return { issues: Array.from({ length: 100 }, (_, i) => issue(i, i === 0 ? inside : before)), total: 101 };
				expect(url.searchParams.get("offset")).toBe("100");
				return { issues: [issue(100)], total: 101 };
			}
			expect(url.searchParams.get("since")).toBe(window.start);
			if (url.pathname === "/api/issues/0/comments") return [
				comment("too early", before), comment("at start", window.start), comment("at end", window.end),
				comment("System's posted remark", inside, "member", "system"), comment("agent with matching id", inside, "agent"),
				comment("another member", inside, "member", "other"),
			];
			if (url.pathname === "/api/issues/100/comments") return [comment("second page", last)];
			throw new Error(`Unexpected route ${url}`);
		});
		const words = await readMulticaComments(window, profile(), transport);
		expect(words.map(item => [item.text, item.at, item.where, item.about])).toEqual([
			["at start", window.start, "WORK-0#at start", "Issue 0"],
			["second page", last, "WORK-100#second page", "Issue 100"],
		]);
		expect(requests).toHaveLength(4);
	});

	test("Multica answers page through a full first page; old questions can have new Brian answers", async () => {
		let detailsRead = 0;
		let secondPage = false;
		const answer = (id: string, at: string, who = "brian") => ({ id, created_at: at, answerer_user_id: who, value: { option: "wire value" }, display_text: id, note: `${id} note` });
		const transport = multicaTransport(url => {
			if (url.pathname === "/api/questions") {
				expect(url.searchParams.get("tab")).toBe("answered");
				if (!url.searchParams.has("cursor")) return { rows: Array.from({ length: 50 }, (_, i) => ({ id: String(i), created_at: before })), next_cursor: "next/+? page" };
				expect(url.searchParams.get("cursor")).toBe("next/+? page");
				secondPage = true;
				return { rows: [{ id: "50", created_at: before }], next_cursor: null };
			}
			const id = url.pathname.match(/^\/api\/questions\/(\d+)$/)?.[1];
			if (id === undefined) throw new Error(`Unexpected route ${url}`);
			detailsRead++;
			return { id, prompt: `Question ${id}`, answers: id === "0"
				? [answer("first", window.start), answer("bot", inside, "system"), answer("end", window.end)]
				: id === "50" ? [answer("last", last)] : [answer("old", before)] };
		});
		const words = await readMulticaAnswers(window, profile(), transport);
		expect(words.map(item => [item.text, item.about])).toEqual([["first\nfirst note", "Question 0"], ["last\nlast note", "Question 50"]]);
		expect(detailsRead).toBe(51);
		expect(secondPage).toBe(true);
	});

	test("an unreadable source is named while the other source's words are sorted", async () => {
		const result = await collectWords([
			{ source: "multica comment", read: async () => { throw new Error("offline"); } },
			{ source: "multica answer", read: async () => [
				{ source: "multica answer", at: last, where: "q#2", text: "later", about: "" },
				{ source: "multica answer", at: inside, where: "q#1", text: "earlier", about: "" },
			] },
		]);
		expect(result.words.map(item => item.text)).toEqual(["earlier", "later"]);
		expect(result.unreadable).toEqual([{ source: "multica comment", problem: "offline" }]);
	});
});
