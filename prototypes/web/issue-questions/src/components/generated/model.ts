export type PersonId = 'brian' | 'sarah' | 'mark' | 'eva' | 'pieter' | 'joeri';
export type QType =
  | 'yesno'
  | 'single'
  | 'multiple'
  | 'ranking'
  | 'scale'
  | 'person'
  | 'text'
  | 'number'
  | 'date'
  | 'file'
  | 'confirm'
  | 'approval';

export const PEOPLE: Record<PersonId, { name: string; initials: string; agent?: boolean; bg: string; fg: string }> = {
  brian: { name: 'Brian', initials: 'BV', bg: '#dbeafe', fg: '#1e3a8a' },
  sarah: { name: 'Sarah', initials: 'SJ', bg: '#dcfce7', fg: '#14532d' },
  mark: { name: 'Mark', initials: 'MK', bg: '#fce7f3', fg: '#831843' },
  eva: { name: 'Eva', initials: 'EV', bg: '#fef9c3', fg: '#713f12' },
  pieter: { name: 'Pieter', initials: 'P', agent: true, bg: '', fg: '' },
  joeri: { name: 'Joeri', initials: 'J', agent: true, bg: '', fg: '' },
};

export const ME: PersonId = 'brian';

export const TYPE_META: Record<QType, { label: string; hint: string; group: 'Choose' | 'Write' | 'Decide' }> = {
  yesno: { label: 'Yes / No', hint: 'Two buttons, optional why', group: 'Choose' },
  single: { label: 'Single choice', hint: 'One option, can suggest one', group: 'Choose' },
  multiple: { label: 'Multiple choice', hint: 'Several options, min and max', group: 'Choose' },
  ranking: { label: 'Ranking', hint: 'Put items in order', group: 'Choose' },
  scale: { label: 'Scale', hint: '1–5 or 0–10 with labels', group: 'Choose' },
  person: { label: 'Person', hint: 'A member or an agent', group: 'Choose' },
  text: { label: 'Text', hint: 'Short or long answer', group: 'Write' },
  number: { label: 'Number', hint: 'With unit and limits', group: 'Write' },
  date: { label: 'Date', hint: 'Date, time or range', group: 'Write' },
  file: { label: 'File', hint: 'Upload or paste', group: 'Write' },
  confirm: { label: 'Confirm facts', hint: 'Correct or not, per statement', group: 'Decide' },
  approval: { label: 'Approval', hint: 'Approve, ask changes, decline', group: 'Decide' },
};

export const TYPE_ORDER: QType[] = ['yesno', 'single', 'multiple', 'ranking', 'scale', 'person', 'text', 'number', 'date', 'file', 'confirm', 'approval'];

export type LinkId = 'WORK-412' | 'WORK-398' | 'agent-runtime';
export const LINKS: Record<LinkId, { key: string; title: string; kind: 'issue' | 'project'; status: 'in_progress' | 'todo' }> = {
  'WORK-412': { key: 'WORK-412', title: 'Export invoices as CSV with VAT breakdown', kind: 'issue', status: 'in_progress' },
  'WORK-398': { key: 'WORK-398', title: 'Onboarding emails for new workspaces', kind: 'issue', status: 'todo' },
  'agent-runtime': { key: 'Project', title: 'Agent runtime', kind: 'project', status: 'todo' },
};

export interface Option {
  label: string;
  suggested?: boolean;
}

/** One question as the answerer sees it. Type-specific settings sit next to the common fields. */
export interface Question {
  id: string;
  type: QType;
  prompt: string;
  context?: string;
  asker: PersonId;
  /** "asked during run #3 · 12 min ago" (issue card) */
  askedLine: string;
  /** "run #3 · 12 min ago" (compact card) */
  metaShort: string;
  /** relative age for list rows */
  age: string;
  link: LinkId;
  via?: string;
  blocks?: boolean;
  due?: { label: string; tone: 'warning' | 'danger' | 'none' };
  anyOf?: number;
  options?: Option[];
  allowOther?: boolean;
  facts?: string[];
  items?: string[];
  unit?: string;
  min?: number;
  max?: number;
  placeholder?: string;
  hint?: string;
  artifact?: { title: string; ref: string };
}

export interface Answer {
  /** The one-line folded form: "Per invoice line" */
  summary: string;
  note?: string;
  when: string;
  /** Agents read an answer only once a person wakes them. */
  seen: boolean;
}

export const Q_VAT: Question = {
  id: 'vat',
  type: 'single',
  prompt: 'Should VAT round per invoice line or on the invoice total?',
  context: 'Dutch rules allow both. Per line matches what Moneybird reports; per total differs by €0.01 on 4 of the 212 test invoices.',
  asker: 'pieter',
  askedLine: 'asked during run #3 · 12 min ago',
  metaShort: 'run #3 · 12 min ago',
  age: '12 min',
  link: 'WORK-412',
  blocks: true,
  due: { label: 'Due today', tone: 'warning' },
  options: [{ label: 'Per invoice line', suggested: true }, { label: 'On the invoice total' }],
  allowOther: true,
};

