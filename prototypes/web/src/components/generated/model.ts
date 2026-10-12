import rawOrgs from './data';

export type IssueType = 'epic' | 'journey' | 'story' | 'bug' | 'enhancement' | 'refactor' | 'chore' | 'spike';
export type Status = 'backlog' | 'todo' | 'in_progress' | 'in_review' | 'done';
export type Priority = 'none' | 'urgent' | 'high' | 'medium' | 'low';

export interface Issue {
  id: string;
  type: IssueType;
  title: string;
  status: Status;
  priority?: Priority;
  assignee?: string;
  lane?: LaneRef;
  release?: string;
  /** The issue's one Markdown description. */
  description?: string;
  /** The squad the issue went to when it was made in the prototype. */
  squad?: string;
  /** True for issues made in this prototype session; only they have activity. */
  created?: boolean;
}

/**
 * A version of the repository with a title, a description and the issues that go in it. Every worklane of
 * the repository holds the same release line, each release with only that worklane's issues.
 */
export interface Release {
  /** Stays the same when the release is renumbered. */
  id: string;
  version: string;
  title: string;
  description: string;
  /** The step from the release before it; the version follows from it unless it is set by hand. */
  bump: Bump;
  /** The version was set by hand: it keeps that number when the release moves. */
  pinned: boolean;
  issues: Issue[];
}

/** A release without its issues: one place on the repository's release line. */
export type ReleaseHead = Omit<Release, 'issues'>;

export interface Lane {
  name: string;
  state: string;
  active: string | null;
  next: string | null;
  releases?: Release[];
  /** Issues of this worklane that are in no release yet. */
  backlog?: Issue[];
  /** The squad that works the worklane and the folder its worktree lives in, set when it is made in the prototype. */
  squad?: string;
  folder?: string;
}

export interface Repo {
  name: string;
  lanes: Lane[];
  /** The last tag the version policy found, the base for the first planned release. */
  lastTag?: string;
  /** The repository does not release: its worklanes plan no releases. */
  noReleases?: boolean;
}

export interface Project {
  name: string;
  repos: Repo[];
}

export interface Org {
  name: string;
  projects: Project[];
}

export interface LaneRef {
  org: string;
  project: string;
  repo: string;
  lane: string;
}

export type ReleaseState = 'shipped' | 'next' | 'planned';

// Statuses the Multica squad board and the Rebl queue agree on today.
const STATUS_OF: Record<string, Status> = {
  'REBL-10': 'in_progress',
  'REBL-21': 'todo',
  'REBL-35': 'todo',
  'REBL-111': 'in_review',
  'REBL-110': 'done',
  'REBL-109': 'done',
};

interface RawLane {
  name: string;
  state: string;
  active: string | null;
  next: string | null;
  releases?: { version: string; title: string; description: string; issues: { id: string; type: string; title: string }[] }[];
}

/** Sample issues that wait in a worklane without a release, keyed `repo/lane`. */
const LANE_BACKLOG: Record<string, { id: string; type: IssueType; title: string }[]> = {
  'appboypov/rebl/Fan accounts and push': [
    { id: 'REBL-86', type: 'story', title: 'Fans get a push for a new event' },
    { id: 'REBL-87', type: 'story', title: 'Push when an event time changes' },
    { id: 'REBL-88', type: 'chore', title: 'Texts Event push' },
    { id: 'REBL-89', type: 'chore', title: 'E2E Event push' },
  ],
};

export function loadOrgs(): Org[] {
  return (rawOrgs as unknown as { name: string; projects: { name: string; repos: { name: string; lanes: RawLane[] }[] }[] }[]).map(
    (o) => ({
      name: o.name,
      projects: o.projects.map((p) => ({
        name: p.name,
        repos: p.repos.map((r) => {
          // The repository's one release line: every lane's releases in version order.
          const raw = r.lanes.flatMap((l) => l.releases ?? []).sort((a, b) => cmpVersion(a.version, b.version));
          const line: ReleaseHead[] = raw.map((rel, i) => {
            const prev = i ? raw[i - 1].version : null;
            const bump = bumpBetween(prev, rel.version);
            return { id: `${r.name}#${rel.version}`, version: rel.version, title: rel.title, description: rel.description, bump, pinned: bumpFrom(prev, bump) !== rel.version };
          });
          return {
            name: r.name,
            // The default branch's worklane leads, as in the approved design.
            lanes: [...r.lanes].sort((a, b) => Number(b.name === 'main') - Number(a.name === 'main')).map((l) => {
              const lane = { org: o.name, project: p.name, repo: r.name, lane: l.name };
              return {
                name: l.name,
                state: l.state,
                active: l.active,
                next: l.next,
                releases: line.map((head) => ({
                  ...head,
                  issues: (l.releases?.find((rel) => rel.version === head.version)?.issues ?? []).map((i) => ({
                    id: i.id,
                    type: (i.type as IssueType) ?? 'story',
                    title: i.title,
                    status: STATUS_OF[i.id] ?? 'backlog',
                    lane,
                    release: head.version,
                  })),
                })),
                backlog: (LANE_BACKLOG[`${r.name}/${l.name}`] ?? []).map((i) => ({ ...i, status: 'backlog' as Status, lane })),
              };
            }),
          };
        }),
      })),
    }),
  );
}

