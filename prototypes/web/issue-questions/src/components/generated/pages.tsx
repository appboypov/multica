import { useEffect, useRef, useState, type ReactNode } from 'react';
import { AlarmClock, ArrowUpRight, Check, ChevronLeft, CircleCheck, CircleHelp, Code2, Folder, Hash, Layers, ListFilter, MessagesSquare, NotebookPen, Play, Server, Settings2, SkipForward, SlidersHorizontal, Tag, Users, X, Zap, CircleDot, CornerDownLeft, CornerUpLeft, UserRoundPlus, Send } from 'lucide-react';
import { Avatar, Btn, Kbd, Pill, StatusGlyph, Switch, TypeGlyph, TypeTile, cx } from './ui';
import { AnswerBody, CompactCard, REFERENCE, REFERENCE_SEED, summarize, type Draft } from './cards';
import { FoldedAnswer, QuestionForYou, SetCard, WakeBar } from './issue';
import { LINKS, PEOPLE, Q_VAT, SET_TITLE, STATIC_TABS, TYPE_META, type LinkId, type Question, type StaticRow } from './model';
import type { App, Item } from './QuestionsPrototype';

/* ───────────────────────── Questions page (screen 4) ───────────────────────── */

type Tab = 'me' | 'work' | 'mine' | 'all' | 'answered';
export function QuestionsPage({
  app
}: {
  app: App;
}) {
  const s = app.state;
  const tab = s.qTab;
  const open = app.openForMe();
  const [focusIdx, setFocusIdx] = useState(0);
  const [hover, setHover] = useState<string | null>(s.hoverRow ?? null);
  const askedByMe = s.asked.length;
  const tabs: Array<[Tab, string, number | null]> = [['me', 'For me', open.length], ['work', 'On my work', 4], ['mine', 'Asked by me', 3 + askedByMe], ['all', 'All open', 8 + open.length], ['answered', 'Answered', null]];
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement).closest('input,textarea')) return;
      if (tab !== 'me') return;
      const k = e.key.toLowerCase();
      if (k === 'arrowdown' || k === 'j') e.preventDefault(), setFocusIdx(i => Math.min(open.length - 1, i + 1));
      if (k === 'arrowup' || k === 'k') e.preventDefault(), setFocusIdx(i => Math.max(0, i - 1));
      if (k === 'enter' && open[focusIdx]) app.openItem(open[focusIdx]);
      if (k === 'a' && open.length) app.go('answer');
      const it = open[focusIdx];
      if (it && it.q.type === 'yesno' && (k === 'y' || k === 'n')) app.answer(it.q.id, k === 'y' ? 'Yes' : 'No');
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [tab, open, focusIdx, app]);
  const groups = groupByLink(open);
  return <div className="flex h-full flex-col">
      <header className="flex h-11 shrink-0 items-center gap-2 border-b border-(--border) pr-3 pl-4">
        <CircleHelp size={16} />
        <span className="flex-1 text-[14px] font-medium">Questions</span>
        <Btn variant="ghost" size="sm">
          <ListFilter size={15} /> Filter
        </Btn>
        <Btn variant="ghost" size="sm">
          <SlidersHorizontal size={15} /> Display
        </Btn>
        <Btn variant="primary" size="lg" className="ml-1" disabled={!open.length} onClick={() => app.go('answer')}>
          <Play size={14} /> Answer all <span className="ml-1 opacity-90">{open.length}</span>
        </Btn>
      </header>
      <div className="flex h-10 shrink-0 items-end gap-1 border-b border-(--border) px-4" role="tablist">
        {tabs.map(([k, l, n]) => <button key={k} role="tab" aria-selected={tab === k} type="button" onClick={() => app.set({
        qTab: k
      })} className={cx('relative flex h-10 cursor-pointer items-center gap-1.5 px-1.5 text-[14px]', tab === k ? 'font-medium text-(--foreground)' : 'text-(--muted-foreground) hover:text-(--foreground)')} style={{
        marginRight: 12
      }}>
            {l}
            {n !== null && <span className="text-[12px] text-(--muted-foreground)">{n}</span>}
            {tab === k && <span className="absolute right-0 bottom-[-1px] left-0 h-[2px] rounded-full bg-(--foreground)" />}
          </button>)}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto">
        {tab === 'me' && (open.length ? groups.map(([link, items]) => <div key={link}>
                <GroupHead link={link} count={items.length} />
                {items.map(it => {
          const idx = open.indexOf(it);
          return <Row key={it.q.id} it={it} focused={focusIdx === idx} hovered={hover === it.q.id} onHover={h => setHover(h ? it.q.id : null)} onOpen={() => app.openItem(it)} onYesNo={v => app.answer(it.q.id, v)} />;
        })}
              </div>) : <Empty app={app} />)}
        {tab !== 'me' && <StaticList app={app} tab={tab} />}
      </div>
      <footer className="flex h-9 shrink-0 items-center gap-1.5 border-t border-(--border) px-4 text-[12px] text-(--muted-foreground)">
        <Kbd>↑↓</Kbd> <span className="mr-2">move</span>
        <Kbd>↵</Kbd> <span className="mr-2">open</span>
        <Kbd>Y</Kbd>
        <Kbd>N</Kbd> <span className="mr-2">answer Yes / No inline</span>
        <Kbd>A</Kbd> <span>answer all</span>
        <span className="flex-1" />
        <span>Grouped by link · {open.length} open for you</span>
      </footer>
    </div>;
}
function groupByLink(items: Item[]) {
  const order: LinkId[] = ['WORK-412', 'WORK-398', 'agent-runtime'];
  return order.map(l => [l, items.filter(i => i.q.link === l)] as const).filter(([, v]) => v.length > 0);
}
function GroupHead({
  link,
  count
}: {
  link: LinkId;
  count: number;
}) {
  const L = LINKS[link];
  return <div className="flex h-9 items-center gap-2 bg-(--muted) px-4 text-[13px]">
      <span className="flex w-[66px] shrink-0 items-center gap-2 text-(--muted-foreground)">
        {L.kind === 'project' ? <Folder size={15} /> : <StatusGlyph status={L.status} size={15} />}
        {L.key}
      </span>
      <span className="font-semibold">{L.title}</span>
      <span className="text-[12px] text-(--muted-foreground)">{count}</span>
    </div>;
}
function Row({
  it,
  focused,
  hovered,
  onHover,
  onOpen,
  onYesNo
}: {
  it: Item;
  focused: boolean;
  hovered: boolean;
  onHover: (h: boolean) => void;
  onOpen: () => void;
  onYesNo: (v: string) => void;
}) {
  const q = it.q;
  return <div role="button" tabIndex={0} onClick={onOpen} onMouseEnter={() => onHover(true)} onMouseLeave={() => onHover(false)} className={cx('group flex h-9 cursor-pointer items-center gap-2 border-b border-(--border) px-4', focused || hovered ? 'bg-(--surface-hover)' : 'bg-(--page-canvas)')}>
      <span className="flex w-[18px] justify-center text-(--muted-foreground)">
        <TypeGlyph type={it.set ? 'set' : q.type} size={14} />
      </span>
      <span className="min-w-0 truncate text-[14px]">{it.set ? SET_TITLE : q.prompt}</span>
      {it.progress && <span className="ml-1.5 shrink-0 text-[14px]">{it.progress}</span>}
      <span className="flex-1" />
      {q.type === 'yesno' && hovered ? <span className="flex gap-1.5" onClick={e => e.stopPropagation()}>
          {['Yes', 'No'].map(v => <button key={v} type="button" onClick={() => onYesNo(v)} className="flex h-[26px] cursor-pointer items-center gap-1.5 rounded-[6px] border border-(--border) bg-(--surface) px-2 text-[13px] hover:border-(--ring)">
              {v} <span className="text-[10px] text-(--muted-foreground)">{v[0]}</span>
            </button>)}
        </span> : <>
          {q.via && <span className="text-[12px] text-(--muted-foreground)">{q.via}</span>}
          {q.blocks && <Pill tone="danger" icon={<CirclePauseSmall />}>
              Blocks work
            </Pill>}
          {q.anyOf && <Pill tone="neutral" icon={<Users size={12} />}>
              any one of {q.anyOf}
            </Pill>}
        </>}
      <span className="ml-1 flex w-[124px] shrink-0 items-center gap-2 text-[13px] text-(--muted-foreground)">
        <Avatar id={q.asker} size={20} /> {PEOPLE[q.asker].name}
      </span>
      <span className={cx('w-[86px] shrink-0 text-right text-[12px]', q.due?.tone === 'warning' ? 'text-[#946c00]' : q.due?.tone === 'danger' ? 'text-(--destructive)' : 'text-(--muted-foreground)')}>
        {q.due?.label ?? it.age}
      </span>
    </div>;
}
function CirclePauseSmall() {
  return <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
      <circle cx="12" cy="12" r="10" />
      <path d="M10 15V9M14 15V9" />
    </svg>;
}
function Empty({
  app
}: {
  app: App;
}) {
  const last = app.lastAnswered();
  return <div className="flex flex-col items-center pt-24 text-center">
      <CircleCheck size={28} className="text-(--faint-foreground)" />
      <p className="mt-3 text-[16px] font-medium">No questions waiting for you</p>
      {last && <p className="mt-1.5 max-w-[460px] text-[13px] text-(--muted-foreground)">
          Last answered: “{last.prompt}” → <span className="text-(--foreground)">{last.summary}</span>
        </p>}
    </div>;
}
function StaticList({
  app,
  tab
}: {
  app: App;
  tab: Exclude<Tab, 'me'>;
}) {
  let rows: StaticRow[] = STATIC_TABS[tab];
  if (tab === 'mine') rows = [...app.state.asked.map((a): StaticRow => ({
    type: a.questions.length > 1 ? 'single' : a.questions[0].type,
    prompt: a.questions.length > 1 ? `${a.questions[0].prompt} (+${a.questions.length - 1})` : a.questions[0].prompt,
    link: 'WORK-412',
    who: a.to[0],
    right: 'Just now'
  })), ...rows];
  if (tab === 'all') rows = [...app.openForMe().map((it): StaticRow => ({
    type: it.q.type,
    prompt: it.set ? SET_TITLE : it.q.prompt,
    link: it.q.link,
    who: it.q.asker,
    right: it.q.due?.label ?? it.age
  })), ...STATIC_TABS.work.slice(1), ...STATIC_TABS.mine, {
    type: 'text',
    prompt: 'Which bank should the importer support next?',
    link: 'WORK-412',
    who: 'eva',
    right: '4 h'
  }, {
    type: 'scale',
    prompt: 'How urgent is the Moneybird sync for your clients?',
    link: 'agent-runtime',
    who: 'joeri',
    right: '1 d'
  }];
  if (tab === 'answered') rows = [...app.answeredRows().map((r): StaticRow => ({
    type: r.q.type,
    prompt: r.q.prompt,
    link: r.q.link,
    who: r.q.asker,
    right: 'Just now',
    answered: r.summary
  })), ...rows];
  return <div>
      {rows.map((r, i) => <div key={i} className="flex h-9 items-center gap-2 border-b border-(--border) px-4 hover:bg-(--surface-hover)">
          <span className="flex w-[18px] justify-center text-(--muted-foreground)">
            <TypeGlyph type={r.type} size={14} />
          </span>
          <span className="w-[72px] shrink-0 text-[12px] text-(--muted-foreground)">{r.link === 'agent-runtime' ? 'Runtime' : r.link}</span>
          <span className="min-w-0 truncate text-[14px]">{r.prompt}</span>
          {r.answered && <span className="shrink-0 text-[13px]">
              <span className="text-(--muted-foreground)">→</span> <span className="font-medium">{r.answered}</span>
            </span>}
          <span className="flex-1" />
          <span className="flex w-[124px] shrink-0 items-center gap-2 text-[13px] text-(--muted-foreground)">
            <Avatar id={r.who} size={20} /> {PEOPLE[r.who].name}
          </span>
          <span className="w-[86px] shrink-0 text-right text-[12px] text-(--muted-foreground)">{r.right}</span>
        </div>)}
    </div>;
}