export const Q_CONFIRM: Question = {
  id: 'confirm',
  type: 'confirm',
  prompt: 'Before I build the export, are these right?',
  asker: 'pieter',
  askedLine: 'asked during run #3 · 20 min ago',
  metaShort: 'run #3 · 20 min ago',
  age: '20 min',
  link: 'WORK-412',
  blocks: true,
  facts: ['Accountants import the file into Moneybird.', 'Moneybird rounds VAT on the invoice total.', 'One file covers one calendar month.'],
};

export const Q_APPROVE: Question = {
  id: 'approve',
  type: 'approval',
  prompt: 'Approve the column layout before I build it?',
  asker: 'pieter',
  askedLine: 'asked during run #4 · just now',
  metaShort: 'run #4 · just now',
  age: 'Just now',
  link: 'WORK-412',
  via: 'via sub-issue WORK-415',
  artifact: { title: 'Column layout v2', ref: 'PR #188' },
};

export const Q_DATE: Question = {
  id: 'launch',
  type: 'date',
  prompt: 'When does the Dutch launch go out?',
  asker: 'mark',
  askedLine: 'asked today',
  metaShort: 'today',
  age: '1 d',
  link: 'WORK-398',
  due: { label: 'Overdue 1 d', tone: 'danger' },
};

export const Q_TONE: Question = {
  id: 'tone',
  type: 'text',
  prompt: 'What tone should the welcome email have?',
  asker: 'sarah',
  askedLine: 'asked 2 h ago',
  metaShort: '2 h ago',
  age: '2 h',
  link: 'WORK-398',
  anyOf: 2,
  placeholder: 'Type your answer…',
};

export const Q_RESTART: Question = {
  id: 'restart',
  type: 'yesno',
  prompt: 'Can Joeri restart the runtime during office hours?',
  asker: 'joeri',
  askedLine: 'asked during run #12 · yesterday',
  metaShort: 'run #12 · yesterday',
  age: 'Yesterday',
  link: 'agent-runtime',
};

/** The question set Pieter asks once woken: three parts, each answered and sent on its own. */
export const SET_PARTS: Question[] = [
  { ...Q_CONFIRM, id: 'set-confirm', prompt: 'Confirm facts', askedLine: 'asked during run #4 · 2 min ago', metaShort: 'run #4 · 2 min ago' },
  {
    id: 'set-months',
    type: 'number',
    prompt: 'How many months back should the first export go?',
    asker: 'pieter',
    askedLine: 'asked during run #4 · 2 min ago',
    metaShort: 'run #4 · 2 min ago',
    age: '2 min',
    link: 'WORK-412',
    unit: 'months',
    min: 1,
    max: 36,
  },
  {
    id: 'set-cancelled',
    type: 'yesno',
    prompt: 'Can the export include cancelled invoices?',
    asker: 'pieter',
    askedLine: 'asked during run #4 · 2 min ago',
    metaShort: 'run #4 · 2 min ago',
    age: '2 min',
    link: 'WORK-412',
  },
];
export const SET_TABS = ['Confirm facts', 'Months back', 'Include cancelled?'];
export const SET_TITLE = 'Three things before I build the CSV writer';

/** Questions on other tabs of the Questions page. Static rows: they show how the tabs split the work. */
export interface StaticRow {
  type: QType;
  prompt: string;
  link: LinkId | 'WORK-415' | 'WORK-421' | 'WORK-399';
  who: PersonId;
  right: string;
  rightTone?: 'warning' | 'danger';
  badge?: string;
  answered?: string;
}
export const STATIC_TABS: Record<'work' | 'mine' | 'all' | 'answered', StaticRow[]> = {
  work: [
    { type: 'single', prompt: 'Should VAT round per invoice line or on the invoice total?', link: 'WORK-412', who: 'pieter', right: 'Due today', rightTone: 'warning', badge: 'Blocks work' },
    { type: 'number', prompt: 'How many months of history should the first export cover?', link: 'WORK-412', who: 'sarah', right: '1 h' },
    { type: 'approval', prompt: 'Approve the column layout before I build it?', link: 'WORK-412', who: 'pieter', right: 'Just now' },
    { type: 'yesno', prompt: 'May the welcome email link to the pricing page?', link: 'WORK-398', who: 'mark', right: '3 h' },
  ],
  mine: [
    { type: 'single', prompt: 'Which export layout should accountants get?', link: 'WORK-412', who: 'sarah', right: '1 of 2 answered' },
    { type: 'date', prompt: 'Which day can Moneybird review the sample?', link: 'WORK-412', who: 'mark', right: 'Due Fri' },
    { type: 'yesno', prompt: 'Can the runtime update ship before the Dutch launch?', link: 'agent-runtime', who: 'joeri', right: '2 d' },
  ],
  all: [],
  answered: [
    { type: 'single', prompt: 'Which bank export format should the importer read first?', link: 'WORK-412', who: 'brian', right: 'Yesterday', answered: 'CAMT.053' },
    { type: 'yesno', prompt: 'Is the 0% rate needed for reverse charge invoices?', link: 'WORK-412', who: 'brian', right: '2 d', answered: 'Yes' },
    { type: 'text', prompt: 'What subject line should the welcome email use?', link: 'WORK-398', who: 'sarah', right: '3 d', answered: 'Welcome to De App Specialist' },
  ],
};
