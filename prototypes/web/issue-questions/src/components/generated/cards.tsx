import { useState, type ReactNode } from 'react';
import { ArrowUpDown, ArrowUpRight, Ban, Calendar, Check, CloudUpload, CornerUpLeft, File as FileIcon, GitPullRequestArrow, GripVertical, NotebookPen, Pencil, Search, Send, UserRoundPlus, X } from 'lucide-react';
import { Avatar, Btn, Kbd, Pill, TypeGlyph, cx } from './ui';
import { PEOPLE, TYPE_META, type PersonId, type QType, type Question } from './model';
export interface Draft {
  choice?: number;
  other?: string;
  picks?: number[];
  facts?: Array<'y' | 'n' | undefined>;
  corrections?: Record<number, string>;
  order?: string[];
  scale?: number;
  num?: string;
  date?: string;
  person?: PersonId;
  text?: string;
  files?: string[];
  approval?: 'approve' | 'changes' | 'decline';
  yes?: boolean;
  why?: string;
}

/** The one line a sent answer folds to; null while the draft is not an answer yet. */
export function summarize(q: Question, d: Draft): string | null {
  switch (q.type) {
    case 'single':
      if (d.choice === undefined) return null;
      if (d.choice === (q.options?.length ?? 0)) return d.other?.trim() ? d.other.trim() : null;
      return q.options![d.choice].label;
    case 'multiple':
      return d.picks?.length ? d.picks.map(i => q.options![i].label).join(', ') : null;
    case 'yesno':
      return d.yes === undefined ? null : d.yes ? 'Yes' : `No${d.why?.trim() ? ` — ${d.why.trim()}` : ''}`;
    case 'confirm':
      {
        const f = d.facts ?? [];
        if ((q.facts ?? []).some((_, i) => !f[i])) return null;
        const wrong = f.map((v, i) => v === 'n' ? i : -1).filter(i => i >= 0);
        const right = f.length - wrong.length;
        if (!wrong.length) return `${right} correct`;
        const c = wrong.map(i => d.corrections?.[i]?.trim()).filter(Boolean);
        return `${right} correct, ${wrong.length} corrected${c.length ? `: “${c.join('”, “')}”` : ''}`;
      }
    case 'ranking':
      return (d.order ?? q.items ?? []).slice(0, 3).join(' → ') + ((q.items?.length ?? 0) > 3 ? ' …' : '');
    case 'scale':
      return d.scale ? `${d.scale} of 5` : null;
    case 'number':
      return d.num?.trim() ? `${d.num.trim()} ${q.unit ?? ''}`.trim() : null;
    case 'date':
      return d.date?.trim() ? d.date.trim() : null;
    case 'person':
      return d.person ? PEOPLE[d.person].name : null;
    case 'text':
      return d.text?.trim() ? d.text.trim() : null;
    case 'file':
      return d.files?.length ? d.files.join(', ') : null;
    case 'approval':
      return d.approval === 'approve' ? 'Approved' : d.approval === 'changes' ? 'Changes requested' : d.approval === 'decline' ? 'Declined' : null;
  }
}

/** Seed drafts so a card opens the way the designs show it (the suggested option ready to accept, a typed value, …). */
export function seedDraft(q: Question): Draft {
  switch (q.type) {
    case 'ranking':
      return {
        order: [...(q.items ?? [])]
      };
    default:
      return {};
  }
}
function OptionRow({
  selected,
  onClick,
  children,
  right,
  index,
  shape
}: {
  selected: boolean;
  onClick: () => void;
  children: ReactNode;
  right?: ReactNode;
  index: number;
  shape: 'radio' | 'check';
}) {
  return <button type="button" role={shape === 'radio' ? 'radio' : 'checkbox'} aria-checked={selected} onClick={onClick} className={cx('flex h-[35px] w-full cursor-pointer items-center gap-2.5 rounded-[8px] border px-2.5 text-left text-[14px] transition-colors', selected ? 'border-(--foreground) bg-(--muted) shadow-[inset_0_0_0_0.5px_var(--foreground)]' : 'border-(--input) bg-(--surface) hover:bg-(--surface-hover)')}>
      {shape === 'radio' ? <span className={cx('flex size-4 shrink-0 items-center justify-center rounded-full border', selected ? 'border-(--foreground) bg-(--foreground)' : 'border-(--input) bg-(--surface)')}>
          {selected && <span className="size-[6px] rounded-full bg-white" />}
        </span> : <span className={cx('flex size-4 shrink-0 items-center justify-center rounded-[4px] border', selected ? 'border-(--foreground) bg-(--foreground) text-white' : 'border-(--input) bg-(--surface)')}>
          {selected && <Check size={12} strokeWidth={3} />}
        </span>}
      <span className="min-w-0 flex-1 truncate">{children}</span>
      {right}
      <Kbd>{index + 1}</Kbd>
    </button>;
}