/* ───────────────────────── Answer all (screens 5 and 8) ───────────────────────── */

export function AnswerAll({
  app
}: {
  app: App;
}) {
  const s = app.state;
  const queue = app.focusQueue();
  const answeredCount = queue.filter(it => app.isDone(it)).length;
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !(e.target as HTMLElement).closest('[role=dialog]')) app.go('questions');
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [app]);
  return <div className="flex h-full flex-col">
      <header className="flex h-[56px] shrink-0 items-center gap-2 border-b border-(--border) pr-4 pl-6 text-[14px]">
        <button type="button" onClick={() => app.go('questions')} className="flex cursor-pointer items-center gap-2 text-(--muted-foreground) hover:text-(--foreground)">
          <ChevronLeft size={15} /> Questions
        </button>
        <span className="mx-3 text-(--faint-foreground)">/</span>
        <span className="font-medium">Answer all</span>
        <div className="ml-2 flex rounded-[9px] bg-(--muted) p-[3px]">
          {(['one', 'page'] as const).map(m => <button key={m} type="button" onClick={() => app.set({
          answerMode: m
        })} className={cx('h-[28px] cursor-pointer rounded-[7px] px-2.5 text-[14px]', s.answerMode === m ? 'bg-(--surface) font-medium shadow-sm' : 'text-(--muted-foreground) hover:text-(--foreground)')}>
              {m === 'one' ? 'One by one' : 'One page'}
            </button>)}
        </div>
        {s.answerMode === 'one' ? <>
            <div className="ml-3 flex gap-1">
              {queue.map((it, i) => <span key={it.q.id} className="h-[4px] w-10 rounded-full" style={{
            background: app.isDone(it) ? 'var(--success)' : i === s.focusIdx ? 'var(--foreground)' : s.skipped.includes(it.q.id) ? 'var(--faint-foreground)' : 'var(--input)'
          }} />)}
            </div>
            <span className="ml-2 text-[14px] text-(--muted-foreground)">
              {Math.min(s.focusIdx + 1, queue.length)} of {queue.length}
            </span>
          </> : <span className="ml-3 text-[14px] text-(--muted-foreground)">
            {answeredCount} of {queue.length} answered
          </span>}
        <span className="flex-1" />
        {s.answerMode === 'one' && <span className="mr-2 text-[14px] text-(--muted-foreground)">
            Answered {answeredCount} · skipped {s.skipped.length}
          </span>}
        <Btn variant="outline" size="lg" onClick={() => app.go('questions')}>
          <X size={15} /> Leave
        </Btn>
        <Kbd className="ml-1 h-[22px] px-1.5 text-[11px]">Esc</Kbd>
      </header>
      {s.answerMode === 'one' ? <FocusMode app={app} queue={queue} /> : <OnePage app={app} queue={queue} />}
    </div>;
}
function FocusMode({
  app,
  queue
}: {
  app: App;
  queue: Item[];
}) {
  const s = app.state;
  const idx = Math.min(s.focusIdx, queue.length - 1);
  const it = queue[idx];
  const allDone = queue.every(x => app.isDone(x));
  const d = (it && s.drafts[it.q.id]) ?? {};
  const go = (by: number) => app.set({
    focusIdx: Math.max(0, Math.min(queue.length - 1, idx + by))
  });
  const next = () => {
    const n = queue.findIndex((x, i) => i > idx && !app.isDone(x));
    const any = n >= 0 ? n : queue.findIndex((x, i) => i !== idx && !app.isDone(x));
    if (any >= 0) app.set({
      focusIdx: any
    });
  };
  const sendAndNext = () => {
    if (!it || app.isDone(it)) return next();
    if (it.set) return;
    const sum = summarize(it.q, d) ?? it.q.options?.find(o => o.suggested)?.label ?? null;
    if (!sum) return;
    app.answer(it.q.id, sum, d.why);
    const rest = queue.filter((x, i) => i !== idx && !app.isDone(x));
    if (!rest.length) return;
    next();
  };
  const noteRef = useRef<HTMLTextAreaElement>(null);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const inField = (e.target as HTMLElement).closest('input,textarea');
      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) return e.preventDefault(), sendAndNext();
      if (inField || !it) return;
      const k = e.key.toLowerCase();
      if (k === 'j') go(1);else if (k === 'k') go(-1);else if (k === 's') app.skip(it.q.id), next();else if (k === 'n') e.preventDefault(), noteRef.current?.focus();else if (/^[1-9]$/.test(k) && it.q.options) {
        const i = Number(k) - 1;
        if (i <= it.q.options.length) app.setDraft(it.q.id, {
          ...d,
          choice: i
        });
      } else if ((k === 'y' || k === 'n') && it.q.type === 'yesno') app.setDraft(it.q.id, {
        ...d,
        yes: k === 'y'
      });
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });
  const unseenAgent = it ? app.unseenFor(it.q.asker, it.q.link) : 0;
  const L = it ? LINKS[it.q.link] : null;
  return <div className="flex min-h-0 flex-1">
      <div className="min-w-0 flex-1 overflow-y-auto">
        <div className="mx-auto max-w-[680px] pt-10 pb-12">
          {allDone ? <div className="rounded-[12px] border border-(--border) bg-(--surface) px-6 py-10 text-center">
              <CircleCheck size={28} className="mx-auto text-(--success)" />
              <p className="mt-3 text-[16px] font-semibold">All {queue.length} answered</p>
              <p className="mt-1 text-[13px] text-(--muted-foreground)">Each answer reached its asker when you sent it. Agents read theirs once you wake them.</p>
              <Btn variant="primary" size="lg" className="mt-5" onClick={() => app.go('questions')}>
                Back to Questions
              </Btn>
            </div> : it && L && <>
                <button type="button" onClick={() => app.openItem(it)} className="mb-3 flex cursor-pointer items-center gap-2 text-[14px] hover:underline">
                  {L.kind === 'project' ? <Folder size={15} className="text-(--muted-foreground)" /> : <StatusGlyph status={L.status} size={15} />}
                  <span className="text-(--muted-foreground)">{L.key}</span>
                  <span className="font-medium">{L.title}</span>
                  <ArrowUpRight size={14} className="text-(--muted-foreground)" />
                </button>
                {app.isDone(it) ? <FoldedAnswer q={it.q} summary={app.summaryOf(it) ?? ''} meta="Brian · just now" onEdit={() => app.editAnswer(it.q.id)} /> : it.set ? <SetCard app={app} /> : <FocusCard app={app} it={it} noteRef={noteRef} onSend={sendAndNext} onSkip={() => (app.skip(it.q.id), next())} />}
                {unseenAgent > 0 && PEOPLE[it.q.asker].agent && <WakeBar className="mt-4" agent={it.q.asker} count={unseenAgent} issue={L.key === 'Project' ? L.title : L.key} onWake={() => app.wake(it.q.asker)} />}
                {!unseenAgent && app.recentlyWoken(it.q.asker) && <p className="mt-4 flex items-center gap-2 text-[13px] text-(--muted-foreground)">
                    <AlarmClock size={15} /> {PEOPLE[it.q.asker].name} was woken with your answers.
                  </p>}
              </>}
        </div>
      </div>
      <aside className="w-[320px] shrink-0 overflow-y-auto border-l border-(--border) px-4 pt-5">
        <p className="px-1 text-[13px] font-medium text-(--muted-foreground)">Up next</p>
        <div className="mt-2.5 flex flex-col gap-1">
          {queue.map((x, i) => <button key={x.q.id} type="button" onClick={() => app.set({
          focusIdx: i
        })} className={cx('flex cursor-pointer items-center gap-3 rounded-[10px] border px-3 py-2.5 text-left', i === idx ? 'border-(--border) bg-(--surface)' : 'border-transparent hover:bg-(--surface-hover)')} style={{
          boxShadow: i === idx ? 'var(--surface-shadow)' : undefined
        }}>
              {app.isDone(x) ? <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-[#e8f3ea]">
                  <CircleCheck size={14} className="text-(--success)" />
                </span> : <span className="flex size-5 shrink-0 items-center justify-center text-(--muted-foreground)">
                  <TypeGlyph type={x.set ? 'set' : x.q.type} size={14} />
                </span>}
              <span className={cx('min-w-0 flex-1 text-[13px] leading-[19px]', i === idx ? 'text-(--foreground)' : 'text-(--muted-foreground)')}>
                {app.isDone(x) ? 'Done: ' : s.skipped.includes(x.q.id) ? 'Skipped: ' : ''}
                {x.set ? SET_TITLE : x.q.prompt}
              </span>
              <Avatar id={x.q.asker} size={20} />
            </button>)}
        </div>
        {it && <>
            <div className="mt-4 border-t border-(--border) pt-4">
              <p className="px-1 text-[13px] font-medium text-(--muted-foreground)">On the {LINKS[it.q.link].kind}</p>
              {it.q.link === 'WORK-412' ? <div className="mt-2.5 flex flex-col gap-3 px-1 text-[13px] leading-[19px] text-(--muted-foreground)">
                  <div className="flex gap-2.5">
                    <Avatar id="brian" size={20} />
                    <span>Picked this up for Pieter. The export format is the one in the attached sample.</span>
                  </div>
                  <div className="flex gap-2.5">
                    <Avatar id="pieter" size={20} />
                    <span>Run #3 ended with this question</span>
                  </div>
                </div> : <div className="mt-2.5 flex gap-2.5 px-1 text-[13px] leading-[19px] text-(--muted-foreground)">
                  <Avatar id={it.q.asker} size={20} />
                  <span>{LINKS[it.q.link].title}: {PEOPLE[it.q.asker].name} asked this {it.age === 'Yesterday' ? 'yesterday' : `${it.age} ago`}.</span>
                </div>}
            </div>
          </>}
      </aside>
    </div>;
}
function FocusCard({
  app,
  it,
  noteRef,
  onSend,
  onSkip
}: {
  app: App;
  it: Item;
  noteRef: React.RefObject<HTMLTextAreaElement | null>;
  onSend: () => void;
  onSkip: () => void;
}) {
  const q = it.q;
  const d = app.state.drafts[q.id] ?? {};
  const set = (nd: Draft) => app.setDraft(q.id, nd);
  const [aside, setAside] = useState<string | null>(null);
  const ready = !!summarize(q, d) || !!q.options?.some(o => o.suggested);
  return <section className="rounded-[12px] border border-(--brand) bg-(--surface) px-4 pt-4 pb-3.5" style={{
    boxShadow: '0 0 0 0.5px var(--brand), var(--surface-shadow)'
  }}>
      <div className="flex h-6 items-center gap-2">
        <TypeTile type={q.type} brand />
        <span className="text-[11px] font-semibold tracking-[0.055em] text-(--brand) uppercase">Question for you</span>
        <span className="text-[12px] text-(--muted-foreground)">· {TYPE_META[q.type].label}</span>
        <span className="flex-1" />
        {q.blocks && <Pill tone="danger" icon={<CirclePauseSmall />}>
            Blocks work
          </Pill>}
        {q.due && <Pill tone="neutral">
            {q.due.tone === 'danger' ? q.due.label : 'Answer by today'}
          </Pill>}
      </div>
      <div className="mt-3 flex items-center gap-2">
        <Avatar id={q.asker} size={22} />
        <span className="text-[14px] font-medium">{PEOPLE[q.asker].name}</span>
        <span className="text-[12px] text-(--muted-foreground)">{q.askedLine}</span>
      </div>
      <h3 className="mt-3 text-[16px] leading-[22px] font-semibold">{q.prompt}</h3>
      {q.context && <p className="mt-2 text-[14px] leading-[21px] text-(--muted-foreground)">{q.context}</p>}
      <div className="mt-3">
        <AnswerBody q={q} d={d} set={set} />
      </div>
      <div className="mt-3 flex items-center gap-1.5 text-[13px]">
        <NotebookPen size={14} className="text-(--muted-foreground)" />
        <span className="font-medium">Note for {PEOPLE[q.asker].name}</span>
        <span className="text-[12px] text-(--muted-foreground)">optional</span>
      </div>
      <textarea ref={noteRef} value={d.why ?? ''} onChange={e => set({
      ...d,
      why: e.target.value
    })} rows={2} placeholder="Add a note…" className="mt-2 block w-full resize-none rounded-[8px] border border-(--input) px-2.5 py-2 text-[14px] leading-[21px] outline-none focus:border-(--ring)" />
      {aside && <p className="mt-3 rounded-[8px] bg-(--muted) px-2.5 py-2 text-[12px] text-(--muted-foreground)">{aside}</p>}
      <div className="mt-3 flex items-center gap-0.5">
        <Btn variant="ghost" size="sm" onClick={() => setAside(`Ask back replies in the question’s thread and keeps it open for ${PEOPLE[q.asker].name}.`)}>
          <CornerUpLeft size={14} /> Ask back
        </Btn>
        <Btn variant="ghost" size="sm" onClick={() => setAside('Reassign hands the question to another member with a note.')}>
          <UserRoundPlus size={14} /> Reassign
        </Btn>
        <Btn variant="ghost" size="sm" onClick={onSkip}>
          <SkipForward size={14} /> Skip <Kbd className="ml-1">S</Kbd>
        </Btn>
        <span className="flex-1" />
        {q.type !== 'approval' && <Btn variant="primary" size="lg" disabled={!ready} onClick={onSend}>
            <CornerDownLeft size={15} /> Send and next
          </Btn>}
        {q.type === 'approval' && <Btn variant="primary" size="lg" disabled={!d.approval} onClick={onSend}>
            <Send size={15} /> Send and next
          </Btn>}
        <Kbd className="ml-1 h-[22px] min-w-[28px] text-[11px]">⌘↵</Kbd>
      </div>
    </section>;
}
function OnePage({
  app,
  queue
}: {
  app: App;
  queue: Item[];
}) {
  const s = app.state;
  const scroller = useRef<HTMLDivElement>(null);
  const [focus, setFocus] = useState<string>(queue.find(x => !app.isDone(x))?.q.id ?? '');
  const ids = queue.map(x => x.q.id);
  const jump = (id: string) => {
    setFocus(id);
    const el = scroller.current?.querySelector(`[data-q="${id}"]`) as HTMLElement | null;
    if (el && scroller.current) scroller.current.scrollTo({
      top: el.offsetTop - 24,
      behavior: 'smooth'
    });
  };
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const inField = (e.target as HTMLElement).closest('input,textarea');
      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        const it = queue.find(x => x.q.id === focus);
        if (it && !app.isDone(it) && !it.set) {
          const sum = summarize(it.q, s.drafts[it.q.id] ?? {});
          if (sum) app.answer(it.q.id, sum);
        }
        return;
      }
      if (inField) return;
      const i = ids.indexOf(focus);
      if (e.key === 'j') jump(ids[Math.min(ids.length - 1, i + 1)]);
      if (e.key === 'k') jump(ids[Math.max(0, i - 1)]);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });
  const groups = groupByLink(queue);
  return <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex min-h-0 flex-1">
        <div ref={scroller} data-scroll="onepage" className="relative min-w-0 flex-1 overflow-y-auto">
          <div className="mx-auto max-w-[760px] pt-6 pb-12">
            {groups.map(([link, items]) => {
            const L = LINKS[link];
            const agent = items.find(x => PEOPLE[x.q.asker].agent)?.q.asker;
            const unseen = agent ? app.unseenFor(agent, link) : 0;
            return <div key={link} className="mb-7">
                  <div className="mb-2.5 flex h-6 items-center gap-2 text-[13px]">
                    {L.kind === 'project' ? <Folder size={15} className="text-(--muted-foreground)" /> : <StatusGlyph status={L.status} size={15} />}
                    <span className="text-(--muted-foreground)">{L.key}</span>
                    <span className="font-medium">{L.title}</span>
                    <ArrowUpRight size={14} className="text-(--muted-foreground)" />
                  </div>
                  <div className="flex flex-col gap-2.5">
                    {items.map(it => <div key={it.q.id} data-q={it.q.id} onPointerDown={() => setFocus(it.q.id)}>
                        {app.isDone(it) ? <FoldedAnswer q={it.q} summary={app.summaryOf(it) ?? ''} meta="Brian · just now" onEdit={() => app.editAnswer(it.q.id)} prompt={it.set ? SET_TITLE : undefined} /> : it.set ? <SetCard app={app} embedded /> : it.q.id === Q_VAT.id ? <div className={cx('rounded-[12px]', focus === it.q.id && 'ring-1 ring-(--brand)')}>
                            <QuestionForYou app={app} q={it.q} focus />
                          </div> : <CompactCard q={it.q} d={s.drafts[it.q.id] ?? {}} set={nd => {
                    app.setDraft(it.q.id, nd);
                    if (it.q.type === 'approval' && nd.approval && nd.approval !== 'changes') app.answer(it.q.id, nd.approval === 'approve' ? 'Approved' : 'Declined');
                  }} focused={focus === it.q.id} onFocus={() => setFocus(it.q.id)} onSend={sum => app.answer(it.q.id, sum)} />}
                      </div>)}
                    {agent && unseen > 0 && <WakeBar agent={agent} count={unseen} issue={L.key === 'Project' ? L.title : L.key} onWake={() => app.wake(agent)} />}
                  </div>
                </div>;
          })}
          </div>
        </div>
        <aside className="w-[320px] shrink-0 overflow-y-auto border-l border-(--border) px-4 pt-5">
          <p className="px-1 text-[13px] font-medium text-(--muted-foreground)">On this page</p>
          <div className="mt-2.5 flex flex-col gap-1">
            {queue.map(x => <button key={x.q.id} type="button" onClick={() => jump(x.q.id)} className={cx('flex cursor-pointer items-center gap-3 rounded-[10px] border px-3 py-2.5 text-left', focus === x.q.id ? 'border-(--border) bg-(--surface)' : 'border-transparent hover:bg-(--surface-hover)')}>
                {app.isDone(x) ? <span className="flex size-5 shrink-0 items-center justify-center rounded-full bg-[#e8f3ea]">
                    <CircleCheck size={14} className="text-(--success)" />
                  </span> : <span className="flex size-5 shrink-0 items-center justify-center text-(--muted-foreground)">
                    <TypeGlyph type={x.set ? 'set' : x.q.type} size={14} />
                  </span>}
                <span className={cx('min-w-0 flex-1 text-[13px] leading-[19px]', focus === x.q.id ? 'text-(--foreground)' : 'text-(--muted-foreground)')}>
                  {x.set ? `${SET_TITLE} · ${app.setAnswered()} of 3` : x.q.prompt}
                </span>
                <Avatar id={x.q.asker} size={20} />
              </button>)}
          </div>
        </aside>
      </div>
      <footer className="flex h-9 shrink-0 items-center gap-1.5 border-t border-(--border) px-4 text-[12px] text-(--muted-foreground)">
        <Kbd>1</Kbd>
        <Kbd>2</Kbd>
        <Kbd>3</Kbd> <span className="mr-2">choose</span>
        <Kbd>N</Kbd> <span className="mr-2">note</span>
        <Kbd>J</Kbd>
        <Kbd>K</Kbd> <span className="mr-2">next / previous question</span>
        <Kbd>⌘↵</Kbd> <span>send the focused question</span>
        <span className="flex-1" />
        <Kbd>Esc</Kbd> <span>back to Questions</span>
      </footer>
    </div>;
}
export function FocusFooter() {
  return <footer className="flex h-9 shrink-0 items-center gap-1.5 border-t border-(--border) px-4 text-[12px] text-(--muted-foreground)">
      <Kbd>1</Kbd>
      <Kbd>2</Kbd>
      <Kbd>3</Kbd> <span className="mr-2">choose</span>
      <Kbd>N</Kbd> <span className="mr-2">note</span>
      <Kbd>S</Kbd> <span className="mr-2">skip</span>
      <Kbd>J</Kbd>
      <Kbd>K</Kbd> <span className="mr-2">next / previous</span>
      <Kbd>⌘↵</Kbd> <span>send and next</span>
      <span className="flex-1" />
      <Kbd>Esc</Kbd> <span>back to Questions</span>
    </footer>;
}

