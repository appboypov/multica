import { Fragment, useCallback, useEffect, useState, type ReactNode } from 'react';

// UJ-1 "Answer an agent's question and wake it" — J3 data-flow prototype.
// Content contract: Guus's data-flow script. Bare names (Pieter, Brian,
// WORK-412, run #3) stand for their ids, as the script writes them.

const INK = '#111111';
const SUB = '#555555';
const LINE = '#E5E5E5';
const ACCENT = '#1F4FD8';
const CHIP = '#F2F2F2';
const SANS = "'Inter', ui-sans-serif, system-ui, sans-serif";
const MONO = "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace";
const FONTS_HREF = 'https://fonts.googleapis.com/css2?family=Inter:wght@400;600;700&family=JetBrains+Mono:wght@400;600&display=swap';
type Kind = 'CLI' | 'HTTP' | 'Bus' | 'WS' | 'UI' | 'Daemon' | 'Server';
type Item = {
  kind?: Kind;
  text?: string;
  code?: string;
};
type StoreEntry = {
  table: string;
  op: 'write' | 'read' | 'none';
  rows?: string[];
  note?: string;
};
type Step = {
  slug: string;
  title: string;
  actor: string;
  time: string;
  what: string;
  input: Item[];
  action: string;
  storeNote: string;
  store: StoreEntry[];
  output: Item[];
  refusals?: Item[];
  answers?: string;
  next: string;
};
const QUESTION_ID = '0199f3b0-7c1e-…';
const NO_QUEUE_ROW: StoreEntry = {
  table: 'agent_task_queue',
  op: 'none'
};
const STEPS: Step[] = [{
  slug: 'ask',
  title: 'Pieter asks during run #3',
  actor: 'Pieter (agent)',
  time: '2026-10-12 20:41',
  what: 'Pieter needs a decision only Brian can make, asks it with `multica question ask`, and ends his run without waiting.',
  input: [{
    kind: 'CLI',
    code: ['multica question ask', '  --type single_choice', '  --prompt "Do invoices round per line or per total?"', '  --context "The old export rounded per line; the spec is silent."', '  --option "Per line"', '  --option "Per total"', '  --suggest "Per line"', '  --answerer Brian'].join('\n')
  }, {
    kind: 'HTTP',
    text: '→ `POST /api/questions` with the task token of run #3 (`X-Agent-ID`, `X-Task-ID` from `middleware.Auth`).'
  }],
  action: 'Pieter asks, prints the question id, then ends run #3.',
  storeNote: 'Writes, one transaction',
  store: [{
    table: 'question',
    op: 'write',
    rows: [`{id: "${QUESTION_ID}", status: "open", asker_agent_id: Pieter, asked_in_task_id: run #3, answer_type: "single_choice", prompt: "Do invoices round per line or per total?", context_line: "The old export rounded per line; the spec is silent.", settings: {options: ["Per line", "Per total"], allow_other: false}, suggested_answer: {option: "Per line"}, due_date: null, blocks_work: false}`]
  }, {
    table: 'question_link ×2',
    op: 'write',
    rows: ['{link_type: "issue", link_id: WORK-412, position: 0}', '{link_type: "task", link_id: run #3, position: 1}']
  }, {
    table: 'question_answerer',
    op: 'write',
    rows: ['{user_id: Brian}']
  }, {
    table: 'question_event',
    op: 'write',
    rows: ['{kind: "asked", actor_type: "agent", details: {}}']
  }, NO_QUEUE_ROW],
  output: [{
    kind: 'HTTP',
    text: '`201` Question (`unseen_answer_count: 0`, `wake_available: false`)'
  }, {
    kind: 'CLI',
    text: 'Prints the question id.'
  }, {
    kind: 'Bus',
    text: '`question:created` `{question_id, issue_id}` (reason empty)'
  }],
  next: 'click Next to see how the server tells Brian.'
}, {
  slug: 'told',
  title: 'Brian is told',
  actor: 'Server',
  time: '2026-10-12 20:41',
  what: 'The server tells every answerer in the inbox; the issue page updates live. No task is queued.',
  input: [{
    kind: 'Bus',
    text: '`question:created` `{question_id, issue_id}`'
  }],
  action: 'None — run #3 ended; Pieter has no queued, dispatched or running task on WORK-412.',
  storeNote: 'Writes',
  store: [{
    table: 'inbox_item',
    op: 'write',
    rows: ['{recipient_type: "member", recipient_id: Brian, type: "question_asked", actor_type: "agent", actor_id: Pieter, issue_id: WORK-412, details: {question_id}, read: false}'],
    note: 'Not in `notifTypeToGroup`: cannot be muted.'
  }, NO_QUEUE_ROW],
  output: [{
    kind: 'WS',
    text: '`inbox:new` to Brian'
  }, {
    kind: 'WS',
    text: '`question:created` to the workspace'
  }],
  next: 'click Next — Brian opens WORK-412 from the inbox item.'
}, {
  slug: 'open-issue',
  title: 'Brian opens WORK-412 from the inbox',
  actor: 'Brian (member)',
  time: '2026-10-12 21:03',
  what: "The inbox item opens the issue at the question's card; the run row says Pieter ended run #3 with a question.",
  input: [{
    kind: 'HTTP',
    text: '`GET /api/issues/{WORK-412}/questions` (beside the timeline and task runs)'
  }, {
    kind: 'UI',
    text: '`highlightQuestionId` from the inbox item'
  }],
  action: 'None.',
  storeNote: 'Reads',
  store: [{
    table: 'question',
    op: 'read',
    rows: [`{id: "${QUESTION_ID}", status: "open", …}`]
  }, {
    table: 'question_link',
    op: 'read',
    rows: ['{link_type: "issue", link_id: WORK-412, position: 0}', '{link_type: "task", link_id: run #3, position: 1}']
  }, {
    table: 'question_answerer',
    op: 'read',
    rows: ['{user_id: Brian}']
  }, {
    table: 'question_answer',
    op: 'read',
    note: 'none'
  }, {
    table: 'question_event',
    op: 'read',
    rows: ['{kind: "asked", actor_type: "agent", details: {}}']
  }, {
    table: '"unseen" query',
    op: 'read',
    note: "Answers updated after Pieter's latest started run on WORK-412 = run #3's `started_at` → `0`"
  }, {
    table: 'HasActiveTaskForIssueAndAgent',
    op: 'read',
    note: '→ `false`'
  }, NO_QUEUE_ROW],
  output: [{
    kind: 'HTTP',
    text: '`Question[]` with the card data: `status: "open"`, `answers: []`, `unseen_answer_count: 0`, `wake_available: false`'
  }, {
    kind: 'UI',
    text: 'Card placed at its `created_at` in the timeline. Questions row "1 open". Run row "ended run #3 with a question" with "Waiting for answer". No Wake offered.'
  }],
  next: 'click Next — Brian presses **Accept suggestion** on the card.'
}, {
  slug: 'accept-suggestion',
  title: 'Brian accepts the suggestion',
  actor: 'Brian (member)',
  time: '2026-10-12 21:04',
  what: 'One click sends the suggested answer. Answering starts nothing: the card now offers Wake.',
  input: [{
    kind: 'HTTP',
    text: '`POST /api/questions/{id}/answer`'
  }, {
    code: '{ value: { option: "Per line" } }'
  }, {
    kind: 'Server',
    text: 'Runs `ValidateAnswer` (single_choice: option must be one of `settings.options`).'
  }],
  action: 'None — no task is enqueued (ADR-0003). If Pieter were running he could read it at once with `multica question answers <id>`.',
  storeNote: 'Writes',
  store: [{
    table: 'question_answer',
    op: 'write',
    rows: ['{answerer_user_id: Brian, value: {option: "Per line"}, display_text: "Per line", note: null}']
  }, {
    table: 'question',
    op: 'write',
    rows: ['{status: "open" → "answered"}']
  }, {
    table: 'question_event',
    op: 'write',
    rows: ['{kind: "answered", actor_type: "member"}']
  }, NO_QUEUE_ROW],
  output: [{
    kind: 'HTTP',
    text: '`200` Question (`status: "answered"`, `unseen_answer_count: 1`, `wake_available: true`)'
  }, {
    kind: 'Bus',
    text: '`question:updated` `{question_id, issue_id, reason: "answered"}`'
  }, {
    kind: 'WS',
    text: 'To the workspace'
  }, {
    kind: 'UI',
    text: 'Card folds to "Per line" with **Edit answer** and **Wake Pieter**. Questions row disappears. "Waiting for answer" badge goes.'
  }],
  next: 'click Next — Brian presses **Wake Pieter**.'
}, {
  slug: 'wake',
  title: 'Brian wakes Pieter',
  actor: 'Brian (member)',
  time: '2026-10-12 21:05',
  what: 'Wake is the only path from an answer to a run. It queues one run for Pieter carrying every answer he has not seen.',
  input: [{
    kind: 'HTTP',
    text: '`POST /api/questions/{id}/wake` (member only)'
  }, {
    kind: 'Server',
    text: 'Guard: unseen > 0 and no active task for (Pieter, WORK-412).'
  }],
  action: "The server queues Pieter's run #4 via `TaskService.EnqueueTaskForQuestionWake(issue, Pieter, Brian, 1)` — no trigger comment.",
  storeNote: 'Writes',
  store: [{
    table: 'agent_task_queue',
    op: 'write',
    rows: [`{status: queued, trigger_evidence_kind: "question_wake", trigger_evidence_ref_id: "${QUESTION_ID}"}`],
    note: 'Run #4 for Pieter on WORK-412. Trigger summary "Woken by Brian with 1 answer".'
  }, {
    table: 'activity_log',
    op: 'write',
    rows: ['{action: "question_wake", details: {agent_id, answer_count: 1, task_id}}']
  }, {
    table: 'question_event',
    op: 'write',
    rows: ['{kind: "woken", details: {task_id, answer_count: 1}}']
  }],
  output: [{
    kind: 'HTTP',
    text: '`202` `{ task_id, answer_count: 1 }`'
  }, {
    kind: 'Bus',
    text: '`question:updated` reason `woken`'
  }, {
    kind: 'UI',
    text: 'Timeline line "Pieter was woken by Brian with 1 answer". `wake_available` now false (task active).'
  }],
  refusals: [{
    code: '409',
    text: 'A pending task already exists (`ErrDuplicatePendingTask`).'
  }, {
    code: '422',
    text: 'Nothing unseen.'
  }],
  next: 'click Next — the daemon claims run #4.'
}, {
  slug: 'woken-run',
  title: "Pieter's new run reads the answer",
  actor: 'Daemon, then Pieter (agent)',
  time: '2026-10-12 21:05',
  what: 'The daemon claims run #4; the server hydrates the answers at claim, so an edit made between Wake and claim still arrives; Pieter goes on with "Per line".',
  input: [{
    kind: 'Daemon',
    text: 'Claim of run #4 (`internal/handler/daemon.go`).'
  }],
  action: 'Pieter reads the `[ANSWERS]` block and continues the work with "Per line"; `multica question answers <id>` shows the question in full.',
  storeNote: 'Reads and writes',
  store: [{
    table: 'question_answer',
    op: 'read',
    rows: ['{answerer_user_id: Brian, value: {option: "Per line"}, display_text: "Per line", note: null}'],
    note: "Unseen answers and ask-backs, anchored on Pieter's latest started run on WORK-412 other than this task (run #3)."
  }, {
    table: 'agent_task_queue',
    op: 'write',
    rows: ['{status: queued → dispatched → running, started_at: 21:05}'],
    note: 'Run #4. `started_at` is the new anchor, so unseen becomes 0.'
  }],
  output: [{
    kind: 'HTTP',
    text: 'Claim response with `question_answers` (question id, prompt, display text, note, who, when).'
  }, {
    kind: 'Daemon',
    text: '`perTurnContextBlocks` appends the `[ANSWERS]` block below, verbatim.'
  }],
  answers: ['[ANSWERS]', 'Brian woke you with 1 answer to your questions:', '- 0199f3b0…  "Do invoices round per line or per total?" → Per line · Brian · 2026-10-12 21:04', '`multica question answers <question-id>` shows a question in full.'].join('\n'),
  next: 'the journey ends here. Click step 1 in the rail, or Back, to walk it again.'
}];
type LiveRow = {
  label: string;
  code?: boolean;
  before: string | null;
  values: string[];
  acceptance?: boolean;
};

