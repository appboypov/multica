import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { ArrowLeft, ArrowRight, Bot, ChevronDown, CircleHelp, CircleUserRound, Compass, Folder, Inbox, Layers, MessageSquare, MousePointerClick, Settings, Sparkles, Users, X, Zap } from 'lucide-react';
import './QuestionsPrototype.css';
import { Avatar, Menu, cx, useDismiss } from './ui';
import { type Draft } from './cards';
import { IssueScreen, type Ask } from './issue';
import { AnswerAll, AnswerTypesScreen, Placeholder, QuestionsPage, SettingsScreen } from './pages';
import { Q_APPROVE, Q_CONFIRM, Q_DATE, Q_RESTART, Q_TONE, Q_VAT, SET_PARTS, SET_TITLE, type Answer, type PersonId, type Question } from './model';
export type Route = 'issue' | 'questions' | 'answer' | 'settings' | 'types' | 'inbox' | 'other';
export interface State {
  route: Route;
  prevRoute: Route;
  otherLabel: string;
  answers: Record<string, Answer>;
  drafts: Record<string, Draft>;
  skipped: string[];
  /** Every wake a person gave, newest last: who, with how many answers. */
  wakes: Array<{
    agent: PersonId;
    count: number;
  }>;
  woken: boolean;
  wokenWith: number;
  run: 'idle' | 'running' | 'ended';
  composer: 'comment' | 'question';
  builderExample: boolean;
  builderMenu: 'type' | 'to' | 'link' | 'due' | null;
  asked: Ask[];
  panelOpen: boolean;
  propMenu: boolean;
  scrollTo?: string;
  issueSetting: 'default' | 'on' | 'off';
  workspaceDefault: boolean;
  eachClosed: boolean;
  remindedEva: boolean;
  qTab: 'me' | 'work' | 'mine' | 'all' | 'answered';
  hoverRow?: string;
  answerMode: 'one' | 'page';
  focusIdx: number;
  clean: boolean;
  guideOpen: boolean;
}

