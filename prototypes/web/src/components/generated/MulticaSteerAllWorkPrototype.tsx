import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent, type ReactNode } from 'react';
import {
  BarChart3,
  Bot,
  BookOpenText,
  Building2,
  ChevronDown,
  ChevronRight,
  CircleDashed,
  CircleMinus,
  CircleUser,
  Ellipsis,
  FolderGit2,
  FolderKanban,
  GitBranch,
  GitCommitHorizontal,
  Inbox,
  ListFilter,
  ListTodo,
  MessageSquare,
  Monitor,
  Plus,
  Search,
  Settings,
  SquarePen,
  Users,
  X,
  Zap,
  Check,
  type LucideIcon,
} from 'lucide-react';
import './MulticaSteerAllWorkPrototype.css';
import {
  type Issue,
  type IssueType,
  type Lane,
  type LaneRef,
  type Org,
  type Priority,
  type Project,
  type Release,
  type ReleaseState,
  type Repo,
  type Status,
  PRIORITY_LABEL,
  SQUADS,
  SQUAD_ISSUES,
  STATUS_LABEL,
  TEMPLATES,
  TYPE_LABEL,
  doneCount,
  initials,
  issueKey,
  laneCount,
  laneLabel,
  loadOrgs,
  plural,
  releaseStates,
  sameLane,
} from './model';
import { Avatar, Button, Chip, Menu, MenuItem, Progress, ReleaseChip, Segmented, StatusIcon, TypeIcon } from './ui';

// ---------------------------------------------------------------------------
// Routing and app state
// ---------------------------------------------------------------------------

type LaneView = 'timeline' | 'board' | 'list';
type SquadView = 'board' | 'list' | 'backlog';

type Route =
  | { name: 'projects' }
  | { name: 'project'; org: string; project: string }
  | { name: 'lane'; ref: LaneRef; view: LaneView }
  | { name: 'squad'; squad: string; view: SquadView }
  | { name: 'other'; label: string };

interface Draft {
  type: IssueType;
  squad?: string;
  lane?: LaneRef;
  release?: string;
  status?: Status;
}

interface AppApi {
  orgs: Org[];
  statusOf: (i: Issue) => Status;
  setStatus: (id: string, s: Status) => void;
  go: (r: Route) => void;
  newIssue: (d?: Partial<Draft>) => void;
  peek: (i: Issue) => void;
  toast: (t: string) => void;
}

function findLane(orgs: Org[], ref: LaneRef): { org: Org; project: Project; repo: Repo; lane: Lane } | null {
  const org = orgs.find((o) => o.name === ref.org);
  const project = org?.projects.find((p) => p.name === ref.project);
  const repo = project?.repos.find((r) => r.name === ref.repo);
  const lane = repo?.lanes.find((l) => l.name === ref.lane);
  return org && project && repo && lane ? { org, project, repo, lane } : null;
}

function allLanes(orgs: Org[]): { ref: LaneRef; lane: Lane }[] {
  return orgs.flatMap((o) =>
    o.projects.flatMap((p) =>
      p.repos.flatMap((r) => r.lanes.map((l) => ({ ref: { org: o.name, project: p.name, repo: r.name, lane: l.name }, lane: l }))),
    ),
  );
}

const DEFAULT_SQUAD = 'Backend';

export const MulticaSteerAllWorkPrototype = () => {
  const [orgs, setOrgs] = useState<Org[]>(loadOrgs);
  const [squadIssues, setSquadIssues] = useState<Record<string, Issue[]>>(SQUAD_ISSUES);
  const [statuses, setStatuses] = useState<Record<string, Status>>({});
  const [route, setRoute] = useState<Route>({ name: 'projects' });
  const [draft, setDraft] = useState<Draft | null>(null);
  const [peeked, setPeeked] = useState<Issue | null>(null);
  const [palette, setPalette] = useState(false);
  const [toastText, setToastText] = useState<string | null>(null);
  const toastTimer = useRef<number | undefined>(undefined);

  const statusOf = useCallback((i: Issue) => statuses[i.id] ?? i.status, [statuses]);

  const toast = useCallback((t: string) => {
    setToastText(t);
    window.clearTimeout(toastTimer.current);
    toastTimer.current = window.setTimeout(() => setToastText(null), 2600);
  }, []);

  // The new-issue dialog takes its defaults from where it was opened.
  const newIssue = useCallback(
    (d: Partial<Draft> = {}) => {
      const ctx: Partial<Draft> =
        route.name === 'lane'
          ? { lane: route.ref }
          : route.name === 'squad'
            ? (() => {
                // A squad's new issue lands where its open work goes: the first open issue's worklane and release.
                const open = (squadIssues[route.squad] ?? []).find((i) => i.release && statusOf(i) !== 'done');
                return { squad: route.squad, lane: open?.lane, release: open?.release };
              })()
            : {};
      setDraft({ type: 'story', ...ctx, ...d });
    },
    [route, squadIssues, statusOf],
  );

  const api: AppApi = {
    orgs,
    statusOf,
    setStatus: (id, s) => setStatuses((m) => ({ ...m, [id]: s })),
    go: (r) => {
      setPeeked(null);
      setRoute(r);
    },
    newIssue,
    peek: setPeeked,
    toast,
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const t = e.target as HTMLElement;
      const typing = t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable;
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setPalette((p) => !p);
      } else if (!typing && !draft && !palette && e.key.toLowerCase() === 'c' && !e.metaKey && !e.ctrlKey) {
        e.preventDefault();
        newIssue();
      } else if (e.key === 'Escape' && peeked && !draft) {
        setPeeked(null);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [draft, palette, peeked, newIssue]);

  const createIssue = (d: Draft, title: string, fields: Record<string, string>, priority: Priority) => {
    const target = d.lane ? findLane(orgs, d.lane) : null;
    const prefix = target?.project.name === 'Rebl' ? 'REBL' : 'WORK';
    const used = [
      ...allLanes(orgs).flatMap((l) => (l.lane.releases ?? []).flatMap((r) => r.issues)),
      ...Object.values(squadIssues).flat(),
    ]
      .filter((i) => i.id.startsWith(prefix + '-'))
      .map((i) => Number(i.id.split('-')[1]));
    const id = `${prefix}-${Math.max(prefix === 'WORK' ? 360 : 0, ...used) + 1}`;
    const issue: Issue = {
      id,
      type: d.type,
      title,
      status: d.status ?? 'backlog',
      priority,
      lane: d.lane,
      release: d.release,
      fields,
      assignee: undefined,
    };
    if (d.lane && d.release) {
      setOrgs((os) =>
        os.map((o) =>
          o.name !== d.lane!.org
            ? o
            : {
                ...o,
                projects: o.projects.map((p) =>
                  p.name !== d.lane!.project
                    ? p
                    : {
                        ...p,
                        repos: p.repos.map((r) =>
                          r.name !== d.lane!.repo
                            ? r
                            : {
                                ...r,
                                lanes: r.lanes.map((l) =>
                                  l.name !== d.lane!.lane
                                    ? l
                                    : {
                                        ...l,
                                        releases: (l.releases ?? []).map((rel) =>
                                          rel.version === d.release ? { ...rel, issues: [...rel.issues, issue] } : rel,
                                        ),
                                      },
                                ),
                              },
                        ),
                      },
                ),
              },
        ),
      );
    }
    if (d.squad) setSquadIssues((m) => ({ ...m, [d.squad!]: [...(m[d.squad!] ?? []), issue] }));
    setDraft(null);
    toast(`${id} created${d.release ? ` in ${d.release}` : ''}${d.squad ? ` for ${d.squad}` : ''}`);
  };

  const updateLanes = (ref: { org: string; project: string; repo: string }, fn: (lanes: Lane[]) => Lane[]) =>
    setOrgs((os) =>
      os.map((o) =>
        o.name !== ref.org
          ? o
          : {
              ...o,
              projects: o.projects.map((p) =>
                p.name !== ref.project ? p : { ...p, repos: p.repos.map((r) => (r.name !== ref.repo ? r : { ...r, lanes: fn(r.lanes) })) },
              ),
            },
      ),
    );

  const addProject = (org: string, name: string) =>
    setOrgs((os) => os.map((o) => (o.name === org ? { ...o, projects: [...o.projects, { name, repos: [] }] } : o)));

  const addRepo = (org: string, project: string, name: string) =>
    setOrgs((os) =>
      os.map((o) =>
        o.name !== org ? o : { ...o, projects: o.projects.map((p) => (p.name === project ? { ...p, repos: [...p.repos, { name, lanes: [] }] } : p)) },
      ),
    );

  let page: ReactNode;
  if (route.name === 'projects') page = <ProjectsPage api={api} onAddProject={addProject} />;
  else if (route.name === 'project')
    page = (
      <ProjectPage
        api={api}
        orgName={route.org}
        projectName={route.project}
        onAddRepo={(n) => addRepo(route.org, route.project, n)}
        onAddLane={(repo, n) =>
          updateLanes({ org: route.org, project: route.project, repo }, (ls) => [...ls, { name: n, state: 'Free', active: null, next: null, releases: [] }])
        }
      />
    );
  else if (route.name === 'lane')
    page = (
      <LanePage
        api={api}
        refLane={route.ref}
        view={route.view}
        onPlanRelease={(goal) =>
          updateLanes(route.ref, (ls) =>
            ls.map((l) => {
              if (l.name !== route.ref.lane) return l;
              const last = l.releases?.[l.releases.length - 1]?.version ?? 'v0.9.0';
              const [maj, min] = last.replace('v', '').split('.').map(Number);
              return { ...l, releases: [...(l.releases ?? []), { version: `v${maj}.${min + 1}.0`, goal, issues: [] }] };
            }),
          )
        }
      />
    );
  else if (route.name === 'squad') page = <SquadPage api={api} squad={route.squad} view={route.view} issues={squadIssues[route.squad] ?? []} />;
  else page = <OtherPage label={route.label} />;

  return (
    <div className="mc-root flex h-screen w-full overflow-hidden text-[14px]">
      <Sidebar api={api} route={route} onSearch={() => setPalette(true)} />
      <main className="flex min-w-0 flex-1 flex-col overflow-hidden border-l" style={{ background: 'var(--page-canvas)', borderColor: 'var(--surface-border)' }}>
        {page}
      </main>
      {peeked && <IssuePeek api={api} issue={peeked} onClose={() => setPeeked(null)} />}
      {draft && <NewIssueDialog api={api} draft={draft} onClose={() => setDraft(null)} onCreate={createIssue} />}
      {palette && <Palette api={api} squadIssues={squadIssues} onClose={() => setPalette(false)} />}
      {toastText && (
        <div
          role="status"
          className="mc-pop fixed bottom-6 left-1/2 z-[80] flex -translate-x-1/2 items-center gap-2 rounded-[8px] px-3 py-2 text-[13px]"
          style={{ background: 'var(--primary)', color: 'var(--primary-foreground)', boxShadow: 'var(--floating-shadow)' }}
        >
          <Check size={14} />
          {toastText}
        </div>
      )}
    </div>
  );
};