// `before` is the value before step 1; null means the question did not exist yet.
const LIVE: LiveRow[] = [{
  label: 'question.status',
  code: true,
  before: null,
  values: ['open', 'open', 'open', 'answered', 'answered', 'answered']
}, {
  label: 'answers',
  before: null,
  values: ['0', '0', '0', '1', '1', '1']
}, {
  label: 'unseen_answer_count',
  code: true,
  before: null,
  values: ['0', '0', '0', '1', '1', '0']
}, {
  label: 'wake_available',
  code: true,
  before: null,
  values: ['false', 'false', 'false', 'true', 'false', 'false']
}, {
  label: "Pieter's task on WORK-412",
  before: null,
  values: ['run #3 running → ended', 'none active', 'none active', 'none active', 'run #4 queued', 'run #4 running']
}, {
  label: "Brian's inbox item",
  before: '–',
  values: ['–', 'unread', 'read', 'read', 'read', 'read']
}, {
  label: 'Runs started since the ask',
  before: '0',
  values: ['0', '0', '0', '0', '0', '1'],
  acceptance: true
}];

// Break opportunities inside identifiers: after . / ( , and at camelCase humps,
// so narrow lanes wrap at word-like seams instead of mid-word.
function breakable(text: string): ReactNode[] {
  const pieces = text.split(/(?<=[./(,])|(?<=[a-z])(?=[A-Z])/g);
  return pieces.map((piece, i) => <Fragment key={i}>
      {i > 0 ? <wbr /> : null}
      {piece}
    </Fragment>);
}

// Inline markup: `code` → chip, **bold** → strong.
function rich(text: string): ReactNode[] {
  const parts = text.split(/(`[^`]+`|\*\*[^*]+\*\*)/g).filter(Boolean);
  return parts.map((part, i) => {
    if (part.startsWith('`') && part.endsWith('`')) {
      return <code key={i} className="rounded-[3px] px-[4px] py-[1px] text-[13px] [box-decoration-break:clone] [overflow-wrap:anywhere]" style={{
        fontFamily: MONO,
        background: CHIP,
        color: INK
      }}>
          {breakable(part.slice(1, -1))}
        </code>;
    }
    if (part.startsWith('**') && part.endsWith('**')) {
      return <strong key={i} className="font-semibold">
          {part.slice(2, -2)}
        </strong>;
    }
    return <Fragment key={i}>{part}</Fragment>;
  });
}
const labelClass = 'text-[12px] font-semibold uppercase leading-[16px] tracking-[0.06em]';
function Label({
  children,
  color = SUB
}: {
  children: ReactNode;
  color?: string;
}) {
  return <div className={labelClass} style={{
    color
  }}>
      {children}
    </div>;
}
function Lane({
  title,
  width,
  sub,
  children
}: {
  title: string;
  width: number;
  sub?: string;
  children: ReactNode;
}) {
  return <section className="min-w-0 shrink-0" style={{
    width
  }} aria-label={title}>
      <div className="h-[52px] border-t pt-[10px]" style={{
      borderColor: INK
    }}>
        <Label color={INK}>{title}</Label>
        {sub ? <div className="mt-[2px] text-[13px] leading-[18px]" style={{
        color: SUB
      }}>
            {sub}
          </div> : null}
      </div>
      {children}
    </section>;
}
function Arrow() {
  return <div className="flex w-[24px] shrink-0 justify-center pt-[14px]" aria-hidden="true">
      <svg width="14" height="8" viewBox="0 0 14 8" fill="none">
        <path d="M0 4H12.5M9.5 1L12.5 4L9.5 7" stroke={INK} strokeWidth="1" />
      </svg>
    </div>;
}
function ItemView({
  item
}: {
  item: Item;
}) {
  return <div className="mb-[14px] last:mb-0">
      {item.kind ? <div className="mb-[4px]">
          <Label>{item.kind}</Label>
        </div> : null}
      {item.code ? <div className="rounded-[3px] px-[8px] py-[6px] text-[13px] leading-[19px] [overflow-wrap:anywhere]" style={{
      fontFamily: MONO,
      background: CHIP,
      color: INK
    }}>
          {item.code.split('\n').map((line, i) => {
        const indent = line.length - line.trimStart().length;
        // Hanging indent: wrapped continuation lines sit 2ch deeper than their own start.
        return <div key={i} style={{
          paddingLeft: `${indent + 2}ch`,
          textIndent: '-2ch'
        }}>
                {line.trimStart()}
              </div>;
      })}
        </div> : null}
      {item.text ? <p className={`m-0 text-[15px] leading-[22px] ${item.code ? 'mt-[6px]' : ''}`} style={{
      color: INK
    }}>
          {rich(item.text)}
        </p> : null}
    </div>;
}
function StoreEntryView({
  entry
}: {
  entry: StoreEntry;
}) {
  if (entry.op === 'none') {
    return <div className="border-t py-[10px] text-[13px] leading-[19px]" style={{
      borderColor: LINE,
      color: INK
    }}>
        <span style={{
        fontFamily: MONO
      }} className="font-semibold">
          {entry.table}
        </span>
        <span style={{
        fontFamily: MONO
      }}>: no new row</span>
      </div>;
  }
  const written = entry.op === 'write';
  return <div className="border-t py-[10px]" style={{
    borderColor: LINE
  }}>
      <div className="flex items-baseline justify-between gap-[8px]">
        <span className="min-w-0 text-[13px] font-semibold leading-[19px] [overflow-wrap:anywhere]" style={{
        fontFamily: MONO,
        color: written ? ACCENT : INK
      }}>
          {breakable(entry.table)}
        </span>
        <span className={`${labelClass} shrink-0 rounded-[3px] px-[5px] py-[1px]`} style={{
        background: CHIP,
        color: written ? ACCENT : SUB
      }}>
          {written ? 'write' : 'read'}
        </span>
      </div>
      {entry.rows?.map((row, i) => <div key={i} className="mt-[6px] border-l-2 pl-[8px] text-[13px] leading-[19px] [overflow-wrap:anywhere]" style={{
      fontFamily: MONO,
      color: INK,
      borderColor: written ? ACCENT : LINE
    }}>
          {row}
        </div>)}
      {entry.note ? <p className="m-0 mt-[6px] text-[13px] leading-[19px]" style={{
      color: SUB
    }}>
          {rich(entry.note)}
        </p> : null}
    </div>;
}
function LiveStatePanel({
  index
}: {
  index: number;
}) {
  const regular = LIVE.filter(r => !r.acceptance);
  const acceptance = LIVE.find(r => r.acceptance);
  const cell = (row: LiveRow) => {
    const value = row.values[index];
    const prev = index === 0 ? row.before : row.values[index - 1];
    const changed = prev !== value;
    return {
      value,
      prev,
      changed
    };
  };
  return <aside className="w-[320px] shrink-0 border-l px-[28px] pb-[40px] pt-[40px]" style={{
    borderColor: LINE
  }} aria-label="Live state">
      <Label color={INK}>Live state</Label>
      <div className="mt-[4px] text-[13px] leading-[18px]" style={{
      color: SUB
    }}>
        After step {index + 1}. Changed values in blue.
      </div>
      <dl className="m-0 mt-[20px]" aria-live="polite">
        {regular.map(row => {
        const {
          value,
          prev,
          changed
        } = cell(row);
        return <div key={row.label} className="border-t py-[12px]" style={{
          borderColor: LINE
        }}>
              <dt className="text-[13px] leading-[18px]" style={{
            color: SUB,
            fontFamily: row.code ? MONO : SANS
          }}>
                {row.label}
              </dt>
              <dd className="m-0 mt-[4px] flex flex-wrap items-baseline gap-x-[8px]">
                <span className="text-[15px] font-semibold leading-[22px]" style={{
              color: changed ? ACCENT : INK
            }}>
                  {value}
                </span>
                {changed && prev !== null ? <s className="text-[12px] leading-[16px]" style={{
              color: SUB
            }}>
                    {prev}
                  </s> : null}
              </dd>
            </div>;
      })}
      </dl>
      {acceptance ? (() => {
      const {
        value,
        prev,
        changed
      } = cell(acceptance);
      return <div className="mt-[8px] border-t pt-[16px]" style={{
        borderColor: INK
      }}>
                <Label color={INK}>Acceptance check</Label>
                <div className="mt-[6px] text-[15px] leading-[22px]" style={{
          color: INK
        }}>
                  {acceptance.label}
                </div>
                <div className="mt-[4px] flex items-baseline gap-x-[10px]">
                  <span className="text-[32px] font-bold leading-[38px] tracking-[-0.01em]" style={{
            color: changed ? ACCENT : INK
          }}>
                    {value}
                  </span>
                  {changed && prev !== null ? <s className="text-[12px] leading-[16px]" style={{
            color: SUB
          }}>
                      {prev}
                    </s> : null}
                </div>
                <p className="m-0 mt-[6px] text-[13px] leading-[19px]" style={{
          color: SUB
        }}>
                  Stays 0 until Brian presses Wake.
                </p>
              </div>;
    })() : null}
    </aside>;
}
function StepRail({
  index,
  onSelect
}: {
  index: number;
  onSelect: (i: number) => void;
}) {
  return <nav className="flex w-[240px] shrink-0 flex-col border-r px-[24px] pb-[40px] pt-[40px]" style={{
    borderColor: LINE
  }} aria-label="Journey steps">
      <Label>UJ-1 · J3 data flow</Label>
      <div className="mt-[6px] text-[15px] font-semibold leading-[21px]" style={{
      color: INK
    }}>
        Answer an agent's question and wake it
      </div>
      <ol className="m-0 mt-[28px] list-none p-0">
        {STEPS.map((step, i) => {
        const current = i === index;
        return <li key={step.slug}>
              <button type="button" onClick={() => onSelect(i)} aria-current={current ? 'step' : undefined} className="-ml-[24px] flex w-[calc(100%+24px)] cursor-pointer items-baseline gap-[10px] border-l-2 bg-transparent py-[9px] pl-[22px] pr-0 text-left outline-none focus-visible:outline-2 focus-visible:outline-offset-[-2px]" style={{
            borderColor: current ? ACCENT : 'transparent',
            outlineColor: INK
          }}>
                <span className="w-[14px] shrink-0 text-[13px] font-semibold leading-[21px]" style={{
              fontFamily: MONO,
              color: current ? ACCENT : SUB
            }}>
                  {i + 1}
                </span>
                <span className={`text-[15px] leading-[21px] ${current ? 'font-semibold' : 'font-normal'}`} style={{
              color: current ? ACCENT : INK
            }}>
                  {step.title}
                </span>
              </button>
            </li>;
      })}
      </ol>
      <div className="mt-[32px] border-t pt-[16px]" style={{
      borderColor: LINE
    }}>
        <Label>Cast</Label>
        <dl className="m-0 mt-[8px] grid grid-cols-[64px_1fr] gap-x-[8px] gap-y-[4px] text-[13px] leading-[19px]">
          <dt style={{
          color: SUB
        }}>Issue</dt>
          <dd className="m-0" style={{
          color: INK
        }}>
            WORK-412 "Invoice totals for the Q4 export"
          </dd>
          <dt style={{
          color: SUB
        }}>Agent</dt>
          <dd className="m-0" style={{
          color: INK
        }}>
            Pieter
          </dd>
          <dt style={{
          color: SUB
        }}>Member</dt>
          <dd className="m-0" style={{
          color: INK
        }}>
            Brian
          </dd>
          <dt style={{
          color: SUB
        }}>Space</dt>
          <dd className="m-0" style={{
          color: INK
        }}>
            Multica
          </dd>
        </dl>
        <p className="m-0 mt-[12px] text-[13px] leading-[19px]" style={{
        color: SUB
      }}>
          In rows, bare names (Pieter, Brian, WORK-412, run #3) stand for their ids.
        </p>
      </div>
    </nav>;
}
const LANES = {
  input: 184,
  action: 148,
  store: 240,
  output: 172
};
export const UJ1DataFlowPrototype = () => {
  const [index, setIndex] = useState(0);
  const step = STEPS[index];
  const last = STEPS.length - 1;
  const go = useCallback((i: number) => setIndex(Math.max(0, Math.min(last, i))), [last]);
  useEffect(() => {
    if (document.querySelector('link[data-uj1-fonts]')) return;
    const link = document.createElement('link');
    link.rel = 'stylesheet';
    link.href = FONTS_HREF;
    link.setAttribute('data-uj1-fonts', '');
    document.head.appendChild(link);
  }, []);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.altKey || e.ctrlKey || e.metaKey) return;
      if (e.key === 'ArrowRight') {
        e.preventDefault();
        setIndex(i => Math.min(last, i + 1));
      } else if (e.key === 'ArrowLeft') {
        e.preventDefault();
        setIndex(i => Math.max(0, i - 1));
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [last]);
  const buttonBase = 'h-[40px] cursor-pointer rounded-[6px] px-[18px] text-[14px] font-semibold leading-[20px] outline-none focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-default';
  return <div className="flex min-h-screen w-full min-w-[1440px] flex-1 items-stretch antialiased" style={{
    background: '#FFFFFF',
    color: INK,
    fontFamily: SANS
  }}>
      <StepRail index={index} onSelect={go} />

      <main className="w-[880px] shrink-0 px-[32px] pb-[40px] pt-[40px]">
        <header>
          <div className="flex items-center gap-[16px]">
            <Label>
              Step {index + 1} of {STEPS.length}
            </Label>
            <Label>{step.time}</Label>
            <Label>Actor · {step.actor}</Label>
          </div>
          <h1 className="m-0 mt-[10px] text-[32px] font-bold leading-[38px] tracking-[-0.01em]" style={{
          color: INK
        }}>
            {step.title}
          </h1>
        </header>

        <section className="mt-[20px] max-w-[720px]">
          <Label>What happens</Label>
          <p className="m-0 mt-[6px] text-[15px] leading-[23px]" style={{
          color: INK
        }}>
            {rich(step.what)}
          </p>
        </section>

        <div className="mt-[32px] flex items-start">
          <Lane title="Input" width={LANES.input}>
            {step.input.map((item, i) => <ItemView key={i} item={item} />)}
          </Lane>
          <Arrow />
          <Lane title="Agent action" width={LANES.action}>
            <p className="m-0 text-[15px] leading-[22px]" style={{
            color: INK
          }}>
              {rich(step.action)}
            </p>
          </Lane>
          <Arrow />
          <Lane title="Store" width={LANES.store} sub={step.storeNote}>
            {step.store.map((entry, i) => <StoreEntryView key={i} entry={entry} />)}
          </Lane>
          <Arrow />
          <Lane title="Output" width={LANES.output}>
            {step.output.map((item, i) => <ItemView key={i} item={item} />)}
            {step.refusals ? <div className="mt-[18px] border-t pt-[10px]" style={{
            borderColor: LINE
          }}>
                <Label>Side note · refusals</Label>
                {step.refusals.map(r => <p key={r.code} className="m-0 mt-[6px] text-[13px] leading-[19px]" style={{
              color: SUB
            }}>
                    <code className="mr-[6px] rounded-[3px] px-[4px] py-[1px] text-[13px]" style={{
                fontFamily: MONO,
                background: CHIP,
                color: INK
              }}>
                      {r.code}
                    </code>
                    {rich(r.text ?? '')}
                  </p>)}
              </div> : null}
          </Lane>
        </div>

        {step.answers ? <section className="mt-[28px]" aria-label="ANSWERS block">
            <Label color={INK}>Output · [ANSWERS] block, verbatim</Label>
            <pre className="m-0 mt-[10px] whitespace-pre-wrap border-l-2 py-[12px] pl-[14px] pr-[12px] text-[13px] leading-[21px] [overflow-wrap:anywhere]" style={{
          fontFamily: MONO,
          color: INK,
          background: CHIP,
          borderColor: ACCENT
        }}>
              {step.answers}
            </pre>
          </section> : null}

        <footer className="mt-[36px] border-t pt-[18px]" style={{
        borderColor: LINE
      }}>
          <div className="flex items-center justify-between gap-[24px]">
            <p className="m-0 text-[15px] leading-[22px]" style={{
            color: INK
          }}>
              <span className="font-semibold">Next:</span> {rich(step.next)}
            </p>
            <div className="flex shrink-0 gap-[8px]">
              <button type="button" onClick={() => go(index - 1)} disabled={index === 0} className={`${buttonBase} border bg-white disabled:opacity-40`} style={{
              borderColor: LINE,
              color: INK,
              outlineColor: INK
            }}>
                Back
              </button>
              <button type="button" onClick={() => go(index + 1)} disabled={index === last} className={`${buttonBase} border disabled:opacity-40`} style={{
              background: INK,
              borderColor: INK,
              color: '#FFFFFF',
              outlineColor: INK
            }}>
                Next
              </button>
            </div>
          </div>
          <p className="m-0 mt-[8px] text-[13px] leading-[19px]" style={{
          color: SUB
        }}>
            Or use the ← and → keys, or click a step in the rail.
          </p>
        </footer>
      </main>

      <LiveStatePanel index={index} />
    </div>;
};
export default UJ1DataFlowPrototype;