/** Every issue of a worklane: those in its releases, then those in no release. */
export function laneIssues(lane: Lane): Issue[] {
  return [...(lane.releases ?? []).flatMap((r) => r.issues), ...(lane.backlog ?? [])];
}

export type Bump = 'patch' | 'minor' | 'major';

/** `v1.4.0` or `1.4.0`; anything else is not a version. */
export function parseVersion(v: string): [number, number, number] | null {
  const m = v.trim().match(/^v?(\d+)\.(\d+)\.(\d+)$/);
  return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null;
}

export function cmpVersion(a: string, b: string): number {
  const [x, y] = [parseVersion(a) ?? [0, 0, 0], parseVersion(b) ?? [0, 0, 0]];
  return x[0] - y[0] || x[1] - y[1] || x[2] - y[2];
}

/** The version a bump gives after `base`; the first release of a repository counts from v0.0.0. */
export function bumpFrom(base: string | null, bump: Bump): string {
  const [maj, min, pat] = (base && parseVersion(base)) || [0, 0, 0];
  return bump === 'major' ? `v${maj + 1}.0.0` : bump === 'minor' ? `v${maj}.${min + 1}.0` : `v${maj}.${min}.${pat + 1}`;
}

export function bumpBetween(prev: string | null, v: string): Bump {
  const [a, b] = [(prev && parseVersion(prev)) || [0, 0, 0], parseVersion(v) ?? [0, 0, 0]];
  return b[0] !== a[0] ? 'major' : b[1] !== a[1] ? 'minor' : 'patch';
}

/** The repository's release line; every worklane holds the same one. */
export function lineOf(repo: Repo): ReleaseHead[] {
  return (repo.lanes[0]?.releases ?? []).map(({ issues: _, ...head }) => head);
}

/** Where the line starts: the release before the first one is the last tag, if there is one. */
export function versionBefore(repo: Repo, line: ReleaseHead[], index: number): { version: string; label: string } | null {
  if (index > 0) return { version: line[index - 1].version, label: `${line[index - 1].version} ${line[index - 1].title}` };
  return repo.lastTag ? { version: repo.lastTag, label: `${repo.lastTag}, the last tag` } : null;
}

/**
 * Gives every release after the shipped ones the version its place gives it: the bump from the release before
 * it, or the version set by hand. Shipped releases keep their tag.
 */
export function renumber(repo: Repo, line: ReleaseHead[], shipped: number): ReleaseHead[] {
  const out: ReleaseHead[] = [];
  line.forEach((h, i) => {
    const prev = versionBefore(repo, out, i)?.version ?? null;
    out.push(i < shipped || h.pinned ? h : { ...h, version: bumpFrom(prev, h.bump) });
  });
  return out;
}

/** The first release on the line whose version is not higher than the one before it. */
export function lineProblem(repo: Repo, line: ReleaseHead[]): { index: number; message: string } | null {
  for (let i = 0; i < line.length; i++) {
    const prev = versionBefore(repo, line, i);
    if (prev && cmpVersion(line[i].version, prev.version) <= 0)
      return { index: i, message: `${line[i].version} ${line[i].title} is not higher than ${prev.label}` };
  }
  return null;
}

/** The line with `head` at `index` (replacing the release there, or added at the end), renumbered and checked. */
export function placeRelease(repo: Repo, statusOf: StatusOf, head: ReleaseHead, index: number): { line: ReleaseHead[]; problem: { index: number; message: string } | null } {
  const cur = lineOf(repo);
  const raw = index < cur.length ? cur.map((h, i) => (i === index ? head : h)) : [...cur, head];
  const line = renumber(repo, raw, shippedCount(repo, statusOf));
  return { line, problem: lineProblem(repo, line) };
}

