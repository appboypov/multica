import { useEffect, useMemo, useState, type ReactNode } from 'react';
import {
  Check,
  ChevronDown,
  Circle,
  CircleCheck,
  CircleDashed,
  CircleDot,
  CircleX,
  Folder,
  FolderGit2,
  FolderOpen,
  GitBranch,
  LoaderCircle,
  Rocket,
  Search,
  SquareTerminal,
  X,
} from 'lucide-react';
import {
  GITHUB_REPOS,
  RUNTIMES,
  SQUADS,
  laneFolder,
  nextVersions,
  type Bump,
  type GithubRepo,
  type Issue,
  type Org,
  type Project,
  type Repo,
  type Status,
} from './model';
import { Button, IconButton, Menu, MenuItem, Overlay, StatusIcon, TypeIcon } from './ui';

// ---------------------------------------------------------------------------
// Shared form parts, mirroring the library's "Creation forms" components
// ---------------------------------------------------------------------------

function Dialog({
  title,
  onClose,
  submit,
  children,
}: {
  title: string;
  onClose: () => void;
  submit: ReactNode;
  children: ReactNode;
}) {
  return (
    <Overlay onClose={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="mc-pop flex max-h-[calc(100vh-64px)] w-[640px] flex-col overflow-hidden rounded-[12px] border"
        style={{ background: 'var(--surface)', borderColor: 'var(--surface-border)', boxShadow: 'var(--floating-shadow)' }}
      >
        <div className="flex h-[52px] shrink-0 items-center gap-2 border-b px-4" style={{ borderColor: 'var(--border)' }}>
          <h2 className="flex-1 truncate text-[16px] font-semibold">{title}</h2>
          <IconButton label="Close" icon={<X size={16} />} onClick={onClose} />
        </div>
        <div className="min-h-0 flex-1 space-y-5 overflow-auto p-5">{children}</div>
        <div className="flex h-14 shrink-0 items-center justify-end gap-2 border-t px-4" style={{ borderColor: 'var(--border)' }}>
          {submit}
        </div>
      </div>
    </Overlay>
  );
}

function Field({ label, hint, help, error, children }: { label: string; hint?: string; help?: ReactNode; error?: string; children: ReactNode }) {
  return (
    <div className="min-w-0 flex-1 space-y-1.5">
      <div className="flex items-center justify-between gap-2">
        <span className="text-[13px] font-medium">{label}</span>
        {hint && (
          <span className="truncate text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
            {hint}
          </span>
        )}
      </div>
      <div className="space-y-1.5">{children}</div>
      {error ? (
        <p className="text-[12px]" style={{ color: 'var(--destructive)' }}>
          {error}
        </p>
      ) : (
        help && (
          <p className="text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
            {help}
          </p>
        )
      )}
    </div>
  );
}

function TextInput({
  value,
  onChange,
  placeholder,
  icon,
  mono,
  autoFocus,
  label,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  icon?: ReactNode;
  mono?: boolean;
  autoFocus?: boolean;
  label: string;
}) {
  return (
    <label
      className="flex h-8 w-full items-center gap-2 rounded-[8px] border px-2.5 focus-within:border-[var(--ring)]"
      style={{ background: 'var(--surface)', borderColor: 'var(--input)' }}
    >
      {icon && <span style={{ color: 'var(--muted-foreground)' }}>{icon}</span>}
      <input
        aria-label={label}
        autoFocus={autoFocus}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className={`min-w-0 flex-1 bg-transparent outline-none placeholder:text-[var(--muted-foreground)] ${mono ? 'mc-mono text-[13px]' : 'text-[14px]'}`}
      />
    </label>
  );
}

/** A select: a menu under a field-shaped trigger. */
function Select<T extends string>({
  value,
  options,
  onChange,
  label,
  width,
}: {
  value: T;
  options: { value: T; label: string; disabled?: string }[];
  onChange: (v: T) => void;
  label: string;
  width?: number;
}) {
  const current = options.find((o) => o.value === value);
  return (
    <div className="block w-full [&>div]:flex [&>div]:w-full" style={width ? { width } : undefined}>
      <Menu
        width={width ?? 280}
        trigger={(open, toggle) => (
          <button
            type="button"
            aria-label={label}
            aria-expanded={open}
            onClick={toggle}
            className="flex h-8 w-full items-center justify-between gap-2 rounded-[8px] border pr-2 pl-2.5 text-left text-[14px] hover:bg-[var(--surface-hover)]"
            style={{ background: 'var(--surface)', borderColor: open ? 'var(--ring)' : 'var(--input)' }}
          >
            <span className="truncate">{current?.label}</span>
            <ChevronDown size={14} style={{ color: 'var(--muted-foreground)' }} />
          </button>
        )}
      >
        {(close) =>
          options.map((o) =>
            o.disabled ? (
              <div key={o.value} className="flex h-8 items-center justify-between gap-2 px-2 text-[13px]" style={{ color: 'var(--faint-foreground)' }}>
                <span className="truncate">{o.label}</span>
                <span className="shrink-0 text-[12px]">{o.disabled}</span>
              </div>
            ) : (
              <MenuItem key={o.value} selected={o.value === value} onClick={() => (onChange(o.value), close())}>
                {o.label}
              </MenuItem>
            ),
          )
        }
      </Menu>
    </div>
  );
}

function Readonly({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <div className="flex h-8 min-w-0 flex-1 items-center gap-2 rounded-[8px] px-2.5" style={{ background: 'var(--muted)' }}>
      <span className="shrink-0" style={{ color: 'var(--muted-foreground)' }}>
        {icon}
      </span>
      <span className="mc-mono truncate text-[13px]">{children}</span>
    </div>
  );
}

function Note({ icon, children }: { icon: ReactNode; children: ReactNode }) {
  return (
    <div className="flex gap-2.5 rounded-[8px] px-3 py-2.5 text-[13px] leading-[19px]" style={{ background: 'var(--muted)', color: 'var(--muted-foreground)' }}>
      <span className="mt-px shrink-0">{icon}</span>
      <p>{children}</p>
    </div>
  );
}

type StepState = 'done' | 'run' | 'pend' | 'fail';

function StepRow({ state, text, detail }: { state: StepState; text: string; detail?: string }) {
  const icon = {
    done: <CircleCheck size={16} style={{ color: 'var(--success)' }} aria-label="Done" />,
    run: <LoaderCircle size={16} className="animate-spin" style={{ color: 'var(--brand)' }} aria-label="Running" />,
    pend: <CircleDashed size={16} style={{ color: 'var(--muted-foreground)' }} aria-label="Waiting" />,
    fail: <CircleX size={16} style={{ color: 'var(--destructive)' }} aria-label="Failed" />,
  }[state];
  return (
    <div className="flex h-8 items-center gap-2.5" role="listitem">
      {icon}
      <span className="flex-1 truncate text-[14px]" style={{ color: state === 'pend' ? 'var(--muted-foreground)' : undefined }}>
        {text}
      </span>
      {detail && (
        <span className="mc-mono truncate text-[12px]" style={{ color: state === 'fail' ? 'var(--destructive)' : 'var(--muted-foreground)' }}>
          {detail}
        </span>
      )}
    </div>
  );
}

const repoShort = (name: string) => name.split('/').pop() ?? name;

// ---------------------------------------------------------------------------
// Add repository: picked from what the GitHub App can see, checked out on a runtime
// ---------------------------------------------------------------------------

export function AddRepositoryDialog({
  orgs,
  project,
  onClose,
  onAdd,
}: {
  orgs: Org[];
  project: Project;
  onClose: () => void;
  onAdd: (repo: Repo) => void;
}) {
  const [q, setQ] = useState('');
  const [picked, setPicked] = useState<GithubRepo | null>(null);
  const [runtime, setRuntime] = useState(RUNTIMES[0].name);
  const [root, setRoot] = useState('~/Repos');
  const [cloned, setCloned] = useState<Record<string, 'running' | 'done'>>({});
  const [policy, setPolicy] = useState('found');

  // Every repository a project already holds, and which project that is.
  const taken = useMemo(() => {
    const m = new Map<string, string>();
    orgs.forEach((o) => o.projects.forEach((p) => p.repos.forEach((r) => m.set(r.name, p.name))));
    return m;
  }, [orgs]);
  const options = [
    ...GITHUB_REPOS.filter((g) => !taken.has(g.name)).map((g) => ({ g, in: null as string | null })),
    ...[...taken.entries()].map(([name, p]) => ({ g: { name, visibility: 'Private', branch: 'main', policy: '', checkedOut: true } as GithubRepo, in: p })),
  ].filter((o) => o.g.name.toLowerCase().includes(q.trim().toLowerCase()));

  const path = picked ? `${root}/${repoShort(picked.name)}` : '';
  const cloneState = picked ? cloned[picked.name] : undefined;
  const present = !!picked && (root !== '~/Repos' || picked.checkedOut || cloneState === 'done');
  const valid = !!picked && present;

  const clone = () => {
    if (!picked) return;
    setCloned((c) => ({ ...c, [picked.name]: 'running' }));
    window.setTimeout(() => setCloned((c) => ({ ...c, [picked.name]: 'done' })), 1600);
  };

  const policyOptions = picked
    ? [
        { value: 'found', label: `${picked.policy} · tags v{version}` },
        ...(picked.policy === 'Tags only' ? [] : [{ value: 'tags', label: 'Tags only · tags v{version}' }]),
        { value: 'none', label: 'This repository does not release' },
      ]
    : [{ value: 'found', label: 'Pick a repository first' }];

  return (
    <Dialog
      title={`Add repository · ${project.name}`}
      onClose={onClose}
      submit={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="brand"
            icon={<Check size={15} />}
            disabled={!valid}
            onClick={() =>
              picked &&
              onAdd({
                name: picked.name,
                lastTag: policy === 'none' ? undefined : picked.lastTag,
                lanes: [{ name: 'main', state: 'Free', active: null, next: null, releases: [], backlog: [] }],
              })
            }
          >
            Add repository
          </Button>
        </>
      }
    >
      <Field
        label="Repository"
        hint="Required"
        help={
          <a href="https://github.com/apps" target="_blank" rel="noreferrer" style={{ color: 'var(--brand)' }}>
            Missing one? Give Multica access on GitHub
          </a>
        }
      >
        <TextInput label="Search repositories" autoFocus value={q} onChange={setQ} placeholder="Search repositories" icon={<Search size={14} />} />
        <div role="radiogroup" aria-label="Repository" className="-mx-1 max-h-[212px] space-y-0.5 overflow-auto px-1">
          {options.map(({ g, in: inProject }) => {
            const sel = picked?.name === g.name;
            return (
              <button
                key={g.name}
                type="button"
                role="radio"
                aria-checked={sel}
                disabled={!!inProject}
                onClick={() => (setPicked(g), setPolicy('found'))}
                className="flex h-12 w-full items-center gap-2.5 rounded-[8px] border px-3 text-left enabled:hover:bg-[var(--surface-hover)] disabled:cursor-not-allowed"
                style={{
                  background: sel ? 'var(--surface-selected)' : undefined,
                  borderColor: sel ? 'var(--ring)' : 'transparent',
                  color: inProject ? 'var(--muted-foreground)' : undefined,
                }}
              >
                <FolderGit2 size={16} style={{ color: 'var(--muted-foreground)' }} />
                <span className="min-w-0 flex-1">
                  <span className="mc-mono block truncate text-[13px] font-medium">{g.name}</span>
                  <span className="block text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                    {g.visibility} · {g.branch}
                  </span>
                </span>
                {inProject && <span className="shrink-0 text-[12px]">In {inProject}</span>}
                {sel && <Check size={15} />}
              </button>
            );
          })}
          {!options.length && (
            <p className="px-3 py-2 text-[13px]" style={{ color: 'var(--muted-foreground)' }}>
              No repository matches “{q}”.
            </p>
          )}
        </div>
      </Field>

      <Field label="Checkout" hint="Required">
        <div className="flex items-center gap-2">
          <Select label="Runtime" width={220} value={runtime} onChange={setRuntime} options={RUNTIMES.map((r) => ({ value: r.name, label: `${r.name} · ${r.online ? 'online' : 'offline'}`, disabled: r.online ? undefined : 'Offline' }))} />
          <Readonly icon={<Folder size={14} />}>{picked ? path : '~/Repos/…'}</Readonly>
          <Menu
            align="right"
            width={240}
            trigger={(_, toggle) => (
              <Button icon={<FolderOpen size={15} />} onClick={toggle} disabled={!picked}>
                Choose
              </Button>
            )}
          >
            {(close) =>
              ['~/Repos', '~/Code'].map((r) => (
                <MenuItem key={r} selected={r === root} icon={<Folder size={14} />} onClick={() => (setRoot(r), close())}>
                  <span className="mc-mono">{picked ? `${r}/${repoShort(picked.name)}` : r}</span>
                </MenuItem>
              ))
            }
          </Menu>
        </div>
        {picked &&
          (present ? (
            <StepRow state="done" text={`Git checkout of ${picked.name}`} detail={`on ${picked.branch} · origin matches`} />
          ) : cloneState === 'running' ? (
            <StepRow state="run" text={`Cloning ${picked.name}`} detail={path} />
          ) : (
            <div className="flex items-center gap-2.5">
              <StepRow state="pend" text={`No folder at ${path}`} />
              <Button onClick={clone}>Clone it there</Button>
            </div>
          ))}
      </Field>

      <Field label="Releases" help={picked ? (policy === 'none' ? 'Its worklanes plan no releases.' : picked.lastTag ? `Last tag ${picked.lastTag}` : 'No tags yet') : undefined}>
        <Select label="Releases" value={policy} onChange={setPolicy} options={policyOptions} />
      </Field>
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// New worklane: a worktree of the repository, made on a runtime before it exists
// ---------------------------------------------------------------------------

const STEPS = ['Fetch origin/main', 'Add worktree', 'Install dependencies', 'Open the worklane'];

export function NewWorklaneDialog({
  project,
  repo,
  onClose,
  onCreate,
}: {
  project: Project;
  repo: Repo;
  onClose: () => void;
  onCreate: (lane: { name: string; folder: string; squad: string }) => void;
}) {
  const [name, setName] = useState('');
  const [folder, setFolder] = useState('');
  const [folderEdited, setFolderEdited] = useState(false);
  const [squad, setSquad] = useState(SQUADS[0].name);
  const [runtime, setRuntime] = useState(RUNTIMES[0].name);
  const [step, setStep] = useState<number | null>(null);

  const f = folderEdited ? folder : laneFolder(name);
  // Folders are unique among the project's worklanes; main works in the normal checkout.
  const usedBy = project.repos.flatMap((r) => r.lanes).find((l) => l.name !== 'main' && (l.folder ?? laneFolder(l.name)) === f);
  const folderError = !f ? undefined : !/^[a-z0-9][a-z0-9-]*$/.test(f) ? 'One folder name: lowercase letters, digits and dashes' : usedBy ? `Already used by ${usedBy.name}` : undefined;
  const valid = !!name.trim() && !!f && !folderError;
  const worktree = `~/Worktrees/${laneFolder(project.name)}/${f || '…'}/${repoShort(repo.name)}`;

  useEffect(() => {
    if (step === null) return;
    if (step >= STEPS.length) {
      onCreate({ name: name.trim(), folder: f, squad });
      return;
    }
    const t = window.setTimeout(() => setStep((s) => (s ?? 0) + 1), [700, 600, 1500, 400][step]);
    return () => window.clearTimeout(t);
  }, [step]); // eslint-disable-line react-hooks/exhaustive-deps

  const creating = step !== null;
  return (
    <Dialog
      title={`New worklane · ${repo.name}`}
      onClose={creating ? () => {} : onClose}
      submit={
        <>
          <Button variant="ghost" onClick={onClose} disabled={creating}>
            Cancel
          </Button>
          <Button variant="brand" icon={creating ? <LoaderCircle size={15} className="animate-spin" /> : <Check size={15} />} disabled={!valid || creating} onClick={() => setStep(0)}>
            {creating ? 'Creating worklane' : 'Create worklane'}
          </Button>
        </>
      }
    >
      {creating ? (
        <>
          <Field label={name.trim()}>
            <Readonly icon={<FolderGit2 size={14} />}>{worktree}</Readonly>
          </Field>
          <Field label={`Creating on ${runtime}`}>
            <div role="list">
              {STEPS.map((s, i) => (
                <StepRow
                  key={s}
                  state={i < step! ? 'done' : i === step ? 'run' : 'pend'}
                  text={s}
                  detail={i === 1 ? worktree : i === 2 ? 'pnpm install' : undefined}
                />
              ))}
            </div>
          </Field>
        </>
      ) : (
        <>
          <div className="flex items-start gap-3">
            <Field label="Name" hint="Required">
              <TextInput label="Name" autoFocus value={name} onChange={setName} placeholder="Fan app" />
            </Field>
            <Field label="Folder" help={`Unique among ${project.name}'s worklanes`} error={folderError}>
              <TextInput label="Folder" mono value={f} onChange={(v) => (setFolder(v), setFolderEdited(true))} placeholder="fan-app" />
            </Field>
          </div>
          <div className="flex items-start gap-3">
            <Field label="Squad">
              <Select label="Squad" value={squad} onChange={setSquad} options={SQUADS.map((s) => ({ value: s.name, label: s.name }))} />
            </Field>
            <Field label="Runtime" help="Online runtimes that can make worktrees">
              <Select label="Runtime" value={runtime} onChange={setRuntime} options={RUNTIMES.map((r) => ({ value: r.name, label: `${r.name} · ${r.online ? 'online' : 'offline'}`, disabled: r.online ? undefined : 'Offline' }))} />
            </Field>
          </div>
          <Field label="Worktree">
            <Readonly icon={<FolderGit2 size={14} />}>{worktree}</Readonly>
            <Readonly icon={<GitBranch size={14} />}>from origin/main</Readonly>
            <Readonly icon={<SquareTerminal size={14} />}>pnpm install</Readonly>
          </Field>
        </>
      )}
    </Dialog>
  );
}

// ---------------------------------------------------------------------------
// Plan release: the next version on the repository's line, with this worklane's issues
// ---------------------------------------------------------------------------

const BUMPS: { value: Bump; label: string }[] = [
  { value: 'patch', label: 'Patch' },
  { value: 'minor', label: 'Minor' },
  { value: 'major', label: 'Major' },
];

export function PlanReleaseDialog({
  repo,
  laneName,
  candidates,
  statusOf,
  onClose,
  onPlan,
}: {
  repo: Repo;
  laneName: string;
  /** This worklane's issues that are in no release yet. */
  candidates: Issue[];
  statusOf: (i: Issue) => Status;
  onClose: () => void;
  onPlan: (version: string, issueIds: string[]) => void;
}) {
  const { base, baseIsTag, next } = nextVersions(repo);
  const [bump, setBump] = useState<Bump>('patch');
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const version = next[bump];
  const toggle = (id: string) =>
    setPicked((s) => {
      const n = new Set(s);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });

  return (
    <Dialog
      title={`Plan release · ${repo.name}`}
      onClose={onClose}
      submit={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="brand" icon={<Check size={15} />} onClick={() => onPlan(version, candidates.filter((i) => picked.has(i.id)).map((i) => i.id))}>
            Plan {version}
          </Button>
        </>
      }
    >
      <Field
        label="Version"
        help={!base ? 'The first release of this repository.' : baseIsTag ? `Follows ${base}, the last tag.` : `Follows ${base}, the last planned release. Moving it renumbers it.`}
      >
        <div role="radiogroup" aria-label="Version" className="grid grid-cols-3 gap-2">
          {BUMPS.map((b) => {
            const sel = b.value === bump;
            return (
              <button
                key={b.value}
                type="button"
                role="radio"
                aria-checked={sel}
                onClick={() => setBump(b.value)}
                className="rounded-[8px] border px-3 py-2.5 text-left hover:bg-[var(--surface-hover)]"
                style={{ background: sel ? 'var(--surface-selected)' : 'var(--surface)', borderColor: sel ? 'var(--ring)' : 'var(--surface-border)' }}
              >
                <span className="flex items-center justify-between text-[13px] font-medium">
                  {b.label}
                  {sel ? <CircleDot size={14} /> : <Circle size={14} style={{ color: 'var(--muted-foreground)' }} />}
                </span>
                <span className="mc-mono mt-1 block text-[15px] font-semibold">{next[b.value]}</span>
              </button>
            );
          })}
        </div>
      </Field>

      <Field label="Issues" hint={`${laneName} · in no release`} help={candidates.length ? undefined : 'Every issue of this worklane is in a release. Add issues later from the timeline.'}>
        <div className="space-y-0.5">
          {candidates.map((i) => {
            const on = picked.has(i.id);
            return (
              <label
                key={i.id}
                className="flex h-9 cursor-pointer items-center gap-2.5 rounded-[6px] px-2.5 hover:bg-[var(--surface-hover)]"
                style={{ background: on ? 'var(--surface-selected)' : undefined }}
              >
                <input type="checkbox" checked={on} onChange={() => toggle(i.id)} className="size-4 accent-[var(--primary)]" aria-label={`${i.id} ${i.title}`} />
                <TypeIcon type={i.type} />
                <span className="mc-mono w-[64px] shrink-0 text-[12px]" style={{ color: 'var(--muted-foreground)' }}>
                  {i.id}
                </span>
                <span className="min-w-0 flex-1 truncate text-[14px]">{i.title}</span>
                <StatusIcon status={statusOf(i)} />
              </label>
            );
          })}
        </div>
      </Field>

      <Note icon={<Rocket size={16} />}>
        When its issues are done and merged, Multica tags {version} on main, writes the release note, publishes the GitHub release and asks you to pass it. It gets its date when it ships.
      </Note>
    </Dialog>
  );
}
