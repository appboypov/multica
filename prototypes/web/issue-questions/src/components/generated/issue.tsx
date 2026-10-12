import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { AlarmClock, ArrowRight, ArrowUp, AtSign, Bell, Calendar, Check, CheckCircle2, ChevronDown, ChevronRight, CircleCheck, CircleHelp, CirclePause, Clock, Folder, GripVertical, Layers, Link2, ListPlus, MessageSquare, MoreHorizontal, PanelRight, Paperclip, Pencil, Plus, Search, Send, SignalHigh, Star, Users, X } from 'lucide-react';
import { Avatar, Btn, Kbd, Menu, Pill, StatusGlyph, Switch, TypeGlyph, TypeTile, cx, useDismiss } from './ui';
import { AnswerBody, EscapeHatches, TypeOverline, summarize, type Draft } from './cards';
import { PEOPLE, Q_VAT, SET_PARTS, SET_TABS, SET_TITLE, TYPE_META, TYPE_ORDER, type PersonId, type QType, type Question } from './model';
import type { App } from './QuestionsPrototype';

/* ───────────────────────── Issue page ───────────────────────── */

export function IssueScreen({
  app
}: {
  app: App;
}) {
  const s = app.state;
  const builderOpen = s.composer === 'question';
  const panel = s.panelOpen && !builderOpen;
  const scrollRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const box = scrollRef.current;
    if (s.scrollTo && box) {
      const el = box.querySelector(`[data-anchor="${s.scrollTo}"]`) as HTMLElement | null;
      const offset = s.scrollTo === 'builder' ? 209 : 64;
      if (el) box.scrollTo({
        top: box.scrollTop + el.getBoundingClientRect().top - box.getBoundingClientRect().top - offset,
        behavior: 'smooth'
      });
      app.set({
        scrollTo: undefined
      });
    }
  }, [s.scrollTo, app]);
  return <div className="flex h-full flex-col">
      <header className="flex h-11 shrink-0 items-center gap-2 border-b border-(--border) pr-3 pl-4 text-[14px]">
        <button type="button" className="cursor-pointer text-(--muted-foreground) hover:text-(--foreground)">
          Billing
        </button>
        <ChevronRight size={14} className="text-(--faint-foreground)" />
        <span className="text-(--muted-foreground)">WORK-412</span>
        <ChevronRight size={14} className="text-(--faint-foreground)" />
        <span className="font-medium">Export invoices as CSV with VAT breakdown</span>
        <span className="flex-1" />
        {[Bell, Link2, MoreHorizontal].map((I, i) => <button key={i} type="button" className="flex size-6 cursor-pointer items-center justify-center rounded-[6px] text-(--muted-foreground) hover:bg-(--surface-hover)">
            <I size={15} />
          </button>)}
        <button type="button" title="Toggle properties" aria-pressed={s.panelOpen} onClick={() => app.set({
        panelOpen: !s.panelOpen
      })} className={cx('flex size-6 cursor-pointer items-center justify-center rounded-[6px] hover:bg-(--surface-hover)', s.panelOpen ? 'text-(--foreground)' : 'text-(--muted-foreground)')}>
          <PanelRight size={15} />
        </button>
      </header>
      <div className="flex min-h-0 flex-1">
        <div ref={scrollRef} data-scroll="issue" className="min-w-0 flex-1 overflow-y-auto">
          <div className={cx('max-w-[1032px] px-12 pt-7', builderOpen ? 'pb-[260px]' : 'pb-16')}>
            <h1 className="text-[24px] leading-[32px] font-bold tracking-[-0.01em]">Export invoices as CSV with VAT breakdown</h1>
            <p className="mt-3 text-[14px] leading-[22px]">
              Accountants need one CSV per month with a VAT line per rate (21%, 9%, 0%). Rounding must match what the bookkeeping tool reports, or the reconciliation
              fails by a few cents.
            </p>
            <div className="mt-9 h-px bg-(--border)" />
            <div className="mt-4 flex h-6 items-center" data-anchor="activity">
              <h2 className="flex-1 text-[16px] font-semibold">Activity</h2>
              <span className="mr-2 text-[12px] text-(--muted-foreground)">Subscribed</span>
              <Avatar id="brian" size={24} />
            </div>
            <Timeline app={app} />
            <Composer app={app} />
          </div>
        </div>
        {panel && <PropertiesPanel app={app} />}
      </div>
    </div>;
}
function Timeline({
  app
}: {
  app: App;
}) {
  const s = app.state;
  const vat = s.answers.vat;
  const pieterUnseen = app.unseenFor('pieter', 'WORK-412');
  const setLive = s.run === 'ended';
  return <div className="mt-4 flex flex-col gap-3">
      <section className="rounded-[12px] border border-(--border) bg-(--surface) px-4 pt-[15px] pb-[13px]" style={{
      boxShadow: 'var(--surface-shadow)'
    }}>
        <div className="flex items-center gap-2">
          <Avatar id="brian" size={24} />
          <span className="text-[14px] font-medium">Brian</span>
          <span className="text-[12px] text-(--muted-foreground)">1 h ago</span>
        </div>
        <p className="mt-0.5 text-[14px] leading-[21px]">Picked this up for Pieter. The export format is the one in the attached sample.</p>
      </section>
      {!vat && <RunRow text="ended run #3 with a question" waiting time="14 min ago" />}
      <div data-anchor="q-vat">
        {vat ? <FoldedAnswer q={Q_VAT} summary={vat.summary} meta={`Brian · ${vat.when}${vat.note ? ' · with a note' : ''}`} onEdit={() => app.editAnswer('vat')} /> : <QuestionForYou app={app} q={Q_VAT} />}
      </div>
      {s.woken && <RunRow text={s.run === 'running' ? `was woken by Brian with ${s.wokenWith} answer${s.wokenWith === 1 ? '' : 's'} · run #4 is working` : `was woken by Brian with ${s.wokenWith} answer${s.wokenWith === 1 ? '' : 's'}, then ended run #4 with 3 questions`} waiting={setLive && !app.setDone()} running={s.run === 'running'} time={s.run === 'running' ? 'just now' : '3 min ago'} />}
      {setLive && <EachCard app={app} />}
      {setLive && <div data-anchor="q-set">
          <SetCard app={app} />
        </div>}
      {pieterUnseen > 0 && !s.woken && <WakeBar agent="pieter" count={pieterUnseen} issue="WORK-412" onWake={() => app.wake('pieter')} />}
      {s.woken && setLive && pieterUnseen > 0 && app.setDone() && <WakeBar agent="pieter" count={pieterUnseen} issue="WORK-412" onWake={() => app.wake('pieter')} />}
      {s.asked.map(a => <AskedCard key={a.id} ask={a} app={app} />)}
    </div>;
}
export function RunRow({
  text,
  waiting,
  running,
  time
}: {
  text: string;
  waiting?: boolean;
  running?: boolean;
  time: string;
}) {
  return <div className="flex h-9 items-center gap-2 pr-1 pl-1">
      <Avatar id="pieter" size={22} />
      <span className="text-[13px] font-medium">Pieter</span>
      <span className="text-[12px] text-(--muted-foreground)">{text}</span>
      {waiting && <>
          <span className="text-[12px] text-(--faint-foreground)">·</span>
          <Pill tone="warning" icon={<CirclePause size={12} />}>
            Waiting for answer
          </Pill>
        </>}
      {running && <Pill tone="brand" icon={<span className="mc-pulse size-1.5 rounded-full bg-(--brand)" />}>
          Running
        </Pill>}
      <span className="flex-1" />
      <span className="text-[12px] text-(--muted-foreground)">{time}</span>
    </div>;
}

