import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react';
import {
  Bookmark,
  Bug,
  Check,
  CircleDashed,
  FlaskConical,
  Flag,
  GitCompareArrows,
  Rocket,
  Route,
  Sparkles,
  Wrench,
  type LucideIcon,
} from 'lucide-react';
import type { IssueType, ReleaseState, Status } from './model';

export const TYPE_ICON: Record<IssueType, LucideIcon> = {
  epic: Flag,
  journey: Route,
  story: Bookmark,
  bug: Bug,
  enhancement: Sparkles,
  refactor: GitCompareArrows,
  chore: Wrench,
  spike: FlaskConical,
};

export function TypeIcon({ type, size = 14 }: { type: IssueType; size?: number }) {
  const Icon = TYPE_ICON[type];
  return <Icon size={size} strokeWidth={2} style={{ color: `var(--type-${type})` }} aria-hidden />;
}

/** Multica's status glyph: dashed ring, ring, pie by progress, check disc. */
export function StatusIcon({ status, size = 14 }: { status: Status; size?: number }) {
  if (status === 'backlog') return <CircleDashed size={size} style={{ color: 'var(--muted-foreground)' }} aria-label="Backlog" />;
  if (status === 'done')
    return (
      <svg width={size} height={size} viewBox="0 0 14 14" aria-label="Done">
        <circle cx="7" cy="7" r="6.5" fill="var(--info)" />
        <path d="M4.2 7.2l1.9 1.9 3.7-3.9" fill="none" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    );
  const color = status === 'todo' ? 'var(--muted-foreground)' : status === 'in_progress' ? 'var(--warning)' : 'var(--success)';
  const pie = status === 'in_progress' ? 'M7 7 L7 3.5 A3.5 3.5 0 0 1 7 10.5 Z' : status === 'in_review' ? 'M7 7 L7 3.5 A3.5 3.5 0 1 1 3.5 7 Z' : '';
  return (
    <svg width={size} height={size} viewBox="0 0 14 14" aria-label={status}>
      <circle cx="7" cy="7" r="6" fill="none" stroke={color} strokeWidth="1.5" />
      {pie && <path d={pie} fill={color} />}
    </svg>
  );
}

export function Avatar({ text, title, size = 20 }: { text: string; title?: string; size?: number }) {
  return (
    <span
      title={title}
      className="inline-flex shrink-0 items-center justify-center rounded-full font-semibold text-white"
      style={{ width: size, height: size, fontSize: size <= 20 ? 9 : 11, background: 'var(--chart-3)' }}
    >
      {text}
    </span>
  );
}

const chipBase =
  'inline-flex h-6 shrink-0 items-center gap-1.5 rounded-[6px] border px-2 text-[12px] whitespace-nowrap transition-colors';

/** Release chip in its three states: shipped (check), next (brand, rocket), planned (dashed). */
export function ReleaseChip({ version, state, onClick }: { version: string; state: ReleaseState; onClick?: () => void }) {
  const next = state === 'next';
  const Tag = onClick ? 'button' : 'span';
  return (
    <Tag
      onClick={onClick}
      className={`${chipBase} mc-mono ${onClick ? 'cursor-pointer hover:bg-[var(--surface-hover)]' : ''}`}
      style={{
        borderColor: next ? 'var(--brand)' : 'var(--surface-border)',
        color: next ? 'var(--brand)' : state === 'shipped' ? 'var(--foreground)' : 'var(--muted-foreground)',
        background: 'var(--surface)',
        fontWeight: next ? 500 : 400,
      }}
    >
      {state === 'shipped' ? <Check size={12} /> : next ? <Rocket size={12} /> : <CircleDashed size={12} />}
      {version}
    </Tag>
  );
}

export function Chip({
  icon,
  children,
  onClick,
  active,
  title,
}: {
  icon?: ReactNode;
  children: ReactNode;
  onClick?: () => void;
  active?: boolean;
  title?: string;
}) {
  const Tag = onClick ? 'button' : 'span';
  return (
    <Tag
      type={onClick ? 'button' : undefined}
      title={title}
      onClick={onClick}
      className={`${chipBase} ${onClick ? 'cursor-pointer hover:bg-[var(--surface-hover)]' : ''}`}
      style={{
        borderColor: active ? 'var(--ring)' : 'var(--surface-border)',
        background: 'var(--surface)',
        color: 'var(--foreground)',
      }}
    >
      {icon}
      {children}
    </Tag>
  );
}

