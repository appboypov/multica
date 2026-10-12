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
  fields?: Record<string, string>;
}

export interface Release {
  version: string;
  goal: string;
  issues: Issue[];
}

export interface Lane {
  name: string;
  state: string;
  active: string | null;
  next: string | null;
  releases?: Release[];
}

export interface Repo {
  name: string;
  lanes: Lane[];
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
  releases?: { version: string; goal: string; issues: { id: string; type: string; title: string }[] }[];
}

export function loadOrgs(): Org[] {
  return (rawOrgs as unknown as { name: string; projects: { name: string; repos: { name: string; lanes: RawLane[] }[] }[] }[]).map(
    (o) => ({
      name: o.name,
      projects: o.projects.map((p) => ({
        name: p.name,
        repos: p.repos.map((r) => ({
          name: r.name,
          // The default branch's worklane leads, as in the approved design.
          lanes: [...r.lanes].sort((a, b) => Number(b.name === 'main') - Number(a.name === 'main')).map((l) => ({
            name: l.name,
            state: l.state,
            active: l.active,
            next: l.next,
            releases: l.releases?.map((rel) => ({
              version: rel.version,
              goal: rel.goal,
              issues: rel.issues.map((i) => ({
                id: i.id,
                type: (i.type as IssueType) ?? 'story',
                title: i.title,
                status: STATUS_OF[i.id] ?? 'backlog',
                lane: { org: o.name, project: p.name, repo: r.name, lane: l.name },
                release: rel.version,
              })),
            })),
          })),
        })),
      })),
    }),
  );
}
export type StatusOf = (issue: Issue) => Status;

export function doneCount(rel: Release, statusOf: StatusOf): number {
  return rel.issues.filter((i) => statusOf(i) === 'done').length;
}

/** The first release not fully done is next; earlier ones shipped, later ones planned. */
export function releaseStates(lane: Lane, statusOf: StatusOf): ReleaseState[] {
  const rels = lane.releases ?? [];
  let nextFound = false;
  return rels.map((r) => {
    const shipped = r.issues.length > 0 && doneCount(r, statusOf) === r.issues.length;
    if (shipped && !nextFound) return 'shipped';
    if (!nextFound) {
      nextFound = true;
      return 'next';
    }
    return 'planned';
  });
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

// ---------------------------------------------------------------------------
// Templates: the issue types madspec-issues defines. Sections carry only what
// and why; code and technical design live in the change.
// ---------------------------------------------------------------------------

export interface TemplateSection {
  key: string;
  name: string;
  required: boolean;
  placeholder: string;
  rows: number;
}

export interface Template {
  type: IssueType;
  name: string;
  hint: string;
  sections: TemplateSection[];
}

const notes: TemplateSection = {
  key: 'notes',
  name: 'Notes',
  required: false,
  placeholder: 'Context, links and open questions',
  rows: 2,
};

export const TEMPLATES: Template[] = [
  {
    type: 'epic',
    name: 'Epic',
    hint: 'An outcome made of stories',
    sections: [
      { key: 'outcome', name: 'Outcome', required: true, placeholder: 'What is true for whom once this epic is done', rows: 3 },
      { key: 'done', name: 'Done when', required: true, placeholder: 'One check per line that proves the outcome', rows: 4 },
      notes,
    ],
  },
  {
    type: 'journey',
    name: 'Journey',
    hint: 'One user journey, one change',
    sections: [
      { key: 'journey', name: 'Journey', required: true, placeholder: 'Who does what, step by step', rows: 3 },
      { key: 'criteria', name: 'Acceptance criteria', required: true, placeholder: 'Given ... when ... then ...', rows: 4 },
      notes,
    ],
  },
  {
    type: 'story',
    name: 'Story',
    hint: 'One slice toward an epic',
    sections: [
      { key: 'description', name: 'Description', required: true, placeholder: 'What changes for whom, and why now', rows: 3 },
      { key: 'criteria', name: 'Acceptance criteria', required: true, placeholder: 'Given ... when ... then ...', rows: 4 },
      notes,
    ],
  },
  {
    type: 'bug',
    name: 'Bug',
    hint: 'Something broken to reproduce',
    sections: [
      { key: 'happens', name: 'What happens', required: true, placeholder: 'Steps to see it, and what you see', rows: 3 },
      { key: 'expected', name: 'What should happen', required: true, placeholder: 'Given ... when ... then ...', rows: 3 },
      notes,
    ],
  },
  {
    type: 'enhancement',
    name: 'Enhancement',
    hint: 'Make a capability better',
    sections: [
      { key: 'description', name: 'Description', required: true, placeholder: 'What gets better for whom, and why now', rows: 3 },
      { key: 'criteria', name: 'Acceptance criteria', required: true, placeholder: 'Given ... when ... then ...', rows: 4 },
      notes,
    ],
  },
  {
    type: 'refactor',
    name: 'Refactor',
    hint: 'Same behaviour, better shape',
    sections: [
      { key: 'why', name: 'Why now', required: true, placeholder: 'What this makes easier, and for whom', rows: 3 },
      { key: 'keeps', name: 'Behaviour that stays', required: true, placeholder: 'Given ... when ... then ... still holds', rows: 4 },
      notes,
    ],
  },
  {
    type: 'chore',
    name: 'Chore',
    hint: 'Upkeep that keeps things running',
    sections: [
      { key: 'description', name: 'Description', required: true, placeholder: 'What needs doing, and why now', rows: 3 },
      { key: 'done', name: 'Done when', required: true, placeholder: 'One check per line', rows: 3 },
      notes,
    ],
  },
  {
    type: 'spike',
    name: 'Spike',
    hint: 'A question to answer in a time box',
    sections: [
      { key: 'question', name: 'Question', required: true, placeholder: 'What we need to know before we decide', rows: 3 },
      { key: 'timebox', name: 'Time box', required: true, placeholder: 'For example: half a day', rows: 1 },
      notes,
    ],
  },
];

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