/** The line with the release at `from` moved to `to`, renumbered and checked. Shipped releases never move. */
export function moveRelease(repo: Repo, statusOf: StatusOf, from: number, to: number): { line: ReleaseHead[]; problem: string | null } {
  const cur = lineOf(repo);
  const shipped = shippedCount(repo, statusOf);
  if (from < shipped) return { line: cur, problem: 'Shipped: its tag is fixed' };
  if (to < 0) return { line: cur, problem: 'It is the first release' };
  if (to < shipped) return { line: cur, problem: 'The release before it has shipped' };
  if (to >= cur.length) return { line: cur, problem: 'It is the last release' };
  const raw = [...cur];
  const [h] = raw.splice(from, 1);
  raw.splice(to, 0, h);
  const line = renumber(repo, raw, shipped);
  const p = lineProblem(repo, line);
  return { line, problem: p ? `Then ${p.message}` : null };
}

/** A worklane's folder: one folder name made from its name. */
export function laneFolder(name: string): string {
  return name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
}

export interface GithubRepo {
  name: string;
  visibility: 'Private' | 'Public';
  branch: string;
  /** The version policy Multica finds in it. */
  policy: string;
  lastTag?: string;
  /** Whether `~/Repos/{name}` already holds its checkout on Brian's Mac. */
  checkedOut: boolean;
}

/** Repositories the GitHub App can see that no project holds yet. */
export const GITHUB_REPOS: GithubRepo[] = [
  { name: 'appboypov/rebl-admin', visibility: 'Private', branch: 'main', policy: 'Version in package.json', lastTag: 'v0.4.2', checkedOut: true },
  { name: 'appboypov/rebl-site', visibility: 'Public', branch: 'main', policy: 'Tags only', lastTag: 'v2.1.0', checkedOut: false },
  { name: 'appboypov/fan-widgets', visibility: 'Private', branch: 'main', policy: 'Version in pubspec.yaml', checkedOut: true },
];

export const RUNTIMES = [
  { name: "Brian's Mac", online: true },
  { name: 'Studio Mac mini', online: false },
];
export type StatusOf = (issue: Issue) => Status;

export function doneCount(rel: Release, statusOf: StatusOf): number {
  return rel.issues.filter((i) => statusOf(i) === 'done').length;
}

/** A release has shipped once it holds issues and every worklane's issues in it are done. */
function shippedAt(repo: Repo, index: number, statusOf: StatusOf): boolean {
  const issues = repo.lanes.flatMap((l) => l.releases?.[index]?.issues ?? []);
  return issues.length > 0 && issues.every((i) => statusOf(i) === 'done');
}

/** How many releases at the start of the line have shipped; their versions are fixed. */
export function shippedCount(repo: Repo, statusOf: StatusOf): number {
  const n = lineOf(repo).length;
  let i = 0;
  while (i < n && shippedAt(repo, i, statusOf)) i++;
  return i;
}

/** The repository's first release not shipped is next; earlier ones shipped, later ones planned. */
export function releaseStates(repo: Repo, statusOf: StatusOf): ReleaseState[] {
  const shipped = shippedCount(repo, statusOf);
  return lineOf(repo).map((_, i) => (i < shipped ? 'shipped' : i === shipped ? 'next' : 'planned'));
}

/** The release a worklane works toward: its first unshipped release that holds its issues, else the repository's next. */
export function laneNextIdx(repo: Repo, lane: Lane, statusOf: StatusOf): number {
  const shipped = shippedCount(repo, statusOf);
  const rels = lane.releases ?? [];
  const own = rels.findIndex((r, i) => i >= shipped && r.issues.some((is) => statusOf(is) !== 'done'));
  return own >= 0 ? own : shipped < rels.length ? shipped : -1;
}

export function laneCount(p: Project): number {
  return p.repos.reduce((n, r) => n + r.lanes.length, 0);
}

export function plural(n: number, one: string, many = one + 's'): string {
  return `${n} ${n === 1 ? one : many}`;
}

export function issueKey(text: string | null): { id: string; title: string } | null {
  if (!text) return null;
  const m = text.match(/^([A-Z][A-Z0-9]*-\d+)\s+(.*)$/);
  return m ? { id: m[1], title: m[2] } : { id: '', title: text };
}

/**
 * What a worklane works on now (`active`) or takes next: its own pick while that issue is not done, else its
 * first issue that is not done, in release order.
 */
export function laneNow(lane: Lane, statusOf: StatusOf): { id: string; title: string; active: boolean } | null {
  const issues = laneIssues(lane);
  const open = (id: string) => {
    const i = issues.find((x) => x.id === id);
    return !i || statusOf(i) !== 'done';
  };
  for (const [text, active] of [
    [lane.active, true],
    [lane.next, false],
  ] as const) {
    const k = issueKey(text);
    if (k && open(k.id)) return { ...k, active };
  }
  const i = issues.find((x) => statusOf(x) !== 'done');
  if (!i) return null;
  const s = statusOf(i);
  return { id: i.id, title: i.title, active: s === 'in_progress' || s === 'in_review' };
}