/** An answered question folds to one line, the way resolved threads fold. */
export function FoldedAnswer({
  q,
  summary,
  meta,
  onEdit,
  prompt,
  fresh
}: {
  q: Question;
  summary: string;
  meta: string;
  onEdit?: () => void;
  prompt?: string;
  fresh?: boolean;
}) {
  const [open, setOpen] = useState(false);
  return <section className={cx('rounded-[12px] border border-(--border) bg-(--surface)', fresh && 'mc-fresh')} style={{
    boxShadow: 'var(--surface-shadow)'
  }}>
      <div className="flex h-[50px] items-center gap-2 px-4">
        <CircleCheck size={16} className="shrink-0 text-(--success)" />
        <Avatar id={q.asker} size={20} />
        <span className="ml-1 min-w-0 truncate text-[13px] text-(--muted-foreground)">{prompt ?? q.prompt}</span>
        <ArrowRight size={14} className="shrink-0 text-(--muted-foreground)" />
        <span className="shrink-0 text-[13px] font-semibold">{summary}</span>
        <span className="shrink-0 truncate text-[12px] text-(--muted-foreground)">· {meta}</span>
        <span className="flex-1" />
        {onEdit && <Btn variant="ghost" size="sm" className="text-(--foreground)" onClick={onEdit}>
            <Pencil size={14} /> Edit answer
          </Btn>}
        <button type="button" aria-label="Expand" onClick={() => setOpen(!open)} className="flex size-7 cursor-pointer items-center justify-center rounded-[6px] text-(--muted-foreground) hover:bg-(--surface-hover)">
          <ChevronDown size={15} className={cx('transition-transform', open && 'rotate-180')} />
        </button>
      </div>
      {open && <div className="border-t border-(--border) px-4 py-3 text-[12px] text-(--muted-foreground)">
          {PEOPLE[q.asker].name} asked: “{q.prompt}”. The answer reached {PEOPLE[q.asker].name} the moment it was sent;{' '}
          {PEOPLE[q.asker].agent ? 'an agent reads it once a person wakes it.' : 'they see it in their inbox.'} Editing it notifies the asker.
        </div>}
    </section>;
}
export function WakeBar({
  agent,
  count,
  issue,
  onWake,
  className
}: {
  agent: PersonId;
  count: number;
  issue: string;
  onWake: () => void;
  className?: string;
}) {
  const name = PEOPLE[agent].name;
  return <div className={cx('flex h-[58px] items-center gap-2.5 rounded-[12px] border border-(--border) bg-(--surface) pr-3 pl-3.5', className)}>
      <AlarmClock size={15} className="text-(--muted-foreground)" />
      <span className="flex-1 text-[13px] text-(--muted-foreground)">
        {name} has {count} answer{count === 1 ? '' : 's'} from you on {issue}. Nothing wakes {agent === 'pieter' || agent === 'joeri' ? 'him' : 'them'} until you do.
      </span>
      <Btn variant="outline" size="lg" onClick={onWake}>
        <AlarmClock size={15} /> Wake {name}
      </Btn>
    </div>;
}
function QuestionHeader({
  q,
  setCount,
  right
}: {
  q: Question;
  setCount?: number;
  right?: ReactNode;
}) {
  return <div className="flex h-5 items-center gap-2">
      <TypeTile type={setCount ? 'set' : q.type} brand />
      <span className="text-[11px] font-semibold tracking-[0.055em] text-(--brand) uppercase">{setCount ? 'Question set for you' : 'Question for you'}</span>
      <span className="text-[12px] text-(--muted-foreground)">· {setCount ? `${setCount} questions` : TYPE_META[q.type].label}</span>
      <span className="flex-1" />
      {right ?? <>
          {q.blocks && <Pill tone="danger" icon={<CirclePause size={12} />}>
              Blocks work
            </Pill>}
          {q.due && <Pill tone="neutral" className="text-(--muted-foreground)" icon={<Calendar size={12} />}>
              Answer by today
            </Pill>}
        </>}
    </div>;
}

/** Screen 1: the open card on the issue. While nothing is picked, the primary action accepts the suggestion. */
export function QuestionForYou({
  app,
  q,
  focus
}: {
  app: App;
  q: Question;
  focus?: boolean;
}) {
  const d = app.state.drafts[q.id] ?? {};
  const set = (nd: Draft) => app.setDraft(q.id, nd);
  const [noteOpen, setNoteOpen] = useState(false);
  const [aside, setAside] = useState<string | null>(null);
  const summary = summarize(q, d);
  const suggested = q.options?.findIndex(o => o.suggested) ?? -1;
  const send = () => {
    if (summary) app.answer(q.id, summary, d.why);else if (suggested >= 0) app.answer(q.id, q.options![suggested].label, d.why);
  };
  return <section className="rounded-[12px] border border-(--brand) bg-(--surface) px-4 pt-[14px] pb-[9px]" style={{
    boxShadow: '0 0 0 0.5px var(--brand), var(--surface-shadow)'
  }}>
      <QuestionHeader q={q} />
      <div className="mt-3 flex items-center gap-2">
        <Avatar id={q.asker} size={24} />
        <span className="text-[14px] font-medium">{PEOPLE[q.asker].name}</span>
        <span className="text-[12px] text-(--muted-foreground)">{q.askedLine}</span>
      </div>
      <h3 className="mt-3 text-[16px] leading-[22px] font-semibold">{q.prompt}</h3>
      {q.context && <p className="mt-3 text-[14px] leading-[20px] text-(--muted-foreground)">{q.context}</p>}
      <div className="mt-[11px]">
        <AnswerBody q={q} d={d} set={set} />
      </div>
      {noteOpen && <textarea autoFocus value={d.why ?? ''} onChange={e => set({
      ...d,
      why: e.target.value
    })} placeholder={`Note for ${PEOPLE[q.asker].name} (optional)`} rows={2} className="mt-3 block w-full resize-none rounded-[8px] border border-(--input) px-2.5 py-2 text-[14px] outline-none focus:border-(--ring)" />}
      {aside && <p className="mt-3 rounded-[8px] bg-(--muted) px-2.5 py-2 text-[12px] text-(--muted-foreground)">{aside}</p>}
      <div className="mt-5 flex items-center gap-2">
        <EscapeHatches onNote={() => setNoteOpen(true)} onAskBack={() => setAside('Ask back replies in the question’s thread and keeps it open; Pieter gets your reply when he is woken.')} onReassign={() => setAside('Reassign hands the question to another member with a note; the card keeps the history.')} />
        <span className="flex-1" />
        {!focus && <>
            <span className="text-[12px] text-(--muted-foreground)">Asked to</span>
            <Avatar id="brian" size={24} />
          </>}
        <Btn variant="primary" size="lg" className="ml-2" onClick={send}>
          {summary ? <Send size={15} /> : <Check size={15} />}
          {summary ? 'Send' : 'Accept suggestion'}
        </Btn>
      </div>
    </section>;
}