export function Button({
  children,
  variant = 'outline',
  onClick,
  disabled,
  icon,
  type = 'button',
}: {
  children: ReactNode;
  variant?: 'primary' | 'brand' | 'outline' | 'ghost';
  onClick?: () => void;
  disabled?: boolean;
  icon?: ReactNode;
  type?: 'button' | 'submit';
}) {
  const styles: Record<string, CSSProperties> = {
    primary: { background: 'var(--primary)', color: 'var(--primary-foreground)' },
    brand: { background: 'var(--brand)', color: 'var(--brand-foreground)' },
    outline: { background: 'var(--surface)', color: 'var(--foreground)', border: '1px solid var(--surface-border)' },
    ghost: { background: 'transparent', color: 'var(--foreground)' },
  };
  return (
    <button
      type={type}
      disabled={disabled}
      onClick={onClick}
      className={`inline-flex h-8 items-center gap-1.5 rounded-[6px] px-3 text-[14px] font-medium transition-opacity ${
        variant === 'ghost' ? 'hover:bg-[var(--surface-hover)]' : 'hover:opacity-90'
      } disabled:cursor-not-allowed disabled:opacity-40`}
      style={styles[variant]}
    >
      {icon}
      {children}
    </button>
  );
}

export function Progress({ done, total }: { done: number; total: number }) {
  const pct = total ? (done / total) * 100 : 0;
  return (
    <div className="h-1 w-full overflow-hidden rounded-full" style={{ background: 'var(--muted)' }}>
      <div className="h-full rounded-full" style={{ width: `${pct}%`, background: 'var(--brand)' }} />
    </div>
  );
}

/** Popover anchored under its trigger; closes on outside click and Escape. */
export function Menu({
  trigger,
  children,
  align = 'left',
  width = 220,
}: {
  trigger: (open: boolean, toggle: () => void) => ReactNode;
  children: (close: () => void) => ReactNode;
  align?: 'left' | 'right';
  width?: number;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey, true);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey, true);
    };
  }, [open]);
  return (
    <div ref={ref} className="relative inline-flex">
      {trigger(open, () => setOpen((o) => !o))}
      {open && (
        <div
          role="menu"
          className="mc-pop absolute top-[calc(100%+4px)] z-50 rounded-[8px] border p-1"
          style={{
            [align]: 0,
            width,
            background: 'var(--surface)',
            borderColor: 'var(--surface-border)',
            boxShadow: 'var(--floating-shadow)',
          }}
        >
          {children(() => setOpen(false))}
        </div>
      )}
    </div>
  );
}

