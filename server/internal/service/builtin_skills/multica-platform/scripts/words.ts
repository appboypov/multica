#!/usr/bin/env bun
/** Brian's words in Multica between --start and --end: his issue comments and his question answers; stdout is one Words object. */
import { readFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";

export type Source = "multica comment" | "multica answer";

export interface Window {
	start: string;
	end: string;
}

export interface Word {
	at: string;
	source: Source;
	where: string;
	text: string;
	about: string;
}

export interface Unreadable {
	source: Source;
	problem: string;
}

export interface Words {
	words: Word[];
	unreadable: Unreadable[];
}

export function inWindow(window: Window, at: string): boolean {
	const time = Date.parse(at);
	return Date.parse(window.start) <= time && time < Date.parse(window.end);
}

export function parseArguments(args: string[]): Window {
	const options: Partial<Window> = {};
	for (let i = 0; i < args.length; i += 2) {
		const name = args[i] === "--start" ? "start" : args[i] === "--end" ? "end" : undefined;
		const value = args[i + 1];
		if (!name || value === undefined || name in options || !Number.isFinite(Date.parse(value))) {
			throw new Error("Usage: bun words.ts --start ISO_TIME --end ISO_TIME");
		}
		options[name] = value;
	}
	if (options.start === undefined || options.end === undefined) throw new Error("Usage: bun words.ts --start ISO_TIME --end ISO_TIME");
	return { start: options.start, end: options.end };
}

function word(source: Source, at: string, where: string, text: string, about: string): Word {
	return { source, at: new Date(at).toISOString(), where, text, about };
}

type Fetch = (url: string, init?: RequestInit) => Promise<Response>;

async function http<T>(url: string, init: RequestInit, request: Fetch): Promise<T> {
	const response = await request(url, init);
	if (!response.ok) throw new Error(`${init.method ?? "GET"} ${url}: HTTP ${response.status}`);
	return await response.json() as T;
}

interface MulticaProfile {
	server_url: string;
	token: string;
	workspace_id: string;
}

async function multica(profileFile: string, request: Fetch) {
	const profile = JSON.parse(readFileSync(profileFile, "utf8")) as MulticaProfile;
	const get = <T>(route: string) => http<T>(`${profile.server_url.replace(/\/$/, "")}${route}`, {
		headers: { Authorization: `Bearer ${profile.token}`, "X-Workspace-ID": profile.workspace_id },
	}, request);
	const brian = await get<{ id: string }>("/api/me");
	return { get, brian: brian.id };
}

interface Issue {
	id: string;
	identifier: string;
	title: string;
	last_activity_at: string;
}

interface Comment {
	id: string;
	author_type: string;
	author_id: string;
	created_at: string;
	content: string;
}

export async function readMulticaComments(window: Window, profileFile: string, request: Fetch = fetch): Promise<Word[]> {
	const api = await multica(profileFile, request);
	const words: Word[] = [];
	let offset = 0;
	while (true) {
		const page = await api.get<{ issues: Issue[]; total: number }>(`/api/issues?limit=100&offset=${offset}`);
		for (const issue of page.issues) {
			if (Date.parse(issue.last_activity_at) < Date.parse(window.start)) continue;
			const comments = await api.get<Comment[]>(`/api/issues/${encodeURIComponent(issue.id)}/comments?since=${encodeURIComponent(window.start)}`);
			for (const comment of comments) {
				if (comment.author_type === "member" && comment.author_id === api.brian && inWindow(window, comment.created_at)) {
					words.push(word("multica comment", comment.created_at, `${issue.identifier}#${comment.id}`, comment.content, issue.title));
				}
			}
		}
		offset += page.issues.length;
		if (!page.issues.length || offset >= page.total) break;
	}
	return words;
}

interface Question {
	id: string;
	prompt: string;
	answers: {
		id: string;
		answerer_user_id: string;
		created_at: string;
		display_text: string;
		note: string | null;
	}[];
}

export async function readMulticaAnswers(window: Window, profileFile: string, request: Fetch = fetch): Promise<Word[]> {
	const api = await multica(profileFile, request);
	const words: Word[] = [];
	let cursor: string | null = null;
	do {
		const page: { rows: { id: string }[]; next_cursor: string | null } = await api.get(
			`/api/questions?tab=answered${cursor ? `&cursor=${encodeURIComponent(cursor)}` : ""}`,
		);
		for (const row of page.rows) {
			const question = await api.get<Question>(`/api/questions/${encodeURIComponent(row.id)}`);
			for (const answer of question.answers) {
				if (answer.answerer_user_id === api.brian && inWindow(window, answer.created_at)) {
					const text = [answer.display_text, answer.note].filter(value => value !== null && value !== "").join("\n");
					words.push(word("multica answer", answer.created_at, `${question.id}#${answer.id}`, text, question.prompt));
				}
			}
		}
		cursor = page.next_cursor;
	} while (cursor);
	return words;
}

export interface Reader {
	source: Source;
	read: () => Promise<Word[]>;
}

export async function collectWords(readers: Reader[]): Promise<Words> {
	const result: Words = { words: [], unreadable: [] };
	await Promise.all(readers.map(async reader => {
		try {
			result.words.push(...await reader.read());
		} catch (error) {
			result.unreadable.push({ source: reader.source, problem: error instanceof Error ? error.message : String(error) });
		}
	}));
	result.words.sort((left, right) => Date.parse(left.at) - Date.parse(right.at));
	return result;
}

export async function main(args = process.argv.slice(2)): Promise<void> {
	const window = parseArguments(args);
	const profile = path.join(os.homedir(), ".multica", "profiles", "local", "config.json");
	console.log(JSON.stringify(await collectWords([
		{ source: "multica comment", read: () => readMulticaComments(window, profile) },
		{ source: "multica answer", read: () => readMulticaAnswers(window, profile) },
	])));
}

if (import.meta.main) {
	try {
		await main();
	} catch (error) {
		console.error(error instanceof Error ? error.message : String(error));
		process.exitCode = 2;
	}
}