/** A question in the viewer's list: a single question, or the question set standing in for its parts. */
export interface Item {
  q: Question;
  set?: boolean;
  age: string;
  progress?: string;
}
const SET_ITEM: Question = {
  ...Q_CONFIRM,
  id: 'set',
  prompt: SET_TITLE,
  askedLine: 'asked during run #4 · 2 min ago',
  metaShort: 'run #4 · 2 min ago',
  age: '2 min'
};
const initial = (clean: boolean): State => ({
  route: 'issue',
  prevRoute: 'issue',
  otherLabel: '',
  answers: {},
  drafts: {
    confirm: {
      facts: ['y', 'n', undefined],
      corrections: {
        1: 'It rounds per line since 2024.'
      }
    },
    'set-months': {
      num: '12'
    },
    tone: {
      text: ''
    }
  },
  skipped: [],
  wakes: [],
  woken: false,
  wokenWith: 0,
  run: 'idle',
  composer: 'comment',
  builderExample: false,
  builderMenu: null,
  asked: [],
  panelOpen: true,
  propMenu: false,
  issueSetting: 'default',
  workspaceDefault: true,
  eachClosed: false,
  remindedEva: false,
  qTab: 'me',
  answerMode: 'one',
  focusIdx: 0,
  clean,
  guideOpen: true
});
export interface App {
  state: State;
  set: (patch: Partial<State>) => void;
  go: (r: Route) => void;
  back: () => void;
  answer: (id: string, summary: string, note?: string) => void;
  editAnswer: (id: string) => void;
  setDraft: (id: string, d: Draft) => void;
  skip: (id: string) => void;
  wake: (agent: PersonId) => void;
  unseenFor: (agent: PersonId, link: string) => number;
  recentlyWoken: (agent: PersonId) => boolean;
  setDone: () => boolean;
  setAnswered: () => number;
  openOnIssue: () => number;
  openForMe: () => Item[];
  focusQueue: () => Item[];
  isDone: (it: Item) => boolean;
  summaryOf: (it: Item) => string | null;
  openItem: (it: Item) => void;
  openBuilder: () => void;
  closeBuilder: () => void;
  postAsk: (a: Ask) => void;
  lastAnswered: () => {
    prompt: string;
    summary: string;
  } | null;
  answeredRows: () => Array<{
    q: Question;
    summary: string;
  }>;
}
const ALL_QUESTIONS: Question[] = [Q_VAT, Q_CONFIRM, Q_APPROVE, Q_DATE, Q_TONE, Q_RESTART, ...SET_PARTS.slice(1)];
function useApp(): App {
  const clean = typeof window !== 'undefined' && /clean/.test(window.location.hash + window.location.search);
  const [state, setState] = useState<State>(() => initial(clean));
  const ref = useRef(state);
  ref.current = state;
  const timer = useRef<number | undefined>(undefined);
  useEffect(() => () => clearTimeout(timer.current), []);
  return useMemo<App>(() => {
    const s = state;
    const set = (patch: Partial<State>) => setState(p => ({
      ...p,
      ...patch
    }));
    const setLive = s.run === 'ended';
    const isAnswered = (id: string) => !!s.answers[id];
    const setDone = () => SET_PARTS.every(p => isAnswered(p.id));
    const setAnswered = () => SET_PARTS.filter(p => isAnswered(p.id)).length;
    const items = (): Item[] => [{
      q: Q_VAT,
      age: Q_VAT.age
    }, setLive ? {
      q: SET_ITEM,
      set: true,
      age: '2 min',
      progress: `· ${setAnswered()} of 3`
    } : {
      q: Q_CONFIRM,
      age: Q_CONFIRM.age,
      progress: `${(s.drafts.confirm?.facts ?? []).filter(Boolean).length} of 3 checked`
    }, {
      q: Q_APPROVE,
      age: Q_APPROVE.age
    }, {
      q: Q_DATE,
      age: Q_DATE.age
    }, {
      q: Q_TONE,
      age: Q_TONE.age
    }, {
      q: Q_RESTART,
      age: Q_RESTART.age
    }];
    const isDone = (it: Item) => it.set ? setDone() : isAnswered(it.q.id);
    const summaryOf = (it: Item) => it.set ? setDone() ? '3 of 3 answered' : null : s.answers[it.q.id]?.summary ?? null;
    const unseenFor = (agent: PersonId, link: string) => ALL_QUESTIONS.filter(q => q.asker === agent && q.link === link && s.answers[q.id] && !s.answers[q.id].seen).length;
    const go = (r: Route) => setState(p => ({
      ...p,
      route: r,
      prevRoute: p.route === r ? p.prevRoute : p.route,
      builderMenu: null,
      propMenu: false
    }));
    const app: App = {
      state: s,
      set,
      go,
      back: () => go(ref.current.prevRoute === 'types' ? 'issue' : ref.current.prevRoute),
      answer: (id, summary, note) => setState(p => ({
        ...p,
        answers: {
          ...p.answers,
          [id]: {
            summary,
            note: note?.trim() || undefined,
            when: 'just now',
            seen: false
          }
        },
        skipped: p.skipped.filter(x => x !== id)
      })),
      editAnswer: id => setState(p => {
        const {
          [id]: _gone,
          ...rest
        } = p.answers;
        return {
          ...p,
          answers: rest
        };
      }),
      setDraft: (id, d) => setState(p => ({
        ...p,
        drafts: {
          ...p.drafts,
          [id]: d
        }
      })),
      skip: id => setState(p => ({
        ...p,
        skipped: p.skipped.includes(id) ? p.skipped : [...p.skipped, id]
      })),
      wake: agent => {
        const count = unseenFor(agent, agent === 'pieter' ? 'WORK-412' : 'agent-runtime');
        setState(p => {
          const answers = {
            ...p.answers
          };
          for (const q of ALL_QUESTIONS) if (q.asker === agent && answers[q.id]) answers[q.id] = {
            ...answers[q.id],
            seen: true
          };
          const first = agent === 'pieter' && !p.woken;
          return {
            ...p,
            answers,
            wakes: [...p.wakes, {
              agent,
              count
            }],
            ...(first ? {
              woken: true,
              wokenWith: count,
              run: 'running' as const
            } : {})
          };
        });
        if (agent === 'pieter' && !ref.current.woken) {
          clearTimeout(timer.current);
          timer.current = window.setTimeout(() => setState(p => ({
            ...p,
            run: 'ended'
          })), 1600);
        }
      },
      unseenFor,
      recentlyWoken: agent => s.wakes.some(w => w.agent === agent),
      setDone,
      setAnswered,
      openOnIssue: () => (isAnswered('vat') ? 0 : 1) + (setLive && !setDone() ? 1 : 0),
      openForMe: () => items().filter(it => !isDone(it)),
      focusQueue: () => {
        const [vat, mid, approve, ...rest] = items();
        return [approve, vat, mid, ...rest];
      },
      isDone,
      summaryOf,
      openItem: it => {
        if (it.q.link === 'WORK-412' && it.q.id !== Q_APPROVE.id) {
          setState(p => ({
            ...p,
            route: 'issue',
            prevRoute: p.route,
            scrollTo: it.set ? 'q-set' : 'q-vat'
          }));
        } else {
          const idx = app.focusQueue().findIndex(x => x.q.id === it.q.id);
          setState(p => ({
            ...p,
            route: 'answer',
            prevRoute: p.route,
            answerMode: 'one',
            focusIdx: Math.max(0, idx)
          }));
        }
      },
      openBuilder: () => set({
        composer: 'question',
        builderMenu: null,
        propMenu: false
      }),
      closeBuilder: () => set({
        composer: 'comment',
        builderMenu: null,
        builderExample: false
      }),
      postAsk: a => setState(p => ({
        ...p,
        asked: [...p.asked, a],
        composer: 'comment',
        builderMenu: null,
        builderExample: false,
        scrollTo: a.id
      })),
      lastAnswered: () => {
        const ids = Object.keys(s.answers);
        const id = ids[ids.length - 1];
        const q = ALL_QUESTIONS.find(x => x.id === id);
        return q ? {
          prompt: q.prompt,
          summary: s.answers[id].summary
        } : null;
      },
      answeredRows: () => ALL_QUESTIONS.filter(q => s.answers[q.id]).reverse().map(q => ({
        q,
        summary: s.answers[q.id].summary
      }))
    };
    return app;
  }, [state]);
}