/** Screen 6: an "each person answers" question, two of three answers in. */
function EachCard({
  app
}: {
  app: App;
}) {
  const s = app.state;
  if (s.eachClosed) return <FoldedAnswer q={{
    ...Q_VAT,
    asker: 'sarah',
    prompt: 'Which export layout should accountants get?'
  }} summary="One CSV per month" meta="closed by Sarah · 2 of 3 answered" />;
  return <section className="rounded-[12px] border border-(--border) bg-(--surface) px-4 pt-4 pb-3.5" style={{
    boxShadow: 'var(--surface-shadow)'
  }}>
      <TypeOverline type="single" right={<Pill tone="neutral" icon={<Users size={12} />}>
            2 of 3 answered
          </Pill>} />
      <div className="mt-3 flex items-center gap-2">
        <Avatar id="sarah" size={22} />
        <span className="text-[14px] font-medium">Sarah</span>
        <span className="text-[12px] text-(--muted-foreground)">asked Brian, Mark and Eva · each answers · 1 h ago</span>
      </div>
      <h3 className="mt-3 text-[16px] leading-[22px] font-semibold">Which export layout should accountants get?</h3>
      <div className="mt-3 flex flex-wrap gap-1.5">
        {([['brian', 'One CSV per month'], ['mark', 'One CSV per month'], ['eva', null]] as Array<[PersonId, string | null]>).map(([p, a]) => <span key={p} className="inline-flex h-8 items-center gap-1.5 rounded-[8px] border border-(--border) px-2 text-[13px]">
            <Avatar id={p} size={18} />
            <span className="font-medium">{PEOPLE[p].name}</span>
            {a ? <Check size={14} className="text-(--success)" /> : <Clock size={14} className="text-(--muted-foreground)" />}
            <span className="text-[12px] text-(--muted-foreground)">{a ?? (s.remindedEva ? 'reminded' : 'waiting')}</span>
          </span>)}
      </div>
      <p className="mt-3 text-[12px] text-(--muted-foreground)">Sarah sees each answer as it arrives and closes the question when she has enough.</p>
      <div className="mt-3 flex items-center">
        <Btn variant="ghost" size="sm" disabled={s.remindedEva} onClick={() => app.set({
        remindedEva: true
      })}>
          <Bell size={14} /> {s.remindedEva ? 'Eva reminded' : 'Remind Eva'}
        </Btn>
        <span className="flex-1" />
        <Btn variant="outline" size="lg" onClick={() => app.set({
        eachClosed: true
      })}>
          <CheckCircle2 size={15} /> Close question
        </Btn>
      </div>
    </section>;
}

/** Screen 6: a question set. Each part is answered and sent on its own. */
export function SetCard({
  app,
  embedded
}: {
  app: App;
  embedded?: boolean;
}) {
  const s = app.state;
  const parts = SET_PARTS;
  const firstOpen = parts.findIndex(p => !s.answers[p.id]);
  const [active, setActive] = useState(firstOpen < 0 ? 0 : firstOpen);
  const answered = parts.filter(p => s.answers[p.id]).length;
  const cur = parts[active];
  const d = s.drafts[cur.id] ?? {};
  const summary = summarize(cur, d);
  const unseen = app.unseenFor('pieter', 'WORK-412');
  const sendCurrent = () => {
    if (summary && !s.answers[cur.id]) app.answer(cur.id, summary);
    const next = parts.findIndex((p, i) => i !== active && !s.answers[p.id] && !(summary && i === active));
    if (next >= 0) setActive(next);
  };
  return <section className={cx('rounded-[12px] border bg-(--surface) px-4 pt-4 pb-3.5', embedded ? '' : '')} style={{
    borderColor: 'var(--brand)',
    boxShadow: '0 0 0 0.5px var(--brand), var(--surface-shadow)'
  }}>
      <QuestionHeader q={{
      ...cur,
      blocks: true
    }} setCount={3} right={<Pill tone="danger" icon={<CirclePause size={12} />}>
            Blocks work
          </Pill>} />
      <div className="mt-3 flex items-center gap-2">
        <Avatar id="pieter" size={22} />
        <span className="text-[14px] font-medium">Pieter</span>
        <span className="text-[12px] text-(--muted-foreground)">asked during run #4 · 2 min ago</span>
      </div>
      <h3 className="mt-3 text-[16px] leading-[22px] font-semibold">{SET_TITLE}</h3>
      {!embedded && <div className="mt-3 inline-flex rounded-[9px] bg-(--muted) p-[3px]" role="tablist">
        {parts.map((p, i) => <button key={p.id} role="tab" aria-selected={i === active} type="button" onClick={() => setActive(i)} className={cx('flex h-[30px] cursor-pointer items-center gap-2 rounded-[7px] px-2.5 text-[14px]', i === active ? 'bg-(--surface) font-medium shadow-sm' : s.answers[p.id] ? 'text-(--foreground)' : 'text-(--muted-foreground)')}>
            {s.answers[p.id] ? <CircleCheck size={15} className="text-(--success)" /> : <span className="flex size-[18px] items-center justify-center rounded-full border border-(--surface-border) text-[10px] text-(--muted-foreground)">{i + 1}</span>}
            {SET_TABS[i]}
          </button>)}
      </div>}
      <div className="mt-3 flex flex-col gap-3">
        {parts.map((p, i) => {
        const a = s.answers[p.id];
        if (a && (embedded || i !== active)) return <div key={p.id} className="flex h-[48px] items-center gap-2 rounded-[10px] bg-(--muted) px-3">
                <CircleCheck size={15} className="text-(--success)" />
                <span className="text-[14px] font-medium">{i === 0 ? 'Confirm facts' : p.prompt}</span>
                <span className="min-w-0 flex-1 truncate text-[12px] text-(--muted-foreground)">{a.summary}</span>
                <Btn variant="ghost" size="sm" className="text-(--foreground)" onClick={() => (app.editAnswer(p.id), setActive(i))}>
                  Change
                </Btn>
              </div>;
        if (embedded || i === active) {
          const pd = s.drafts[p.id] ?? {};
          const ps = summarize(p, pd);
          return <div key={p.id} className="rounded-[10px] border border-(--border) px-3 pt-3 pb-3">
                <div className="mb-2.5 flex items-center gap-2">
                  <TypeTile type={p.type} size={20} />
                  <span className="text-[14px] font-medium">{i === 0 ? 'Before I build the export, are these right?' : p.prompt}</span>
                </div>
                <AnswerBody q={p} d={pd} set={nd => app.setDraft(p.id, nd)} />
                {embedded && <div className="mt-2.5 flex justify-end">
                    <Btn variant="primary" size="sm" disabled={!ps} onClick={() => ps && app.answer(p.id, ps)}>
                      <Send size={14} /> Send
                    </Btn>
                  </div>}
              </div>;
        }
        return <button key={p.id} type="button" onClick={() => setActive(i)} className="flex h-10 cursor-pointer items-center gap-2 rounded-[10px] border border-(--border) px-3 text-left hover:bg-(--surface-hover)">
              <TypeTile type={p.type} size={20} />
              <span className="flex-1 text-[14px] text-(--muted-foreground)">{i === 0 ? 'Before I build the export, are these right?' : p.prompt}</span>
              <span className="text-[12px] text-(--muted-foreground)">{TYPE_META[p.type].label}</span>
            </button>;
      })}
      </div>
      <div className="mt-3 flex items-center gap-2">
        <EscapeHatches />
        <span className="flex-1" />
        <span className="mr-2 text-[12px] text-(--muted-foreground)">{answered} of 3 answered</span>
        {!embedded && unseen > 0 && <Btn variant="outline" size="lg" onClick={() => app.wake('pieter')}>
            <AlarmClock size={15} /> Wake Pieter
          </Btn>}
        {!embedded && answered < 3 && <Btn variant="primary" size="lg" disabled={!summary && !s.answers[cur.id]} onClick={sendCurrent}>
            <ArrowRight size={15} /> {parts.filter(p => !s.answers[p.id]).length <= 1 && !s.answers[cur.id] ? 'Send' : 'Next question'}
          </Btn>}
      </div>
    </section>;
}

