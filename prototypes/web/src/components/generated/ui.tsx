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