// ---------------------------------------------------------------------------
// Sidebar: Multica's real navigation
// ---------------------------------------------------------------------------

function Sidebar({ api, route, onSearch }: { api: AppApi; route: Route; onSearch: () => void }) {
  const active =
    route.name === 'projects' || route.name === 'project' || route.name === 'lane'
      ? 'Projects'
      : route.name === 'squad'
        ? 'Squads'
        : route.label;
  const item = (label: string, Icon: LucideIcon, onClick: () => void, trailing?: ReactNode) => (
    <button
      key={label}
      onClick={onClick}
      aria-current={active === label ? 'page' : undefined}
      className="flex h-[30px] w-full items-center gap-2 rounded-[6px] px-2 text-left text-[14px] transition-colors hover:bg-[var(--sidebar-accent)]"
      style={active === label ? { background: 'var(--sidebar-accent)', color: 'var(--foreground)', fontWeight: 500 } : { color: 'var(--muted-foreground)' }}
    >
      <Icon size={16} />
      <span className="flex-1 truncate">{label}</span>
      {trailing}
    </button>
  );
  const other = (label: string) => () => api.go({ name: 'other', label });
  const keycap = (k: string) => (
    <span className="rounded-[3px] border px-1 text-[11px] leading-4" style={{ borderColor: 'var(--surface-border)', background: 'var(--surface)', color: 'var(--muted-foreground)' }}>
      {k}
    </span>
  );
  const section = (label: string) => (
    <div className="px-2 pt-3 pb-1 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
      {label}
    </div>
  );
  return (
    <aside className="flex w-[256px] shrink-0 flex-col px-2 py-2" style={{ background: 'var(--app-shell)' }} aria-label="Sidebar">
      <button className="mb-1 flex h-9 items-center gap-2 rounded-[6px] px-2 hover:bg-[var(--sidebar-accent)]">
        <span className="flex size-5 items-center justify-center rounded-[4px] text-[11px] font-semibold text-white" style={{ background: 'var(--primary)' }}>
          D
        </span>
        <span className="flex-1 text-left text-[14px] font-medium">De App Specialist</span>
        <ChevronDown size={14} style={{ color: 'var(--muted-foreground)' }} />
      </button>
      {item('Search...', Search, onSearch, keycap('⌘K'))}
      {item('New Issue', SquarePen, () => api.newIssue(), keycap('C'))}
      <div className="h-2" />
      {item('Inbox', Inbox, other('Inbox'), <span className="text-[12px]">96</span>)}
      {item('My Issues', CircleUser, other('My Issues'))}
      {item('Chat', MessageSquare, other('Chat'))}
      {section('Work')}
      {item('Issues', ListTodo, other('Issues'))}
      {item('Projects', FolderKanban, () => api.go({ name: 'projects' }))}
      {item('Autopilot', Zap, other('Autopilot'))}
      {section('AI Team')}
      {item('Agents', Bot, other('Agents'))}
      {item('Squads', Users, () => api.go({ name: 'squad', squad: DEFAULT_SQUAD, view: 'board' }))}
      {item('Skills', BookOpenText, other('Skills'))}
      {item('Runtimes', Monitor, other('Runtimes'))}
      <div className="flex-1" />
      {item('Analytics', BarChart3, other('Analytics'))}
      {item('Settings', Settings, other('Settings'))}
    </aside>
  );
}

// ---------------------------------------------------------------------------
// Page header
// ---------------------------------------------------------------------------

function PageHeader({ icon: Icon, crumbs, count, actions }: { icon: LucideIcon; crumbs: { label: string; onClick?: () => void; menu?: ReactNode }[]; count?: number; actions?: ReactNode }) {
  return (
    <header className="flex h-12 shrink-0 items-center gap-2 border-b px-4" style={{ borderColor: 'var(--border)' }}>
      <Icon size={16} style={{ color: 'var(--muted-foreground)' }} />
      <nav className="flex min-w-0 items-center gap-1.5 text-[14px] font-medium" aria-label="Breadcrumb">
        {crumbs.map((c, i) => (
          <span key={i} className="flex min-w-0 items-center gap-1.5">
            {i > 0 && <ChevronRight size={14} style={{ color: 'var(--muted-foreground)' }} />}
            {c.menu ??
              (c.onClick ? (
                <button onClick={c.onClick} className="truncate hover:underline">
                  {c.label}
                </button>
              ) : (
                <span className="truncate">{c.label}</span>
              ))}
          </span>
        ))}
      </nav>
      {count !== undefined && (
        <span className="text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
          {count}
        </span>
      )}
      <div className="flex-1" />
      {actions}
    </header>
  );
}

function FilterMenu({ options, value, onChange }: { options: { value: string; label: string; icon?: ReactNode }[]; value: string[]; onChange: (v: string[]) => void }) {
  return (
    <Menu
      align="right"
      trigger={(open, toggle) => (
        <Button variant="ghost" icon={<ListFilter size={15} />} onClick={toggle}>
          Filter{value.length ? ` · ${value.length}` : ''}
        </Button>
      )}
    >
      {() => (
        <>
          {options.map((o) => (
            <MenuItem
              key={o.value}
              icon={o.icon}
              selected={value.includes(o.value)}
              onClick={() => onChange(value.includes(o.value) ? value.filter((v) => v !== o.value) : [...value, o.value])}
            >
              {o.label}
            </MenuItem>
          ))}
          {value.length > 0 && (
            <>
              <div className="my-1 h-px" style={{ background: 'var(--border)' }} />
              <MenuItem onClick={() => onChange([])}>Clear filter</MenuItem>
            </>
          )}
        </>
      )}
    </Menu>
  );
}

const TYPE_FILTER = TEMPLATES.map((t) => ({ value: t.type, label: t.name, icon: <TypeIcon type={t.type} /> }));

// ---------------------------------------------------------------------------
// Release chain: last shipped, next with progress, planned
// ---------------------------------------------------------------------------