// ---------------------------------------------------------------------------
// Templates: the Markdown issue templates in ~/Work (emoji headings, a question
// callout under each). An issue keeps one Markdown description; the new-issue
// fields only fill that Markdown once. Sections that hold implementation
// details (OpenSpec change, skills and tools, data flow, tests, suggested
// approach and steps, code structure, investigation areas and experiments)
// are left out: code and technical design live in the change.
// ---------------------------------------------------------------------------

/** How a new-issue field's text becomes Markdown under its heading. */
export type InputFormat = 'text' | 'bullets' | 'checks' | 'scenarios';

export interface TemplateInput {
  key: string;
  label: string;
  required: boolean;
  placeholder: string;
  rows: number;
  format: InputFormat;
}

export interface TemplateBlock {
  /** The Markdown heading, with its level and emoji. */
  heading: string;
  /** The question callout under the heading. */
  question?: string;
  /** The new-issue field that fills this section. */
  input?: TemplateInput;
}

export interface Template {
  type: IssueType;
  name: string;
  hint: string;
  blocks: TemplateBlock[];
  /** A filled-in issue of this type, for "Fill example" in the new-issue dialog. */
  example: { title: string; fields: Record<string, string>; priority: Priority };
}

const input = (key: string, label: string, format: InputFormat, placeholder: string, rows = 3, required = true): TemplateInput => ({
  key,
  label,
  required,
  placeholder,
  rows,
  format,
});

const DEPENDENCIES: TemplateBlock = { heading: '## 🔗 Dependencies', question: 'Which issues must be completed first?' };
const RELATED: TemplateBlock = { heading: '## 🔀 Related Issues', question: 'Which related issues provide context (non-blocking)?' };
const REFERENCES: TemplateBlock = { heading: '## 📚 References' };
const SCOPE: TemplateBlock[] = [{ heading: '## 📦 Scope' }, { heading: '### In scope' }, { heading: '### Out of scope' }];

/** The per-scenario part of the story and journey templates. */
const SCENARIO: TemplateBlock[] = [
  { heading: '#### 👤 User Story', question: 'Who benefits and what do they want to achieve?' },
  { heading: '#### ✨ Scenario', question: 'What do the sequences of { 👤 User, 🧠 System, 🎨 Screen } [ verb ] steps look like when this exists?' },
  { heading: '#### 📍 Currently', question: 'What is the current state?' },
  { heading: '#### 🎯 Should', question: 'What should the state be?' },
  { heading: '#### ⚠️ Constraints', question: 'What limitations or constraints exist?' },
  { heading: '#### ✅ Acceptance Criteria', question: 'How do we measure successful achievement of this scenario?' },
];

const ONE_PER_LINE = 'One per line';