/* ───────────────────────── Properties ───────────────────────── */

function PropertiesPanel({
  app
}: {
  app: App;
}) {
  const s = app.state;
  const menu = s.propMenu;
  const setMenu = (v: boolean) => app.set({
    propMenu: v
  });
  const ref = useDismiss(menu, () => app.set({
    propMenu: false
  }));
  const open = app.openOnIssue();
  const effective = s.issueSetting === 'default' ? s.workspaceDefault : s.issueSetting === 'on';
  const rows: Array<[string, ReactNode]> = [['Status', <>
        <StatusGlyph status="in_progress" /> In progress
      </>], ['Assignee', <>
        <Avatar id="pieter" size={20} /> Pieter
      </>], ['Project', <>
        <Folder size={15} className="text-(--muted-foreground)" /> Billing
      </>], ['Priority', <>
        <SignalHigh size={15} className="text-(--warning)" /> High
      </>]];
  return <aside className="w-[320px] shrink-0 overflow-y-auto border-l border-(--border) px-4 pt-[14px]">
      <div className="flex h-4 items-center px-2">
        <span className="flex-1 text-[12px] font-medium text-(--muted-foreground)">Properties</span>
        <ChevronDown size={15} className="text-(--muted-foreground)" />
      </div>
      <div className="flex flex-col">
        {rows.map(([k, v]) => <div key={k} className="flex h-[34px] items-center px-2 text-[13px]">
            <span className="w-[108px] shrink-0 text-[12px] text-(--muted-foreground)">{k}</span>
            <span className="flex items-center gap-1.5">{v}</span>
          </div>)}
        <button type="button" onClick={() => app.set({
        scrollTo: app.state.run === 'ended' && !app.setDone() ? 'q-set' : 'q-vat'
      })} className={cx('flex h-[34px] cursor-pointer items-center rounded-[6px] px-2 text-left text-[13px]', open ? 'bg-[#ecf3f9] hover:bg-[#e3eef8]' : 'hover:bg-(--surface-hover)')}>
          <span className="w-[108px] shrink-0 text-[12px] text-(--muted-foreground)">Questions</span>
          {open ? <span className="flex items-center gap-1.5 text-(--brand)">
              <CircleHelp size={15} /> <span className="font-medium">{open} open</span> <span className="text-[12px] text-(--muted-foreground)">·</span>
              <span className="text-(--destructive)">blocks work</span>
            </span> : <span className="text-(--muted-foreground)">None open</span>}
        </button>
        <div className="relative" ref={ref}>
          <button type="button" onClick={() => setMenu(!menu)} aria-expanded={menu} className="flex h-[34px] w-full cursor-pointer items-center rounded-[6px] px-2 text-left text-[13px] hover:bg-(--surface-hover)">
            <span className="w-[108px] shrink-0 text-[12px] text-(--muted-foreground)">Agent questions</span>
            <span className="flex items-center gap-1.5">
              <BotIcon /> {effective ? 'On' : 'Off'}
              {s.issueSetting === 'default' && <span className="text-[12px] text-(--muted-foreground)">· workspace default</span>}
            </span>
          </button>
          {menu && <Menu className="top-[38px] right-0 w-[218px]">
              {([['default', 'Workspace default', s.workspaceDefault ? 'On' : 'Off'], ['on', 'On', ''], ['off', 'Off', '']] as const).map(([k, l, sub]) => <button key={k} role="menuitem" type="button" onClick={() => (app.set({
            issueSetting: k
          }), setMenu(false))} className={cx('flex w-full cursor-pointer items-center rounded-[7px] px-2 text-left hover:bg-(--surface-hover)', sub ? 'py-1.5' : 'h-[34px]', s.issueSetting === k && 'bg-(--surface-hover)')}>
                  <span className="flex-1">
                    <span className="block text-[13px]">{l}</span>
                    {sub && <span className="block text-[12px] text-(--muted-foreground)">{sub}</span>}
                  </span>
                  {s.issueSetting === k && <Check size={15} />}
                </button>)}
            </Menu>}
        </div>
        <div className="flex h-[34px] items-center px-2 text-[13px]">
          <span className="w-[108px] shrink-0 text-[12px] text-(--muted-foreground)">Labels</span>
          <span className="flex gap-2">
            <Pill tone="neutral">export</Pill>
            <Pill tone="neutral">vat</Pill>
          </span>
        </div>
      </div>
      <div className="mt-4 border-t border-(--border) pt-4">
        <p className="px-2 text-[12px] font-medium text-(--muted-foreground)">Agent runs</p>
        <div className="flex flex-col">
          {s.woken && <div className="flex h-[34px] items-center gap-2 px-2 text-[13px]">
              {s.run === 'running' ? <span className="mc-pulse size-2 rounded-full bg-(--brand)" /> : <CirclePause size={15} className="text-(--warning)" />}
              <span className="font-medium">Run #4</span>
              <span className="text-[12px] text-(--muted-foreground)">{s.run === 'running' ? 'Running · just now' : 'Waiting for answers · 3 min ago'}</span>
            </div>}
          <div className="flex h-[34px] items-center gap-2 px-2 text-[13px]">
            {s.woken ? <CircleCheck size={15} className="text-(--success)" /> : <CirclePause size={15} className="text-(--warning)" />}
            <span className="font-medium">Run #3</span>
            <span className="text-[12px] text-(--muted-foreground)">{s.woken ? 'Completed · 14 min ago' : 'Ended with a question for Brian'}</span>
          </div>
          <div className="flex h-[34px] items-center gap-2 px-2 text-[13px]">
            <CircleCheck size={15} className="text-(--success)" />
            <span className="font-medium">Run #2</span>
            <span className="text-[12px] text-(--muted-foreground)">Completed · 2 h ago</span>
          </div>
        </div>
      </div>
    </aside>;
}
function BotIcon() {
  return <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" className="text-(--muted-foreground)">
      <path d="M12 8V4H8" />
      <rect width="16" height="12" x="4" y="8" rx="2" />
      <path d="M2 14h2M20 14h2M15 13v2M9 13v2" />
    </svg>;
}