function ReleaseChain({ api, lane, refLane, max = 2 }: { api: AppApi; lane: Lane; refLane: LaneRef; max?: number }) {
  const rels = lane.releases ?? [];
  if (!rels.length)
    return (
      <span className="text-[12px]" style={{ color: 'var(--faint-foreground)' }}>
        No releases planned
      </span>
    );
  const states = releaseStates(lane, api.statusOf);
  const shipped = rels.filter((_, i) => states[i] === 'shipped');
  const nextIdx = states.indexOf('next');
  const planned = rels.filter((_, i) => states[i] === 'planned').slice(0, max - 1);
  const open = () => api.go({ name: 'lane', ref: refLane, view: 'timeline' });
  const line = <span className="h-px w-3 shrink-0" style={{ background: 'var(--surface-border)' }} />;
  return (
    <div className="flex min-w-0 items-center gap-1">
      {shipped.length ? (
        <ReleaseChip version={shipped[shipped.length - 1].version} state="shipped" onClick={open} />
      ) : (
        <Chip icon={<CircleMinus size={12} />}>nothing shipped</Chip>
      )}
      {nextIdx >= 0 && (
        <>
          {line}
          <ReleaseChip version={rels[nextIdx].version} state="next" onClick={open} />
          <span className="text-[12px] tabular-nums" style={{ color: 'var(--muted-foreground)' }}>
            {doneCount(rels[nextIdx], api.statusOf)}/{rels[nextIdx].issues.length}
          </span>
        </>
      )}
      {planned.map((r) => (
        <span key={r.version} className="flex items-center gap-1">
          {line}
          <ReleaseChip version={r.version} state="planned" onClick={open} />
        </span>
      ))}
    </div>
  );
}

function LaneRow({ api, lane, refLane, indent }: { api: AppApi; lane: Lane; refLane: LaneRef; indent: number }) {
  const active = issueKey(lane.active);
  const next = issueKey(lane.next);
  const now = active ?? next;
  return (
    <div
      role="link"
      tabIndex={0}
      onClick={() => api.go({ name: 'lane', ref: refLane, view: 'timeline' })}
      onKeyDown={(e) => e.key === 'Enter' && api.go({ name: 'lane', ref: refLane, view: 'timeline' })}
      className="group grid cursor-pointer items-center gap-4 border-t py-2 pr-4 hover:bg-[var(--surface-hover)]"
      style={{ gridTemplateColumns: `minmax(0,400px) 1fr 16px`, paddingLeft: indent, borderColor: 'var(--border)' }}
    >
      <div className="min-w-0">
        <div className="flex items-center gap-1.5">
          <GitBranch size={14} style={{ color: 'var(--muted-foreground)' }} />
          <span className="truncate font-medium">{lane.name}</span>
          {lane.state === 'Busy' && (
            <span className="rounded-[4px] px-1.5 text-[11px]" style={{ background: 'color-mix(in oklab, var(--warning) 14%, transparent)', color: '#8a6700' }}>
              Busy
            </span>
          )}
        </div>
        <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
          {now ? (
            <>
              <StatusIcon status={active ? 'in_progress' : 'todo'} size={13} />
              <span className="truncate">
                {active ? '' : 'Next: '}
                {now.id} {now.title}
              </span>
            </>
          ) : (
            <span style={{ color: 'var(--faint-foreground)' }}>Nothing queued</span>
          )}
        </div>
      </div>
      <ReleaseChain api={api} lane={lane} refLane={refLane} />
      <ChevronRight size={16} style={{ color: 'var(--faint-foreground)' }} />
    </div>
  );
}

function Disclosure({ open }: { open: boolean }) {
  return open ? <ChevronDown size={14} style={{ color: 'var(--muted-foreground)' }} /> : <ChevronRight size={14} style={{ color: 'var(--muted-foreground)' }} />;
}