/** The answer control of each type. Every control is live: picking, typing and toggling update the draft. */
export function AnswerBody({
  q,
  d,
  set,
  dense
}: {
  q: Question;
  d: Draft;
  set: (d: Draft) => void;
  dense?: boolean;
}) {
  const [query, setQuery] = useState('');
  const [focusRow, setFocusRow] = useState(0);
  switch (q.type) {
    case 'single':
      {
        const opts = q.options ?? [];
        return <div className="flex flex-col gap-1.5">
          {opts.map((o, i) => <OptionRow key={o.label} index={i} shape="radio" selected={d.choice === i} onClick={() => set({
            ...d,
            choice: i
          })} right={o.suggested ? <Pill tone="brand">Suggested</Pill> : undefined}>
              {o.label}
            </OptionRow>)}
          {q.allowOther && (d.choice === opts.length ? <div className="flex h-[35px] items-center gap-2.5 rounded-[8px] border border-(--foreground) bg-(--surface) px-2.5 shadow-[inset_0_0_0_0.5px_var(--foreground)]">
                <span className="flex size-4 shrink-0 items-center justify-center rounded-full border border-(--foreground) bg-(--foreground)">
                  <span className="size-[6px] rounded-full bg-white" />
                </span>
                <input autoFocus value={d.other ?? ''} onChange={e => set({
              ...d,
              other: e.target.value
            })} placeholder="Your own answer…" className="min-w-0 flex-1 bg-transparent text-[14px] outline-none" />
                <Kbd>{opts.length + 1}</Kbd>
              </div> : <OptionRow index={opts.length} shape="radio" selected={false} onClick={() => set({
            ...d,
            choice: opts.length
          })}>
                Other...
              </OptionRow>)}
        </div>;
      }
    case 'multiple':
      {
        const picks = d.picks ?? [];
        return <div className="flex flex-col gap-1">
          {(q.options ?? []).map((o, i) => <OptionRow key={o.label} index={i} shape="check" selected={picks.includes(i)} onClick={() => set({
            ...d,
            picks: picks.includes(i) ? picks.filter(p => p !== i) : [...picks, i].sort()
          })}>
              {o.label}
            </OptionRow>)}
          <p className="mt-2 text-[11px] text-(--muted-foreground)">Pick at least one · {picks.length} picked</p>
        </div>;
      }
    case 'yesno':
      return <div>
          <div className="grid grid-cols-2 gap-2">
            {[true, false].map(v => <button key={String(v)} type="button" onClick={() => set({
            ...d,
            yes: v
          })} className={cx('flex h-10 cursor-pointer items-center justify-center gap-2 rounded-[8px] border text-[14px] font-medium transition-colors', d.yes === v ? 'border-(--foreground) bg-(--muted) shadow-[inset_0_0_0_0.5px_var(--foreground)]' : 'border-(--border) bg-(--surface) hover:bg-(--surface-hover)')}>
                {v ? <Check size={15} /> : <X size={15} />}
                {v ? 'Yes' : 'No'}
                <Kbd>{v ? 'Y' : 'N'}</Kbd>
              </button>)}
          </div>
          {d.yes === false ? <input autoFocus value={d.why ?? ''} onChange={e => set({
          ...d,
          why: e.target.value
        })} placeholder="Why not? One line is enough." className="mt-2 h-9 w-full rounded-[8px] border border-(--input) bg-(--surface) px-2.5 text-[13px] outline-none focus:border-(--ring)" /> : <p className="mt-2.5 text-[11px] text-(--muted-foreground)">Choosing No asks for a one-line reason.</p>}
        </div>;
    case 'confirm':
      {
        const f = d.facts ?? [];
        const done = (q.facts ?? []).filter((_, i) => f[i]).length;
        return <div className="flex flex-col gap-1.5">
          {(q.facts ?? []).map((fact, i) => <div key={fact} className="rounded-[8px] border border-(--border) bg-(--surface) px-2.5 py-2">
              <div className="flex items-center gap-3">
                <span className="min-w-0 flex-1 text-[13px] leading-[18px]">{fact}</span>
                <div className="flex shrink-0 items-center rounded-[7px] bg-(--muted) p-[3px]">
                  {(['y', 'n'] as const).map(v => <button key={v} type="button" onClick={() => {
                  const next = [...f];
                  next[i] = v;
                  set({
                    ...d,
                    facts: next
                  });
                }} className={cx('flex h-6 cursor-pointer items-center gap-1 rounded-[5px] px-1.5 text-[12px] transition-colors', f[i] === v ? 'bg-(--surface) text-(--foreground) shadow-sm' : 'text-(--muted-foreground) hover:text-(--foreground)')}>
                      {v === 'y' ? <Check size={12} className={f[i] === 'y' ? 'text-(--success)' : ''} /> : <X size={12} className={f[i] === 'n' ? 'text-(--destructive)' : ''} />}
                      {v === 'y' ? 'Correct' : 'Not correct'}
                    </button>)}
                </div>
              </div>
              {f[i] === 'n' && <div className="mt-2 flex h-8 items-center gap-2 rounded-[7px] border border-(--border) px-2.5">
                  <Pencil size={13} className="text-(--muted-foreground)" />
                  <input autoFocus value={d.corrections?.[i] ?? ''} onChange={e => set({
                ...d,
                corrections: {
                  ...d.corrections,
                  [i]: e.target.value
                }
              })} placeholder="What is right?" className="min-w-0 flex-1 bg-transparent text-[13px] outline-none" />
                </div>}
            </div>)}
          <p className="mt-1.5 text-[11px] text-(--muted-foreground)">
            {done} of {q.facts?.length} checked
          </p>
        </div>;
      }
    case 'ranking':
      {
        const order = d.order ?? q.items ?? [];
        const move = (i: number, by: number) => {
          const j = i + by;
          if (j < 0 || j >= order.length) return;
          const next = [...order];
          [next[i], next[j]] = [next[j], next[i]];
          set({
            ...d,
            order: next
          });
          setFocusRow(j);
        };
        return <div>
          <div className="flex flex-col gap-1">
            {order.map((item, i) => <div key={item} tabIndex={0} onFocus={() => setFocusRow(i)} onKeyDown={e => {
              if (e.altKey && e.key === 'ArrowUp') e.preventDefault(), move(i, -1);
              if (e.altKey && e.key === 'ArrowDown') e.preventDefault(), move(i, 1);
            }} className={cx('flex h-9 items-center gap-2.5 rounded-[8px] border bg-(--surface) px-2.5 text-[13px] outline-none', focusRow === i ? 'border-(--ring)' : 'border-(--border)')}>
                <GripVertical size={14} className="text-(--faint-foreground)" />
                <span className="w-3 text-[12px] text-(--muted-foreground)">{i + 1}</span>
                <span className="flex-1">{item}</span>
                <button type="button" aria-label="Move" onClick={() => move(i, i === 0 ? 1 : -1)} className="cursor-pointer text-(--muted-foreground) hover:text-(--foreground)">
                  <ArrowUpDown size={14} />
                </button>
              </div>)}
          </div>
          <p className="mt-2.5 text-[11px] text-(--muted-foreground)">Drag, or ⌥↑ ⌥↓ to move the focused item.</p>
        </div>;
      }
    case 'scale':
      return <div>
          <div className="grid grid-cols-5 gap-1.5">
            {[1, 2, 3, 4, 5].map(n => <button key={n} type="button" onClick={() => set({
            ...d,
            scale: n
          })} className={cx('h-10 cursor-pointer rounded-[8px] border text-[14px] font-medium transition-colors', d.scale === n ? 'border-(--foreground) bg-(--foreground) text-white' : 'border-(--border) bg-(--surface) hover:bg-(--surface-hover)')}>
                {n}
              </button>)}
          </div>
          <div className="mt-2.5 flex justify-between text-[11px] text-(--muted-foreground)">
            <span>Not at all</span>
            <span>Exactly</span>
          </div>
        </div>;
    case 'number':
      return <div className="flex items-center gap-2.5">
          <input inputMode="numeric" value={d.num ?? ''} onChange={e => set({
          ...d,
          num: e.target.value.replace(/[^0-9]/g, '')
        })} placeholder={String(q.min ?? 0)} className="h-9 w-[124px] rounded-[8px] border border-(--input) bg-(--surface) px-2.5 text-[14px] outline-none focus:border-(--ring)" />
          <span className="flex-1 text-[14px] text-(--muted-foreground)">{q.unit}</span>
          <span className="text-[11px] text-(--muted-foreground)">
            {q.min} to {q.max}
          </span>
        </div>;
    case 'date':
      return <div>
          <div className="flex h-9 items-center gap-2 rounded-[8px] border border-(--input) bg-(--surface) px-2.5">
            <Calendar size={14} className="text-(--muted-foreground)" />
            <input value={d.date ?? ''} onChange={e => set({
            ...d,
            date: e.target.value
          })} placeholder="Fri 30 Oct 2026" className="min-w-0 flex-1 bg-transparent text-[14px] outline-none" />
          </div>
          <div className="mt-2.5 flex gap-1.5">
            {[['Today', 'Mon 12 Oct 2026'], ['Next Friday', 'Fri 23 Oct 2026'], ['End of month', 'Sat 31 Oct 2026']].map(([l, v]) => <button key={l} type="button" onClick={() => set({
            ...d,
            date: v
          })} className="h-6 cursor-pointer rounded-[5px] bg-(--muted) px-1.5 text-[11px] text-(--muted-foreground) hover:text-(--foreground)">
                {l}
              </button>)}
          </div>
          <p className="mt-2.5 text-[11px] text-(--muted-foreground)">Type “next friday” or pick from the calendar.</p>
        </div>;
    case 'person':
      {
        const list: Array<[PersonId, string]> = [['sarah', 'Member'], ['joeri', 'Agent'], ['mark', 'Member']];
        return <div>
          <div className="flex h-9 items-center gap-2 rounded-[8px] border border-(--input) bg-(--surface) px-2.5">
            <Search size={14} className="text-(--muted-foreground)" />
            <input value={query} onChange={e => setQuery(e.target.value)} placeholder="Search people and agents" className="min-w-0 flex-1 bg-transparent text-[14px] outline-none" />
          </div>
          <div className="mt-2 flex flex-col gap-0.5">
            {list.filter(([id]) => PEOPLE[id].name.toLowerCase().includes(query.toLowerCase())).map(([id, role]) => <button key={id} type="button" onClick={() => set({
              ...d,
              person: id
            })} className={cx('flex h-9 cursor-pointer items-center gap-2 rounded-[7px] px-1.5 text-left', d.person === id ? 'bg-(--muted)' : 'hover:bg-(--surface-hover)')}>
                  <Avatar id={id} size={22} />
                  <span className="text-[13px] font-medium">{PEOPLE[id].name}</span>
                  <span className="flex-1 text-[12px] text-(--muted-foreground)">{role}</span>
                  {d.person === id && <Check size={15} />}
                </button>)}
          </div>
        </div>;
      }
    case 'text':
      return <div>
          <textarea value={d.text ?? ''} onChange={e => set({
          ...d,
          text: e.target.value
        })} placeholder={q.placeholder ?? 'Type your answer…'} rows={dense ? 2 : 3} className="block w-full resize-none rounded-[8px] border border-(--input) bg-(--surface) px-2.5 py-2 text-[14px] leading-[21px] outline-none focus:border-(--ring)" />
          <p className="mt-2.5 text-[11px] text-(--muted-foreground)">Mentions, links and attachments work as in comments.</p>
        </div>;
    case 'file':
      return <div>
          <button type="button" onClick={() => set({
          ...d,
          files: [...(d.files ?? []), 'moneybird-sept.csv'].filter((v, i, a) => a.indexOf(v) === i)
        })} className="flex h-[86px] w-full cursor-pointer flex-col items-center justify-center gap-0.5 rounded-[8px] border border-(--input) bg-(--muted) text-center hover:bg-(--surface-hover)">
            <CloudUpload size={17} className="mb-1 text-(--muted-foreground)" />
            <span className="text-[13px] font-medium">Drop files, or paste a screenshot</span>
            <span className="text-[12px] text-(--muted-foreground)">CSV, XLSX, PNG · up to 10 MB</span>
          </button>
          {(d.files ?? []).map(f => <div key={f} className="mt-2 flex h-9 items-center gap-2 rounded-[8px] border border-(--border) bg-(--surface) px-2.5">
              <FileIcon size={14} className="text-(--muted-foreground)" />
              <span className="text-[13px]">{f}</span>
              <span className="flex-1 text-[12px] text-(--muted-foreground)">24 KB</span>
              <button type="button" aria-label="Remove" onClick={() => set({
            ...d,
            files: (d.files ?? []).filter(x => x !== f)
          })} className="cursor-pointer text-(--muted-foreground) hover:text-(--foreground)">
                <X size={14} />
              </button>
            </div>)}
        </div>;
    case 'approval':
      return <div>
          <a href="#" onClick={e => e.preventDefault()} className="flex h-10 items-center gap-2 rounded-[8px] bg-(--muted) px-3 text-[13px] hover:bg-(--surface-selected)">
            <GitPullRequestArrow size={15} className="text-(--success)" />
            <span className="font-medium">{q.artifact?.title}</span>
            <span className="flex-1 text-[11px] text-(--muted-foreground)">{q.artifact?.ref}</span>
            <ArrowUpRight size={14} className="text-(--muted-foreground)" />
          </a>
          <div className="mt-3 flex items-center gap-2">
            <Btn variant={d.approval === 'approve' ? 'primary' : 'primary'} onClick={() => set({
            ...d,
            approval: 'approve'
          })} className={d.approval && d.approval !== 'approve' ? 'opacity-50' : ''}>
              <Check size={15} /> Approve
            </Btn>
            <Btn variant="outline" onClick={() => set({
            ...d,
            approval: 'changes'
          })} className={d.approval === 'changes' ? 'border-(--foreground)' : ''}>
              <Pencil size={14} /> Request changes
            </Btn>
            <span className="flex-1" />
            <Btn variant="danger-ghost" onClick={() => set({
            ...d,
            approval: 'decline'
          })} className={d.approval === 'decline' ? 'bg-[#fdebec]' : ''}>
              <Ban size={14} /> Decline
            </Btn>
          </div>
          {d.approval === 'changes' ? <input autoFocus value={d.why ?? ''} onChange={e => set({
          ...d,
          why: e.target.value
        })} placeholder="What should change?" className="mt-2 h-9 w-full rounded-[8px] border border-(--input) bg-(--surface) px-2.5 text-[13px] outline-none focus:border-(--ring)" /> : <p className="mt-2.5 text-[11px] text-(--muted-foreground)">Request changes needs a note; Decline closes the question.</p>}
        </div>;
  }
}