export const TEMPLATES: Template[] = [
  {
    type: 'epic',
    name: 'Epic',
    hint: 'An outcome made of stories',
    blocks: [
      DEPENDENCIES,
      RELATED,
      {
        heading: '## 🎯 Thesis & outcome',
        question:
          'What is this body of work and what does “done” mean — using one coherent scope (single epic, theme, or umbrella with child epics, without duplicating the same story under different labels)?',
      },
      { heading: '### What we are doing', input: input('doing', 'What we are doing', 'text', 'The body of work, in a few sentences') },
      { heading: '### What success looks like' },
      { heading: '## 🏢 Strategic alignment', question: 'How does this connect to company goals or OKRs?' },
      { heading: '## 💡 Hypothesis', question: 'What do we believe will happen if we pursue this?' },
      { heading: '## 💰 Value & investment', question: 'Why does this matter, what does it cost, and how do we judge importance?' },
      { heading: '### Opportunity cost of inaction' },
      { heading: '### Expected ROI' },
      { heading: '### Impact (product & engineering)' },
      { heading: '## 👥 Stakeholders', question: 'Who is involved and accountable?' },
      {
        heading: '## 🗺️ Delivery plan',
        question: 'What is the timeline for the whole scope (rows may be phases, child epics, milestones, or sprint goals — use what fits)?',
      },
      {
        heading: '## 👤 User stories',
        question: 'Which user stories comprise this work, in delivery order?',
        input: input('stories', 'User stories', 'checks', `${ONE_PER_LINE}, in delivery order`, 3, false),
      },
      {
        heading: '## 🎯 Abilities',
        question: 'What can actors do when this is complete?',
        input: input('abilities', 'Abilities', 'checks', `{Actor} is able to {outcome}. ${ONE_PER_LINE}`),
      },
      { heading: '## 📍 Currently', question: 'What is the baseline state across this scope?' },
      { heading: '## 🚀 End goal', question: 'What is the tangible measurable end state?' },
      { heading: '## ⚠️ Constraints', question: 'What hard limits, policies, or boundaries apply?' },
      { heading: '## 🚦 Risks & dependencies', question: 'What could block or derail delivery (capture each item in the table)?' },
      { heading: '## 📊 Success metrics', question: 'How will we know it worked?' },
      { heading: '### Leading (short-term)' },
      { heading: '### Lagging (long-term)' },
      { heading: '### Concrete KPIs' },
      {
        heading: '## ✅ Acceptance criteria',
        question: 'How do we know this scope is complete?',
        input: input('acceptance', 'Acceptance criteria', 'checks', ONE_PER_LINE),
      },
      { heading: '## 📎 Related work', question: 'What related epics, docs, designs, or prior art should we link?' },
      { heading: '## 📋 Next steps', question: 'What immediate actions come next?' },
      REFERENCES,
    ],
    example: {
      title: 'Fans manage their own account',
      priority: 'high',
      fields: {
        doing:
          'A fan signs up, signs in and keeps their profile and notification choices up to date in the Rebl app, without asking the Rebl team for help.',
        stories: 'A fan creates an account\nA fan changes their display name\nA fan turns event notifications on and off',
        abilities:
          'Fan is able to create an account and land on the home screen\nFan is able to change their name, photo and language\nFan is able to turn event notifications on and off',
        acceptance: 'Every ability works in the fan app on iPhone and Android\nThe Rebl team gets no account requests by mail for a month',
      },
    },
  },
  {
    type: 'journey',
    name: 'Journey',
    hint: 'One user journey, one change',
    blocks: [
      {
        heading: '## 🗺️ User Journey',
        question: 'What is this journey about in one sentence?',
        input: input('journey', 'User journey', 'text', 'What this journey is about, in one sentence', 2),
      },
      {
        heading: '## ✨ Scenarios',
        question: 'What are the scenarios that make up this journey?',
        input: input('scenarios', 'Scenarios', 'scenarios', `{Actor} is able to {outcome}. ${ONE_PER_LINE}; each gets its scenario sections`, 4),
      },
    ],
    example: {
      title: 'A fan books a table for an event',
      priority: 'high',
      fields: {
        journey: 'A fan books a table for an event and pays for it in the app.',
        scenarios: 'Fan is able to pick a table and the number of guests\nFan is able to pay for the booking\nFan is able to see the booking under My bookings',
      },
    },
  },
  {
    type: 'story',
    name: 'Story',
    hint: 'One slice toward an epic',
    blocks: [
      DEPENDENCIES,
      RELATED,
      {
        heading: '## 👤 User Story',
        question: 'Who benefits and what do they want to achieve?',
        input: input('story', 'User story', 'bullets', 'As a {actor}, I want {action}, so that {benefit}.', 2),
      },
      { heading: '## 🚀 End Goal', question: 'What is the tangible measurable end goal?' },
      {
        heading: '## 📍 Currently',
        question: 'What is the current state?',
        input: input('currently', 'Currently', 'bullets', 'The current state', 2, false),
      },
      {
        heading: '## 🎯 Should',
        question: 'What should the state be?',
        input: input('should', 'Should', 'bullets', 'The state it should be in', 2, false),
      },
      { heading: '## 🗺️ User Journey', question: 'What is this journey about in one sentence?' },
      { heading: '## ✨ Scenarios', question: 'What scenarios make up this journey (each with a mermaid sequence diagram for its flow)?' },
      { heading: '## 🎨 UI', question: 'What does the updated UI look like in ASCII?' },
      { heading: '## ⚠️ Constraints', question: 'What limitations or constraints exist for the overall story?' },
      {
        heading: '## ✅ Acceptance Criteria',
        question: 'How do we measure successful achievement of the end goal?',
        input: input('acceptance', 'Acceptance criteria', 'checks', `Given … when … then …. ${ONE_PER_LINE}`),
      },
      REFERENCES,
    ],
    example: {
      title: 'A fan changes their display name',
      priority: 'medium',
      fields: {
        story: 'As a fan, I want to change my display name, so that friends recognise me in guest lists.',
        currently: 'Fans keep the name they signed up with.',
        should: 'Fans change their display name on their profile, and the new name shows everywhere.',
        acceptance:
          'Given a signed-in fan on their profile, when they change their display name and save, then the new name shows on their profile\nGiven a fan on a guest list, when they change their display name, then the guest list shows the new name',
      },
    },
  },
  {
    type: 'bug',
    name: 'Bug',
    hint: 'Something broken to reproduce',
    blocks: [
      DEPENDENCIES,
      RELATED,
      { heading: '## 💥 Event', question: 'When did the problem occur?' },
      {
        heading: '## 🦋 Expected Result',
        question: 'What did you expect to happen?',
        input: input('expected', 'Expected result', 'text', 'What you expected to happen', 2),
      },
      {
        heading: '## 🐛 Actual Result',
        question: 'What actually happened?',
        input: input('actual', 'Actual result', 'text', 'What actually happened', 2),
      },
      {
        heading: '## 👣 Steps to Reproduce',
        question: 'Which steps can we take to reproduce the problem?',
        input: input('steps', 'Steps to reproduce', 'checks', ONE_PER_LINE, 3, false),
      },
      {
        heading: '## 🌍 Environment',
        question: 'Where did the bug occur (OS, browser, app version, device, authentication state, etc.)?',
        input: input('environment', 'Environment', 'bullets', 'OS, browser, app version, device, signed in or not', 2, false),
      },
      { heading: '## ✅ Acceptance Criteria', question: 'How do we measure the successful fix of the bug?' },
      REFERENCES,
    ],
    example: {
      title: 'Invitation link opens an empty screen',
      priority: 'urgent',
      fields: {
        expected: 'The new team member lands on the app store page for Rebl, and after installing, the invitation opens in the app.',
        actual: 'The browser shows an empty white page and nothing else happens.',
        steps: 'Open a team invitation mail on a phone without the Rebl app\nTap "Join the team"',
        environment: 'Android, Chrome\nNot signed in, Rebl app not installed\nSeen by two new team members this week',
      },
    },
  },
  {
    type: 'enhancement',
    name: 'Enhancement',
    hint: 'Make a capability better',
    blocks: [
      DEPENDENCIES,
      RELATED,
      {
        heading: '## ✨ Enhancement',
        question: 'What existing capability are we improving, and what changes for users or operators?',
        input: input('enhancement', 'Enhancement', 'text', 'The capability, and what changes for whom'),
      },
      { heading: '## 💡 Motivation', question: 'Why does this improvement matter now (pain, opportunity, feedback, metrics)?' },
      ...SCOPE,
      {
        heading: '## 📍 Current behavior',
        question: 'How does it work today?',
        input: input('current', 'Current behavior', 'bullets', 'How it works today', 2, false),
      },
      {
        heading: '## 🎯 Desired behavior',
        question: 'How should it work after this enhancement?',
        input: input('desired', 'Desired behavior', 'bullets', 'How it should work', 2, false),
      },
      {
        heading: '## ⚠️ Constraints',
        question: 'What compatibility, performance budgets, design system rules, release timing, or platform limits apply?',
      },
      {
        heading: '## ✅ Acceptance criteria',
        question: 'How do we know the enhancement is done and acceptable?',
        input: input('acceptance', 'Acceptance criteria', 'checks', ONE_PER_LINE),
      },
      { heading: '## 📊 Success signals', question: 'What metrics, feedback, or qualitative checks after release would show success (optional)?' },
      REFERENCES,
    ],
    example: {
      title: "The team list shows each member's role",
      priority: 'medium',
      fields: {
        enhancement: 'The team list shows each member’s role next to their name, so team owners no longer open each member to check who can edit events.',
        current: 'The team list shows names only.',
        desired: 'Each member shows their role next to their name.',
        acceptance: 'Given a team with an owner, two editors and a viewer, when the owner opens the team list, then each member shows their role',
      },
    },
  },
  {
    type: 'refactor',
    name: 'Refactor',
    hint: 'Same behaviour, better shape',
    blocks: [
      DEPENDENCIES,
      RELATED,
      {
        heading: '## 🔁 Refactor',
        question: 'What structure are we changing and why (internal reshaping, not new capability unless listed in scope)?',
        input: input('refactor', 'Refactor', 'text', 'What changes shape, and why now'),
      },
      {
        heading: '## 🤝 Behavioral contract',
        question: 'What must stay the same for callers, users, or APIs, and are there any intentional behavior changes (rare — call them out)?',
        input: input('contract', 'Behavioral contract', 'bullets', `What stays the same. ${ONE_PER_LINE}`),
      },
      ...SCOPE,
      {
        heading: '## ✅ Acceptance criteria',
        question: 'How do we know the refactor is complete and safe?',
        input: input('acceptance', 'Acceptance criteria', 'checks', ONE_PER_LINE, 3, false),
      },
      REFERENCES,
    ],
    example: {
      title: 'Team member screens show roles the same way',
      priority: 'low',
      fields: {
        refactor:
          'The team list, the member page and the invitation screen each show roles differently. One way of showing them makes the next role change a single change.',
        contract: 'Anyone still sees "Editor" for a member with the editor role\nThe actions per role stay as they are today',
        acceptance: 'The team list, member page and invitation screen show roles the same way\nNo role shows a different label than before',
      },
    },
  },
  {
    type: 'chore',
    name: 'Chore',
    hint: 'Upkeep that keeps things running',
    blocks: [
      DEPENDENCIES,
      RELATED,
      {
        heading: '## 🧹 Chore',
        question: 'What maintenance or housekeeping is this, and why now (not a user-facing feature or bug fix)?',
        input: input('chore', 'Chore', 'text', 'What needs doing, and why now'),
      },
      ...SCOPE,
      {
        heading: '## 📍 Baseline',
        question: 'What exists before this chore?',
        input: input('baseline', 'Baseline', 'bullets', 'What exists before', 2, false),
      },
      {
        heading: '## 🎯 Target state',
        question: 'What exists after this chore?',
        input: input('target', 'Target state', 'bullets', 'What exists after', 2, false),
      },
      { heading: '## ⚠️ Blast radius & safety', question: 'What could break, who is affected, and how to roll back or verify nothing regressed?' },
      {
        heading: '## ✅ Acceptance criteria',
        question: 'How do we know this chore is done?',
        input: input('acceptance', 'Acceptance criteria', 'checks', ONE_PER_LINE),
      },
      REFERENCES,
    ],
    example: {
      title: 'Renew the Apple push certificate',
      priority: 'high',
      fields: {
        chore: 'The Apple push certificate for the fan app runs out on 1 November. Without it, iPhone users get no event notifications.',
        baseline: 'The certificate expires on 1 November.',
        target: 'A new certificate is in place.',
        acceptance: 'A test notification reaches an iPhone\nThe next expiry date is in the team calendar',
      },
    },
  },
  {
    type: 'spike',
    name: 'Spike',
    hint: 'A question to answer in a time box',
    blocks: [
      {
        heading: '## 🔬 Spike Objective',
        question: 'What question(s) are we trying to answer?',
        input: input('objective', 'Spike objective', 'text', 'What we need to know before we decide'),
      },
      {
        heading: '## ❓ Research Questions',
        question: 'What specific unknowns need to be resolved?',
        input: input('questions', 'Research questions', 'checks', ONE_PER_LINE, 3, false),
      },
      { heading: '## 🎯 Success Criteria', question: 'How do we know the spike is complete?' },
      {
        heading: '## ⏱️ Time Box',
        question: 'How much time is allocated for this investigation?',
        input: input('timebox', 'Time box', 'text', 'For example: half a day', 1),
      },
      { heading: '## 📚 Resources', question: 'What documentation, repos, or contacts might help?' },
      { heading: '## 📝 Expected Output', question: 'What artifacts will be produced?' },
      { heading: '## 🔗 Related Tickets', question: 'Which work is blocked pending this spike?' },
    ],
    example: {
      title: 'Can System One share booking data with us?',
      priority: 'medium',
      fields: {
        objective:
          'Venues keep their bookings in System One. Before we plan table bookings, we need to know whether System One can share those bookings with Rebl, and on what terms.',
        questions: 'Does System One share bookings with other apps?\nWhat does sharing cost per venue?\nWho at the venue has to agree?',
        timebox: 'One day',
      },
    },
  },
];