/* ───────────────────────── Walkthrough ───────────────────────── */

interface Step {
  title: string;
  what: string;
  next: string;
  /** Bring the prototype to this step without undoing what the reader already did. */
  enter: (app: App) => void;
}
const STEPS: Step[] = [{
  title: 'A question for you on the issue',
  what: 'Pieter, an agent, ended run #3 with a question instead of guessing. It sits in the timeline as a card outlined in blue: who asks, what it blocks, a Suggested option. Properties shows “1 open · blocks work”.',
  next: 'Pick an option and Send, or click Accept suggestion.',
  enter: app => app.set({
    route: 'issue',
    panelOpen: true,
    composer: 'comment',
    builderMenu: null,
    propMenu: false,
    scrollTo: 'activity'
  })
}, {
  title: 'Answered: the card folds, nothing wakes',
  what: 'The answer reached Pieter the moment you sent it, and the card folds to one line with Edit answer. Pieter stays asleep: only a person wakes an agent, so Wake Pieter appears.',
  next: 'Click Wake Pieter, or open Agent questions in Properties first.',
  enter: app => {
    if (!app.state.answers.vat) app.answer('vat', 'Per invoice line');
    app.set({
      route: 'issue',
      panelOpen: true,
      composer: 'comment',
      builderMenu: null,
      propMenu: false,
      scrollTo: 'q-vat'
    });
  }
}, {
  title: 'Agent questions on this issue',
  what: 'Whether agents may ask questions is a workspace default each issue can change: Workspace default, On or Off. A change applies from the next run.',
  next: 'Pick a value, then click Question in the comment box to ask one yourself.',
  enter: app => app.set({
    route: 'issue',
    panelOpen: true,
    composer: 'comment',
    builderMenu: null,
    propMenu: true
  })
}, {
  title: 'Ask a question',
  what: 'The Question toggle (or typing /question) turns the comment box into the question builder in place: prompt, context, options with one Suggested, Linked to, Asked to (pick Billing squad to add its people; any one or each person answers), Answer by and Blocks work. Add question asks several at the same time.',
  next: 'Click the type button (Single choice) to change the answer type.',
  enter: app => app.set({
    route: 'issue',
    panelOpen: false,
    composer: 'question',
    builderExample: true,
    builderMenu: null,
    propMenu: false,
    scrollTo: 'builder'
  })
}, {
  title: 'Pick the answer type',
  what: 'Twelve types in three groups: Choose, Write and Decide, each with a one-line description, and a filter on top. Ask posts the card and notifies the answerers; it starts no run.',
  next: 'Click “See every answer type” at the bottom of the menu.',
  enter: app => app.set({
    route: 'issue',
    panelOpen: false,
    composer: 'question',
    builderExample: true,
    builderMenu: 'type',
    propMenu: false,
    scrollTo: 'builder'
  })
}, {
  title: 'Every answer type',
  what: 'Each type in its open state as the answerer sees it on the issue, in the inbox and in focus mode. They all keep Note, Ask back and Reassign. Try the controls: they answer and fold.',
  next: 'Click Next to see what happens after Pieter is woken.',
  enter: app => app.go('types')
}, {
  title: 'Woken: a question set, and an each-person question',
  what: 'Waking added “Pieter was woken by Brian with 1 answer”; run #4 ended with a question set of three, answered part by part, each sent on its own. Above it, Sarah’s question needs each person’s answer: 2 of 3 are in.',
  next: 'Answer Months back, then Next question, or Remind Eva. Then click Questions in the sidebar.',
  enter: app => {
    if (!app.state.answers.vat) app.answer('vat', 'Per invoice line');
    if (!app.state.woken) app.set({
      woken: true,
      wokenWith: 1,
      run: 'ended',
      wakes: [{
        agent: 'pieter',
        count: 1
      }]
    });
    if (!app.state.answers.confirm) app.answer('confirm', '2 correct, 1 corrected: “It rounds per line since 2024.”');
    app.set({
      route: 'issue',
      panelOpen: false,
      composer: 'comment',
      builderMenu: null,
      propMenu: false,
      scrollTo: 'q-vat'
    });
  }
}, {
  title: 'Every open question in one place',
  what: 'Questions lists what waits on you, grouped by link, with tabs for On my work, Asked by me, All open and Answered. Hover a Yes / No row to answer inline; ↑↓, ↵, Y, N and A work from the keyboard.',
  next: 'Click Answer all.',
  enter: app => app.set({
    route: 'questions',
    qTab: 'me',
    builderMenu: null,
    propMenu: false
  })
}, {
  title: 'Answer all, one by one',
  what: 'Focus mode shows one question at a time with its issue beside it and the queue in Up next. 1–3 choose, N adds a note, S skips, J/K move, ⌘↵ sends and moves on. The Wake bar offers to wake an agent once it has answers.',
  next: 'Send this one, or switch to One page in the header.',
  enter: app => app.set({
    route: 'answer',
    answerMode: 'one',
    builderMenu: null,
    propMenu: false
  })
}, {
  title: 'Answer all on one page',
  what: 'Every open question on one scrolling page, grouped by issue, each answerable where it stands. A sent answer folds to one line; each issue offers Wake once it has answers; the rail jumps to a question.',
  next: 'Click Leave (Esc), then open Settings from the workspace menu.',
  enter: app => app.set({
    route: 'answer',
    answerMode: 'page',
    builderMenu: null,
    propMenu: false
  })
}, {
  title: 'The workspace setting',
  what: 'Settings → Issues & workflow → Questions holds the default for every issue and shows exactly what agents get while questions are on. Switch it off: agents get none of it.',
  next: 'Toggle On by default. Back to the start with the first step.',
  enter: app => app.go('settings')
}];
function currentStep(s: State): number {
  if (s.route === 'types') return 5;
  if (s.route === 'settings') return 10;
  if (s.route === 'answer') return s.answerMode === 'one' ? 8 : 9;
  if (s.route === 'questions') return 7;
  if (s.builderMenu === 'type') return 4;
  if (s.composer === 'question') return 3;
  if (s.propMenu) return 2;
  if (s.woken) return 6;
  if (s.answers.vat) return 1;
  return 0;
}
function Guide({
  app
}: {
  app: App;
}) {
  const s = app.state;
  if (s.clean) return null;
  const i = currentStep(s);
  const step = STEPS[i];
  if (!s.guideOpen) return <button type="button" data-guide="" onClick={() => app.set({
    guideOpen: true
  })} className="fixed right-4 bottom-12 z-50 flex h-8 cursor-pointer items-center gap-1.5 rounded-full border border-(--border) bg-(--surface) px-3 text-[12px] font-medium" style={{
    boxShadow: 'var(--menu-shadow)'
  }}>
        <Compass size={14} className="text-(--brand)" /> Guide · step {i + 1} of {STEPS.length}
      </button>;
  return <aside aria-label="Walkthrough" data-guide="" className="mc-pop fixed right-4 bottom-12 z-50 w-[344px] rounded-[12px] border border-(--border) bg-(--surface) p-3.5" style={{
    boxShadow: 'var(--floating-shadow)'
  }}>
      <div className="flex items-center gap-2">
        <Compass size={14} className="text-(--brand)" />
        <span className="flex-1 text-[11px] font-medium text-(--muted-foreground)">
          Walkthrough · step {i + 1} of {STEPS.length}
        </span>
        <button type="button" aria-label="Hide the walkthrough" onClick={() => app.set({
        guideOpen: false
      })} className="flex size-6 cursor-pointer items-center justify-center rounded-[6px] text-(--muted-foreground) hover:bg-(--surface-hover)">
          <X size={14} />
        </button>
      </div>
      <p className="mt-2 text-[14px] leading-[19px] font-semibold">{step.title}</p>
      <p className="mt-1.5 text-[12px] leading-[17px] text-(--muted-foreground)">{step.what}</p>
      <p className="mt-2.5 flex gap-2 rounded-[8px] bg-[#ebf4fd] px-2.5 py-2 text-[12px] leading-[17px] text-[#0b5cb5]">
        <MousePointerClick size={14} className="mt-0.5 shrink-0" />
        <span>{step.next}</span>
      </p>
      <div className="mt-3 flex items-center gap-1.5">
        <div className="flex flex-1 gap-[3px]">
          {STEPS.map((_, j) => <button key={j} type="button" aria-label={`Step ${j + 1}`} onClick={() => STEPS[j].enter(app)} className="h-1.5 flex-1 cursor-pointer rounded-full" style={{
          background: j === i ? 'var(--brand)' : j < i ? '#9ec5f0' : 'var(--input)'
        }} />)}
        </div>
        <button type="button" disabled={i === 0} onClick={() => STEPS[i - 1].enter(app)} className="ml-2 flex h-7 cursor-pointer items-center gap-1 rounded-[6px] border border-(--surface-border) px-2 text-[12px] font-medium hover:bg-(--surface-hover) disabled:cursor-default disabled:opacity-40">
          <ArrowLeft size={13} /> Back
        </button>
        <button type="button" onClick={() => STEPS[(i + 1) % STEPS.length].enter(app)} className="flex h-7 cursor-pointer items-center gap-1 rounded-[6px] bg-(--primary) px-2 text-[12px] font-medium text-white hover:bg-[#27272a]">
          {i === STEPS.length - 1 ? 'Start over' : 'Next'} <ArrowRight size={13} />
        </button>
      </div>
    </aside>;
}

