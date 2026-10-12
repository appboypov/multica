import { useCallback, useEffect, useMemo, useRef, useState, type DragEvent, type ReactNode } from 'react';
import {
  ArrowLeft,
  ArrowRight,
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
  Pencil,
  Pin,
  Plus,
  Search,
  Settings,
  SquarePen,
  Tag,
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
  type ReleaseHead,
  type ReleaseState,
  type Repo,
  type Status,
  PRIORITY_LABEL,
  STATUS_LABEL,
  TEMPLATES,
  TYPE_LABEL,
  doneCount,
  initials,
  laneNow,
  laneCount,
  laneIssues,
  laneLabel,
  laneNextIdx,
  lineOf,
  loadOrgs,
  moveRelease,
  plural,
  releaseStates,
  sameLane,
  templateInputs,
  templateMarkdown,
} from './model';
import { AddRepositoryDialog, NewWorklaneDialog, ReleaseDialog } from './creation';
import { Avatar, Button, Chip, IconButton, Markdown, Menu, MenuItem, Overlay, Progress, ReleaseChip, Segmented, StatusIcon, TypeIcon } from './ui';

// ---------------------------------------------------------------------------
// Routing and app state
// ---------------------------------------------------------------------------

type LaneView = 'timeline' | 'board' | 'list';

type Route =
  | { name: 'projects' }
  | { name: 'organisations' }
  | { name: 'organisation'; org: string }
  | { name: 'worklanes' }
  | { name: 'releases' }
  | { name: 'project'; org: string; project: string; addLane?: string }
  /** `release`: the id of the release the timeline opens with in view. */
  | { name: 'lane'; ref: LaneRef; view: LaneView; release?: string }
  | { name: 'other'; label: string }
  | { name: 'issue'; id: string };

interface Draft {
  type: IssueType;
  lane?: LaneRef;
  release?: string;
  status?: Status;
}