function ProjectTile({ name, size = 24 }: { name: string; size?: number }) {
  return (
    <span
      className="flex shrink-0 items-center justify-center rounded-[6px] border font-semibold"
      style={{ width: size, height: size, fontSize: size > 30 ? 16 : 13, background: 'var(--surface)', borderColor: 'var(--surface-border)' }}
    >
      {name.trim().charAt(0).toUpperCase()}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Screen 1: all projects, grouped by organisation
// ---------------------------------------------------------------------------

function ProjectsPage({ api, onAddProject }: { api: AppApi; onAddProject: (org: string, name: string) => void }) {
  const [openOrgs, setOpenOrgs] = useState<Set<string>>(new Set(['Mijn Shine', 'Rebl Records']));
  const [openProjects, setOpenProjects] = useState<Set<string>>(new Set(['Rebl Records/Rebl']));
  const [filter, setFilter] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);
  const toggle = (set: Set<string>, setter: (s: Set<string>) => void, key: string) => {
    const n = new Set(set);
    if (n.has(key)) n.delete(key);
    else n.add(key);
    setter(n);
  };
  const withLanes = filter.includes('lanes');
  const total = api.orgs.reduce((n, o) => n + o.projects.length, 0);
  const visibleOrgs = api.orgs
    .map((o) => ({ ...o, projects: withLanes ? o.projects.filter((p) => laneCount(p) > 0) : o.projects }))
    .filter((o) => o.projects.length);

  return (
    <>
      <PageHeader
        icon={FolderKanban}
        crumbs={[{ label: 'Projects' }]}
        count={total}
        actions={
          <>
            <FilterMenu options={[{ value: 'lanes', label: 'Only projects with worklanes' }]} value={filter} onChange={setFilter} />
            <Button variant="primary" icon={<Plus size={15} />} onClick={() => setCreating(true)}>
              New project
            </Button>
          </>
        }
      />
      <div className="flex-1 overflow-auto px-6 pb-10">
        <div className="sticky top-0 z-10 grid gap-4 border-b py-3 pr-4 pl-4 text-[12px]" style={{ gridTemplateColumns: 'minmax(0,400px) 1fr', background: 'var(--page-canvas)', borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
          <span>Name</span>
          <span>Last · Next · Planned releases</span>
        </div>
        {visibleOrgs.map((o) => {
          const orgOpen = openOrgs.has(o.name);
          const lanes = o.projects.reduce((n, p) => n + laneCount(p), 0);
          return (
            <section key={o.name}>
              <button
                onClick={() => toggle(openOrgs, setOpenOrgs, o.name)}
                aria-expanded={orgOpen}
                className="flex h-[39px] w-full items-center gap-2 px-4 text-left hover:bg-[var(--surface-hover)]"
              >
                <Disclosure open={orgOpen} />
                <Building2 size={14} style={{ color: 'var(--muted-foreground)' }} />
                <span className="text-[13px] font-semibold">{o.name}</span>
                <span className="text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
                  {plural(o.projects.length, 'project')}
                  {lanes ? ` · ${plural(lanes, 'worklane')}` : ''}
                </span>
              </button>
              {orgOpen &&
                o.projects.map((p) => {
                  const key = `${o.name}/${p.name}`;
                  const pOpen = openProjects.has(key);
                  const lc = laneCount(p);
                  return (
                    <div key={key} style={{ background: 'var(--surface)' }}>
                      <div className="flex h-12 items-center gap-3 border-t px-4" style={{ borderColor: 'var(--border)' }}>
                        <button
                          aria-label={pOpen ? `Collapse ${p.name}` : `Expand ${p.name}`}
                          aria-expanded={pOpen}
                          onClick={() => toggle(openProjects, setOpenProjects, key)}
                          className="rounded-[4px] p-0.5 hover:bg-[var(--surface-hover)]"
                        >
                          <Disclosure open={pOpen} />
                        </button>
                        <ProjectTile name={p.name} />
                        <button onClick={() => api.go({ name: 'project', org: o.name, project: p.name })} className="truncate text-[15px] font-medium hover:underline">
                          {p.name}
                        </button>
                        <div className="flex-1" />
                        <span className="text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
                          {p.repos.length ? `${plural(p.repos.length, 'repository', 'repositories')} · ${plural(lc, 'worklane')}` : 'No repositories'}
                        </span>
                        <Avatar text="BR" title="Lead: Brian" />
                      </div>
                      {pOpen &&
                        p.repos.map((r) => (
                          <div key={r.name}>
                            <div className="flex h-9 items-center gap-2 border-t pr-4 pl-10" style={{ borderColor: 'var(--border)', background: 'var(--page-canvas)' }}>
                              <FolderGit2 size={14} style={{ color: 'var(--muted-foreground)' }} />
                              <span className="mc-mono text-[13px] font-medium">{r.name}</span>
                              <span className="mc-mono flex items-center gap-1 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                                <GitCommitHorizontal size={12} />
                                main
                              </span>
                              <div className="flex-1" />
                              <span className="text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                                {r.lanes.length ? plural(r.lanes.length, 'worklane') : 'No worklanes yet'}
                              </span>
                            </div>
                            {r.lanes.map((l) => (
                              <LaneRow key={l.name} api={api} lane={l} refLane={{ org: o.name, project: p.name, repo: r.name, lane: l.name }} indent={64} />
                            ))}
                          </div>
                        ))}
                      {pOpen && !p.repos.length && (
                        <div className="border-t py-3 pl-14 text-[13px]" style={{ borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
                          No repositories yet.{' '}
                          <button className="underline" onClick={() => api.go({ name: 'project', org: o.name, project: p.name })}>
                            Open the project
                          </button>{' '}
                          to add one.
                        </div>
                      )}
                    </div>
                  );
                })}
            </section>
          );
        })}
      </div>
      {creating && (
        <NewProjectDialog
          orgs={api.orgs.map((o) => o.name)}
          onClose={() => setCreating(false)}
          onCreate={(org, name) => {
            onAddProject(org, name);
            setOpenOrgs((s) => new Set(s).add(org));
            setCreating(false);
            api.toast(`${name} added to ${org}`);
          }}
        />
      )}
    </>
  );
}

function NewProjectDialog({ orgs, onClose, onCreate }: { orgs: string[]; onClose: () => void; onCreate: (org: string, name: string) => void }) {
  const [name, setName] = useState('');
  const [org, setOrg] = useState('De App Specialist');
  return (
    <Overlay onClose={onClose}>
      <form
        className="mc-pop w-[440px] rounded-[12px] border"
        style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)', boxShadow: 'var(--floating-shadow)' }}
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim()) onCreate(org, name.trim());
        }}
      >
        <div className="flex items-center justify-between border-b px-4 py-3" style={{ borderColor: 'var(--border)' }}>
          <h2 className="text-[16px] font-semibold">New project</h2>
          <IconButton label="Close" onClick={onClose} icon={<X size={16} />} />
        </div>
        <div className="space-y-3 p-4">
          <input
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Project name"
            className="w-full bg-transparent text-[18px] font-medium outline-none placeholder:text-[var(--faint-foreground)]"
          />
          <Menu
            width={260}
            trigger={(_, toggle) => (
              <Chip icon={<Building2 size={13} />} onClick={toggle}>
                {org}
              </Chip>
            )}
          >
            {(close) =>
              orgs.map((o) => (
                <MenuItem key={o} selected={o === org} onClick={() => (setOrg(o), close())}>
                  {o}
                </MenuItem>
              ))
            }
          </Menu>
        </div>
        <div className="flex justify-end gap-2 border-t px-4 py-3" style={{ borderColor: 'var(--border)' }}>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="brand" type="submit" disabled={!name.trim()} icon={<Check size={15} />}>
            Create project
          </Button>
        </div>
      </form>
    </Overlay>
  );
}

// ---------------------------------------------------------------------------
// Screen 2: one project, its repositories and worklanes
// ---------------------------------------------------------------------------

function InlineAdd({ placeholder, onAdd, onCancel }: { placeholder: string; onAdd: (v: string) => void; onCancel: () => void }) {
  const [v, setV] = useState('');
  return (
    <form
      className="flex items-center gap-2 border-t px-4 py-2"
      style={{ borderColor: 'var(--border)' }}
      onSubmit={(e) => {
        e.preventDefault();
        if (v.trim()) onAdd(v.trim());
      }}
    >
      <input
        autoFocus
        value={v}
        onChange={(e) => setV(e.target.value)}
        onKeyDown={(e) => e.key === 'Escape' && onCancel()}
        placeholder={placeholder}
        className="h-8 flex-1 rounded-[6px] border px-2 text-[13px] outline-none focus:border-[var(--ring)]"
        style={{ borderColor: 'var(--input)' }}
      />
      <Button variant="ghost" onClick={onCancel}>
        Cancel
      </Button>
      <Button variant="primary" type="submit" disabled={!v.trim()}>
        Add
      </Button>
    </form>
  );
}

function ProjectPage({
  api,
  orgName,
  projectName,
  onAddRepo,
  onAddLane,
}: {
  api: AppApi;
  orgName: string;
  projectName: string;
  onAddRepo: (name: string) => void;
  onAddLane: (repo: string, name: string) => void;
}) {
  const project = api.orgs.find((o) => o.name === orgName)?.projects.find((p) => p.name === projectName);
  const [addingRepo, setAddingRepo] = useState(false);
  const [addingLane, setAddingLane] = useState<string | null>(null);
  if (!project) return <OtherPage label="Project not found" />;
  const lc = laneCount(project);
  const meta = (Icon: LucideIcon, text: string) => (
    <span className="flex items-center gap-1.5">
      <Icon size={14} />
      {text}
    </span>
  );
  return (
    <>
      <PageHeader
        icon={FolderKanban}
        crumbs={[{ label: 'Projects', onClick: () => api.go({ name: 'projects' }) }, { label: project.name }]}
        actions={
          <Button variant="primary" icon={<Plus size={15} />} onClick={() => api.newIssue(project.repos[0]?.lanes[0] ? { lane: { org: orgName, project: projectName, repo: project.repos[0].name, lane: project.repos[0].lanes[0].name } } : {})}>
            New issue
          </Button>
        }
      />
      <div className="flex-1 overflow-auto px-6 py-6">
        <div className="flex items-start gap-4">
          <ProjectTile name={project.name} size={48} />
          <div className="min-w-0 flex-1">
            <h1 className="text-[20px] font-semibold">{project.name}</h1>
            <div className="mt-2 flex flex-wrap items-center gap-4 text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
              <span className="flex items-center gap-1.5">
                <Avatar text="BR" size={18} />
                Brian
              </span>
              {meta(FolderGit2, plural(project.repos.length, 'repository', 'repositories'))}
              {meta(GitBranch, plural(lc, 'worklane'))}
              {meta(Building2, orgName)}
            </div>
          </div>
          <Button icon={<Plus size={15} />} onClick={() => setAddingRepo(true)}>
            Repository
          </Button>
        </div>
        <div className="mt-6 space-y-4">
          {addingRepo && (
            <div className="rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)' }}>
              <InlineAdd
                placeholder="owner/repository"
                onCancel={() => setAddingRepo(false)}
                onAdd={(n) => {
                  onAddRepo(n);
                  setAddingRepo(false);
                  api.toast(`${n} added to ${project.name}`);
                }}
              />
            </div>
          )}
          {!project.repos.length && !addingRepo && (
            <div className="rounded-[12px] border border-dashed px-6 py-10 text-center" style={{ borderColor: 'var(--surface-border)', color: 'var(--muted-foreground)' }}>
              <FolderGit2 size={20} className="mx-auto mb-2" />
              <p className="text-[14px]">No repositories yet</p>
              <div className="mt-3">
                <Button icon={<Plus size={15} />} onClick={() => setAddingRepo(true)}>
                  Repository
                </Button>
              </div>
            </div>
          )}
          {project.repos.map((r) => (
            <section key={r.name} className="overflow-hidden rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)', boxShadow: 'var(--surface-shadow)' }}>
              <div className="flex h-12 items-center gap-2 px-4">
                <FolderGit2 size={15} style={{ color: 'var(--muted-foreground)' }} />
                <span className="mc-mono text-[14px] font-medium">{r.name}</span>
                <span className="mc-mono flex items-center gap-1 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                  <GitCommitHorizontal size={12} />
                  main
                </span>
                <span className="text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                  {plural(r.lanes.length, 'worklane')}
                </span>
                <div className="flex-1" />
                <Button variant="ghost" icon={<Plus size={15} />} onClick={() => setAddingLane(r.name)}>
                  Add worklane
                </Button>
              </div>
              {r.lanes.map((l) => (
                <LaneRow key={l.name} api={api} lane={l} refLane={{ org: orgName, project: projectName, repo: r.name, lane: l.name }} indent={16} />
              ))}
              {!r.lanes.length && addingLane !== r.name && (
                <div className="border-t px-4 py-3 text-[13px]" style={{ borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
                  No worklanes yet
                </div>
              )}
              {addingLane === r.name && (
                <InlineAdd
                  placeholder="Worklane name"
                  onCancel={() => setAddingLane(null)}
                  onAdd={(n) => {
                    onAddLane(r.name, n);
                    setAddingLane(null);
                    api.toast(`Worklane ${n} added to ${r.name}`);
                  }}
                />
              )}
            </section>
          ))}
        </div>
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// Screen 3: a worklane's release timeline
// ---------------------------------------------------------------------------

function IssueRow({ api, issue }: { api: AppApi; issue: Issue }) {
  return (
    <button
      onClick={() => api.peek(issue)}
      className="flex h-8 w-full items-center gap-2 px-4 text-left hover:bg-[var(--surface-hover)]"
    >
      <StatusIcon status={api.statusOf(issue)} />
      <TypeIcon type={issue.type} />
      <span className="mc-mono w-[68px] shrink-0 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
        {issue.id}
      </span>
      <span className="min-w-0 flex-1 truncate text-[14px]">{issue.title}</span>
    </button>
  );
}

const STATE_LABEL: Record<ReleaseState, string> = { shipped: 'Shipped', next: 'Next release', planned: 'Planned' };

function LanePage({ api, refLane, view, onPlanRelease }: { api: AppApi; refLane: LaneRef; view: LaneView; onPlanRelease: (goal: string) => void }) {
  const found = findLane(api.orgs, refLane);
  const [types, setTypes] = useState<string[]>([]);
  const [planning, setPlanning] = useState(false);
  if (!found) return <OtherPage label="Worklane not found" />;
  const { lane } = found;
  const rels = lane.releases ?? [];
  const states = releaseStates(lane, api.statusOf);
  const filt = (is: Issue[]) => (types.length ? is.filter((i) => types.includes(i.type)) : is);
  const shippedAny = states.includes('shipped');
  const nxt = issueKey(lane.next ?? lane.active);
  const setView = (v: LaneView) => api.go({ name: 'lane', ref: refLane, view: v });

  return (
    <>
      <PageHeader
        icon={GitBranch}
        crumbs={[
          { label: 'Projects', onClick: () => api.go({ name: 'projects' }) },
          { label: refLane.project, onClick: () => api.go({ name: 'project', org: refLane.org, project: refLane.project }) },
          { label: refLane.repo, onClick: () => api.go({ name: 'project', org: refLane.org, project: refLane.project }) },
          { label: refLane.lane },
        ]}
        actions={
          <>
            <FilterMenu options={TYPE_FILTER} value={types} onChange={setTypes} />
            <Button variant="primary" icon={<Plus size={15} />} onClick={() => api.newIssue({ lane: refLane, release: rels[states.indexOf('next')]?.version })}>
              New issue
            </Button>
          </>
        }
      />
      <div className="flex h-14 shrink-0 items-center gap-3 border-b px-6" style={{ borderColor: 'var(--border)' }}>
        <Chip icon={<FolderGit2 size={13} />}>
          <span className="mc-mono">{refLane.repo}</span>
        </Chip>
        <span className="truncate text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
          {lane.state} · {shippedAny ? `last shipped ${rels[states.lastIndexOf('shipped')].version}` : 'nothing shipped yet'}
          {nxt ? ` · ${lane.active ? 'now' : 'next'}: ${nxt.id} ${nxt.title}` : ''}
        </span>
        <div className="flex-1" />
        <Segmented<LaneView>
          value={view}
          onChange={setView}
          options={[
            { value: 'timeline', label: 'Timeline' },
            { value: 'board', label: 'Board' },
            { value: 'list', label: 'List' },
          ]}
        />
      </div>
      {view === 'timeline' && (
        <div className="flex-1 overflow-auto px-6 pt-6 pb-10">
          <div className="flex items-start gap-4">
            {rels.map((rel, i) => (
              <ReleaseColumn key={rel.version} api={api} rel={rel} state={states[i]} issues={filt(rel.issues)} refLane={refLane} />
            ))}
            <div className="w-[300px] shrink-0">
              <div className="relative mb-5 h-3">
                <span className="absolute top-1/2 left-0 size-3 -translate-y-1/2 rounded-full border border-dashed" style={{ borderColor: 'var(--ring)', background: 'var(--page-canvas)' }} />
              </div>
              {planning ? (
                <div className="overflow-hidden rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)' }}>
                  <div className="px-4 pt-3 text-[13px] font-medium">Plan a release</div>
                  <InlineAdd
                    placeholder="Release goal"
                    onCancel={() => setPlanning(false)}
                    onAdd={(g) => {
                      onPlanRelease(g);
                      setPlanning(false);
                      api.toast(`Release planned: ${g}`);
                    }}
                  />
                </div>
              ) : (
                <button
                  onClick={() => setPlanning(true)}
                  className="flex h-24 w-full flex-col items-center justify-center gap-1 rounded-[12px] border border-dashed text-[13px] hover:bg-[var(--surface-hover)]"
                  style={{ borderColor: 'var(--surface-border)', color: 'var(--muted-foreground)' }}
                >
                  <Plus size={16} />
                  Plan release
                </button>
              )}
            </div>
          </div>
        </div>
      )}
      {view === 'board' && (
        <StatusBoard
          api={api}
          columns={['backlog', 'todo', 'in_progress', 'in_review', 'done']}
          issues={filt(rels.flatMap((r) => r.issues))}
          onAdd={(s) => api.newIssue({ lane: refLane, release: rels[states.indexOf('next')]?.version, status: s })}
          showLane={false}
        />
      )}
      {view === 'list' && (
        <div className="flex-1 overflow-auto px-6 py-4">
          {rels.map((rel, i) => (
            <section key={rel.version} className="mb-4 overflow-hidden rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)' }}>
              <div className="flex h-11 items-center gap-2 px-4">
                <ReleaseChip version={rel.version} state={states[i]} />
                <span className="font-medium">{rel.goal}</span>
                <div className="flex-1" />
                <span className="text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                  {doneCount(rel, api.statusOf)} of {rel.issues.length} done
                </span>
              </div>
              {filt(rel.issues).map((is) => (
                <div key={is.id} className="border-t" style={{ borderColor: 'var(--border)' }}>
                  <IssueRow api={api} issue={is} />
                </div>
              ))}
            </section>
          ))}
          {!rels.length && <EmptyNote text="No releases planned for this worklane yet." />}
        </div>
      )}
    </>
  );
}

function ReleaseColumn({ api, rel, state, issues, refLane }: { api: AppApi; rel: Release; state: ReleaseState; issues: Issue[]; refLane: LaneRef }) {
  const done = doneCount(rel, api.statusOf);
  const next = state === 'next';
  return (
    <div className="w-[544px] shrink-0">
      <div className="relative mb-5 h-3">
        <span className="absolute top-1/2 right-[-16px] left-3 h-px -translate-y-1/2" style={{ background: 'var(--surface-border)' }} />
        <span
          className="absolute top-1/2 left-0 size-3 -translate-y-1/2 rounded-full border-[1.5px]"
          style={{
            borderColor: next ? 'var(--brand)' : state === 'shipped' ? 'var(--info)' : 'var(--ring)',
            background: state === 'shipped' ? 'var(--info)' : 'var(--page-canvas)',
          }}
        />
      </div>
      <section className="overflow-hidden rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: next ? 'color-mix(in oklab, var(--brand) 35%, var(--surface-border))' : 'var(--surface-border)', boxShadow: 'var(--surface-shadow)' }}>
        <div className="px-4 pt-3 pb-3">
          <div className="flex items-center gap-2">
            <ReleaseChip version={rel.version} state={state} />
            <span className="text-[12px]" style={{ color: next ? 'var(--brand)' : 'var(--muted-foreground)' }}>
              {STATE_LABEL[state]}
            </span>
            <div className="flex-1" />
            <Menu
              align="right"
              width={200}
              trigger={(_, toggle) => <IconButton label="Release actions" icon={<Ellipsis size={16} />} onClick={toggle} />}
            >
              {(close) => (
                <MenuItem
                  icon={<Plus size={14} />}
                  onClick={() => {
                    close();
                    api.newIssue({ lane: refLane, release: rel.version });
                  }}
                >
                  Add issue to release
                </MenuItem>
              )}
            </Menu>
          </div>
          <h3 className="mt-2 text-[16px] font-semibold">{rel.goal}</h3>
          <div className="mt-3 flex items-center gap-3">
            <Progress done={done} total={rel.issues.length} />
            <span className="shrink-0 text-[12px] tabular-nums" style={{ color: 'var(--muted-foreground)' }}>
              {done} of {rel.issues.length} done
            </span>
          </div>
        </div>
        <div className="max-h-[560px] overflow-auto border-t py-1" style={{ borderColor: 'var(--border)' }}>
          {issues.map((is) => (
            <IssueRow key={is.id} api={api} issue={is} />
          ))}
          {!issues.length && <div className="px-4 py-3 text-[13px]" style={{ color: 'var(--muted-foreground)' }}>No issues</div>}
        </div>
        <button
          onClick={() => api.newIssue({ lane: refLane, release: rel.version })}
          className="flex h-9 w-full items-center gap-2 border-t px-4 text-left text-[13px] hover:bg-[var(--surface-hover)]"
          style={{ borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}
        >
          <Plus size={14} />
          Add issue to release
        </button>
      </section>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Board shared by the worklane board view and the squad board
// ---------------------------------------------------------------------------

function StatusBoard({ api, columns, issues, onAdd, showLane }: { api: AppApi; columns: Status[]; issues: Issue[]; onAdd: (s: Status) => void; showLane: boolean }) {
  const [over, setOver] = useState<Status | null>(null);
  const onDrop = (s: Status) => (e: DragEvent) => {
    e.preventDefault();
    const id = e.dataTransfer.getData('text/plain');
    const is = issues.find((i) => i.id === id);
    if (is && api.statusOf(is) !== s) {
      api.setStatus(id, s);
      api.toast(`${id} moved to ${STATUS_LABEL[s]}`);
    }
    setOver(null);
  };
  return (
    <div className="flex flex-1 gap-2 overflow-auto p-4">
      {columns.map((s) => {
        const col = issues.filter((i) => api.statusOf(i) === s);
        return (
          <section
            key={s}
            onDragOver={(e) => {
              e.preventDefault();
              setOver(s);
            }}
            onDragLeave={() => setOver((o) => (o === s ? null : o))}
            onDrop={onDrop(s)}
            className="flex w-[280px] shrink-0 flex-col rounded-[12px] p-1.5 transition-colors"
            style={{ background: over === s ? 'var(--surface-selected)' : 'var(--muted)' }}
            aria-label={STATUS_LABEL[s]}
          >
            <div className="flex h-9 items-center gap-2 px-2">
              <StatusIcon status={s} />
              <span className="font-medium">{STATUS_LABEL[s]}</span>
              <span className="text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                {col.length}
              </span>
              <div className="flex-1" />
              <IconButton label={`New issue in ${STATUS_LABEL[s]}`} icon={<Plus size={15} />} onClick={() => onAdd(s)} />
            </div>
            <div className="flex flex-1 flex-col gap-1.5 overflow-auto">
              {col.map((is) => (
                <BoardCard key={is.id} api={api} issue={is} showLane={showLane} />
              ))}
              {!col.length && (
                <div className="px-2 py-1 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                  No issues
                </div>
              )}
            </div>
          </section>
        );
      })}
    </div>
  );
}

function BoardCard({ api, issue, showLane }: { api: AppApi; issue: Issue; showLane: boolean }) {
  const found = issue.lane ? findLane(api.orgs, issue.lane) : null;
  const relIdx = found?.lane.releases?.findIndex((r) => r.version === issue.release) ?? -1;
  const relState = found && relIdx >= 0 ? releaseStates(found.lane, api.statusOf)[relIdx] : 'planned';
  return (
    <article
      draggable
      onDragStart={(e) => e.dataTransfer.setData('text/plain', issue.id)}
      onClick={() => api.peek(issue)}
      className="cursor-pointer rounded-[8px] border px-2.5 py-2.5 transition-colors hover:border-[var(--ring)]"
      style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)', boxShadow: 'var(--surface-shadow)' }}
    >
      <div className="flex items-center gap-1.5">
        <TypeIcon type={issue.type} />
        <span className="mc-mono text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
          {issue.id}
        </span>
        <div className="flex-1" />
        {issue.assignee && <Avatar text={initials(issue.assignee)} title={issue.assignee} />}
      </div>
      <h4 className="mt-1.5 text-[14px] leading-[18px] font-medium">{issue.title}</h4>
      {(issue.release || (showLane && issue.lane)) && (
        <div className="mt-2 flex flex-wrap gap-1">
          {issue.release && <ReleaseChip version={issue.release} state={relState} />}
          {showLane && issue.lane && <Chip icon={<GitBranch size={12} />}>{laneLabel(issue.lane)}</Chip>}
        </div>
      )}
    </article>
  );
}

// ---------------------------------------------------------------------------
// Screen 4: a squad's kanban board
// ---------------------------------------------------------------------------

function SquadPage({ api, squad, view, issues }: { api: AppApi; squad: string; view: SquadView; issues: Issue[] }) {
  const [types, setTypes] = useState<string[]>([]);
  const shown = types.length ? issues.filter((i) => types.includes(i.type)) : issues;
  const backlog = shown.filter((i) => api.statusOf(i) === 'backlog');
  const lanes = useMemo(() => {
    const seen: LaneRef[] = [];
    issues.forEach((i) => i.lane && !seen.some((s) => sameLane(s, i.lane)) && seen.push(i.lane));
    return seen
      .map((ref) => ({ ref, found: findLane(api.orgs, ref) }))
      .map(({ ref, found }) => {
        const states = found ? releaseStates(found.lane, api.statusOf) : [];
        const rel = found?.lane.releases?.[states.indexOf('next')];
        return { ref, rel };
      });
  }, [issues, api.orgs, api.statusOf]);
  const setView = (v: SquadView) => api.go({ name: 'squad', squad, view: v });
  return (
    <>
      <PageHeader
        icon={Users}
        crumbs={[
          { label: 'Squads' },
          {
            label: squad,
            menu: (
              <Menu
                width={220}
                trigger={(_, toggle) => (
                  <button onClick={toggle} className="flex items-center gap-1 rounded-[4px] px-1 hover:bg-[var(--surface-hover)]">
                    {squad}
                    <ChevronDown size={14} style={{ color: 'var(--muted-foreground)' }} />
                  </button>
                )}
              >
                {(close) =>
                  SQUADS.map((s) => (
                    <MenuItem
                      key={s.name}
                      selected={s.name === squad}
                      icon={<Users size={14} />}
                      onClick={() => {
                        close();
                        api.go({ name: 'squad', squad: s.name, view });
                      }}
                    >
                      {s.name} <span style={{ color: 'var(--muted-foreground)' }}>· {s.leader}</span>
                    </MenuItem>
                  ))
                }
              </Menu>
            ),
          },
        ]}
        actions={
          <>
            <FilterMenu options={TYPE_FILTER} value={types} onChange={setTypes} />
            <Button variant="primary" icon={<Plus size={15} />} onClick={() => api.newIssue({ squad })}>
              New issue
            </Button>
          </>
        }
      />
      <div className="flex h-[52px] shrink-0 items-center gap-2 border-b px-4" style={{ borderColor: 'var(--border)' }}>
        <span className="mr-2 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
          Next releases
        </span>
        {lanes.filter((l) => l.rel).length === 0 && (
          <span className="text-[12px]" style={{ color: 'var(--faint-foreground)' }}>
            No planned release in this squad's worklanes
          </span>
        )}
        {lanes
          .filter((l) => l.rel)
          .map(({ ref, rel }) => (
            <span key={laneLabel(ref)} className="flex items-center gap-1.5">
              <Chip icon={<GitBranch size={12} />} onClick={() => api.go({ name: 'lane', ref, view: 'timeline' })}>
                {laneLabel(ref)}
              </Chip>
              <ReleaseChip version={rel!.version} state="next" onClick={() => api.go({ name: 'lane', ref, view: 'timeline' })} />
              <span className="text-[12px] tabular-nums" style={{ color: 'var(--muted-foreground)' }}>
                {doneCount(rel!, api.statusOf)}/{rel!.issues.length}
              </span>
              <span className="mr-3 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                {rel!.goal}
              </span>
            </span>
          ))}
        <div className="flex-1" />
        <Segmented<SquadView>
          value={view}
          onChange={setView}
          options={[
            { value: 'board', label: 'Board' },
            { value: 'list', label: 'List' },
            { value: 'backlog', label: `Backlog ${backlog.length}` },
          ]}
        />
      </div>
      {view === 'board' && (
        <StatusBoard api={api} columns={['todo', 'in_progress', 'in_review', 'done']} issues={shown} onAdd={(s) => api.newIssue({ squad, status: s })} showLane />
      )}
      {view !== 'board' && (
        <div className="flex-1 overflow-auto px-6 py-4">
          {(view === 'backlog' ? (['backlog'] as Status[]) : (['todo', 'in_progress', 'in_review', 'done', 'backlog'] as Status[])).map((s) => {
            const rows = shown.filter((i) => api.statusOf(i) === s);
            if (!rows.length && view !== 'backlog') return null;
            return (
              <section key={s} className="mb-4 overflow-hidden rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)' }}>
                <div className="flex h-10 items-center gap-2 px-4">
                  <StatusIcon status={s} />
                  <span className="font-medium">{STATUS_LABEL[s]}</span>
                  <span className="text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                    {rows.length}
                  </span>
                </div>
                {rows.map((is) => (
                  <div key={is.id} className="border-t" style={{ borderColor: 'var(--border)' }}>
                    <IssueRow api={api} issue={is} />
                  </div>
                ))}
                {!rows.length && <div className="border-t px-4 py-3 text-[13px]" style={{ borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>Nothing waiting</div>}
              </section>
            );
          })}
          {!shown.length && <EmptyNote text={`${squad} has no issues yet.`} />}
        </div>
      )}
    </>
  );
}

// ---------------------------------------------------------------------------
// Issue peek: what and why only, read from the issue's template sections
// ---------------------------------------------------------------------------

function IssuePeek({ api, issue, onClose }: { api: AppApi; issue: Issue; onClose: () => void }) {
  const tpl = TEMPLATES.find((t) => t.type === issue.type)!;
  const status = api.statusOf(issue);
  return (
    <aside
      className="mc-pop fixed top-2 right-2 bottom-2 z-40 flex w-[440px] flex-col overflow-hidden rounded-[12px] border"
      style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)', boxShadow: 'var(--floating-shadow)' }}
      aria-label={`Issue ${issue.id}`}
    >
      <div className="flex h-12 items-center gap-2 border-b px-4" style={{ borderColor: 'var(--border)' }}>
        <TypeIcon type={issue.type} />
        <span className="text-[13px] font-medium">{TYPE_LABEL[issue.type]}</span>
        <span className="mc-mono text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
          {issue.id}
        </span>
        <div className="flex-1" />
        <IconButton label="Close" icon={<X size={16} />} onClick={onClose} />
      </div>
      <div className="flex-1 overflow-auto p-4">
        <h2 className="text-[18px] leading-6 font-semibold">{issue.title}</h2>
        <div className="mt-3 flex flex-wrap gap-1.5">
          <Menu
            width={180}
            trigger={(_, toggle) => (
              <Chip icon={<StatusIcon status={status} size={12} />} onClick={toggle}>
                {STATUS_LABEL[status]}
              </Chip>
            )}
          >
            {(close) =>
              (Object.keys(STATUS_LABEL) as Status[]).map((s) => (
                <MenuItem
                  key={s}
                  icon={<StatusIcon status={s} />}
                  selected={s === status}
                  onClick={() => {
                    close();
                    api.setStatus(issue.id, s);
                  }}
                >
                  {STATUS_LABEL[s]}
                </MenuItem>
              ))
            }
          </Menu>
          {issue.lane && (
            <Chip icon={<GitBranch size={12} />} onClick={() => api.go({ name: 'lane', ref: issue.lane!, view: 'timeline' })}>
              {laneLabel(issue.lane)}
            </Chip>
          )}
          {issue.release && <ReleaseChip version={issue.release} state="planned" />}
          {issue.assignee && <Chip icon={<Avatar text={initials(issue.assignee)} size={14} />}>{issue.assignee}</Chip>}
        </div>
        <div className="mt-5 space-y-4">
          {tpl.sections.map((s) => (
            <div key={s.key}>
              <div className="text-[13px] font-medium">{s.name}</div>
              <p className="mt-1 text-[14px] whitespace-pre-wrap" style={{ color: issue.fields?.[s.key] ? 'var(--foreground)' : 'var(--faint-foreground)' }}>
                {issue.fields?.[s.key] || 'Not written in this prototype'}
              </p>
            </div>
          ))}
        </div>
      </div>
      <div className="border-t px-4 py-2 text-[12px]" style={{ borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
        Code and technical design go in the change, not in the issue.
      </div>
    </aside>
  );
}

// ---------------------------------------------------------------------------
// Screen 5: a new issue always starts from a template
// ---------------------------------------------------------------------------

function NewIssueDialog({ api, draft, onClose, onCreate }: { api: AppApi; draft: Draft; onClose: () => void; onCreate: (d: Draft, title: string, fields: Record<string, string>, p: Priority) => void }) {
  const [d, setD] = useState<Draft>(draft);
  const [title, setTitle] = useState('');
  const [fields, setFields] = useState<Record<string, string>>({});
  const [priority, setPriority] = useState<Priority>('none');
  const tpl = TEMPLATES.find((t) => t.type === d.type)!;
  const lanes = useMemo(() => allLanes(api.orgs), [api.orgs]);
  const laneObj = d.lane ? findLane(api.orgs, d.lane)?.lane : undefined;
  const valid = title.trim() !== '' && tpl.sections.every((s) => !s.required || (fields[s.key] ?? '').trim() !== '');
  const submit = () => valid && onCreate(d, title.trim(), fields, priority);

  return (
    <Overlay onClose={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="New issue"
        className="mc-pop flex max-h-[calc(100vh-64px)] w-[920px] flex-col overflow-hidden rounded-[12px] border"
        style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)', boxShadow: 'var(--floating-shadow)' }}
        onKeyDown={(e) => {
          if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) submit();
        }}
      >
        <div className="flex h-[52px] shrink-0 items-center justify-between border-b px-4" style={{ borderColor: 'var(--border)' }}>
          <h2 className="text-[16px] font-semibold">New issue</h2>
          <IconButton label="Close" icon={<X size={16} />} onClick={onClose} />
        </div>
        <div className="flex min-h-0 flex-1">
          <div className="w-[264px] shrink-0 overflow-auto border-r p-2" style={{ background: 'var(--page-canvas)', borderColor: 'var(--border)' }} role="radiogroup" aria-label="Template">
            <div className="px-2 pt-1 pb-2 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
              Template
            </div>
            {TEMPLATES.map((t) => (
              <button
                key={t.type}
                role="radio"
                aria-checked={t.type === d.type}
                onClick={() => setD({ ...d, type: t.type })}
                className="flex w-full items-start gap-2.5 rounded-[8px] px-2 py-2 text-left hover:bg-[var(--surface-hover)]"
                style={t.type === d.type ? { background: 'var(--surface-selected)' } : undefined}
              >
                <span className="mt-0.5">
                  <TypeIcon type={t.type} size={15} />
                </span>
                <span className="min-w-0">
                  <span className="block text-[14px] font-medium">{t.name}</span>
                  <span className="block truncate text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                    {t.hint}
                  </span>
                </span>
              </button>
            ))}
          </div>
          <div className="min-w-0 flex-1 overflow-auto px-6 py-5">
            <input
              autoFocus
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Issue title"
              aria-label="Issue title"
              className="w-full bg-transparent text-[18px] font-medium outline-none placeholder:text-[var(--faint-foreground)]"
            />
            <div className="mt-3 flex flex-wrap gap-1.5">
              <Chip icon={<TypeIcon type={d.type} size={13} />} title="The template sets the type">
                {tpl.name}
              </Chip>
              <Menu
                width={220}
                trigger={(_, toggle) => (
                  <Chip icon={<Users size={13} />} onClick={toggle}>
                    {d.squad ?? 'No squad'}
                  </Chip>
                )}
              >
                {(close) => (
                  <>
                    {SQUADS.map((s) => (
                      <MenuItem key={s.name} icon={<Users size={14} />} selected={s.name === d.squad} onClick={() => (setD({ ...d, squad: s.name }), close())}>
                        {s.name}
                      </MenuItem>
                    ))}
                    <MenuItem selected={!d.squad} onClick={() => (setD({ ...d, squad: undefined }), close())}>
                      No squad
                    </MenuItem>
                  </>
                )}
              </Menu>
              <Menu
                width={320}
                trigger={(_, toggle) => (
                  <Chip icon={<GitBranch size={13} />} onClick={toggle}>
                    {d.lane ? laneLabel(d.lane) : 'No worklane'}
                  </Chip>
                )}
              >
                {(close) => (
                  <div className="max-h-[320px] overflow-auto">
                    {lanes.map(({ ref }) => (
                      <MenuItem
                        key={`${ref.org}/${ref.project}/${ref.repo}/${ref.lane}`}
                        icon={<GitBranch size={14} />}
                        selected={sameLane(ref, d.lane)}
                        onClick={() => {
                          const l = findLane(api.orgs, ref)!.lane;
                          const st = releaseStates(l, api.statusOf);
                          setD({ ...d, lane: ref, release: l.releases?.[st.indexOf('next')]?.version });
                          close();
                        }}
                      >
                        {laneLabel(ref)}
                      </MenuItem>
                    ))}
                  </div>
                )}
              </Menu>
              <Menu
                width={260}
                trigger={(_, toggle) => (
                  <Chip icon={<CircleDashed size={13} />} onClick={toggle}>
                    <span className={d.release ? 'mc-mono' : ''}>{d.release ?? 'No release'}</span>
                  </Chip>
                )}
              >
                {(close) => (
                  <>
                    {(laneObj?.releases ?? []).map((r) => (
                      <MenuItem key={r.version} selected={r.version === d.release} onClick={() => (setD({ ...d, release: r.version }), close())}>
                        <span className="mc-mono">{r.version}</span> <span style={{ color: 'var(--muted-foreground)' }}>· {r.goal}</span>
                      </MenuItem>
                    ))}
                    {!laneObj?.releases?.length && (
                      <div className="px-2 py-1.5 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                        {d.lane ? 'This worklane has no releases planned' : 'Pick a worklane first'}
                      </div>
                    )}
                    <MenuItem selected={!d.release} onClick={() => (setD({ ...d, release: undefined }), close())}>
                      No release
                    </MenuItem>
                  </>
                )}
              </Menu>
              <Menu
                width={180}
                trigger={(_, toggle) => (
                  <Chip icon={<Ellipsis size={13} />} onClick={toggle}>
                    {PRIORITY_LABEL[priority]}
                  </Chip>
                )}
              >
                {(close) =>
                  (Object.keys(PRIORITY_LABEL) as Priority[]).map((p) => (
                    <MenuItem key={p} selected={p === priority} onClick={() => (setPriority(p), close())}>
                      {PRIORITY_LABEL[p]}
                    </MenuItem>
                  ))
                }
              </Menu>
            </div>
            <div className="mt-5 space-y-4">
              {tpl.sections.map((s) => (
                <label key={`${d.type}-${s.key}`} className="block">
                  <span className="flex items-baseline gap-2">
                    <span className="text-[13px] font-medium">{s.name}</span>
                    <span className="text-[11px]" style={{ color: 'var(--muted-foreground)' }}>
                      {s.required ? 'Required' : 'Optional'}
                    </span>
                  </span>
                  <textarea
                    rows={s.rows}
                    value={fields[s.key] ?? ''}
                    onChange={(e) => setFields({ ...fields, [s.key]: e.target.value })}
                    placeholder={s.placeholder}
                    className="mt-1.5 block w-full resize-none rounded-[8px] border px-2.5 py-2 text-[14px] outline-none placeholder:text-[var(--faint-foreground)] focus:border-[var(--ring)]"
                    style={{ borderColor: 'var(--input)' }}
                  />
                </label>
              ))}
            </div>
            <p className="mt-4 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
              Code and technical design go in the change, not in the issue.
            </p>
          </div>
        </div>
        <div className="flex h-14 shrink-0 items-center justify-end gap-2 border-t px-4" style={{ borderColor: 'var(--border)' }}>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="brand" icon={<Check size={15} />} disabled={!valid} onClick={submit}>
            Create {tpl.name.toLowerCase()}
          </Button>
        </div>
      </div>
    </Overlay>
  );
}

// ---------------------------------------------------------------------------
// Search palette
// ---------------------------------------------------------------------------

function Palette({ api, squadIssues, onClose }: { api: AppApi; squadIssues: Record<string, Issue[]>; onClose: () => void }) {
  const [q, setQ] = useState('');
  const [sel, setSel] = useState(0);
  const items = useMemo(() => {
    const all: { label: string; hint: string; icon: ReactNode; run: () => void }[] = [];
    api.orgs.forEach((o) =>
      o.projects.forEach((p) => {
        all.push({ label: p.name, hint: o.name, icon: <FolderKanban size={14} />, run: () => api.go({ name: 'project', org: o.name, project: p.name }) });
        p.repos.forEach((r) =>
          r.lanes.forEach((l) => {
            const ref = { org: o.name, project: p.name, repo: r.name, lane: l.name };
            all.push({ label: `${p.name} · ${l.name}`, hint: r.name, icon: <GitBranch size={14} />, run: () => api.go({ name: 'lane', ref, view: 'timeline' }) });
            (l.releases ?? []).forEach((rel) =>
              rel.issues.forEach((is) => all.push({ label: `${is.id} ${is.title}`, hint: `${p.name} · ${rel.version}`, icon: <TypeIcon type={is.type} />, run: () => api.peek(is) })),
            );
          }),
        );
      }),
    );
    SQUADS.forEach((s) => {
      all.push({ label: `${s.name} squad`, hint: s.leader, icon: <Users size={14} />, run: () => api.go({ name: 'squad', squad: s.name, view: 'board' }) });
      (squadIssues[s.name] ?? []).forEach((is) => all.push({ label: `${is.id} ${is.title}`, hint: s.name, icon: <TypeIcon type={is.type} />, run: () => api.peek(is) }));
    });
    const needle = q.trim().toLowerCase();
    return (needle ? all.filter((i) => `${i.label} ${i.hint}`.toLowerCase().includes(needle)) : all.filter((i) => !/^[A-Z]+-\d+/.test(i.label))).slice(0, 10);
  }, [q, api, squadIssues]);
  const run = (i: number) => {
    items[i]?.run();
    onClose();
  };
  return (
    <Overlay onClose={onClose} top>
      <div className="mc-pop w-[600px] overflow-hidden rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)', boxShadow: 'var(--floating-shadow)' }}>
        <div className="flex items-center gap-2 border-b px-4" style={{ borderColor: 'var(--border)' }}>
          <Search size={16} style={{ color: 'var(--muted-foreground)' }} />
          <input
            autoFocus
            value={q}
            onChange={(e) => (setQ(e.target.value), setSel(0))}
            onKeyDown={(e) => {
              if (e.key === 'ArrowDown') (e.preventDefault(), setSel((s) => Math.min(s + 1, items.length - 1)));
              if (e.key === 'ArrowUp') (e.preventDefault(), setSel((s) => Math.max(s - 1, 0)));
              if (e.key === 'Enter') run(sel);
            }}
            placeholder="Search projects, worklanes, squads and issues"
            className="h-12 flex-1 bg-transparent text-[15px] outline-none placeholder:text-[var(--faint-foreground)]"
          />
        </div>
        <div className="max-h-[360px] overflow-auto p-1">
          {items.map((it, i) => (
            <button
              key={`${it.label}-${it.hint}`}
              onMouseEnter={() => setSel(i)}
              onClick={() => run(i)}
              className="flex h-9 w-full items-center gap-2 rounded-[6px] px-3 text-left text-[14px]"
              style={i === sel ? { background: 'var(--surface-hover)' } : undefined}
            >
              <span style={{ color: 'var(--muted-foreground)' }}>{it.icon}</span>
              <span className="min-w-0 flex-1 truncate">{it.label}</span>
              <span className="truncate text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                {it.hint}
              </span>
            </button>
          ))}
          {!items.length && <div className="px-3 py-4 text-[13px]" style={{ color: 'var(--muted-foreground)' }}>Nothing found</div>}
        </div>
      </div>
    </Overlay>
  );
}

// ---------------------------------------------------------------------------
// Small parts
// ---------------------------------------------------------------------------

function Overlay({ children, onClose, top }: { children: ReactNode; onClose: () => void; top?: boolean }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose();
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [onClose]);
  return (
    <div
      className={`fixed inset-0 z-50 flex justify-center p-8 ${top ? 'items-start pt-[14vh]' : 'items-center'}`}
      style={{ background: 'rgb(9 9 11 / 0.32)' }}
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
    >
      {children}
    </div>
  );
}

function IconButton({ label, icon, onClick }: { label: string; icon: ReactNode; onClick: () => void }) {
  return (
    <button aria-label={label} title={label} onClick={onClick} className="flex size-7 items-center justify-center rounded-[6px] hover:bg-[var(--surface-hover)]" style={{ color: 'var(--muted-foreground)' }}>
      {icon}
    </button>
  );
}

function EmptyNote({ text }: { text: string }) {
  return (
    <div className="rounded-[12px] border border-dashed px-6 py-10 text-center text-[14px]" style={{ borderColor: 'var(--surface-border)', color: 'var(--muted-foreground)' }}>
      {text}
    </div>
  );
}

function OtherPage({ label }: { label: string }) {
  return (
    <>
      <PageHeader icon={Inbox} crumbs={[{ label }]} />
      <div className="flex flex-1 items-center justify-center p-10">
        <EmptyNote text={`${label} stays as Multica has it today; this prototype covers Projects, worklanes, Squads and New issue.`} />
      </div>
    </>
  );
}