/* ───────────────────────── Shell ───────────────────────── */

function Sidebar({
  app
}: {
  app: App;
}) {
  const s = app.state;
  const [ws, setWs] = useState(false);
  const wsRef = useDismiss(ws, () => setWs(false));
  const open = app.openForMe().length;
  const active = s.route === 'questions' || s.route === 'answer' ? 'Questions' : s.route === 'issue' ? 'Issues' : s.route === 'inbox' ? 'Inbox' : s.route === 'other' ? s.otherLabel : '';
  const item = (label: string, Icon: typeof Inbox, count?: number, onClick?: () => void) => <button key={label} type="button" onClick={onClick ?? (() => app.set({
    route: 'other',
    otherLabel: label,
    prevRoute: s.route
  }))} className={cx('flex h-7 w-full shrink-0 cursor-pointer items-center gap-2 rounded-[6px] px-2 text-left text-[14px]', active === label ? 'bg-(--sidebar-accent) font-medium text-(--foreground)' : 'text-(--muted-foreground) hover:bg-(--sidebar-accent) hover:text-(--foreground)')}>
      <Icon size={16} strokeWidth={1.75} />
      <span className="flex-1">{label}</span>
      {count !== undefined && count > 0 && <span className="pr-0.5 text-[12px] text-(--muted-foreground)">{count}</span>}
    </button>;
  const label = (t: string) => <p className="mt-[14px] mb-1 px-2 text-[12px] leading-4 font-medium text-(--muted-foreground)">{t}</p>;
  return <nav className="flex w-[240px] shrink-0 flex-col px-2 pb-[10px]">
      <div className="relative mt-2" ref={wsRef}>
        <button type="button" onClick={() => setWs(!ws)} className="flex h-9 w-full cursor-pointer items-center gap-[9px] rounded-[6px] px-2 hover:bg-(--sidebar-accent)">
          <span className="size-[18px] rounded-[5px] bg-(--brand)" />
          <span className="text-[14px] font-semibold">De App Specialist</span>
          <ChevronDown size={15} className="text-(--muted-foreground)" />
        </button>
        {ws && <Menu className="top-[40px] left-0 w-[220px]">
            <button type="button" onClick={() => (setWs(false), app.go('settings'))} className="flex h-9 w-full cursor-pointer items-center gap-2 rounded-[7px] px-2 text-left text-[13px] hover:bg-(--surface-hover)">
              <Settings size={15} /> Settings
            </button>
          </Menu>}
      </div>
      <div className="mt-[6px] flex flex-col gap-0.5">
        {item('Inbox', Inbox, 3, () => app.set({
        route: 'other',
        otherLabel: 'Inbox',
        prevRoute: s.route
      }))}
        {item('Questions', CircleHelp, open, () => app.go('questions'))}
        {item('My issues', CircleUserRound)}
        {item('Chat', MessageSquare)}
        {label('Work')}
        {item('Issues', Layers, undefined, () => app.go('issue'))}
        {item('Projects', Folder)}
        {item('Autopilots', Zap)}
        {label('AI team')}
        {item('Agents', Bot)}
        {item('Squads', Users)}
        {item('Skills', Sparkles)}
      </div>
      <span className="flex-1" />
      <button type="button" className="flex h-9 cursor-pointer items-center gap-2 rounded-[6px] px-2 hover:bg-(--sidebar-accent)">
        <Avatar id="brian" size={24} />
        <span className="text-[14px] font-medium">Brian</span>
      </button>
    </nav>;
}
export const QuestionsPrototype = () => {
  const app = useApp();
  const s = app.state;
  const setRef = useRef(app.set);
  setRef.current = app.set;
  const toggleClean = useCallback((e: KeyboardEvent) => {
    if (e.key === '`' && !(e.target as HTMLElement).closest('input,textarea')) setRef.current({
      clean: !app.state.clean
    });
  }, [app.state.clean]);
  useEffect(() => {
    window.addEventListener('keydown', toggleClean);
    return () => window.removeEventListener('keydown', toggleClean);
  }, [toggleClean]);
  const shell = s.route !== 'types';
  return <div className="mc-root flex h-screen min-h-[900px] w-full min-w-[1440px]" data-route={s.route}>
      {shell ? <>
          <Sidebar app={app} />
          <main className="my-2 mr-[7px] min-w-0 flex-1 overflow-hidden rounded-[12px] border border-(--border) bg-(--page-canvas)">
            {s.route === 'issue' && <IssueScreen app={app} />}
            {s.route === 'questions' && <QuestionsPage app={app} />}
            {s.route === 'answer' && <AnswerAll app={app} />}
            {s.route === 'settings' && <SettingsScreen app={app} />}
            {(s.route === 'other' || s.route === 'inbox') && <Placeholder title={s.otherLabel}>
                {s.otherLabel} is not part of this prototype. Open <b className="mx-1 font-medium text-(--foreground)">Issues</b> or
                <b className="mx-1 font-medium text-(--foreground)">Questions</b> in the sidebar.
              </Placeholder>}
          </main>
        </> : <main className="min-w-0 flex-1">
          <AnswerTypesScreen app={app} />
        </main>}
      <Guide app={app} />
    </div>;
};