interface AppApi {
  orgs: Org[];
  statusOf: (i: Issue) => Status;
  setStatus: (id: string, s: Status) => void;
  /** The day the last of these issues was done in this session: a shipped release's date. */
  shipDate: (issues: Issue[]) => string | undefined;
  /** The issue's one Markdown description, with edits made in this session. */
  descriptionOf: (i: Issue) => string;
  setDescription: (id: string, md: string) => void;
  go: (r: Route) => void;
  newIssue: (d?: Partial<Draft>) => void;
  peek: (i: Issue) => void;
  /** Opens the issue's own page. */
  open: (i: Issue) => void;
  findIssue: (id: string) => Issue | null;
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

export const MulticaSteerAllWorkPrototype = () => {
  const [orgs, setOrgs] = useState<Org[]>(loadOrgs);
  // Issues made without a worklane: Multica keeps them on its Issues page.
  const [looseIssues, setLooseIssues] = useState<Issue[]>([]);
  const [statuses, setStatuses] = useState<Record<string, Status>>({});
  // The day each issue was set to done in this session.
  const [doneOn, setDoneOn] = useState<Record<string, number>>({});
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
      const f = route.name === 'lane' ? findLane(orgs, route.ref) : null;
      const ctx: Partial<Draft> = f && route.name === 'lane' ? { lane: route.ref, release: f.lane.releases?.[laneNextIdx(f.repo, f.lane, statusOf)]?.version } : {};
      setDraft({ type: 'story', ...ctx, ...d });
    },
    [route, orgs, statusOf],
  );

  const api: AppApi = {
    orgs,
    statusOf,
    setStatus: (id, s) => {
      setStatuses((m) => ({ ...m, [id]: s }));
      setDoneOn((m) => (s === 'done' ? { ...m, [id]: Date.now() } : Object.fromEntries(Object.entries(m).filter(([k]) => k !== id))));
    },
    shipDate: (issues) => {
      const last = Math.max(0, ...issues.map((i) => doneOn[i.id] ?? 0));
      return last ? new Date(last).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' }) : undefined;
    },
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
      setRoute({ name: 'issue', id: i.id });
    },
    findIssue: (id) =>
      allLanes(orgs)
        .flatMap((l) => laneIssues(l.lane))
        .concat(looseIssues)
        .find((i) => i.id === id) ?? null,
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
    const used = [...allLanes(orgs).flatMap((l) => laneIssues(l.lane)), ...looseIssues]
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
      created: true,
    };
    // An issue made in the Board's Done column is done from the start, so its release can show when it shipped.
    if (issue.status === 'done') setDoneOn((m) => ({ ...m, [id]: Date.now() }));
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
    } else setLooseIssues((l) => [...l, issue]);
    setDraft(null);
    setFresh(id);
    setPeeked(null);
    setRoute({ name: 'issue', id });
    toast(`${id} created${d.release ? ` in ${d.release}` : ''}`);
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

  /**
   * Writes the repository's release line to every worklane, renumbered: each worklane keeps its own issues in
   * each release, and every issue follows its release to the new version. Planning a release also takes the
   * picked issues out of the worklane's backlog; they are ready to pick up.
   */
  const saveLine = (ref: LaneRef, line: ReleaseHead[], into?: { release: string; ids: string[] }) => {
    const found = findLane(orgs, ref);
    if (!found) return;
    const byId = new Map(line.map((h) => [h.id, h.version]));
    const renamed = new Map(lineOf(found.repo).map((h) => [h.version, byId.get(h.id) ?? h.version]));
    const inRepo = (i: Issue) => !!i.lane && i.lane.org === ref.org && i.lane.project === ref.project && i.lane.repo === ref.repo;
    const follow = (i: Issue): Issue => (inRepo(i) && i.release && renamed.has(i.release) ? { ...i, release: renamed.get(i.release) } : i);
    const target = into ? line.find((h) => h.id === into.release)!.version : null;
    const ready = (i: Issue): Issue => ({ ...i, release: target!, status: i.status === 'backlog' ? 'todo' : i.status });
    updateLanes(ref, (ls) =>
      ls.map((l) => {
        const moved = into && l.name === ref.lane ? (l.backlog ?? []).filter((i) => into.ids.includes(i.id)) : [];
        return {
          ...l,
          releases: line.map((h) => ({
            ...h,
            issues: [...(l.releases?.find((r) => r.id === h.id)?.issues ?? []).map(follow), ...(h.id === into?.release ? moved.map(ready) : [])],
          })),
          backlog: moved.length ? (l.backlog ?? []).filter((i) => !into!.ids.includes(i.id)) : l.backlog,
        };
      }),
    );
  };

  let page: ReactNode;
  if (route.name === 'projects') page = <ProjectsPage api={api} onAddProject={addProject} />;
  else if (route.name === 'organisations') page = <OrganisationsPage api={api} />;
  else if (route.name === 'organisation') page = <OrganisationPage key={route.org} api={api} orgName={route.org} />;
  else if (route.name === 'worklanes') page = <WorklanesPage api={api} />;
  else if (route.name === 'releases') page = <ReleasesPage api={api} />;
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
            {
              name: lane.name,
              folder: lane.folder,
              state: 'Free',
              active: null,
              next: null,
              // A new worklane holds the repository's release line, with none of its issues yet.
              releases: (ls[0]?.releases ?? []).map((r) => ({ ...r, issues: [] })),
              backlog: [],
            },
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
        focusRelease={route.release}
        onSaveLine={(line, into) => saveLine(route.ref, line, into)}
      />
    );
  else if (route.name === 'issue') page = <IssuePage key={route.id} api={api} id={route.id} />;
  else page = <OtherPage label={route.label} />;

  return (
    <div className="mc-root flex h-screen w-full overflow-hidden text-[14px]">
      <Sidebar api={api} route={route} onSearch={() => setPalette(true)} />
      <main className="flex min-w-0 flex-1 flex-col overflow-hidden border-l" style={{ background: 'var(--page-canvas)', borderColor: 'var(--surface-border)' }}>
        {page}
      </main>
      {peeked && <IssuePeek api={api} issue={peeked} onClose={() => setPeeked(null)} />}
      {draft && <NewIssueDialog api={api} draft={draft} onClose={() => setDraft(null)} onCreate={createIssue} />}
      {palette && <Palette api={api} looseIssues={looseIssues} onClose={() => setPalette(false)} />}
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
  // An issue on a worklane is found through Projects; one without a worklane on Multica's Issues page.
  const active =
    route.name === 'projects' || route.name === 'project' || route.name === 'lane'
      ? 'Projects'
      : route.name === 'organisations' || route.name === 'organisation'
        ? 'Organisations'
        : route.name === 'worklanes'
          ? 'Worklanes'
          : route.name === 'releases'
            ? 'Releases'
            : route.name === 'issue'
              ? api.findIssue(route.id)?.lane
                ? 'Projects'
                : 'Issues'
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
      {item('Organisations', Building2, () => api.go({ name: 'organisations' }))}
      {item('Worklanes', GitBranch, () => api.go({ name: 'worklanes' }))}
      {item('Releases', Tag, () => api.go({ name: 'releases' }))}
      {item('Autopilot', Zap, other('Autopilot'))}
      {section('AI Team')}
      {item('Agents', Bot, other('Agents'))}
      {item('Squads', Users, other('Squads'))}
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
// Release chain: last shipped, the release this worklane works toward with progress, its later releases
// ---------------------------------------------------------------------------

function ReleaseChain({ api, lane, refLane, max = 2 }: { api: AppApi; lane: Lane; refLane: LaneRef; max?: number }) {
  const repo = findLane(api.orgs, refLane)?.repo;
  const rels = lane.releases ?? [];
  if (!repo || repo.noReleases || !rels.length)
    return (
      <span className="text-[12px]" style={{ color: 'var(--faint-foreground)' }}>
        {repo?.noReleases ? 'Does not release' : 'No releases planned'}
      </span>
    );
  const states = releaseStates(repo, api.statusOf);
  const shipped = rels.filter((_, i) => states[i] === 'shipped');
  const nextIdx = laneNextIdx(repo, lane, api.statusOf);
  // Later releases that hold this worklane's issues; the others are the repository's, empty here.
  const planned = rels.filter((r, i) => i > nextIdx && nextIdx >= 0 && r.issues.length > 0).slice(0, max - 1);
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
          <ReleaseChip version={rels[nextIdx].version} state={states[nextIdx]} onClick={open} />
          <span className="text-[12px] tabular-nums" style={{ color: 'var(--muted-foreground)' }}>
            {doneCount(rels[nextIdx], api.statusOf)}/{rels[nextIdx].issues.length}
          </span>
        </>
      )}
      {planned.map((r) => (
        <span key={r.id} className="flex items-center gap-1 max-[1000px]:hidden">
          {line}
          <ReleaseChip version={r.version} state="planned" onClick={open} />
        </span>
      ))}
    </div>
  );
}