export function templateInputs(t: Template): TemplateInput[] {
  return t.blocks.flatMap((b) => (b.input ? [b.input] : []));
}

const LIST_MARK = /^\s*(?:[-*+]\s+(?:\[[ xX]\]\s+)?|\d+[.)]\s+)/;

function lines(text: string): string[] {
  return text
    .split('\n')
    .map((l) => l.replace(LIST_MARK, '').trim())
    .filter(Boolean);
}

function section(b: TemplateBlock, body?: string): string {
  return [b.heading, b.question && `> ${b.question}`, body && `\n${body}`].filter(Boolean).join('\n');
}

/** The template's Markdown, with what was entered converted into the sections it fills. */
export function templateMarkdown(t: Template, fields: Record<string, string>): string {
  return t.blocks
    .map((b) => {
      const text = b.input ? (fields[b.input.key] ?? '').trim() : '';
      if (!b.input || !text) return section(b);
      switch (b.input.format) {
        case 'text':
          return section(b, text);
        case 'bullets':
          return section(b, lines(text).map((l) => `- ${l}`).join('\n'));
        case 'checks':
          return section(b, lines(text).map((l) => `- [ ] ${l}`).join('\n'));
        case 'scenarios':
          return [section(b), ...lines(text).map((l) => [`### 👤 ${l}`, ...SCENARIO.map((s) => section(s))].join('\n\n'))].join('\n\n');
      }
    })
    .join('\n\n');
}