/** Note · Ask back · Reassign: the escape hatches every type keeps. */
export function EscapeHatches({
  short,
  onNote,
  onAskBack,
  onReassign
}: {
  short?: boolean;
  onNote?: () => void;
  onAskBack?: () => void;
  onReassign?: () => void;
}) {
  return <div className="flex items-center gap-0.5">
      {onNote && <Btn variant="ghost" size="sm" onClick={onNote}>
          <NotebookPen size={14} /> {short ? 'Note' : 'Add a note'}
        </Btn>}
      <Btn variant="ghost" size="sm" onClick={onAskBack}>
        <CornerUpLeft size={14} /> Ask back
      </Btn>
      <Btn variant="ghost" size="sm" onClick={onReassign}>
        <UserRoundPlus size={14} /> Reassign
      </Btn>
    </div>;
}

/** The type overline: tile + uppercase label. */
export function TypeOverline({
  type,
  right
}: {
  type: QType;
  right?: ReactNode;
}) {
  return <div className="flex h-5 items-center gap-2">
      <span className="flex size-5 items-center justify-center rounded-[5px] bg-(--muted) text-(--muted-foreground)">
        <TypeGlyph type={type} size={12} />
      </span>
      <span className="flex-1 text-[11px] font-semibold tracking-[0.06em] text-(--muted-foreground) uppercase">{TYPE_META[type].label}</span>
      {right}
    </div>;
}