function LaneRow({ api, lane, refLane, indent }: { api: AppApi; lane: Lane; refLane: LaneRef; indent: number }) {
  const now = laneNow(lane, api.statusOf);
  const active = now?.active;
  return (
    <div
      role="link"
      tabIndex={0}
      onClick={() => api.go({ name: 'lane', ref: refLane, view: 'timeline' })}
      onKeyDown={(e) => e.key === 'Enter' && api.go({ name: 'lane', ref: refLane, view: 'timeline' })}
      className="group grid cursor-pointer items-center gap-4 border-t py-2 pr-4 hover:bg-[var(--surface-hover)]"
      style={{ gridTemplateColumns: `minmax(160px,400px) minmax(max-content,1fr) 16px`, paddingLeft: indent, borderColor: 'var(--border)' }}
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

function OrgTile({ size = 24 }: { size?: number }) {
  return (
    <span
      className="flex shrink-0 items-center justify-center rounded-[6px] border"
      style={{ width: size, height: size, background: 'var(--muted)', borderColor: 'var(--surface-border)', color: 'var(--muted-foreground)' }}
    >
      <Building2 size={size > 30 ? 22 : 14} />
    </span>
  );
}

function toggled(set: Set<string>, key: string) {
  const n = new Set(set);
  if (n.has(key)) n.delete(key);
  else n.add(key);
  return n;
}

function TreeHead() {
  return (
    <div className="sticky top-0 z-10 grid gap-4 border-b py-3 pr-4 pl-4 text-[12px]" style={{ gridTemplateColumns: 'minmax(160px,400px) 1fr', background: 'var(--page-canvas)', borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
      <span>Name</span>
      <span className="whitespace-nowrap">Last · Next · Planned releases</span>
    </div>
  );
}

/**
 * An organisation's projects as a tree: project, its repositories with their branch, their worklanes. `rows`
 * replaces a repository's worklanes with other rows, for a page that only lists; `meta` the project's counts.
 */
function ProjectTree({
  api,
  org,
  projects,
  openProjects,
  onToggleProject,
  closedRepos,
  onToggleRepo,
  rows,
  meta,
}: {
  api: AppApi;
  org: string;
  projects: Project[];
  openProjects: Set<string>;
  onToggleProject: (key: string) => void;
  closedRepos: Set<string>;
  onToggleRepo: (key: string) => void;
  rows?: (repo: Repo, project: Project) => ReactNode;
  meta?: (project: Project) => string;
}) {
  return (
    <>
      {projects.map((p) => {
        const key = `${org}/${p.name}`;
        const pOpen = openProjects.has(key);
        const lc = laneCount(p);
        return (
          <div key={key} style={{ background: 'var(--surface)' }}>
            <div className="flex h-12 items-center gap-3 border-t px-4" style={{ borderColor: 'var(--border)' }}>
              <button
                aria-label={pOpen ? `Collapse ${p.name}` : `Expand ${p.name}`}
                aria-expanded={pOpen}
                onClick={() => onToggleProject(key)}
                className="rounded-[4px] p-0.5 hover:bg-[var(--surface-hover)]"
              >
                <Disclosure open={pOpen} />
              </button>
              <ProjectTile name={p.name} />
              <button onClick={() => api.go({ name: 'project', org, project: p.name })} className="truncate text-[14px] font-medium hover:underline">
                {p.name}
              </button>
              <div className="flex-1" />
              <span className="text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
                {meta ? meta(p) : p.repos.length ? `${plural(p.repos.length, 'repository', 'repositories')} · ${plural(lc, 'worklane')}` : 'No repositories'}
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
                        onClick={() => onToggleRepo(rKey)}
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
                      {!rows && (
                        <Button variant="ghost" icon={<Plus size={14} />} onClick={() => api.go({ name: 'project', org, project: p.name, addLane: r.name })}>
                          Worklane
                        </Button>
                      )}
                    </div>
                    {rOpen && rows?.(r, p)}
                    {rOpen &&
                      !rows &&
                      r.lanes.map((l) => (
                        <LaneRow key={l.name} api={api} lane={l} refLane={{ org, project: p.name, repo: r.name, lane: l.name }} indent={64} />
                      ))}
                    {rOpen && !rows && !r.lanes.length && (
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
                <button className="underline" onClick={() => api.go({ name: 'project', org, project: p.name })}>
                  Open the project
                </button>{' '}
                to add one.
              </div>
            )}
          </div>
        );
      })}
    </>
  );
}

/** An organisation's group header: its name opens its page; the rest of the row folds what it groups. */
function OrgHeader({ api, org, open, onToggle, meta }: { api: AppApi; org: string; open: boolean; onToggle: () => void; meta: string }) {
  return (
    <div onClick={onToggle} className="flex h-[39px] w-full cursor-pointer items-center gap-2 px-4 hover:bg-[var(--surface-hover)]">
      <button
        aria-label={open ? `Collapse ${org}` : `Expand ${org}`}
        aria-expanded={open}
        onClick={(e) => (e.stopPropagation(), onToggle())}
        className="flex items-center rounded-[4px] p-0.5"
      >
        <Disclosure open={open} />
      </button>
      <Building2 size={14} style={{ color: 'var(--muted-foreground)' }} />
      <button onClick={(e) => (e.stopPropagation(), api.go({ name: 'organisation', org }))} className="text-[13px] font-semibold hover:underline">
        {org}
      </button>
      <span className="text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
        {meta}
      </span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Screen 4c: every worklane of the workspace, grouped by organisation, project and repository
// ---------------------------------------------------------------------------

function WorklanesPage({ api }: { api: AppApi }) {
  const [closedOrgs, setClosedOrgs] = useState<Set<string>>(new Set());
  const [closedProjects, setClosedProjects] = useState<Set<string>>(new Set());
  const [closedRepos, setClosedRepos] = useState<Set<string>>(new Set());
  // Only the groups that hold worklanes.
  const groups = api.orgs
    .map((o) => ({ ...o, projects: o.projects.map((p) => ({ ...p, repos: p.repos.filter((r) => r.lanes.length) })).filter((p) => p.repos.length) }))
    .filter((o) => o.projects.length);
  const openProjects = new Set(groups.flatMap((o) => o.projects.map((p) => `${o.name}/${p.name}`)).filter((k) => !closedProjects.has(k)));
  return (
    <>
      <PageHeader icon={GitBranch} crumbs={[{ label: 'Worklanes' }]} count={allLanes(api.orgs).length} />
      <div className="flex-1 overflow-auto px-6 pb-10">
        <TreeHead />
        {groups.map((o) => {
          const open = !closedOrgs.has(o.name);
          return (
            <section key={o.name}>
              <OrgHeader
                api={api}
                org={o.name}
                open={open}
                onToggle={() => setClosedOrgs(toggled(closedOrgs, o.name))}
                meta={plural(o.projects.reduce((n, p) => n + laneCount(p), 0), 'worklane')}
              />
              {open && (
                <ProjectTree
                  api={api}
                  org={o.name}
                  projects={o.projects}
                  openProjects={openProjects}
                  onToggleProject={(k) => setClosedProjects(toggled(closedProjects, k))}
                  closedRepos={closedRepos}
                  onToggleRepo={(k) => setClosedRepos(toggled(closedRepos, k))}
                />
              )}
            </section>
          );
        })}
        {!groups.length && <EmptyNote text="No worklanes yet." />}
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// Screen 4d: every repository's release line, grouped by organisation, project and repository
// ---------------------------------------------------------------------------

// The name column takes what is left, so Issues and Worklanes line up between the header and the indented rows.
const RELEASE_COLUMNS = 'minmax(0,1fr) 140px 200px 16px';

function ReleasesPage({ api }: { api: AppApi }) {
  const releases = (r: Repo) => lineOf(r).length;
  const [closedOrgs, setClosedOrgs] = useState<Set<string>>(new Set());
  const [closedProjects, setClosedProjects] = useState<Set<string>>(new Set());
  const [closedRepos, setClosedRepos] = useState<Set<string>>(new Set());
  // Only the groups that hold releases, and repositories that say they do not release.
  const groups = api.orgs
    .map((o) => ({ ...o, projects: o.projects.map((p) => ({ ...p, repos: p.repos.filter((r) => releases(r) || r.noReleases) })).filter((p) => p.repos.length) }))
    .filter((o) => o.projects.length);
  const openProjects = new Set(groups.flatMap((o) => o.projects.map((p) => `${o.name}/${p.name}`)).filter((k) => !closedProjects.has(k)));
  const projectReleases = (p: Project) => p.repos.reduce((k, r) => k + releases(r), 0);
  const total = groups.reduce((n, o) => n + o.projects.reduce((m, p) => m + projectReleases(p), 0), 0);
  return (
    <>
      <PageHeader icon={Tag} crumbs={[{ label: 'Releases' }]} count={total} />
      <div className="flex-1 overflow-auto px-6 pb-10">
        <div
          className="sticky top-0 z-10 grid gap-4 border-b py-3 pr-4 pl-4 text-[12px]"
          style={{ gridTemplateColumns: RELEASE_COLUMNS, background: 'var(--page-canvas)', borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}
        >
          <span>Name</span>
          <span>Issues</span>
          <span>Worklanes</span>
          <span />
        </div>
        {groups.map((o) => {
          const open = !closedOrgs.has(o.name);
          return (
            <section key={o.name}>
              <OrgHeader
                api={api}
                org={o.name}
                open={open}
                onToggle={() => setClosedOrgs(toggled(closedOrgs, o.name))}
                meta={plural(o.projects.reduce((n, p) => n + projectReleases(p), 0), 'release')}
              />
              {open && (
                <ProjectTree
                  api={api}
                  org={o.name}
                  projects={o.projects}
                  openProjects={openProjects}
                  onToggleProject={(k) => setClosedProjects(toggled(closedProjects, k))}
                  closedRepos={closedRepos}
                  onToggleRepo={(k) => setClosedRepos(toggled(closedRepos, k))}
                  meta={(p) => `${plural(p.repos.length, 'repository', 'repositories')} · ${plural(projectReleases(p), 'release')}`}
                  rows={(r, p) => {
                    if (r.noReleases)
                      return (
                        <div className="border-t py-2.5 pl-16 text-[13px]" style={{ borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
                          This repository does not release
                        </div>
                      );
                    const states = releaseStates(r, api.statusOf);
                    return lineOf(r).map((head, i) => (
                      <ReleaseRow key={head.id} api={api} repo={r} refRepo={{ org: o.name, project: p.name, repo: r.name }} head={head} index={i} state={states[i]} />
                    ));
                  }}
                />
              )}
            </section>
          );
        })}
        {!groups.length && <EmptyNote text="No releases yet." />}
      </div>
    </>
  );
}

/** One release of a repository's line; it opens the timeline of the first worklane with issues in it, else `main`. */
function ReleaseRow({
  api,
  repo,
  refRepo,
  head,
  index,
  state,
}: {
  api: AppApi;
  repo: Repo;
  refRepo: Omit<LaneRef, 'lane'>;
  head: ReleaseHead;
  index: number;
  state: ReleaseState;
}) {
  const lanes = repo.lanes.filter((l) => l.releases?.[index]?.issues.length);
  const issues = lanes.flatMap((l) => l.releases![index].issues);
  const done = issues.filter((i) => api.statusOf(i) === 'done').length;
  const target = lanes[0] ?? repo.lanes.find((l) => l.name === 'main') ?? repo.lanes[0];
  const open = () => target && api.go({ name: 'lane', ref: { ...refRepo, lane: target.name }, view: 'timeline', release: head.id });
  const muted = { color: 'var(--muted-foreground)' };
  return (
    <div
      role="link"
      tabIndex={0}
      onClick={open}
      onKeyDown={(e) => e.key === 'Enter' && open()}
      className="grid cursor-pointer items-center gap-4 border-t py-2.5 pr-4 pl-16 hover:bg-[var(--surface-hover)]"
      style={{ gridTemplateColumns: RELEASE_COLUMNS, borderColor: 'var(--border)' }}
    >
      <div className="flex min-w-0 items-center gap-3">
        <ReleaseChip version={head.version} state={state} />
        <span className="truncate text-[14px] font-medium">{head.title}</span>
        {head.pinned && <SetByHand />}
        {state === 'shipped' && (
          <span className="shrink-0 text-[12px] tabular-nums" style={muted}>
            {api.shipDate(issues)}
          </span>
        )}
      </div>
      <div className="flex items-center gap-2">
        <div className="w-16">
          <Progress done={done} total={issues.length} />
        </div>
        <span className="text-[12px] tabular-nums" style={muted}>
          {done}/{issues.length}
        </span>
      </div>
      <div className="flex min-w-0 flex-wrap gap-1">
        {lanes.length ? (
          lanes.map((l) => (
            <Chip key={l.name} icon={<GitBranch size={13} />}>
              {l.name}
            </Chip>
          ))
        ) : (
          <span className="text-[12px]" style={{ color: 'var(--faint-foreground)' }}>
            No issues yet
          </span>
        )}
      </div>
      <ChevronRight size={16} style={muted} />
    </div>
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
        <TreeHead />
        {visibleOrgs.map((o) => {
          const orgOpen = openOrgs.has(o.name);
          const lanes = o.projects.reduce((n, p) => n + laneCount(p), 0);
          return (
            <section key={o.name}>
              <OrgHeader
                api={api}
                org={o.name}
                open={orgOpen}
                onToggle={() => setOpenOrgs(toggled(openOrgs, o.name))}
                meta={`${plural(o.projects.length, 'project')}${lanes ? ` · ${plural(lanes, 'worklane')}` : ''}`}
              />
              {orgOpen && (
                <ProjectTree
                  api={api}
                  org={o.name}
                  projects={o.projects}
                  openProjects={openProjects}
                  onToggleProject={(k) => setOpenProjects(toggled(openProjects, k))}
                  closedRepos={closedRepos}
                  onToggleRepo={(k) => setClosedRepos(toggled(closedRepos, k))}
                />
              )}
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
// Screen 4: the organisations, each grouping projects
// ---------------------------------------------------------------------------

const ORG_COLUMNS = 'minmax(160px,1fr) 140px 140px 16px';

function OrganisationsPage({ api }: { api: AppApi }) {
  return (
    <>
      <PageHeader icon={Building2} crumbs={[{ label: 'Organisations' }]} count={api.orgs.length} />
      <div className="flex-1 overflow-auto px-6 pb-10">
        <div className="sticky top-0 z-10 grid gap-4 border-b py-3 px-4 text-[12px]" style={{ gridTemplateColumns: ORG_COLUMNS, background: 'var(--page-canvas)', borderColor: 'var(--border)', color: 'var(--muted-foreground)' }}>
          <span>Name</span>
          <span>Projects</span>
          <span>Repositories</span>
          <span />
        </div>
        <div style={{ background: 'var(--surface)' }}>
          {api.orgs.map((o) => (
            <div
              key={o.name}
              role="link"
              tabIndex={0}
              onClick={() => api.go({ name: 'organisation', org: o.name })}
              onKeyDown={(e) => e.key === 'Enter' && api.go({ name: 'organisation', org: o.name })}
              className="grid h-12 cursor-pointer items-center gap-4 border-t px-4 hover:bg-[var(--surface-hover)]"
              style={{ gridTemplateColumns: ORG_COLUMNS, borderColor: 'var(--border)' }}
            >
              <span className="flex min-w-0 items-center gap-3">
                <OrgTile />
                <span className="truncate text-[14px] font-medium">{o.name}</span>
              </span>
              <span className="text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
                {plural(o.projects.length, 'project')}
              </span>
              <span className="text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
                {plural(
                  o.projects.reduce((n, p) => n + p.repos.length, 0),
                  'repository',
                  'repositories',
                )}
              </span>
              <ChevronRight size={16} style={{ color: 'var(--faint-foreground)' }} />
            </div>
          ))}
        </div>
      </div>
    </>
  );
}

// ---------------------------------------------------------------------------
// Screen 4b: one organisation and its projects, as the tree of screen 1
// ---------------------------------------------------------------------------

function OrganisationPage({ api, orgName }: { api: AppApi; orgName: string }) {
  const org = api.orgs.find((o) => o.name === orgName);
  // Projects with repositories start open; the others stay folded.
  const [openProjects, setOpenProjects] = useState<Set<string>>(() => new Set(org?.projects.filter((p) => p.repos.length).map((p) => `${orgName}/${p.name}`)));
  const [closedRepos, setClosedRepos] = useState<Set<string>>(new Set());
  if (!org) return <OtherPage label="Organisation not found" />;
  const counts: [LucideIcon, string][] = [
    [FolderKanban, plural(org.projects.length, 'project')],
    [
      FolderGit2,
      plural(
        org.projects.reduce((n, p) => n + p.repos.length, 0),
        'repository',
        'repositories',
      ),
    ],
    [GitBranch, plural(org.projects.reduce((n, p) => n + laneCount(p), 0), 'worklane')],
  ];
  return (
    <>
      <PageHeader icon={Building2} crumbs={[{ label: 'Organisations', onClick: () => api.go({ name: 'organisations' }) }, { label: org.name }]} />
      <div className="flex-1 overflow-auto px-6 pb-10">
        <div className="flex items-start gap-4 py-6">
          <OrgTile size={48} />
          <div className="min-w-0 flex-1">
            <h1 className="text-[20px] font-semibold">{org.name}</h1>
            <div className="mt-2 flex flex-wrap items-center gap-4 text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
              {counts.map(([Icon, text]) => (
                <span key={text} className="flex items-center gap-1.5">
                  <Icon size={14} />
                  {text}
                </span>
              ))}
            </div>
          </div>
        </div>
        <TreeHead />
        <ProjectTree
          api={api}
          org={org.name}
          projects={org.projects}
          openProjects={openProjects}
          onToggleProject={(k) => setOpenProjects(toggled(openProjects, k))}
          closedRepos={closedRepos}
          onToggleRepo={(k) => setClosedRepos(toggled(closedRepos, k))}
        />
        {!org.projects.length && <EmptyNote text={`${org.name} has no projects yet.`} />}
      </div>
    </>
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
  onAddLane: (repo: string, lane: { name: string; folder: string }) => void;
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
  return found && idx >= 0 ? releaseStates(found.repo, api.statusOf)[idx] : 'planned';
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

/** Saves a new release line; `into` puts the picked backlog issues of this worklane into one release of it. */
type SaveLine = (line: ReleaseHead[], into?: { release: string; ids: string[] }) => void;

function LanePage({
  api,
  refLane,
  view,
  focusRelease,
  onSaveLine,
}: {
  api: AppApi;
  refLane: LaneRef;
  view: LaneView;
  /** The id of the release the timeline opens with in view. */
  focusRelease?: string;
  onSaveLine: SaveLine;
}) {
  const found = findLane(api.orgs, refLane);
  const [types, setTypes] = useState<string[]>([]);
  const [planning, setPlanning] = useState(false);
  const [editing, setEditing] = useState<string | null>(null);
  const timeline = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const box = timeline.current;
    const col = box && [...box.querySelectorAll<HTMLElement>('[data-release]')].find((e) => e.dataset.release === focusRelease);
    if (box && col) box.scrollLeft += col.getBoundingClientRect().left - box.getBoundingClientRect().left - 24;
  }, [focusRelease, view]);
  if (!found) return <OtherPage label="Worklane not found" />;
  const { lane, repo } = found;
  const backlog = lane.backlog ?? [];
  const rels = lane.releases ?? [];
  const states = releaseStates(repo, api.statusOf);
  const nextRel = rels[laneNextIdx(repo, lane, api.statusOf)]?.version;
  const filt = (is: Issue[]) => (types.length ? is.filter((i) => types.includes(i.type)) : is);
  const shippedAny = states.includes('shipped');
  const nxt = laneNow(lane, api.statusOf);
  const setView = (v: LaneView) => api.go({ name: 'lane', ref: refLane, view: v });
  const editRel = rels.find((r) => r.id === editing);
  const actions = (rel: Release, i: number) => (
    <ReleaseActions api={api} repo={repo} rel={rel} index={i} refLane={refLane} onEdit={() => setEditing(rel.id)} onSaveLine={onSaveLine} />
  );

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
            <Button variant="primary" icon={<Plus size={15} />} onClick={() => api.newIssue({ lane: refLane, release: nextRel })}>
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
          {lane.state} · {repo.noReleases ? 'does not release' : shippedAny ? `last shipped ${rels[states.lastIndexOf('shipped')].version}` : 'nothing shipped yet'}
          {nxt ? ` · ${nxt.active ? 'now' : 'next'}: ${nxt.id} ${nxt.title}` : ''}
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
        <div ref={timeline} className="flex-1 overflow-auto px-6 pt-6 pb-10">
          <div className="flex items-start gap-4">
            {rels.map((rel, i) => (
              <ReleaseColumn key={rel.id} api={api} rel={rel} state={states[i]} issues={filt(rel.issues)} refLane={refLane} actions={actions(rel, i)} />
            ))}
            <div className="w-[300px] shrink-0">
              <div className="relative mb-5 h-3">
                <span className="absolute top-1/2 left-0 size-3 -translate-y-1/2 rounded-full border border-dashed" style={{ borderColor: 'var(--ring)', background: 'var(--page-canvas)' }} />
              </div>
              {repo.noReleases ? (
                <EmptyNote text={`${repo.name} does not release, so its worklanes plan no releases.`} />
              ) : (
                <button
                  onClick={() => setPlanning(true)}
                  className="flex h-24 w-full flex-col items-center justify-center gap-1 rounded-[12px] border border-dashed text-[13px] hover:bg-[var(--surface-hover)]"
                  style={{ borderColor: 'var(--surface-border)', color: 'var(--muted-foreground)' }}
                >
                  <Plus size={16} />
                  Plan release
                  {!!backlog.length && <span className="text-[12px]">{plural(backlog.length, 'issue')} in no release</span>}
                </button>
              )}
            </div>
          </div>
        </div>
      )}
      {planning && (
        <ReleaseDialog
          repo={repo}
          laneName={lane.name}
          candidates={backlog}
          statusOf={api.statusOf}
          onClose={() => setPlanning(false)}
          onSave={(line, head, ids) => {
            onSaveLine(line, { release: head.id, ids });
            setPlanning(false);
            api.toast(`${head.version} ${head.title} planned`);
          }}
        />
      )}
      {editRel && (
        <ReleaseDialog
          repo={repo}
          laneName={lane.name}
          statusOf={api.statusOf}
          release={editRel}
          onClose={() => setEditing(null)}
          onSave={(line, head) => {
            onSaveLine(line);
            setEditing(null);
            api.toast(`${head.version} ${head.title} saved`);
          }}
        />
      )}
      {view === 'board' && (
        <StatusBoard
          api={api}
          columns={['backlog', 'todo', 'in_progress', 'in_review', 'done']}
          issues={filt(laneIssues(lane))}
          onAdd={(s) => api.newIssue({ lane: refLane, release: nextRel, status: s })}
        />
      )}
      {view === 'list' && (
        <div className="flex-1 overflow-auto px-6 py-4">
          {rels.map((rel, i) => (
            <section key={rel.id} className="mb-4 overflow-hidden rounded-[12px] border" style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)' }}>
              <div className="flex h-11 items-center gap-2 px-4">
                <ReleaseChip version={rel.version} state={states[i]} />
                <span className="min-w-0 truncate text-[14px] font-medium">{rel.title}</span>
                {rel.pinned && <SetByHand />}
                <div className="flex-1" />
                <span className="shrink-0 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                  {rel.issues.length ? `${doneCount(rel, api.statusOf)} of ${rel.issues.length} done` : 'No issues of this worklane'}
                </span>
                {actions(rel, i)}
              </div>
              {rel.description && (
                <p className="-mt-1 px-4 pb-3 text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
                  {rel.description}
                </p>
              )}
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
          {!rels.length && !backlog.length && (
            <EmptyNote text={repo.noReleases ? `${repo.name} does not release, so its worklanes plan no releases.` : 'No releases planned for this worklane yet.'} />
          )}
        </div>
      )}
    </>
  );
}

/** Marks a release whose version was set by hand: it keeps that number when it moves. */
function SetByHand() {
  return (
    <span className="flex shrink-0 items-center gap-1 text-[12px]" style={{ color: 'var(--muted-foreground)' }} title="Version set by hand: it keeps this number when the release moves">
      <Pin size={12} />
      Set by hand
    </span>
  );
}

/**
 * A release's actions: add an issue, edit it, or move it one place on the repository's line. A move renumbers
 * the releases after the shipped ones; it is refused, with the reason, when a version would no longer be higher
 * than the release before it.
 */
function ReleaseActions({
  api,
  repo,
  rel,
  index,
  refLane,
  onEdit,
  onSaveLine,
}: {
  api: AppApi;
  repo: Repo;
  rel: Release;
  index: number;
  refLane: LaneRef;
  onEdit: () => void;
  onSaveLine: SaveLine;
}) {
  const move = (to: number, label: string, icon: ReactNode, close: () => void) => {
    const { line, problem } = moveRelease(repo, api.statusOf, index, to);
    return (
      <MenuItem
        icon={icon}
        disabled={problem ?? undefined}
        onClick={() => {
          close();
          const cur = lineOf(repo);
          const changed = line.flatMap((h) => {
            const was = cur.find((c) => c.id === h.id)!;
            return was.version !== h.version ? [`${was.version} → ${h.version}`] : [];
          });
          onSaveLine(line);
          api.toast(`${rel.title} moved${changed.length ? `: ${changed.join(', ')}` : ''}`);
        }}
      >
        {label}
      </MenuItem>
    );
  };
  return (
    <Menu align="right" width={260} trigger={(_, toggle) => <IconButton label="Release actions" icon={<Ellipsis size={16} />} onClick={toggle} />}>
      {(close) => (
        <>
          <MenuItem
            icon={<Plus size={14} />}
            disabled={releaseStates(repo, api.statusOf)[index] === 'shipped' ? 'Shipped: add it to a later release' : undefined}
            onClick={() => {
              close();
              api.newIssue({ lane: refLane, release: rel.version });
            }}
          >
            Add issue to release
          </MenuItem>
          <MenuItem
            icon={<Pencil size={14} />}
            onClick={() => {
              close();
              onEdit();
            }}
          >
            Edit release
          </MenuItem>
          {move(index - 1, 'Move earlier', <ArrowLeft size={14} />, close)}
          {move(index + 1, 'Move later', <ArrowRight size={14} />, close)}
        </>
      )}
    </Menu>
  );
}

function ReleaseColumn({ api, rel, state, issues, refLane, actions }: { api: AppApi; rel: Release; state: ReleaseState; issues: Issue[]; refLane: LaneRef; actions: ReactNode }) {
  const done = doneCount(rel, api.statusOf);
  const next = state === 'next';
  // A release of the repository that holds none of this worklane's issues stays narrow.
  const empty = rel.issues.length === 0;
  return (
    <div data-release={rel.id} className={`${empty ? 'w-[300px]' : 'w-[544px]'} shrink-0`}>
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
            {rel.pinned && <SetByHand />}
            <div className="flex-1" />
            {actions}
          </div>
          <h3 className="mt-2 text-[15px] font-medium">{rel.title}</h3>
          {rel.description && (
            <p className="mt-1 text-[13px] leading-[18px]" style={{ color: 'var(--muted-foreground)' }}>
              {rel.description}
            </p>
          )}
          {!empty && (
            <div className="mt-3 flex items-center gap-3">
              <Progress done={done} total={rel.issues.length} />
              <span className="shrink-0 text-[12px] tabular-nums" style={{ color: 'var(--muted-foreground)' }}>
                {done} of {rel.issues.length} done
              </span>
            </div>
          )}
        </div>
        <div className="py-1">
          {issues.map((is) => (
            <IssueRow key={is.id} api={api} issue={is} />
          ))}
          {!issues.length && (
            <div className="px-4 py-3 text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
              {empty ? 'No issues of this worklane' : 'No issues'}
            </div>
          )}
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
// The worklane's Board view: its issues by status
// ---------------------------------------------------------------------------

function StatusBoard({ api, columns, issues, onAdd }: { api: AppApi; columns: Status[]; issues: Issue[]; onAdd: (s: Status) => void }) {
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
                <BoardCard key={is.id} api={api} issue={is} />
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

function BoardCard({ api, issue }: { api: AppApi; issue: Issue }) {
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
      {issue.release && (
        <div className="mt-2 flex flex-wrap gap-1">
          <ReleaseChip version={issue.release} state={relState} />
        </div>
      )}
    </article>
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

function IssuePage({ api, id }: { api: AppApi; id: string }) {
  const issue = api.findIssue(id);
  if (!issue) return <OtherPage label="Issue not found" />;
  const tpl = TEMPLATES.find((t) => t.type === issue.type)!;
  const found = issue.lane ? findLane(api.orgs, issue.lane) : null;
  const rel = found?.lane.releases?.find((r) => r.version === issue.release);
  const relState = releaseStateOf(api, issue);
  const toLane = issue.lane ? () => api.go({ name: 'lane', ref: issue.lane!, view: 'timeline' }) : undefined;
  // An issue without a worklane lives on Multica's Issues page.
  const crumbs = issue.lane
    ? [
        { label: 'Projects', onClick: () => api.go({ name: 'projects' }) },
        { label: issue.lane!.project, onClick: () => api.go({ name: 'project', org: issue.lane!.org, project: issue.lane!.project }) },
        { label: issue.lane!.repo, onClick: () => api.go({ name: 'project', org: issue.lane!.org, project: issue.lane!.project }) },
        { label: issue.lane!.lane, onClick: toLane },
        ...(issue.release ? [{ label: issue.release, onClick: toLane }] : []),
        { label: issue.id },
      ]
    : [{ label: 'Issues', onClick: () => api.go({ name: 'other', label: 'Issues' }) }, { label: issue.id }];
  const typeLabel = tpl.name.toLowerCase();
  return (
    <>
      <PageHeader icon={issue.lane ? GitBranch : ListTodo} crumbs={crumbs} />
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
              <div className="mt-2 text-[14px] font-medium">{rel.title}</div>
              {rel.description && (
                <p className="mt-1 text-[12px] leading-[17px]" style={{ color: 'var(--muted-foreground)' }}>
                  {rel.description}
                </p>
              )}
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
  const found = d.lane ? findLane(api.orgs, d.lane) : null;
  const laneObj = found?.lane;
  const relStates = found ? releaseStates(found.repo, api.statusOf) : [];
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
                          const f = findLane(api.orgs, ref)!;
                          setD({ ...d, lane: ref, release: f.lane.releases?.[laneNextIdx(f.repo, f.lane, api.statusOf)]?.version });
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
                width={300}
                trigger={(_, toggle) => (
                  <Chip icon={<CircleDashed size={13} />} onClick={toggle}>
                    <span className={d.release ? 'mc-mono' : undefined}>{d.release ?? 'No release'}</span>
                    {d.release && <span className="max-w-[160px] truncate">{laneObj?.releases?.find((r) => r.version === d.release)?.title}</span>}
                  </Chip>
                )}
              >
                {(close) => (
                  <>
                    {(laneObj?.releases ?? []).map((r, i) => (
                      <MenuItem
                        key={r.version}
                        selected={r.version === d.release}
                        disabled={relStates[i] === 'shipped' ? 'Shipped' : undefined}
                        onClick={() => (setD({ ...d, release: r.version }), close())}
                      >
                        <span className="mc-mono">{r.version}</span>
                        <span className="ml-2 truncate" style={{ color: 'var(--muted-foreground)' }}>
                          {r.title}
                        </span>
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

function Palette({ api, looseIssues, onClose }: { api: AppApi; looseIssues: Issue[]; onClose: () => void }) {
  const [q, setQ] = useState('');
  const [sel, setSel] = useState(0);
  const items = useMemo(() => {
    const all: { label: string; hint: string; icon: ReactNode; run: () => void }[] = [];
    api.orgs.forEach((o) => {
      all.push({ label: o.name, hint: plural(o.projects.length, 'project'), icon: <Building2 size={14} />, run: () => api.go({ name: 'organisation', org: o.name }) });
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
      });
    });
    looseIssues.forEach((is) => all.push({ label: `${is.id} ${is.title}`, hint: 'No worklane', icon: <TypeIcon type={is.type} />, run: () => api.peek(is) }));
    const needle = q.trim().toLowerCase();
    return (needle ? all.filter((i) => `${i.label} ${i.hint}`.toLowerCase().includes(needle)) : all.filter((i) => !/^[A-Z]+-\d+/.test(i.label))).slice(0, 10);
  }, [q, api, looseIssues]);
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
            placeholder="Search organisations, projects, worklanes and issues"
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
        <EmptyNote text={`${label} stays as Multica has it today; this prototype covers Projects, Organisations, Worklanes, Releases and New issue.`} />
      </div>
    </>
  );
}
