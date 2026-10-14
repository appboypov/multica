# ADR-0012: The server holds the issue templates, and a type is set once

- Status: accepted
- Date: 2026-10-13
- Supersedes: none

## Context

Brian creates every issue in New issue from one of eight templates: Epic, Journey, Story, Bug, Enhancement, Refactor, Chore and Spike. A template's sections become fields, and the fields fill one Markdown description, which is all the issue keeps. Agents and the CLI create issues too, with or without a type. An issue created with a type and no description gets its template's Markdown, so the server needs the templates as well as the clients. Issues already in Multica have no type. UJ-6 filters by type and opens New issue from a Board column, and UJ-8 lists a release's issues by type.

## Decision

- `server/internal/issuetemplate` holds the eight templates as data: each a type and its sections, each section a heading, a question and an optional field with its key, label, placeholder and format. It also holds `Markdown(template, fields)`.
- The server serves them at `GET /api/issue-templates`. Web and desktop render New issue from that answer and do not keep their own copy of the templates.
- The clients render the fields into Markdown with a TypeScript port of `Markdown` in `packages/core/issues/templates/`. Both run the same case file, `server/internal/issuetemplate/testdata/cases.json`.
- A template's headings, questions, labels and placeholders are template text. They stay as written in every language, because they become the Markdown agents read. Type names and hints are UI copy, translated by type key.
- `issue.type` is one of the eight or NULL, "No type". It is set at creation or once on the issue page, and never changes after that. The server refuses a second set with `type_fixed`.
- The server never refuses a description for missing template headings, and never rewrites a description it was sent.

## Consequences

- A template changes in one place, and every client and the CLI path follow on the next read.
- The Markdown function lives in two languages. The shared case file catches drift.
- A newer server can add a format the client does not know. Zod then falls back to a text field.
- UJ-6 and UJ-8 read `issue.type` as it is. No later journey needs a type migration for existing issues.