/** A compact open card: the answer types sheet, the inbox and the one-page list use it. */
export function CompactCard({
  q,
  d,
  set,
  onSend,
  focused,
  onFocus,
  footerNote
}: {
  q: Question;
  d: Draft;
  set: (d: Draft) => void;
  onSend?: (summary: string) => void;
  focused?: boolean;
  onFocus?: () => void;
  footerNote?: ReactNode;
}) {
  const summary = summarize(q, d);
  const [note, setNote] = useState<string | null>(null);
  const [aside, setAside] = useState<string | null>(null);
  const sendsItself = q.type === 'approval';
  return <section onPointerDown={onFocus} onFocusCapture={onFocus} className={cx('rounded-[12px] border bg-(--surface) px-4 pt-4 pb-3 transition-shadow', focused ? 'border-(--brand) shadow-[0_0_0_1px_var(--brand)]' : 'border-(--border)')} style={{
    boxShadow: focused ? undefined : 'var(--surface-shadow)'
  }}>
      <TypeOverline type={q.type} />
      <div className="mt-3 flex items-center gap-2">
        <Avatar id={q.asker} size={20} />
        <span className="text-[13px] font-medium">{PEOPLE[q.asker].name}</span>
        <span className="text-[11px] text-(--muted-foreground)">{q.metaShort}</span>
      </div>
      <h3 className="mt-3 text-[16px] leading-[22px] font-semibold">{q.prompt}</h3>
      <div className="mt-3">
        <AnswerBody q={q} d={d} set={set} />
      </div>
      {note !== null && <textarea autoFocus value={note} onChange={e => setNote(e.target.value)} placeholder={`Note for ${PEOPLE[q.asker].name} (optional)`} rows={2} className="mt-3 block w-full resize-none rounded-[8px] border border-(--input) px-2.5 py-2 text-[13px] outline-none focus:border-(--ring)" />}
      {aside && <p className="mt-3 rounded-[8px] bg-(--muted) px-2.5 py-2 text-[12px] text-(--muted-foreground)">{aside}</p>}
      <div className="mt-3 flex items-center gap-2">
        <EscapeHatches short onNote={() => setNote(n => n === null ? '' : n)} onAskBack={() => setAside(`Ask back opens a reply in the question's thread; ${PEOPLE[q.asker].name} sees it and the question stays open.`)} onReassign={() => setAside('Reassign hands the question to another member with a note; the card keeps the history.')} />
        <span className="flex-1" />
        {footerNote}
        {!sendsItself && onSend && <Btn variant="primary" disabled={!summary} onClick={() => summary && onSend(summary)}>
            <Send size={14} /> Send
          </Btn>}
      </div>
    </section>;
}
export const REFERENCE: Question[] = [{
  id: 'r-text',
  type: 'text',
  prompt: 'What should the export file be called?',
  asker: 'sarah',
  askedLine: '',
  metaShort: '2 h ago',
  age: '',
  link: 'WORK-412',
  placeholder: 'Type your answer…'
}, {
  id: 'r-yesno',
  type: 'yesno',
  prompt: 'Can the export include cancelled invoices?',
  asker: 'pieter',
  askedLine: '',
  metaShort: 'run #3 · 5 min ago',
  age: '',
  link: 'WORK-412'
}, {
  id: 'r-single',
  type: 'single',
  prompt: 'Should VAT round per invoice line or on the total?',
  asker: 'pieter',
  askedLine: '',
  metaShort: 'run #3 · 12 min ago',
  age: '',
  link: 'WORK-412',
  options: [{
    label: 'Per invoice line',
    suggested: true
  }, {
    label: 'On the invoice total'
  }],
  allowOther: true
}, {
  id: 'r-multiple',
  type: 'multiple',
  prompt: 'Which VAT rates must the export show?',
  asker: 'sarah',
  askedLine: '',
  metaShort: '1 h ago',
  age: '',
  link: 'WORK-412',
  options: [{
    label: '21% standard'
  }, {
    label: '9% reduced'
  }, {
    label: '0% exempt'
  }, {
    label: 'Reverse charge'
  }]
}, {
  id: 'r-confirm',
  type: 'confirm',
  prompt: 'Before I build the export, are these right?',
  asker: 'pieter',
  askedLine: '',
  metaShort: 'run #3 · 20 min ago',
  age: '',
  link: 'WORK-412',
  facts: ['Accountants import the file into Moneybird.', 'Moneybird rounds VAT on the invoice total.', 'One file covers one calendar month.']
}, {
  id: 'r-ranking',
  type: 'ranking',
  prompt: 'Order the columns the way accountants read them.',
  asker: 'mark',
  askedLine: '',
  metaShort: 'yesterday',
  age: '',
  link: 'WORK-412',
  items: ['Invoice date', 'Invoice number', 'Customer', 'Net amount', 'VAT amount']
}, {
  id: 'r-scale',
  type: 'scale',
  prompt: 'How close do the sample totals match your books?',
  asker: 'pieter',
  askedLine: '',
  metaShort: 'run #3 · 1 h ago',
  age: '',
  link: 'WORK-412'
}, {
  id: 'r-number',
  type: 'number',
  prompt: 'How many months back should the first export go?',
  asker: 'sarah',
  askedLine: '',
  metaShort: '3 h ago',
  age: '',
  link: 'WORK-412',
  unit: 'months',
  min: 1,
  max: 36
}, {
  id: 'r-date',
  type: 'date',
  prompt: 'When must the first export reach the accountant?',
  asker: 'mark',
  askedLine: '',
  metaShort: 'today',
  age: '',
  link: 'WORK-412'
}, {
  id: 'r-person',
  type: 'person',
  prompt: 'Who should review the first export?',
  asker: 'pieter',
  askedLine: '',
  metaShort: 'run #3 · 2 h ago',
  age: '',
  link: 'WORK-412'
}, {
  id: 'r-file',
  type: 'file',
  prompt: 'Attach a sample export from Moneybird.',
  asker: 'sarah',
  askedLine: '',
  metaShort: '4 h ago',
  age: '',
  link: 'WORK-412'
}, {
  id: 'r-approval',
  type: 'approval',
  prompt: 'Approve the column layout before I build it?',
  asker: 'pieter',
  askedLine: '',
  metaShort: 'run #4 · just now',
  age: '',
  link: 'WORK-412',
  artifact: {
    title: 'Column layout v2',
    ref: 'PR #188'
  }
}];

/** Drafts that make the reference sheet show each control filled in, as the design does. */
export const REFERENCE_SEED: Record<string, Draft> = {
  'r-text': {
    text: 'invoices-2026-09-van-dijk.csv'
  },
  'r-single': {
    choice: 0
  },
  'r-multiple': {
    picks: [0, 1, 3]
  },
  'r-confirm': {
    facts: ['y', 'n', undefined],
    corrections: {
      1: 'It rounds per line since 2024.'
    }
  },
  'r-ranking': {
    order: ['Invoice date', 'Invoice number', 'Customer', 'Net amount', 'VAT amount']
  },
  'r-scale': {
    scale: 4
  },
  'r-number': {
    num: '12'
  },
  'r-date': {
    date: 'Fri 30 Oct 2026'
  },
  'r-person': {
    person: 'sarah'
  },
  'r-file': {
    files: ['moneybird-sept.csv']
  }
};