/* ───────────────────────── Composer & question builder ───────────────────────── */

interface BuilderQ {
  type: QType;
  prompt: string;
  context: string;
  options: {
    label: string;
    suggested: boolean;
  }[];
  allowOther: boolean;
}
export interface Ask {
  id: string;
  questions: BuilderQ[];
  to: PersonId[];
  policy: 'any' | 'each';
  due: string;
  blocks: boolean;
}
const blankQ = (type: QType = 'single'): BuilderQ => ({
  type,
  prompt: '',
  context: '',
  options: [{
    label: '',
    suggested: false
  }, {
    label: '',
    suggested: false
  }],
  allowOther: true
});
const EXAMPLE_Q: BuilderQ = {
  type: 'single',
  prompt: 'Which export layout should accountants get?',
  context: '',
  options: [{
    label: 'One CSV per month',
    suggested: true
  }, {
    label: 'One CSV per VAT rate',
    suggested: false
  }, {
    label: 'One XLSX, a sheet per rate',
    suggested: false
  }],
  allowOther: true
};
function Composer({
  app
}: {
  app: App;
}) {
  const s = app.state;
  const [text, setText] = useState('');
  if (s.composer === 'question') return <Builder app={app} />;
  return <section className="mt-4 rounded-[12px] border border-(--border) bg-(--surface) px-3 pt-3 pb-px" style={{
    boxShadow: 'var(--surface-shadow)'
  }}>
      <input value={text} onChange={e => {
      const v = e.target.value;
      if (v.trim() === '/question') {
        setText('');
        app.openBuilder();
      } else setText(v);
    }} placeholder="Leave a comment…" className="h-6 w-full bg-transparent text-[14px] outline-none placeholder:text-(--muted-foreground)" />
      {text.startsWith('/') && <div className="mt-1 w-[280px] rounded-[8px] border border-(--border) p-1" style={{
      boxShadow: 'var(--menu-shadow)'
    }}>
          <button type="button" onClick={() => (setText(''), app.openBuilder())} className="flex h-9 w-full cursor-pointer items-center gap-2 rounded-[6px] bg-(--surface-hover) px-2 text-[13px]">
            <CircleHelp size={15} /> /question <span className="text-[11px] text-(--muted-foreground)">Ask a question</span>
          </button>
        </div>}
      <div className="mt-4 flex items-center gap-1">
        <button type="button" aria-label="Attach" className="flex h-7 w-4 cursor-pointer items-center justify-center text-(--muted-foreground) hover:text-(--foreground)">
          <Paperclip size={16} />
        </button>
        <button type="button" aria-label="Mention" className="flex h-7 w-4 cursor-pointer items-center justify-center text-(--muted-foreground) hover:text-(--foreground)">
          <AtSign size={16} />
        </button>
        <Btn variant="ghost" size="sm" onClick={() => app.openBuilder()}>
          <CircleHelp size={14} /> Question
        </Btn>
        <span className="flex-1" />
        <button type="button" aria-label="Send comment" onClick={() => setText('')} className="flex size-7 cursor-pointer items-center justify-center rounded-[8px] bg-(--primary) text-white hover:bg-[#27272a]">
          <ArrowUp size={14} />
        </button>
      </div>
    </section>;
}
const ASKABLE: PersonId[] = ['brian', 'sarah', 'mark', 'eva'];
function Builder({
  app
}: {
  app: App;
}) {
  const s = app.state;
  const [qs, setQs] = useState<BuilderQ[]>(() => [s.builderExample ? structuredClone(EXAMPLE_Q) : blankQ()]);
  const [cur, setCur] = useState(0);
  const [to, setTo] = useState<PersonId[]>(s.builderExample ? ['brian', 'sarah'] : ['brian']);
  const [policy, setPolicy] = useState<'any' | 'each'>('any');
  const [due, setDue] = useState('Fri 16 Oct');
  const [blocks, setBlocks] = useState(true);
  const [links, setLinks] = useState<string[]>(['WORK-412']);
  const menu = s.builderMenu;
  const setMenu = (m: typeof s.builderMenu) => app.set({
    builderMenu: m
  });
  const q = qs[cur];
  const upd = (patch: Partial<BuilderQ>) => setQs(qs.map((x, i) => i === cur ? {
    ...x,
    ...patch
  } : x));
  const canAsk = qs.every(x => x.prompt.trim()) && to.length > 0;
  const ask = () => {
    if (!canAsk) return;
    app.postAsk({
      id: `ask-${Date.now()}`,
      questions: qs,
      to,
      policy,
      due,
      blocks
    });
  };
  const choiceType = q.type === 'single' || q.type === 'multiple' || q.type === 'ranking';
  return <section data-anchor="builder" className="relative mt-4 rounded-[12px] border border-(--ring) bg-(--surface) px-4 pt-[13px] pb-3.5" style={{
    boxShadow: 'var(--surface-shadow)'
  }} onKeyDown={e => {
    if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) e.preventDefault(), ask();
  }}>
      <div className="flex items-center gap-2">
        <div className="flex rounded-[8px] bg-(--muted) p-[3px]">
          <button type="button" onClick={() => app.closeBuilder()} className="flex h-[22px] cursor-pointer items-center gap-1.5 rounded-[6px] px-2.5 text-[13px] font-medium text-(--muted-foreground) hover:text-(--foreground)">
            <MessageSquare size={14} /> Comment
          </button>
          <span className="flex h-[22px] items-center gap-1.5 rounded-[6px] bg-(--surface) px-2.5 text-[13px] font-medium shadow-sm">
            <CircleHelp size={14} /> Question
          </span>
        </div>
        {qs.length > 1 && <div className="ml-2 flex items-center gap-1">
            {qs.map((x, i) => <button key={i} type="button" onClick={() => setCur(i)} className={cx('flex h-7 cursor-pointer items-center gap-1.5 rounded-[7px] px-2 text-[12px]', i === cur ? 'bg-(--muted) font-medium' : 'text-(--muted-foreground) hover:bg-(--surface-hover)')}>
                <TypeGlyph type={x.type} size={13} /> {i + 1}
                {x.prompt ? '' : ' · new'}
              </button>)}
          </div>}
        <span className="flex-1" />
        <div className="relative">
          <Btn variant="outline" onClick={() => setMenu(menu === 'type' ? null : 'type')}>
            <TypeGlyph type={q.type} size={14} /> {TYPE_META[q.type].label}
          </Btn>
          {menu === 'type' && <TypeMenu value={q.type} onPick={t => (upd({
          type: t
        }), setMenu(null))} onClose={() => setMenu(null)} onReference={() => app.go('types')} />}
        </div>
        <button type="button" aria-label="Collapse" onClick={() => app.closeBuilder()} className="flex size-7 cursor-pointer items-center justify-center rounded-[6px] text-(--muted-foreground) hover:bg-(--surface-hover)">
          <ChevronDown size={16} />
        </button>
      </div>
      <input autoFocus={!s.builderExample} value={q.prompt} onChange={e => upd({
      prompt: e.target.value
    })} placeholder="Ask your question" className="mt-3 h-[22px] w-full bg-transparent text-[16px] leading-[22px] font-semibold outline-none placeholder:font-semibold" />
      <input value={q.context} onChange={e => upd({
      context: e.target.value
    })} placeholder="Add context: why you ask, what it affects (optional)" className="mt-[3px] h-5 w-full bg-transparent text-[14px] outline-none placeholder:text-(--faint-foreground)" />
      {choiceType ? <div className="mt-1.5 flex flex-col gap-1.5">
          {q.options.map((o, i) => <div key={i} className="group flex h-9 items-center gap-2 rounded-[8px] border border-(--input) bg-(--surface) px-2.5">
              <GripVertical size={14} className="cursor-grab text-(--faint-foreground)" />
              {q.type === 'single' ? <span className="size-4 shrink-0 rounded-full border border-(--input)" /> : q.type === 'multiple' ? <span className="size-4 shrink-0 rounded-[4px] border border-(--input)" /> : <span className="w-4 text-center text-[12px] text-(--muted-foreground)">{i + 1}</span>}
              <input value={o.label} onChange={e => upd({
          options: q.options.map((x, j) => j === i ? {
            ...x,
            label: e.target.value
          } : x)
        })} placeholder={`Option ${i + 1}`} className="min-w-0 flex-1 bg-transparent text-[14px] outline-none" />
              {q.type === 'single' && (o.suggested ? <button type="button" onClick={() => upd({
          options: q.options.map(x => ({
            ...x,
            suggested: false
          }))
        })} className="cursor-pointer">
                    <Pill tone="brand" icon={<Star size={12} />}>
                      Suggested
                    </Pill>
                  </button> : <button type="button" onClick={() => upd({
          options: q.options.map((x, j) => ({
            ...x,
            suggested: j === i
          }))
        })} className="flex cursor-pointer items-center gap-1.5 px-1 text-[13px] text-(--muted-foreground) opacity-0 group-hover:opacity-100 hover:text-(--foreground) focus-visible:opacity-100">
                    <Star size={14} /> Mark suggested
                  </button>)}
              <button type="button" aria-label="Remove option" onClick={() => upd({
          options: q.options.filter((_, j) => j !== i)
        })} className="ml-1 flex size-6 cursor-pointer items-center justify-center rounded-[5px] text-(--muted-foreground) hover:bg-(--surface-hover)">
                <X size={15} />
              </button>
            </div>)}
          <div className="flex h-8 items-center">
            <Btn variant="ghost" size="sm" className="-ml-1" onClick={() => upd({
          options: [...q.options, {
            label: '',
            suggested: false
          }]
        })}>
              <Plus size={15} /> Add option
            </Btn>
            <span className="flex-1" />
            {q.type === 'single' && <label className="flex items-center gap-2.5 text-[13px] text-(--muted-foreground)">
                Allow “Other”
                <Switch checked={q.allowOther} onChange={v => upd({
            allowOther: v
          })} label="Allow Other" />
              </label>}
          </div>
        </div> : <div className="mt-3 rounded-[8px] border border-dashed border-(--surface-border) px-3 py-3 text-[13px] text-(--muted-foreground)">
          <span className="font-medium text-(--foreground)">{TYPE_META[q.type].label}</span> · {TYPE_META[q.type].hint}. The answerer sees the {TYPE_META[q.type].label.toLowerCase()} control
          shown on the answer types sheet.{' '}
          <button type="button" onClick={() => app.go('types')} className="cursor-pointer text-(--brand) hover:underline">
            See it
          </button>
        </div>}
      <div className="mt-[10px] h-px bg-(--border)" />
      <div className="mt-[14px] flex items-center gap-2">
        <div className="relative">
          <span className="flex h-8 items-center gap-2 rounded-[8px] border border-(--input) pr-1.5 pl-2.5 text-[13px]">
            <span className="text-[12px] text-(--muted-foreground)">Linked to</span>
            {links.map(l => <span key={l} className="flex items-center gap-1.5 text-[13px]">
                <Layers size={15} /> {l}
              </span>)}
            <button type="button" aria-label="Add link" onClick={() => setMenu(menu === 'link' ? null : 'link')} className="flex size-6 cursor-pointer items-center justify-center rounded-[5px] hover:bg-(--surface-hover)">
              <Plus size={15} />
            </button>
          </span>
          {menu === 'link' && <LinkMenu picked={links} onPick={l => (setLinks(links.includes(l) ? links.filter(x => x !== l) : [...links, l]), setMenu(null))} onClose={() => setMenu(null)} />}
        </div>
        <div className="relative">
          <button type="button" onClick={() => setMenu(menu === 'to' ? null : 'to')} className="flex h-8 cursor-pointer items-center gap-2 rounded-[8px] border border-(--input) px-2.5 text-[13px] hover:bg-(--surface-hover)">
            <span className="text-[12px] text-(--muted-foreground)">Asked to</span>
            <span className="flex -space-x-1">
              {to.slice(0, 3).map(p => <span key={p} className="rounded-full ring-2 ring-white">
                  <Avatar id={p} size={20} />
                </span>)}
            </span>
            <span className="text-[13px]">{to.length ? to.map(p => PEOPLE[p].name).join(', ') : 'Nobody yet'}</span>
            {to.length > 1 && <span className="text-(--muted-foreground)">· {policy === 'any' ? 'any one' : 'each'}</span>}
            <ChevronDown size={14} className="text-(--muted-foreground)" />
          </button>
          {menu === 'to' && <AskedToMenu to={to} setTo={setTo} policy={policy} setPolicy={setPolicy} onClose={() => setMenu(null)} />}
        </div>
        <div className="relative">
          <button type="button" onClick={() => setMenu(menu === 'due' ? null : 'due')} className="flex h-8 cursor-pointer items-center gap-2 rounded-[8px] border border-(--input) px-2.5 text-[13px] hover:bg-(--surface-hover)">
            <span className="text-[12px] text-(--muted-foreground)">Answer by</span>
            <Calendar size={15} />
            <span className="text-[13px]">{due || 'No date'}</span>
          </button>
          {menu === 'due' && <DueMenu onPick={v => (setDue(v), setMenu(null))} onClose={() => setMenu(null)} />}
        </div>
        <span className="flex-1" />
        <label className="flex items-center gap-2.5 text-[13px] text-(--muted-foreground)">
          Blocks work
          <Switch checked={blocks} onChange={setBlocks} label="Blocks work" />
        </label>
      </div>
      <div className="mt-[14px] flex items-center gap-2">
        <Btn variant="ghost" size="sm" className="-ml-1 text-(--foreground)" onClick={() => (setQs([...qs, blankQ()]), setCur(qs.length))}>
          <ListPlus size={16} /> Add question
        </Btn>
        <span className="text-[12px] text-(--muted-foreground)">{qs.length > 1 ? `${qs.length} questions go out at the same time, as a question set` : 'asks several questions at the same time'}</span>
        <span className="flex-1" />
        <Btn variant="outline" onClick={() => app.closeBuilder()}>
          Cancel
        </Btn>
        <Btn variant="primary" disabled={!canAsk} onClick={ask}>
          <Send size={15} /> Ask
        </Btn>
        <Kbd>⌘↵</Kbd>
      </div>
    </section>;
}
export function TypeMenu({
  value,
  onPick,
  onClose,
  onReference
}: {
  value: QType;
  onPick: (t: QType) => void;
  onClose: () => void;
  onReference?: () => void;
}) {
  const ref = useDismiss(true, onClose);
  const [f, setF] = useState('');
  const groups = useMemo(() => {
    const m = TYPE_ORDER.filter(t => `${TYPE_META[t].label} ${TYPE_META[t].hint}`.toLowerCase().includes(f.toLowerCase()));
    return {
      Choose: m.filter(t => TYPE_META[t].group === 'Choose'),
      Write: m.filter(t => TYPE_META[t].group === 'Write'),
      Decide: m.filter(t => TYPE_META[t].group === 'Decide')
    };
  }, [f]);
  const item = (t: QType) => <button key={t} role="option" aria-selected={t === value} type="button" onClick={() => onPick(t)} className={cx('flex h-[43px] w-full cursor-pointer items-center gap-3 rounded-[7px] px-1.5 text-left', t === value ? 'bg-(--surface-hover)' : 'hover:bg-(--surface-hover)')}>
      <TypeTile type={t} size={22} />
      <span className="min-w-0 flex-1">
        <span className="block text-[14px] leading-[18px]">{TYPE_META[t].label}</span>
        <span className="block text-[12px] leading-[16px] text-(--muted-foreground)">{TYPE_META[t].hint}</span>
      </span>
      {t === value && <Check size={15} className="mr-1.5" />}
    </button>;
  return <div ref={ref}>
      <Menu className="top-[44px] right-[-22px] w-[588px] p-0">
        <div className="flex h-[38px] items-center gap-2 border-b border-(--border) px-3">
          <Search size={15} className="text-(--muted-foreground)" />
          <input autoFocus value={f} onChange={e => setF(e.target.value)} placeholder="Question type..." className="flex-1 bg-transparent text-[14px] outline-none" />
        </div>
        <div className="grid grid-cols-2 gap-x-2 p-1">
          <div>
            <p className="px-2 pt-2 pb-1 text-[12px] text-(--muted-foreground)">Choose</p>
            {groups.Choose.map(item)}
          </div>
          <div>
            {groups.Write.length > 0 && <p className="px-2 pt-2 pb-1 text-[12px] text-(--muted-foreground)">Write</p>}
            {groups.Write.map(item)}
            {groups.Decide.length > 0 && <p className="px-2 pt-2.5 pb-1 text-[12px] text-(--muted-foreground)">Decide</p>}
            {groups.Decide.map(item)}
          </div>
        </div>
        {onReference && <button type="button" onClick={onReference} className="flex h-9 w-full cursor-pointer items-center gap-2 border-t border-(--border) px-3 text-left text-[12px] text-(--muted-foreground) hover:text-(--foreground)">
            See every answer type as the answerer sees it <ArrowRight size={13} />
          </button>}
      </Menu>
    </div>;
}
function AskedToMenu({
  to,
  setTo,
  policy,
  setPolicy,
  onClose
}: {
  to: PersonId[];
  setTo: (p: PersonId[]) => void;
  policy: 'any' | 'each';
  setPolicy: (p: 'any' | 'each') => void;
  onClose: () => void;
}) {
  const ref = useDismiss(true, onClose);
  const [f, setF] = useState('');
  const toggle = (p: PersonId) => setTo(to.includes(p) ? to.filter(x => x !== p) : [...to, p]);
  return <div ref={ref}>
      <Menu className="bottom-[39px] left-0 w-[340px] p-0">
        <div className="flex h-[38px] items-center gap-2 border-b border-(--border) px-3">
          <Search size={14} className="text-(--muted-foreground)" />
          <input autoFocus value={f} onChange={e => setF(e.target.value)} placeholder="Search people or squads..." className="flex-1 bg-transparent text-[13px] outline-none" />
        </div>
        <div className="p-1">
          <p className="px-2 pt-[5px] pb-1 text-[12px] font-medium text-(--muted-foreground)">People</p>
          {ASKABLE.filter(p => PEOPLE[p].name.toLowerCase().includes(f.toLowerCase())).map(p => <button key={p} type="button" role="option" aria-selected={to.includes(p)} onClick={() => toggle(p)} className={cx('flex h-8 w-full cursor-pointer items-center gap-2 rounded-[6px] px-1.5 text-left', to.includes(p) ? 'bg-(--surface-hover)' : 'hover:bg-(--surface-hover)')}>
              <Avatar id={p} size={24} />
              <span className="flex-1 text-[13px]">{PEOPLE[p].name}</span>
              {to.includes(p) && <Check size={14} className="mr-1.5" />}
            </button>)}
          {'billing squad'.includes(f.toLowerCase()) && <>
              <p className="px-2 pt-2 pb-1 text-[12px] font-medium text-(--muted-foreground)">Squads</p>
              <button type="button" onClick={() => setTo([...to, ...(['mark', 'eva'] as PersonId[]).filter(p => !to.includes(p))])} className="flex w-full cursor-pointer items-center gap-2 rounded-[6px] px-1.5 py-1 text-left hover:bg-(--surface-hover)">
                <span className="flex size-6 items-center justify-center rounded-full bg-(--muted) text-(--muted-foreground)">
                  <Users size={12} />
                </span>
                <span className="flex-1">
                  <span className="block text-[13px]">Billing squad</span>
                  <span className="block text-[12px] text-(--muted-foreground)">Adds its people: Mark, Eva</span>
                </span>
              </button>
            </>}
        </div>
        <div className="flex gap-1 border-t border-(--border) p-1.5">
          {(['any', 'each'] as const).map(p => <button key={p} type="button" onClick={() => setPolicy(p)} className={cx('h-7 flex-1 cursor-pointer rounded-[6px] text-[12px]', policy === p ? 'bg-(--muted) font-medium' : 'text-(--muted-foreground) hover:text-(--foreground)')}>
              {p === 'any' ? 'Any one answers' : 'Each person answers'}
            </button>)}
        </div>
      </Menu>
    </div>;
}
function LinkMenu({
  picked,
  onPick,
  onClose
}: {
  picked: string[];
  onPick: (l: string) => void;
  onClose: () => void;
}) {
  const ref = useDismiss(true, onClose);
  const items: Array<[string, string]> = [['WORK-412', 'Export invoices as CSV with VAT breakdown'], ['WORK-415', 'Column layout for the CSV export'], ['Billing', 'Project'], ['Run #3', 'Pieter’s run on WORK-412']];
  return <div ref={ref}>
      <Menu className="bottom-[44px] left-0 w-[340px]">
        <p className="px-2 pt-1 pb-1 text-[12px] text-(--muted-foreground)">Link the question to</p>
        {items.map(([k, t]) => <button key={k} type="button" onClick={() => onPick(k)} className="flex h-9 w-full cursor-pointer items-center gap-2 rounded-[7px] px-2 text-left hover:bg-(--surface-hover)">
            <Layers size={14} className="text-(--muted-foreground)" />
            <span className="text-[13px] font-medium">{k}</span>
            <span className="min-w-0 flex-1 truncate text-[12px] text-(--muted-foreground)">{t}</span>
            {picked.includes(k) && <Check size={14} />}
          </button>)}
      </Menu>
    </div>;
}
function DueMenu({
  onPick,
  onClose
}: {
  onPick: (v: string) => void;
  onClose: () => void;
}) {
  const ref = useDismiss(true, onClose);
  return <div ref={ref}>
      <Menu className="bottom-[44px] left-0 w-[220px]">
        {['Today', 'Fri 16 Oct', 'Mon 19 Oct', 'Fri 23 Oct', ''].map(v => <button key={v || 'none'} type="button" onClick={() => onPick(v)} className="flex h-9 w-full cursor-pointer items-center gap-2 rounded-[7px] px-2 text-left text-[13px] hover:bg-(--surface-hover)">
            <Calendar size={14} className="text-(--muted-foreground)" /> {v || 'No date'}
          </button>)}
      </Menu>
    </div>;
}

