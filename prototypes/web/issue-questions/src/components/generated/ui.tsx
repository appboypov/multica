import { useEffect, useRef, type ReactNode, type CSSProperties } from 'react';
import { Bot, Calendar, CheckCheck, CircleDot, Gauge, Hash, ListChecks, ListOrdered, ListTodo, Paperclip, Stamp, ToggleLeft, Type, UserRound, type LucideIcon } from 'lucide-react';
import { PEOPLE, type PersonId, type QType } from './model';
export const TYPE_ICON: Record<QType | 'set', LucideIcon> = {
  text: Type,
  yesno: ToggleLeft,
  single: CircleDot,
  multiple: ListChecks,
  confirm: CheckCheck,
  ranking: ListOrdered,
  scale: Gauge,
  number: Hash,
  date: Calendar,
  person: UserRound,
  file: Paperclip,
  approval: Stamp,
  set: ListTodo
};
export function cx(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(' ');
}

/** A member's initials on a pastel disc, or an agent's bot glyph on a near-black tile. */
export function Avatar({
  id,
  size = 20
}: {
  id: PersonId;
  size?: number;
}) {
  const p = PEOPLE[id];
  if (p.agent) return <span title={p.name} className="inline-flex shrink-0 items-center justify-center rounded-[6px] bg-(--foreground) text-white" style={{
    width: size,
    height: size
  }}>
        <Bot size={Math.round(size * 0.62)} strokeWidth={2} aria-hidden />
      </span>;
  return <span title={p.name} className="inline-flex shrink-0 items-center justify-center rounded-full font-semibold" style={{
    width: size,
    height: size,
    fontSize: size <= 18 ? 8 : size <= 22 ? 9 : 10,
    background: p.bg,
    color: p.fg,
    letterSpacing: 0.2
  }}>
      {p.initials}
    </span>;
}
export function Kbd({
  children,
  className
}: {
  children: ReactNode;
  className?: string;
}) {
  return <kbd className={cx('pointer-events-none inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-[3px] border border-(--input) bg-(--surface) px-[5px] font-sans text-[11px] font-medium leading-none text-(--muted-foreground)', className)}>
      {children}
    </kbd>;
}
type BtnVariant = 'primary' | 'outline' | 'ghost' | 'danger-ghost';
export function Btn({
  children,
  variant = 'outline',
  size = 'md',
  onClick,
  className,
  disabled,
  title,
  style,
  type = 'button'
}: {
  children: ReactNode;
  variant?: BtnVariant;
  size?: 'sm' | 'md' | 'lg';
  onClick?: () => void;
  className?: string;
  disabled?: boolean;
  title?: string;
  style?: CSSProperties;
  type?: 'button' | 'submit';
}) {
  const v = variant === 'primary' ? 'bg-(--primary) text-(--primary-foreground) hover:bg-[#27272a] border-transparent' : variant === 'outline' ? 'border-(--surface-border) bg-(--surface) text-(--foreground) hover:bg-(--surface-hover)' : variant === 'danger-ghost' ? 'border-transparent text-(--destructive) hover:bg-[#fdebec]' : 'border-transparent text-(--muted-foreground) hover:bg-(--surface-hover) hover:text-(--foreground)';
  const s = size === 'sm' ? 'h-7 px-2 text-[13px] gap-1.5 rounded-[6px]' : 'h-8 px-3 text-[14px] gap-1.5 rounded-[8px]';
  return <button type={type} title={title} disabled={disabled} onClick={onClick} style={style} className={cx('inline-flex shrink-0 cursor-pointer items-center justify-center border font-medium whitespace-nowrap transition-colors select-none active:translate-y-px disabled:cursor-default disabled:opacity-45 [&_svg]:shrink-0', v, s, className)}>
      {children}
    </button>;
}
export function Switch({
  checked,
  onChange,
  label
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return <button type="button" role="switch" aria-checked={checked} aria-label={label} onClick={() => onChange(!checked)} className="relative inline-flex h-[18px] w-8 shrink-0 cursor-pointer items-center rounded-full transition-colors" style={{
    background: checked ? 'var(--primary)' : 'var(--input)'
  }}>
      <span className="block size-[14px] rounded-full bg-white shadow-sm transition-transform" style={{
      transform: checked ? 'translateX(16px)' : 'translateX(2px)'
    }} />
    </button>;
}

/** Tinted pill: Blocks work, Waiting for answer, Suggested, counts. */
export function Pill({
  tone,
  icon,
  children,
  className
}: {
  tone: 'danger' | 'warning' | 'brand' | 'neutral' | 'success';
  icon?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  const t = {
    danger: 'bg-[#fdebec] text-(--destructive)',
    warning: 'bg-[#f6eed7] text-[#946c00]',
    brand: 'bg-[#0070e314] text-(--brand)',
    neutral: 'bg-(--muted) text-(--muted-foreground)',
    success: 'bg-[#e8f3ea] text-(--success)'
  }[tone];
  return <span className={cx('inline-flex h-5 shrink-0 items-center gap-1 rounded-[4px] px-1.5 text-[12px] font-medium leading-none whitespace-nowrap', t, className)}>
      {icon}
      {children}
    </span>;
}

/** Close on outside pointer-down and Escape. */
export function useDismiss(open: boolean, onClose: () => void) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (!open) return;
    const down = (e: PointerEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation();
        onClose();
      }
    };
    const t = setTimeout(() => document.addEventListener('pointerdown', down), 0);
    document.addEventListener('keydown', key, true);
    return () => {
      clearTimeout(t);
      document.removeEventListener('pointerdown', down);
      document.removeEventListener('keydown', key, true);
    };
  }, [open, onClose]);
  return ref;
}
export function Menu({
  children,
  className,
  style
}: {
  children: ReactNode;
  className?: string;
  style?: CSSProperties;
}) {
  return <div className={cx('mc-pop absolute z-40 rounded-[10px] border border-(--border) bg-(--surface) p-1', className)} style={{
    boxShadow: 'var(--menu-shadow)',
    ...style
  }}>
      {children}
    </div>;
}
export function TypeGlyph({
  type,
  size = 14,
  className
}: {
  type: QType | 'set';
  size?: number;
  className?: string;
}) {
  const Icon = TYPE_ICON[type];
  return <Icon size={size} strokeWidth={1.75} className={cx('shrink-0', className)} aria-hidden />;
}