export const TYPE_LABEL: Record<IssueType, string> = Object.fromEntries(TEMPLATES.map((t) => [t.type, t.name])) as Record<
  IssueType,
  string
>;

export const STATUS_LABEL: Record<Status, string> = {
  backlog: 'Backlog',
  todo: 'Todo',
  in_progress: 'In progress',
  in_review: 'In review',
  done: 'Done',
};

export const PRIORITY_LABEL: Record<Priority, string> = {
  none: 'No priority',
  urgent: 'Urgent',
  high: 'High',
  medium: 'Medium',
  low: 'Low',
};

// ---------------------------------------------------------------------------
// Squads and their work, as the local Multica holds them.
// ---------------------------------------------------------------------------

export interface Squad {
  name: string;
  leader: string;
}

export const SQUADS: Squad[] = [
  { name: 'Frontend', leader: 'Thomas' },
  { name: 'Backend', leader: 'Joeri' },
  { name: 'Design', leader: 'Guus' },
  { name: 'Quality', leader: 'Sarkout' },
  { name: 'Management', leader: 'Skuddy' },
];

const REBL_MAIN: LaneRef = { org: 'Rebl Records', project: 'Rebl', repo: 'appboypov/rebl', lane: 'main' };
const AMD_ASSETS: LaneRef = { org: 'IVGA Vastgoed', project: 'AMD', repo: 'appboypov/ivga-amd', lane: 'Asset dashboard' };

