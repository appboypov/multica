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
  Maximize2,
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
  laneIssues,
  laneLabel,
  loadOrgs,
  plural,
  releaseStates,
  sameLane,
  templateInputs,
  templateMarkdown,
} from './model';
import { AddRepositoryDialog, NewWorklaneDialog, PlanReleaseDialog } from './creation';
import { Avatar, Button, Chip, IconButton, Markdown, Menu, MenuItem, Overlay, Progress, ReleaseChip, Segmented, StatusIcon, TypeIcon } from './ui';

// ---------------------------------------------------------------------------
// Routing and app state
// ---------------------------------------------------------------------------

type LaneView = 'timeline' | 'board' | 'list';
type SquadView = 'board' | 'list' | 'backlog';

type Route =
  | { name: 'projects' }
  | { name: 'project'; org: string; project: string; addLane?: string }
  | { name: 'lane'; ref: LaneRef; view: LaneView }
  | { name: 'squad'; squad: string; view: SquadView }
  | { name: 'other'; label: string }
  | { name: 'issue'; id: string; from: 'Projects' | 'Squads' };

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
  /** The issue's one Markdown description, with edits made in this session. */
  descriptionOf: (i: Issue) => string;
  setDescription: (id: string, md: string) => void;
  go: (r: Route) => void;
  newIssue: (d?: Partial<Draft>) => void;
  peek: (i: Issue) => void;
  /** Opens the issue's own page. */
  open: (i: Issue) => void;
  findIssue: (id: string) => Issue | null;
  squadOf: (i: Issue) => string | undefined;
  /** The issue made last in this session; it flashes where it lands. */
  fresh: string | null;
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
  const [descriptions, setDescriptions] = useState<Record<string, string>>({});
  const [route, setRoute] = useState<Route>({ name: 'projects' });
  const [draft, setDraft] = useState<Draft | null>(null);
  const [peeked, setPeeked] = useState<Issue | null>(null);
  const [fresh, setFresh] = useState<string | null>(null);
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
      // A worklane's new issue lands in that worklane's next release.
      const nextOf = (ref: LaneRef | undefined) => {
        const l = ref ? findLane(orgs, ref)?.lane : undefined;
        const st = l ? releaseStates(l, statusOf) : [];
        return l?.releases?.[st.indexOf('next')]?.version;
      };
      // A worklane's new issue also goes to the squad that works the worklane.
      const ctx: Partial<Draft> =
        route.name === 'lane'
          ? { lane: route.ref, release: nextOf(route.ref), squad: findLane(orgs, route.ref)?.lane.squad }
          : route.name === 'squad'
            ? (() => {
                // A squad's new issue goes to the worklane of its open work, in that worklane's next release.
                const lane = (squadIssues[route.squad] ?? []).find((i) => i.lane && statusOf(i) !== 'done')?.lane;
                return { squad: route.squad, lane, release: nextOf(lane) };
              })()
            : {};
      setDraft({ type: 'story', ...ctx, ...d });
    },
    [route, orgs, squadIssues, statusOf],
  );

  const api: AppApi = {
    orgs,
    statusOf,
    setStatus: (id, s) => setStatuses((m) => ({ ...m, [id]: s })),
    descriptionOf: (i) => descriptions[i.id] ?? i.description ?? '',
    setDescription: (id, md) => setDescriptions((m) => ({ ...m, [id]: md })),
    go: (r) => {
      setPeeked(null);
      setRoute(r);
    },
    newIssue,
    peek: setPeeked,
    open: (i) => {
      setPeeked(null);
      setRoute({ name: 'issue', id: i.id, from: i.lane && !(route.name === 'squad') ? 'Projects' : 'Squads' });
    },
    findIssue: (id) =>
      allLanes(orgs)
        .flatMap((l) => laneIssues(l.lane))
        .concat(Object.values(squadIssues).flat())
        .find((i) => i.id === id) ?? null,
    squadOf: (i) => i.squad ?? Object.keys(squadIssues).find((s) => squadIssues[s].some((x) => x.id === i.id)),
    fresh,
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

  const createIssue = (d: Draft, title: string, description: string, priority: Priority) => {
    const target = d.lane ? findLane(orgs, d.lane) : null;
    const prefix = target?.project.name === 'Rebl' ? 'REBL' : 'WORK';
    const used = [
      ...allLanes(orgs).flatMap((l) => laneIssues(l.lane)),
      ...Object.values(squadIssues).flat(),
    ]
      .filter((i) => i.id.startsWith(prefix + '-'))
      .map((i) => Number(i.id.split('-')[1]));
    const id = `${prefix}-${Math.max(prefix === 'WORK' ? 360 : 0, ...used) + 1}`;
    const issue: Issue = {
      id,
      type: d.type,
      title,
      // Kanban: work planned into a release is ready to pick up; work without one waits in the backlog.
      status: d.status ?? (d.release ? 'todo' : 'backlog'),
      priority,
      lane: d.lane,
      release: d.release,
      description,
      squad: d.squad,
      created: true,
    };
    // A worklane holds its issue in the chosen release, or in its backlog until a release is planned for it.
    if (d.lane) {
      const lane = d.lane.lane;
      updateLanes(d.lane, (ls) =>
        ls.map((l) =>
          l.name !== lane
            ? l
            : d.release
              ? { ...l, releases: (l.releases ?? []).map((rel) => (rel.version === d.release ? { ...rel, issues: [...rel.issues, issue] } : rel)) }
              : { ...l, backlog: [...(l.backlog ?? []), issue] },
        ),
      );
    }
    if (d.squad) setSquadIssues((m) => ({ ...m, [d.squad!]: [...(m[d.squad!] ?? []), issue] }));
    setDraft(null);
    setFresh(id);
    setPeeked(null);
    setRoute({ name: 'issue', id, from: route.name === 'squad' || (!d.lane && d.squad) ? 'Squads' : 'Projects' });
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

  const addRepo = (org: string, project: string, repo: Repo) =>
    setOrgs((os) =>
      os.map((o) =>
        o.name !== org ? o : { ...o, projects: o.projects.map((p) => (p.name === project ? { ...p, repos: [...p.repos, repo] } : p)) },
      ),
    );

  // A planned release takes the picked issues out of the worklane's backlog; they are ready to pick up.
  const planRelease = (ref: LaneRef, version: string, ids: string[]) => {
    const ready = (i: Issue): Issue => ({ ...i, release: version, status: i.status === 'backlog' ? 'todo' : i.status });
    updateLanes(ref, (ls) =>
      ls.map((l) => {
        if (l.name !== ref.lane) return l;
        const moved = (l.backlog ?? []).filter((i) => ids.includes(i.id));
        return {
          ...l,
          releases: [...(l.releases ?? []), { version, issues: moved.map(ready) }],
          backlog: (l.backlog ?? []).filter((i) => !ids.includes(i.id)),
        };
      }),
    );
    setSquadIssues((m) => Object.fromEntries(Object.entries(m).map(([s, is]) => [s, is.map((i) => (ids.includes(i.id) ? ready(i) : i))])));
  };

  let page: ReactNode;
  if (route.name === 'projects') page = <ProjectsPage api={api} onAddProject={addProject} />;
  else if (route.name === 'project')
    page = (
      <ProjectPage
        key={`${route.org}/${route.project}/${route.addLane ?? ''}`}
        api={api}
        orgName={route.org}
        projectName={route.project}
        initialAddLane={route.addLane}
        onAddRepo={(repo) => addRepo(route.org, route.project, repo)}
        onAddLane={(repo, lane) => {
          updateLanes({ org: route.org, project: route.project, repo }, (ls) => [
            ...ls,
            { name: lane.name, folder: lane.folder, squad: lane.squad, state: 'Free', active: null, next: null, releases: [], backlog: [] },
          ]);
          setRoute({ name: 'lane', ref: { org: route.org, project: route.project, repo, lane: lane.name }, view: 'timeline' });
          toast(`Worklane ${lane.name} is ready`);
        }}
      />
    );
  else if (route.name === 'lane')
    page = (
      <LanePage
        api={api}
        refLane={route.ref}
        view={route.view}
        onPlanRelease={(version, ids) => {
          planRelease(route.ref, version, ids);
          toast(`${version} planned`);
        }}
      />
    );
  else if (route.name === 'squad') page = <SquadPage api={api} squad={route.squad} view={route.view} issues={squadIssues[route.squad] ?? []} />;
  else if (route.name === 'issue') page = <IssuePage key={route.id} api={api} id={route.id} from={route.from} />;
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
        : route.name === 'issue'
          ? route.from
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
      <Menu
        width={240}
        trigger={(_, toggle) => (
          <button onClick={toggle} aria-label="Workspace" className="mb-1 flex h-9 flex-1 items-center gap-2 rounded-[6px] px-2 hover:bg-[var(--sidebar-accent)]">
            <span className="flex size-5 items-center justify-center rounded-[4px] text-[11px] font-semibold text-white" style={{ background: 'var(--primary)' }}>
              D
            </span>
            <span className="flex-1 text-left text-[14px] font-medium">De App Specialist</span>
            <ChevronDown size={14} style={{ color: 'var(--muted-foreground)' }} />
          </button>
        )}
      >
        {(close) => (
          <>
            <div className="px-2 pt-1 pb-1 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
              Workspaces
            </div>
            <MenuItem
              selected
              onClick={close}
              icon={
                <span className="flex size-4 items-center justify-center rounded-[3px] text-[10px] font-semibold text-white" style={{ background: 'var(--primary)' }}>
                  D
                </span>
              }
            >
              De App Specialist
            </MenuItem>
            <div className="my-1 h-px" style={{ background: 'var(--border)' }} />
            <MenuItem icon={<Settings size={14} />} onClick={() => (close(), api.go({ name: 'other', label: 'Settings' }))}>
              Workspace settings
            </MenuItem>
          </>
        )}
      </Menu>
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
      width={280}
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
      style={{ width: size, height: size, fontSize: size > 30 ? 16 : 13, background: 'var(--muted)', borderColor: 'var(--surface-border)', color: size > 30 ? 'var(--muted-foreground)' : 'var(--foreground)' }}
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
  const [closedRepos, setClosedRepos] = useState<Set<string>>(new Set());
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
                        <button onClick={() => api.go({ name: 'project', org: o.name, project: p.name })} className="truncate text-[14px] font-medium hover:underline">
                          {p.name}
                        </button>
                        <div className="flex-1" />
                        <span className="text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
                          {p.repos.length ? `${plural(p.repos.length, 'repository', 'repositories')} · ${plural(lc, 'worklane')}` : 'No repositories'}
                        </span>
                        <Avatar text="BR" title="Lead: Brian" />
                      </div>
                      {pOpen &&
                        p.repos.map((r) => {
                          const rKey = `${key}/${r.name}`;
                          const rOpen = !closedRepos.has(rKey);
                          return (
                            <div key={r.name}>
                              <div className="flex h-9 items-center gap-2 border-t pr-4 pl-10" style={{ borderColor: 'var(--border)', background: 'var(--page-canvas)' }}>
                                <button
                                  aria-label={rOpen ? `Collapse ${r.name}` : `Expand ${r.name}`}
                                  aria-expanded={rOpen}
                                  onClick={() => toggle(closedRepos, setClosedRepos, rKey)}
                                  className="rounded-[4px] p-0.5 hover:bg-[var(--surface-hover)]"
                                >
                                  <Disclosure open={rOpen} />
                                </button>
                                <FolderGit2 size={14} style={{ color: 'var(--muted-foreground)' }} />
                                <span className="mc-mono text-[13px] font-medium">{r.name}</span>
                                <span className="mc-mono flex items-center gap-1 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                                  <GitCommitHorizontal size={12} />
                                  main
                                </span>
                                <div className="flex-1" />
                                <Button variant="ghost" icon={<Plus size={14} />} onClick={() => api.go({ name: 'project', org: o.name, project: p.name, addLane: r.name })}>
                                  Worklane
                                </Button>
                              </div>
                              {rOpen &&
                                r.lanes.map((l) => (
                                  <LaneRow key={l.name} api={api} lane={l} refLane={{ org: o.name, project: p.name, repo: r.name, lane: l.name }} indent={64} />
                                ))}
                              {rOpen && !r.lanes.length && (
                                <div className="border-t py-2.5 pl-16 text-[13px]" style={{ borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
                                  No worklanes yet
                                </div>
                              )}
                            </div>
                          );
                        })}
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

function ProjectPage({
  api,
  orgName,
  projectName,
  onAddRepo,
  onAddLane,
  initialAddLane,
}: {
  api: AppApi;
  orgName: string;
  projectName: string;
  onAddRepo: (repo: Repo) => void;
  onAddLane: (repo: string, lane: { name: string; folder: string; squad: string }) => void;
  initialAddLane?: string;
}) {
  const project = api.orgs.find((o) => o.name === orgName)?.projects.find((p) => p.name === projectName);
  const [addingRepo, setAddingRepo] = useState(false);
  const [addingLane, setAddingLane] = useState<string | null>(initialAddLane ?? null);
  const [repoFilter, setRepoFilter] = useState<string[]>([]);
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
          <>
            <FilterMenu options={[{ value: 'lanes', label: 'Only repositories with worklanes' }]} value={repoFilter} onChange={setRepoFilter} />
          <Button variant="primary" icon={<Plus size={15} />} onClick={() => api.newIssue(project.repos[0]?.lanes[0] ? { lane: { org: orgName, project: projectName, repo: project.repos[0].name, lane: project.repos[0].lanes[0].name } } : {})}>
            New issue
          </Button>
          </>
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
            <AddRepositoryDialog
              orgs={api.orgs}
              project={project}
              onClose={() => setAddingRepo(false)}
              onAdd={(repo) => {
                onAddRepo(repo);
                setAddingRepo(false);
                api.toast(`${repo.name} added to ${project.name}`);
              }}
            />
          )}
          {!project.repos.length && (
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
          {project.repos.filter((r) => !repoFilter.length || r.lanes.length).map((r) => (
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
              {!r.lanes.length && (
                <div className="border-t px-4 py-3 text-[13px]" style={{ borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
                  No worklanes yet
                </div>
              )}
              {addingLane === r.name && (
                <NewWorklaneDialog project={project} repo={r} onClose={() => setAddingLane(null)} onCreate={(lane) => onAddLane(r.name, lane)} />
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

/** The issue made last flashes once and scrolls into view wherever it shows. */
function useFresh<T extends HTMLElement>(api: AppApi, issue: Issue) {
  const ref = useRef<T>(null);
  const on = api.fresh === issue.id;
  useEffect(() => {
    if (on) ref.current?.scrollIntoView({ block: 'nearest' });
  }, [on]);
  return { ref, fresh: on ? ' mc-fresh' : '' };
}

function releaseStateOf(api: AppApi, issue: Issue): ReleaseState {
  const found = issue.lane ? findLane(api.orgs, issue.lane) : null;
  const idx = found?.lane.releases?.findIndex((r) => r.version === issue.release) ?? -1;
  return found && idx >= 0 ? releaseStates(found.lane, api.statusOf)[idx] : 'planned';
}

function IssueRow({ api, issue }: { api: AppApi; issue: Issue }) {
  const { ref, fresh } = useFresh<HTMLButtonElement>(api, issue);
  return (
    <button
      ref={ref}
      onClick={() => api.peek(issue)}
      className={`flex h-8 w-full items-center gap-2 px-4 text-left hover:bg-[var(--surface-hover)]${fresh}`}
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

function LanePage({
  api,
  refLane,
  view,
  onPlanRelease,
}: {
  api: AppApi;
  refLane: LaneRef;
  view: LaneView;
  onPlanRelease: (version: string, issueIds: string[]) => void;
}) {
  const found = findLane(api.orgs, refLane);
  const [types, setTypes] = useState<string[]>([]);
  const [planning, setPlanning] = useState(false);
  if (!found) return <OtherPage label="Worklane not found" />;
  const { lane, repo } = found;
  const backlog = lane.backlog ?? [];
  const rels = lane.releases ?? [];
  const states = releaseStates(lane, api.statusOf);
  const filt = (is: Issue[]) => (types.length ? is.filter((i) => types.includes(i.type)) : is);
  const shippedAny = states.includes('shipped');
  const nxt = issueKey(lane.active ?? lane.next);
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
              <button
                onClick={() => setPlanning(true)}
                className="flex h-24 w-full flex-col items-center justify-center gap-1 rounded-[12px] border border-dashed text-[13px] hover:bg-[var(--surface-hover)]"
                style={{ borderColor: 'var(--surface-border)', color: 'var(--muted-foreground)' }}
              >
                <Plus size={16} />
                Plan release
                {!!backlog.length && <span className="text-[12px]">{plural(backlog.length, 'issue')} in no release</span>}
              </button>
              {planning && (
                <PlanReleaseDialog
                  repo={repo}
                  laneName={lane.name}
                  candidates={backlog}
                  statusOf={api.statusOf}
                  onClose={() => setPlanning(false)}
                  onPlan={(version, ids) => {
                    onPlanRelease(version, ids);
                    setPlanning(false);
                  }}
                />
              )}
            </div>
          </div>
        </div>
      )}
      {view === 'board' && (
        <StatusBoard
          api={api}
          columns={['backlog', 'todo', 'in_progress', 'in_review', 'done']}
          issues={filt(laneIssues(lane))}
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
          {!!filt(backlog).length && (
            <section className="mb-4 overflow-hidden rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)' }}>
              <div className="flex h-11 items-center gap-2 px-4">
                <span className="font-medium">No release</span>
                <div className="flex-1" />
                <span className="text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                  {plural(backlog.length, 'issue')}
                </span>
              </div>
              {filt(backlog).map((is) => (
                <div key={is.id} className="border-t" style={{ borderColor: 'var(--border)' }}>
                  <IssueRow api={api} issue={is} />
                </div>
              ))}
            </section>
          )}
          {!rels.length && !backlog.length && <EmptyNote text="No releases planned for this worklane yet." />}
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
      <section className="rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: next ? 'color-mix(in oklab, var(--brand) 35%, var(--surface-border))' : 'var(--surface-border)', boxShadow: 'var(--surface-shadow)' }}>
        {/* The release head stays in view while a long release scrolls, so an issue is never seen without its release. */}
        <div className="sticky -top-6 z-10 rounded-t-[12px] border-b px-4 pt-3 pb-3" style={{ background: 'var(--surface)', borderColor: 'var(--border)' }}>
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
          <div className="mt-3 flex items-center gap-3">
            <Progress done={done} total={rel.issues.length} />
            <span className="shrink-0 text-[12px] tabular-nums" style={{ color: 'var(--muted-foreground)' }}>
              {done} of {rel.issues.length} done
            </span>
          </div>
        </div>
        <div className="py-1">
          {issues.map((is) => (
            <IssueRow key={is.id} api={api} issue={is} />
          ))}
          {!issues.length && <div className="px-4 py-3 text-[13px]" style={{ color: 'var(--muted-foreground)' }}>No issues</div>}
        </div>
        <button
          onClick={() => api.newIssue({ lane: refLane, release: rel.version })}
          className="flex h-9 w-full items-center gap-2 rounded-b-[12px] border-t px-4 text-left text-[13px] hover:bg-[var(--surface-hover)]"
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
  const relState = releaseStateOf(api, issue);
  const { ref, fresh } = useFresh<HTMLElement>(api, issue);
  return (
    <article
      ref={ref}
      draggable
      onDragStart={(e) => e.dataTransfer.setData('text/plain', issue.id)}
      onClick={() => api.peek(issue)}
      className={`cursor-pointer rounded-[8px] border px-2.5 py-2.5 transition-colors hover:border-[var(--ring)]${fresh}`}
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
              <span className="mr-3 text-[12px] tabular-nums" style={{ color: 'var(--muted-foreground)' }}>
                {doneCount(rel!, api.statusOf)}/{rel!.issues.length}
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
// An issue holds what and why only, read from its template's sections
// ---------------------------------------------------------------------------

const RULE = 'Code and technical design go in the change, not in the issue.';

function StatusMenu({ api, issue, trigger }: { api: AppApi; issue: Issue; trigger: (status: Status, toggle: () => void) => ReactNode }) {
  const status = api.statusOf(issue);
  return (
    <Menu width={180} trigger={(_, toggle) => trigger(status, toggle)}>
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
  );
}

function IssueDescription({ api, issue, size }: { api: AppApi; issue: Issue; size: 'peek' | 'page' }) {
  const [mode, setMode] = useState<'preview' | 'markdown'>('preview');
  const md = api.descriptionOf(issue);
  const empty = (
    <p className="text-[14px]" style={{ color: 'var(--faint-foreground)' }}>
      {issue.created ? 'No description' : 'No description in this prototype'}
    </p>
  );
  if (size === 'peek') return md.trim() ? <Markdown text={md} size="peek" /> : empty;
  return (
    <section aria-label="Description">
      <div className="mb-4 flex items-center justify-between">
        <span className="text-[12px] font-medium" style={{ color: 'var(--muted-foreground)' }}>
          Description
        </span>
        <Segmented
          value={mode}
          options={[
            { value: 'preview', label: 'Preview' },
            { value: 'markdown', label: 'Markdown' },
          ]}
          onChange={setMode}
        />
      </div>
      {mode === 'markdown' ? (
        <textarea
          aria-label="Description Markdown"
          value={md}
          onChange={(e) => api.setDescription(issue.id, e.target.value)}
          rows={Math.max(8, md.split('\n').length + 1)}
          spellCheck={false}
          className="mc-mono block w-full resize-none rounded-[8px] border px-3 py-2.5 text-[13px] leading-[20px] outline-none focus:border-[var(--ring)]"
          style={{ borderColor: 'var(--input)', background: 'var(--surface)' }}
        />
      ) : md.trim() ? (
        <Markdown text={md} />
      ) : (
        empty
      )}
    </section>
  );
}

function IssuePeek({ api, issue, onClose }: { api: AppApi; issue: Issue; onClose: () => void }) {
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
        <IconButton label="Open issue" icon={<Maximize2 size={15} />} onClick={() => api.open(issue)} />
        <IconButton label="Close" icon={<X size={16} />} onClick={onClose} />
      </div>
      <div className="flex-1 overflow-auto p-4">
        <h2 className="text-[18px] leading-6 font-semibold">{issue.title}</h2>
        <div className="mt-3 flex flex-wrap gap-1.5">
          <StatusMenu
            api={api}
            issue={issue}
            trigger={(status, toggle) => (
              <Chip icon={<StatusIcon status={status} size={12} />} onClick={toggle}>
                {STATUS_LABEL[status]}
              </Chip>
            )}
          />
          {issue.lane && (
            <Chip icon={<GitBranch size={12} />} onClick={() => api.go({ name: 'lane', ref: issue.lane!, view: 'timeline' })}>
              {laneLabel(issue.lane)}
            </Chip>
          )}
          {issue.release && <ReleaseChip version={issue.release} state={releaseStateOf(api, issue)} />}
          {issue.assignee && <Chip icon={<Avatar text={initials(issue.assignee)} size={14} />}>{issue.assignee}</Chip>}
        </div>
        <div className="mt-5">
          <IssueDescription api={api} issue={issue} size="peek" />
        </div>
      </div>
      <div className="border-t px-4 py-2 text-[12px]" style={{ borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
        {RULE}
      </div>
    </aside>
  );
}

// ---------------------------------------------------------------------------
// Screen 6: an issue's own page
// ---------------------------------------------------------------------------

function PropRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <span className="flex h-8 items-center text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
        {label}
      </span>
      <span className="flex min-w-0 items-center">{children}</span>
    </>
  );
}

function PropValue({ icon, children, onClick, muted }: { icon: ReactNode; children: ReactNode; onClick?: () => void; muted?: boolean }) {
  const cls = 'flex h-8 min-w-0 items-center gap-2 rounded-[6px] px-2 text-left text-[13px]';
  const body = (
    <>
      <span className="flex shrink-0 items-center" style={{ color: 'var(--muted-foreground)' }}>
        {icon}
      </span>
      <span className="truncate" style={muted ? { color: 'var(--muted-foreground)' } : undefined}>
        {children}
      </span>
    </>
  );
  return onClick ? (
    <button onClick={onClick} className={`${cls} hover:bg-[var(--surface-hover)]`}>
      {body}
    </button>
  ) : (
    <span className={cls}>{body}</span>
  );
}

function ActivityEntry({ children }: { children: ReactNode }) {
  return (
    <li className="flex items-start gap-2.5 text-[13px] leading-5">
      <Avatar text="BR" size={20} title="Brian" />
      <span className="pt-px">
        {children}
        <span style={{ color: 'var(--muted-foreground)' }}> · just now</span>
      </span>
    </li>
  );
}

function IssuePage({ api, id, from }: { api: AppApi; id: string; from: 'Projects' | 'Squads' }) {
  const issue = api.findIssue(id);
  if (!issue) return <OtherPage label="Issue not found" />;
  const tpl = TEMPLATES.find((t) => t.type === issue.type)!;
  const squad = api.squadOf(issue);
  const found = issue.lane ? findLane(api.orgs, issue.lane) : null;
  const rel = found?.lane.releases?.find((r) => r.version === issue.release);
  const relState = releaseStateOf(api, issue);
  const toLane = issue.lane ? () => api.go({ name: 'lane', ref: issue.lane!, view: 'timeline' }) : undefined;
  const toSquad = squad ? () => api.go({ name: 'squad', squad, view: 'board' }) : undefined;
  const viaLane = issue.lane && (from === 'Projects' || !squad);
  const crumbs = viaLane
    ? [
        { label: 'Projects', onClick: () => api.go({ name: 'projects' }) },
        { label: issue.lane!.project, onClick: () => api.go({ name: 'project', org: issue.lane!.org, project: issue.lane!.project }) },
        { label: issue.lane!.repo, onClick: () => api.go({ name: 'project', org: issue.lane!.org, project: issue.lane!.project }) },
        { label: issue.lane!.lane, onClick: toLane },
        ...(issue.release ? [{ label: issue.release, onClick: toLane }] : []),
        { label: issue.id },
      ]
    : [{ label: 'Squads', onClick: toSquad }, { label: squad ?? 'No squad', onClick: toSquad }, { label: issue.id }];
  const typeLabel = tpl.name.toLowerCase();
  return (
    <>
      <PageHeader icon={viaLane ? GitBranch : Users} crumbs={crumbs} />
      <div className="flex min-h-0 flex-1">
        <div className="min-w-0 flex-1 overflow-auto" style={{ background: 'var(--surface)' }}>
          <article className="mx-auto max-w-[760px] px-10 pt-8 pb-16">
            <div className="flex items-center gap-2">
              <TypeIcon type={issue.type} size={16} />
              <span className="text-[13px] font-medium">{tpl.name}</span>
              <span className="mc-mono text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                {issue.id}
              </span>
            </div>
            <h1 className="mt-2 text-[24px] leading-[30px] font-semibold">{issue.title}</h1>
            <div className="mt-8">
              <IssueDescription api={api} issue={issue} size="page" />
            </div>
            <p className="mt-8 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
              {RULE}
            </p>
            <div className="mt-8 border-t pt-6" style={{ borderColor: 'var(--border)' }}>
              <h2 className="text-[16px] font-semibold">Activity</h2>
              {issue.created ? (
                <ul className="mt-4 space-y-3">
                  <ActivityEntry>
                    <b className="font-medium">Brian</b> created this {typeLabel} from the {tpl.name} template
                  </ActivityEntry>
                  {issue.release && issue.lane && (
                    <ActivityEntry>
                      <b className="font-medium">Brian</b> planned it in {issue.release} on {laneLabel(issue.lane)}
                    </ActivityEntry>
                  )}
                  {squad && (
                    <ActivityEntry>
                      <b className="font-medium">Brian</b> gave it to the {squad} squad
                    </ActivityEntry>
                  )}
                </ul>
              ) : (
                <p className="mt-3 text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
                  Activity from before this prototype is not shown.
                </p>
              )}
            </div>
          </article>
        </div>
        <aside className="w-[320px] shrink-0 overflow-auto border-l p-4" style={{ borderColor: 'var(--border)' }} aria-label="Properties">
          <h2 className="px-2 text-[12px] font-medium" style={{ color: 'var(--muted-foreground)' }}>
            Properties
          </h2>
          <div className="mt-1 grid grid-cols-[88px_1fr] gap-x-2 gap-y-0.5 pl-2">
            <PropRow label="Type">
              <PropValue icon={<TypeIcon type={issue.type} />}>{tpl.name}</PropValue>
            </PropRow>
            <PropRow label="Status">
              <StatusMenu
                api={api}
                issue={issue}
                trigger={(status, toggle) => (
                  <PropValue icon={<StatusIcon status={status} />} onClick={toggle}>
                    {STATUS_LABEL[status]}
                  </PropValue>
                )}
              />
            </PropRow>
            <PropRow label="Priority">
              <PropValue icon={<Ellipsis size={14} />} muted={(issue.priority ?? 'none') === 'none'}>
                {PRIORITY_LABEL[issue.priority ?? 'none']}
              </PropValue>
            </PropRow>
            <PropRow label="Squad">
              <PropValue icon={<Users size={14} />} onClick={toSquad} muted={!squad}>
                {squad ?? 'No squad'}
              </PropValue>
            </PropRow>
            <PropRow label="Assignee">
              <PropValue icon={issue.assignee ? <Avatar text={initials(issue.assignee)} size={16} /> : <CircleUser size={14} />} muted={!issue.assignee}>
                {issue.assignee ?? 'Unassigned'}
              </PropValue>
            </PropRow>
            <PropRow label="Worklane">
              <PropValue icon={<GitBranch size={14} />} onClick={toLane} muted={!issue.lane}>
                {issue.lane ? laneLabel(issue.lane) : 'No worklane'}
              </PropValue>
            </PropRow>
            <PropRow label="Release">
              {issue.release ? (
                <button onClick={toLane} className="flex h-8 items-center rounded-[6px] px-1.5 hover:bg-[var(--surface-hover)]">
                  <ReleaseChip version={issue.release} state={relState} />
                </button>
              ) : (
                <PropValue icon={<CircleDashed size={14} />} muted>
                  No release
                </PropValue>
              )}
            </PropRow>
          </div>
          {rel && issue.lane && (
            <section className="mt-5 rounded-[12px] border p-3" style={{ borderColor: 'var(--surface-border)', background: 'var(--surface)' }} aria-label="Release">
              <div className="flex items-center gap-2">
                <ReleaseChip version={rel.version} state={relState} />
                <span className="text-[12px]" style={{ color: relState === 'next' ? 'var(--brand)' : 'var(--muted-foreground)' }}>
                  {STATE_LABEL[relState]}
                </span>
              </div>
              <div className="mt-2 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                {laneLabel(issue.lane)} · {issue.lane.repo}
              </div>
              <div className="mt-3 flex items-center gap-2">
                <div className="flex-1">
                  <Progress done={doneCount(rel, api.statusOf)} total={rel.issues.length} />
                </div>
                <span className="text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                  {doneCount(rel, api.statusOf)} of {rel.issues.length} done
                </span>
              </div>
              <div className="mt-2">
                <Button variant="ghost" icon={<GitBranch size={14} />} onClick={toLane}>
                  Open timeline
                </Button>
              </div>
            </section>
          )}
        </aside>
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// Screen 5: a new issue always starts from a template
// ---------------------------------------------------------------------------

function NewIssueDialog({ api, draft, onClose, onCreate }: { api: AppApi; draft: Draft; onClose: () => void; onCreate: (d: Draft, title: string, description: string, p: Priority) => void }) {
  const [d, setD] = useState<Draft>(draft);
  const [title, setTitle] = useState('');
  const [fields, setFields] = useState<Record<string, string>>({});
  const [priority, setPriority] = useState<Priority>('none');
  const tpl = TEMPLATES.find((t) => t.type === d.type)!;
  const lanes = useMemo(() => allLanes(api.orgs), [api.orgs]);
  const laneObj = d.lane ? findLane(api.orgs, d.lane)?.lane : undefined;
  const [pane, setPane] = useState<'fields' | 'markdown'>('fields');
  const inputs = templateInputs(tpl);
  // The fields only fill the template's Markdown; the issue keeps that Markdown alone.
  const markdown = templateMarkdown(tpl, fields);
  const valid = title.trim() !== '' && inputs.every((s) => !s.required || (fields[s.key] ?? '').trim() !== '');
  const submit = () => valid && onCreate(d, title.trim(), markdown, priority);

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
          <div className="w-[272px] shrink-0 overflow-auto border-r p-2" style={{ background: 'var(--page-canvas)', borderColor: 'var(--border)' }} role="radiogroup" aria-label="Template">
            <div className="px-2 pt-1 pb-2 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
              Template
            </div>
            {TEMPLATES.map((t) => (
              <button
                key={t.type}
                role="radio"
                aria-checked={t.type === d.type}
                onClick={() => setD({ ...d, type: t.type })}
                className="flex w-full items-center gap-2 rounded-[8px] px-2 py-2 text-left hover:bg-[var(--surface-hover)]"
                style={t.type === d.type ? { background: 'var(--surface-selected)' } : undefined}
              >
                <TypeIcon type={t.type} size={15} />
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
              className="w-full bg-transparent text-[16px] font-medium outline-none placeholder:text-[var(--faint-foreground)]"
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
                width={380}
                trigger={(_, toggle) => (
                  <Chip icon={<GitBranch size={13} />} onClick={toggle}>
                    {d.lane ? laneLabel(d.lane) : 'No worklane'}
                  </Chip>
                )}
              >
                {(close) => (
                  <div className="max-h-[320px] overflow-auto">
                    {[...lanes].sort((a, b) => Number(sameLane(b.ref, d.lane)) - Number(sameLane(a.ref, d.lane))).map(({ ref }) => (
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
                        <span className="ml-1.5 mc-mono text-[11px]" style={{ color: 'var(--muted-foreground)' }}>
                          {ref.repo}
                        </span>
                      </MenuItem>
                    ))}
                  </div>
                )}
              </Menu>
              <Menu
                width={260}
                trigger={(_, toggle) => (
                  <Chip icon={<CircleDashed size={13} />} onClick={toggle}>
                    <span>{d.release ?? 'No release'}</span>
                  </Chip>
                )}
              >
                {(close) => (
                  <>
                    {(laneObj?.releases ?? []).map((r) => (
                      <MenuItem key={r.version} selected={r.version === d.release} onClick={() => (setD({ ...d, release: r.version }), close())}>
                        <span className="mc-mono">{r.version}</span>
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
            <div className="mt-5">
              <Segmented
                value={pane}
                options={[
                  { value: 'fields', label: 'Fields' },
                  { value: 'markdown', label: 'Markdown' },
                ]}
                onChange={setPane}
              />
            </div>
            {pane === 'fields' ? (
              <div className="mt-4 space-y-4">
                {inputs.map((s) => (
                  <label key={`${d.type}-${s.key}`} className="block">
                    <span className="flex items-baseline gap-2">
                      <span className="text-[13px] font-medium">{s.label}</span>
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
            ) : (
              <pre
                aria-label="Description Markdown"
                className="mc-mono mt-4 whitespace-pre-wrap rounded-[8px] border px-3 py-2.5 text-[12px] leading-[19px]"
                style={{ borderColor: 'var(--surface-border)', background: 'var(--page-canvas)' }}
              >
                {markdown}
              </pre>
            )}
            <p className="mt-4 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
              These fields fill the {tpl.name} template's Markdown. The issue keeps only that Markdown as its description; code and technical design go in the change.
            </p>
          </div>
        </div>
        <div className="flex h-14 shrink-0 items-center justify-end gap-2 border-t px-4" style={{ borderColor: 'var(--border)' }}>
          <Button
            variant="ghost"
            onClick={() => {
              setTitle(tpl.example.title);
              setFields({ ...tpl.example.fields });
              setPriority(tpl.example.priority);
            }}
          >
            Fill example
          </Button>
          <div className="flex-1" />
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