/** The small square icon tile used before type labels and in list rows (Pencil TypeIcon: 20 square, muted fill, 12 icon). */
export function TypeTile({
  type,
  size = 20,
  brand
}: {
  type: QType | 'set';
  size?: number;
  brand?: boolean;
}) {
  return <span className={cx('inline-flex shrink-0 items-center justify-center rounded-[4px] bg-(--muted)', brand ? 'text-(--brand)' : 'text-(--muted-foreground)')} style={{
    width: size,
    height: size
  }}>
      <TypeGlyph type={type} size={Math.round(size * 0.6)} />
    </span>;
}

/** Multica status glyph: in progress = dashed amber ring, todo = grey ring. */
export function StatusGlyph({
  status,
  size = 14
}: {
  status: 'in_progress' | 'todo' | 'done';
  size?: number;
}) {
  if (status === 'done') return <svg width={size} height={size} viewBox="0 0 14 14" aria-label="Done">
        <circle cx="7" cy="7" r="6.5" fill="var(--info)" />
        <path d="M4.2 7.2l1.9 1.9 3.7-3.9" fill="none" stroke="#fff" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
      </svg>;
  if (status === 'in_progress') return <svg width={size} height={size} viewBox="0 0 14 14" aria-label="In progress">
        <circle cx="7" cy="7" r="6" fill="none" stroke="var(--warning)" strokeWidth="1.4" strokeDasharray="2.2 1.8" />
      </svg>;
  return <svg width={size} height={size} viewBox="0 0 14 14" aria-label="Todo">
      <circle cx="7" cy="7" r="6" fill="none" stroke="var(--muted-foreground)" strokeWidth="1.3" />
    </svg>;
}