export const SQUAD_ISSUES: Record<string, Issue[]> = {
  Backend: [
    { id: 'WORK-342', type: 'bug', title: 'REBL-111 Return the created invitation membership', status: 'in_review', assignee: 'Service Builder', lane: REBL_MAIN, release: 'v1.0.1' },
    { id: 'WORK-357', type: 'journey', title: '4. CLI (madspec-command-builder)', status: 'in_review', assignee: 'Command Builder', lane: AMD_ASSETS },
    { id: 'WORK-341', type: 'bug', title: 'REBL-110 Deliver setup mail for racing invitations', status: 'done', assignee: 'Service Builder', lane: REBL_MAIN, release: 'v1.0.1' },
    { id: 'WORK-347', type: 'bug', title: 'REBL-110 adapter: report and guard password setup', status: 'done', assignee: 'API Builder', lane: REBL_MAIN, release: 'v1.0.1' },
    { id: 'WORK-340', type: 'bug', title: 'REBL-109 Reject requests and invitations from a removed membership', status: 'done', assignee: 'Service Builder', lane: REBL_MAIN, release: 'v1.0.1' },
    { id: 'WORK-339', type: 'bug', title: 'REBL-109 contract: membershipId on team members and versioned team requests', status: 'done', assignee: 'Service Builder', lane: REBL_MAIN, release: 'v1.0.1' },
    { id: 'WORK-344', type: 'refactor', title: 'REBL-113 Align Backend contract boundaries', status: 'backlog', assignee: 'Service Builder', lane: REBL_MAIN, release: 'v1.0.1' },
    { id: 'WORK-343', type: 'bug', title: 'REBL-112 Refuse malformed stored team members', status: 'backlog', assignee: 'Service Builder', lane: REBL_MAIN, release: 'v1.0.1' },
    { id: 'WORK-346', type: 'chore', title: 'PR 26 Backend fix tasks: REBL-109 to REBL-112', status: 'backlog', lane: REBL_MAIN, release: 'v1.0.1' },
  ],
  Frontend: [
    { id: 'WORK-360', type: 'journey', title: '7. Renderer view models and screens (madspec-view-builder)', status: 'in_review', assignee: 'View Builder', lane: AMD_ASSETS },
    { id: 'WORK-359', type: 'journey', title: '6. Renderer converters and copy (madspec-view-builder)', status: 'in_review', assignee: 'View Builder', lane: AMD_ASSETS },
    { id: 'WORK-358', type: 'journey', title: '5. Library components (madspec-component-builder)', status: 'in_review', assignee: 'Component Builder', lane: AMD_ASSETS },
  ],
  Quality: [
    { id: 'WORK-345', type: 'story', title: 'REBL-114 Place team flows under Testing', status: 'backlog', assignee: 'E2E Test Builder', lane: REBL_MAIN, release: 'v1.0.1' },
  ],
  Design: [],
  Management: [],
};

export function initials(name: string): string {
  return name
    .split(/\s+/)
    .filter(Boolean)
    .map((w) => w[0])
    .join('')
    .slice(0, 2)
    .toUpperCase();
}

export function laneLabel(ref: LaneRef): string {
  return `${ref.project} · ${ref.lane}`;
}

export function sameLane(a: LaneRef | undefined, b: LaneRef | undefined): boolean {
  return !!a && !!b && a.org === b.org && a.project === b.project && a.repo === b.repo && a.lane === b.lane;
}