export function MenuItem({
  children,
  onClick,
  selected,
  icon,
}: {
  children: ReactNode;
  onClick: () => void;
  selected?: boolean;
  icon?: ReactNode;
}) {
  return (
    <button
      role="menuitem"
      onClick={onClick}
      className="flex h-8 w-full items-center gap-2 rounded-[6px] px-2 text-left text-[13px] hover:bg-[var(--surface-hover)]"
    >
      {icon}
      <span className="min-w-0 flex-1 truncate">{children}</span>
      {selected && <Check size={14} style={{ color: 'var(--muted-foreground)' }} />}
    </button>
  );
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className="inline-flex h-8 items-center gap-0.5 rounded-[8px] p-0.5" style={{ background: 'var(--muted)' }} role="tablist">
      {options.map((o) => (
        <button
          key={o.value}
          role="tab"
          aria-selected={o.value === value}
          onClick={() => onChange(o.value)}
          className="h-7 rounded-[6px] px-2.5 text-[13px] transition-colors"
          style={
            o.value === value
              ? { background: 'var(--surface)', color: 'var(--foreground)', boxShadow: 'var(--surface-shadow)', fontWeight: 500 }
              : { color: 'var(--muted-foreground)' }
          }
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Markdown: the subset the issue templates use. Headings, question callouts,
// checklists, lists and paragraphs, with **bold** and `code` inline.
// ---------------------------------------------------------------------------

type MdBlock =
  | { kind: 'heading'; level: number; text: string }
  | { kind: 'callout'; text: string }
  | { kind: 'checks'; items: { done: boolean; text: string }[] }
  | { kind: 'bullets'; items: string[] }
  | { kind: 'numbered'; items: string[] }
  | { kind: 'paragraph'; text: string };

function parseMarkdown(md: string): MdBlock[] {
  const out: MdBlock[] = [];
  const last = () => out[out.length - 1];
  let gap = true;
  for (const raw of md.split('\n')) {
    const line = raw.trimEnd();
    if (!line.trim()) {
      gap = true;
      continue;
    }
    const h = line.match(/^(#{1,6})\s+(.*)$/);
    const q = line.match(/^>\s?(.*)$/);
    const c = line.match(/^\s*[-*+]\s+\[([ xX])\]\s+(.*)$/);
    const b = line.match(/^\s*[-*+]\s+(.*)$/);
    const n = line.match(/^\s*\d+[.)]\s+(.*)$/);
    const prev = last();
    if (h) out.push({ kind: 'heading', level: h[1].length, text: h[2] });
    else if (q) prev?.kind === 'callout' && !gap ? (prev.text += `\n${q[1]}`) : out.push({ kind: 'callout', text: q[1] });
    else if (c) {
      const item = { done: c[1] !== ' ', text: c[2] };
      prev?.kind === 'checks' ? prev.items.push(item) : out.push({ kind: 'checks', items: [item] });
    } else if (b) prev?.kind === 'bullets' ? prev.items.push(b[1]) : out.push({ kind: 'bullets', items: [b[1]] });
    else if (n) prev?.kind === 'numbered' ? prev.items.push(n[1]) : out.push({ kind: 'numbered', items: [n[1]] });
    else if (prev?.kind === 'paragraph' && !gap) prev.text += `\n${line}`;
    else out.push({ kind: 'paragraph', text: line.trim() });
    gap = false;
  }
  return out;
}

function Inline({ text }: { text: string }) {
  return (
    <>
      {text.split(/(\*\*[^*]+\*\*|`[^`]+`)/g).map((part, i) =>
        part.startsWith('**') && part.endsWith('**') && part.length > 4 ? (
          <strong key={i} className="font-semibold">
            {part.slice(2, -2)}
          </strong>
        ) : part.startsWith('`') && part.endsWith('`') && part.length > 2 ? (
          <code key={i} className="mc-mono rounded-[4px] px-1 text-[0.9em]" style={{ background: 'var(--muted)' }}>
            {part.slice(1, -1)}
          </code>
        ) : (
          part
        ),
      )}
    </>
  );
}

const HEADING_CLASS: Record<'page' | 'peek', Record<number, string>> = {
  page: { 1: 'text-[20px] font-semibold', 2: 'text-[16px] font-semibold', 3: 'text-[14px] font-semibold', 4: 'text-[13px] font-semibold' },
  peek: { 1: 'text-[15px] font-semibold', 2: 'text-[14px] font-semibold', 3: 'text-[13px] font-semibold', 4: 'text-[13px] font-medium' },
};

/** Renders an issue's Markdown description. */
export function Markdown({ text, size = 'page' }: { text: string; size?: 'page' | 'peek' }) {
  const body = size === 'page' ? 'text-[15px] leading-[24px]' : 'text-[14px] leading-[21px]';
  return (
    <div className={`mc-md ${size === 'page' ? 'space-y-2.5' : 'space-y-2'}`}>
      {parseMarkdown(text).map((b, i) => {
        switch (b.kind) {
          case 'heading': {
            const Tag = (`h${Math.min(b.level + 1, 6)}` as 'h2');
            return (
              <Tag key={i} className={`${HEADING_CLASS[size][Math.min(b.level, 4)]} ${i ? (b.level <= 2 ? 'pt-4' : b.level === 3 ? 'pt-3' : 'pt-1') : ''}`}>
                <Inline text={b.text} />
              </Tag>
            );
          }
          case 'callout':
            return (
              <div
                key={i}
                className={`whitespace-pre-wrap rounded-[8px] px-3 py-2 ${size === 'page' ? 'text-[13px] leading-[20px]' : 'text-[12px] leading-[18px]'}`}
                style={{ background: 'var(--muted)', color: 'var(--muted-foreground)' }}
              >
                <Inline text={b.text} />
              </div>
            );
          case 'checks':
            return (
              <ul key={i} className={`space-y-1 ${body}`}>
                {b.items.map((it, j) => (
                  <li key={j} className="flex items-start gap-2">
                    <span
                      className="mt-[5px] flex size-[14px] shrink-0 items-center justify-center rounded-[4px] border"
                      style={{ borderColor: 'var(--input)', background: it.done ? 'var(--brand)' : 'var(--surface)' }}
                      aria-hidden
                    >
                      {it.done && <Check size={10} style={{ color: 'var(--brand-foreground)' }} />}
                    </span>
                    <span>
                      <Inline text={it.text} />
                    </span>
                  </li>
                ))}
              </ul>
            );
          case 'bullets':
            return (
              <ul key={i} className={`list-disc space-y-1 pl-5 ${body}`}>
                {b.items.map((it, j) => (
                  <li key={j}>
                    <Inline text={it} />
                  </li>
                ))}
              </ul>
            );
          case 'numbered':
            return (
              <ol key={i} className={`list-decimal space-y-1 pl-5 ${body}`}>
                {b.items.map((it, j) => (
                  <li key={j}>
                    <Inline text={it} />
                  </li>
                ))}
              </ol>
            );
          case 'paragraph':
            return (
              <p key={i} className={`whitespace-pre-wrap ${body}`}>
                <Inline text={b.text} />
              </p>
            );
        }
      })}
    </div>
  );
}