/* ───────────────────────── Settings (screen 7) ───────────────────────── */

export function SettingsScreen({
  app
}: {
  app: App;
}) {
  const on = app.state.workspaceDefault;
  const nav: Array<[string, Array<[string, typeof Settings2]>]> = [['De App Specialist', [['General', Settings2], ['Members', Users]]], ['Issues & workflow', [['Issue Statuses', CircleDot], ['Wakeups', AlarmClock], ['Questions', CircleHelp], ['Labels', Tag], ['Properties', Hash], ['Quick Actions', Zap]]], ['Connections & extensions', [['Code', Code2], ['Channels', MessagesSquare], ['MCP servers', Server]]]];
  return <div className="flex h-full">
      <nav className="w-[232px] shrink-0 border-r border-(--border) px-3 pt-8">
        {nav.map(([g, items]) => <div key={g} className="mb-3">
            <p className="px-2 pb-1.5 text-[12px] font-medium text-(--muted-foreground)">{g}</p>
            {items.map(([l, I]) => <button key={l} type="button" className={cx('flex h-8 w-full cursor-pointer items-center gap-2 rounded-[6px] px-2 text-left text-[14px]', l === 'Questions' ? 'bg-(--sidebar-accent) font-medium' : 'text-(--muted-foreground) hover:bg-(--surface-hover)')}>
                <I size={15} /> {l}
              </button>)}
          </div>)}
      </nav>
      <div className="min-w-0 flex-1 overflow-y-auto px-16 pt-12">
        <div className="max-w-[720px]">
          <h1 className="text-[20px] leading-[26px] font-semibold">Questions</h1>
          <p className="mt-1 text-[14px] text-(--muted-foreground)">The default for every issue. Each issue can change it in its properties.</p>
          <h2 className="mt-7 text-[16px] font-semibold">Agents can ask questions</h2>
          <div className="mt-2.5 rounded-[12px] border border-(--border) bg-(--surface)" style={{
          boxShadow: 'var(--surface-shadow)'
        }}>
            <div className="flex items-center px-4 py-3">
              <div className="flex-1">
                <p className="text-[14px] font-medium">On by default</p>
                <p className="text-[12px] text-(--muted-foreground)">3 open issues have their own setting and ignore this default.</p>
              </div>
              <Switch checked={on} onChange={v => app.set({
              workspaceDefault: v
            })} label="On by default" />
            </div>
            <div className="border-t border-(--border) px-4 pt-3.5 pb-4">
              <p className="text-[14px] font-medium">{on ? 'What agents get while questions are on' : 'What agents get while questions are off'}</p>
              {on ? <>
                  <pre className="mc-mono mt-2.5 rounded-[8px] bg-(--muted) px-3 py-3 text-[12px] leading-[18px] whitespace-pre-wrap text-(--foreground)">
                    <span className="font-medium text-(--muted-foreground)">## Questions</span>
                    {'\n\n'}
                    {'`multica question` asks people one or more questions at the same time and reads their answers.'}
                    {'\n\n'}
                    {'`multica question --help` lists its commands, answer types and options.'}
                  </pre>
                  <p className="mt-2.5 text-[12px] text-(--muted-foreground)">While questions are off, agents get none of this, and `multica question` is not in their CLI help.</p>
                </> : <div className="mt-2.5 rounded-[8px] border border-dashed border-(--surface-border) px-3 py-4 text-[13px] text-(--muted-foreground)">
                  Nothing. The brief has no Questions section, `multica --help` does not list `multica question`, and `multica question ask` refuses. People can still ask
                  people.
                </div>}
            </div>
          </div>
        </div>
      </div>
    </div>;
}