/** What Ask posts: a card on the timeline for the people asked. Nothing starts a run. */
function AskedCard({
  ask,
  app
}: {
  ask: Ask;
  app: App;
}) {
  const set = ask.questions.length > 1;
  const first = ask.questions[0];
  const names = ask.to.map(p => PEOPLE[p].name);
  const nameList = names.length > 1 ? `${names.slice(0, -1).join(', ')} and ${names[names.length - 1]}` : names[0];
  const forMe = ask.to.includes('brian');
  return <section data-anchor={ask.id} className="mc-fresh rounded-[12px] border border-(--border) bg-(--surface) px-4 pt-4 pb-3.5" style={{
    boxShadow: 'var(--surface-shadow)'
  }}>
      <div className="flex h-6 items-center gap-2">
        <TypeGlyph type={set ? 'set' : first.type} size={15} className="text-(--muted-foreground)" />
        <span className="text-[11px] font-semibold tracking-[0.06em] text-(--muted-foreground) uppercase">{set ? 'Question set' : TYPE_META[first.type].label}</span>
        {set && <span className="text-[12px] text-(--muted-foreground)">· {ask.questions.length} questions</span>}
        <span className="flex-1" />
        {ask.blocks && <Pill tone="danger" icon={<CirclePause size={12} />}>
            Blocks work
          </Pill>}
        {ask.due && <Pill tone="neutral" icon={<Calendar size={12} />}>
            Answer by {ask.due}
          </Pill>}
      </div>
      <div className="mt-3 flex items-center gap-2">
        <Avatar id="brian" size={22} />
        <span className="text-[14px] font-medium">Brian</span>
        <span className="text-[12px] text-(--muted-foreground)">
          asked {nameList}
          {ask.to.length > 1 ? ` · ${ask.policy === 'any' ? 'any one answers' : 'each answers'}` : ''} · just now
        </span>
      </div>
      {ask.questions.map((q, i) => <div key={i} className={cx(set && 'mt-3 rounded-[10px] border border-(--border) px-3 py-2.5', !set && 'mt-3')}>
          <h3 className={cx('font-semibold', set ? 'flex items-center gap-2 text-[14px]' : 'text-[16px] leading-[22px]')}>
            {set && <TypeTile type={q.type} size={20} />}
            {q.prompt}
          </h3>
          {q.context && <p className="mt-1 text-[13px] text-(--muted-foreground)">{q.context}</p>}
          {(q.type === 'single' || q.type === 'multiple') && <div className="mt-2 flex flex-wrap gap-1.5">
              {q.options.filter(o => o.label.trim()).map(o => <span key={o.label} className="inline-flex h-7 items-center gap-1.5 rounded-[7px] border border-(--border) px-2 text-[13px]">
                    {o.label}
                    {o.suggested && <Pill tone="brand">Suggested</Pill>}
                  </span>)}
              {q.allowOther && q.type === 'single' && <span className="inline-flex h-7 items-center rounded-[7px] border border-dashed border-(--surface-border) px-2 text-[13px] text-(--muted-foreground)">Other…</span>}
            </div>}
        </div>)}
      <div className="mt-3 flex items-center gap-2 text-[12px] text-(--muted-foreground)">
        <Clock size={14} />
        <span className="flex-1">
          Waiting for {ask.to.length > 1 && ask.policy === 'any' ? `any one of ${ask.to.length}` : nameList}. {names.length > 1 && ask.policy === 'each' ? `0 of ${names.length} answered. ` : ''}No run started.
        </span>
        {forMe && <Btn variant="outline" size="sm" onClick={() => app.go('questions')}>
            In your Questions <ArrowRight size={13} />
          </Btn>}
      </div>
    </section>;
}