/* ───────────────────────── Answer types (screen 3) ───────────────────────── */

export function AnswerTypesScreen({
  app
}: {
  app: App;
}) {
  const [drafts, setDrafts] = useState<Record<string, Draft>>(() => ({
    ...REFERENCE_SEED
  }));
  const [sent, setSent] = useState<Record<string, string>>({});
  return <div className="h-full overflow-y-auto bg-(--page-canvas)" data-scroll="types">
      <div className="px-12 pt-10 pb-12">
        <div className="flex items-start">
          <div className="flex-1">
            <h1 className="text-[24px] leading-[32px] font-semibold">Answer types</h1>
            <p className="mt-1 text-[13px] text-(--muted-foreground)">
              Every type in its open state, as the answerer sees it on the issue, in the inbox and in focus mode. Each keeps Note, Ask back and Reassign so nobody is forced into a
              false answer.
            </p>
          </div>
          {!app.state.clean && <Btn variant="outline" onClick={() => app.back()}>
              <ChevronLeft size={15} /> Back
            </Btn>}
        </div>
        <div className="mt-6 grid grid-cols-3 items-start gap-6">
          {REFERENCE.map(q => sent[q.id] ? <section key={q.id} className="rounded-[12px] border border-(--border) bg-(--surface) px-4 py-4">
                <div className="flex items-center gap-2 text-[13px]">
                  <CircleCheck size={16} className="text-(--success)" />
                  <span className="text-(--muted-foreground)">{q.prompt}</span>
                </div>
                <p className="mt-2 text-[14px] font-semibold">{sent[q.id]}</p>
                <Btn variant="ghost" size="sm" className="mt-2 -ml-2 text-(--foreground)" onClick={() => setSent(({
            [q.id]: _,
            ...rest
          }) => rest)}>
                  Edit answer
                </Btn>
              </section> : <CompactCard key={q.id} q={q} d={drafts[q.id] ?? {}} set={nd => {
          setDrafts({
            ...drafts,
            [q.id]: nd
          });
          if (q.type === 'approval' && nd.approval && nd.approval !== 'changes') setSent({
            ...sent,
            [q.id]: nd.approval === 'approve' ? 'Approved' : 'Declined'
          });
        }} onSend={sum => setSent({
          ...sent,
          [q.id]: sum
        })} />)}
        </div>
      </div>
    </div>;
}
export function Placeholder({
  title,
  children
}: {
  title: string;
  children?: ReactNode;
}) {
  return <div className="flex h-full flex-col">
      <header className="flex h-11 shrink-0 items-center gap-2 border-b border-(--border) px-4 text-[14px] font-medium">
        <Layers size={15} /> {title}
      </header>
      <div className="flex flex-1 flex-col items-center justify-center text-center text-[13px] text-(--muted-foreground)">
        <Check size={20} className="mb-2 text-(--faint-foreground)" />
        {children ?? 'Not part of this prototype.'}
      </div>
    </div>;
}