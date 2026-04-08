// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { Plus, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { CronExpressionParser } from 'cron-parser';
import dayjs from '@/lib/dayjs';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type Frequency =
  | 'every-minute'
  | 'every-5-min'
  | 'every-15-min'
  | 'every-30-min'
  | 'hourly'
  | 'daily'
  | 'weekday'
  | 'weekly'
  | 'monthly'
  | 'custom';

type ScheduleEntry = {
  frequency: Frequency;
  minute: number;
  hour: number;
  dayOfWeek: number;
  dayOfMonth: number;
  raw: string;
};

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const FREQUENCIES: { value: Frequency; label: string }[] = [
  { value: 'every-minute', label: 'Every minute' },
  { value: 'every-5-min', label: 'Every 5 min' },
  { value: 'every-15-min', label: 'Every 15 min' },
  { value: 'every-30-min', label: 'Every 30 min' },
  { value: 'hourly', label: 'Hourly' },
  { value: 'daily', label: 'Daily' },
  { value: 'weekday', label: 'Weekdays' },
  { value: 'weekly', label: 'Weekly' },
  { value: 'monthly', label: 'Monthly' },
  { value: 'custom', label: 'Custom' },
];

const DAYS_OF_WEEK: { value: string; label: string }[] = [
  { value: '0', label: 'Sunday' },
  { value: '1', label: 'Monday' },
  { value: '2', label: 'Tuesday' },
  { value: '3', label: 'Wednesday' },
  { value: '4', label: 'Thursday' },
  { value: '5', label: 'Friday' },
  { value: '6', label: 'Saturday' },
];

const HOURS = Array.from({ length: 24 }, (_, i) => i);
const MINUTES = Array.from({ length: 60 }, (_, i) => i);
const DAYS_OF_MONTH = Array.from({ length: 31 }, (_, i) => i + 1);

// ---------------------------------------------------------------------------
// Entry helpers
// ---------------------------------------------------------------------------

function defaultEntry(): ScheduleEntry {
  return {
    frequency: 'daily',
    minute: 0,
    hour: 9,
    dayOfWeek: 1,
    dayOfMonth: 1,
    raw: '',
  };
}

const isNum = (s: string | undefined): s is string => /^\d+$/.test(s ?? '');

function parseCronToEntry(expr: string): ScheduleEntry {
  const trimmed = expr.trim();
  const parts = trimmed.split(/\s+/);

  if (parts.length !== 5) {
    return { ...defaultEntry(), frequency: 'custom', raw: trimmed };
  }

  const [min, hour, dom, mon, dow] = parts;

  if (
    min === '*' &&
    hour === '*' &&
    dom === '*' &&
    mon === '*' &&
    dow === '*'
  ) {
    return { ...defaultEntry(), frequency: 'every-minute' };
  }

  if (
    min?.startsWith('*/') &&
    hour === '*' &&
    dom === '*' &&
    mon === '*' &&
    dow === '*'
  ) {
    const n = parseInt(min.slice(2), 10);
    if (n === 5) return { ...defaultEntry(), frequency: 'every-5-min' };
    if (n === 15) return { ...defaultEntry(), frequency: 'every-15-min' };
    if (n === 30) return { ...defaultEntry(), frequency: 'every-30-min' };
  }

  if (isNum(min) && hour === '*' && dom === '*' && mon === '*' && dow === '*') {
    return { ...defaultEntry(), frequency: 'hourly', minute: parseInt(min, 10) };
  }

  if (
    isNum(min) &&
    isNum(hour) &&
    dom === '*' &&
    mon === '*' &&
    dow === '*'
  ) {
    return {
      ...defaultEntry(),
      frequency: 'daily',
      minute: parseInt(min, 10),
      hour: parseInt(hour, 10),
    };
  }

  if (
    isNum(min) &&
    isNum(hour) &&
    dom === '*' &&
    mon === '*' &&
    dow === '1-5'
  ) {
    return {
      ...defaultEntry(),
      frequency: 'weekday',
      minute: parseInt(min, 10),
      hour: parseInt(hour, 10),
    };
  }

  if (
    isNum(min) &&
    isNum(hour) &&
    dom === '*' &&
    mon === '*' &&
    isNum(dow)
  ) {
    return {
      ...defaultEntry(),
      frequency: 'weekly',
      minute: parseInt(min, 10),
      hour: parseInt(hour, 10),
      dayOfWeek: parseInt(dow, 10),
    };
  }

  if (
    isNum(min) &&
    isNum(hour) &&
    isNum(dom) &&
    mon === '*' &&
    dow === '*'
  ) {
    return {
      ...defaultEntry(),
      frequency: 'monthly',
      minute: parseInt(min, 10),
      hour: parseInt(hour, 10),
      dayOfMonth: parseInt(dom, 10),
    };
  }

  return { ...defaultEntry(), frequency: 'custom', raw: trimmed };
}

function entryToCron(entry: ScheduleEntry): string {
  switch (entry.frequency) {
    case 'every-minute':
      return '* * * * *';
    case 'every-5-min':
      return '*/5 * * * *';
    case 'every-15-min':
      return '*/15 * * * *';
    case 'every-30-min':
      return '*/30 * * * *';
    case 'hourly':
      return `${entry.minute} * * * *`;
    case 'daily':
      return `${entry.minute} ${entry.hour} * * *`;
    case 'weekday':
      return `${entry.minute} ${entry.hour} * * 1-5`;
    case 'weekly':
      return `${entry.minute} ${entry.hour} * * ${entry.dayOfWeek}`;
    case 'monthly':
      return `${entry.minute} ${entry.hour} ${entry.dayOfMonth} * *`;
    case 'custom':
      return entry.raw;
  }
}

function nextRunLabel(cronExpr: string): string | null {
  if (!cronExpr.trim()) return null;
  try {
    const interval = CronExpressionParser.parse(cronExpr);
    const next = interval.next();
    return dayjs(next.toDate()).format('ddd, MMM D [at] HH:mm');
  } catch {
    return null;
  }
}

const pad2 = (n: number) => String(n).padStart(2, '0');

// ---------------------------------------------------------------------------
// Components
// ---------------------------------------------------------------------------

type CronScheduleInputProps = {
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
};

export function CronScheduleInput({
  value,
  onChange,
  disabled,
}: CronScheduleInputProps) {
  const entries = React.useMemo(() => {
    const lines = value
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean);
    return lines.map(parseCronToEntry);
  }, [value]);

  function update(index: number, patch: Partial<ScheduleEntry>) {
    const next = entries.map((e, i) =>
      i === index ? { ...e, ...patch } : e
    );
    onChange(next.map(entryToCron).join('\n'));
  }

  function add() {
    onChange(
      [...entries.map(entryToCron), entryToCron(defaultEntry())].join('\n')
    );
  }

  function remove(index: number) {
    onChange(
      entries
        .filter((_, i) => i !== index)
        .map(entryToCron)
        .join('\n')
    );
  }

  return (
    <div className="space-y-2">
      {entries.map((entry, i) => (
        <ScheduleRow
          key={i}
          entry={entry}
          onChange={(patch) => update(i, patch)}
          onRemove={() => remove(i)}
          disabled={disabled}
        />
      ))}
      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={add}
        disabled={disabled}
        className="h-7 text-xs"
      >
        <Plus className="mr-1 size-3" />
        Add schedule
      </Button>
    </div>
  );
}

function ScheduleRow({
  entry,
  onChange,
  onRemove,
  disabled,
}: {
  entry: ScheduleEntry;
  onChange: (patch: Partial<ScheduleEntry>) => void;
  onRemove: () => void;
  disabled?: boolean;
}) {
  const needsHour = ['daily', 'weekday', 'weekly', 'monthly'].includes(
    entry.frequency
  );
  const needsMinute = [
    'hourly',
    'daily',
    'weekday',
    'weekly',
    'monthly',
  ].includes(entry.frequency);
  const needsDow = entry.frequency === 'weekly';
  const needsDom = entry.frequency === 'monthly';
  const isCustom = entry.frequency === 'custom';

  const cronExpr = entryToCron(entry);
  const nextRun =
    isCustom && !entry.raw ? null : nextRunLabel(cronExpr);

  return (
    <div className="space-y-0.5">
      <div className="flex items-center gap-1.5 flex-wrap">
        {/* Frequency */}
        <Select
          value={entry.frequency}
          onValueChange={(v) => onChange({ frequency: v as Frequency })}
          disabled={disabled}
        >
          <SelectTrigger className="w-[130px]" size="sm">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {FREQUENCIES.map((f) => (
              <SelectItem key={f.value} value={f.value}>
                {f.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        {/* Day of week (weekly) */}
        {needsDow && (
          <>
            <span className="text-xs text-muted-foreground">on</span>
            <Select
              value={String(entry.dayOfWeek)}
              onValueChange={(v) =>
                onChange({ dayOfWeek: parseInt(v, 10) })
              }
              disabled={disabled}
            >
              <SelectTrigger className="w-[110px]" size="sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DAYS_OF_WEEK.map((d) => (
                  <SelectItem key={d.value} value={d.value}>
                    {d.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </>
        )}

        {/* Day of month (monthly) */}
        {needsDom && (
          <>
            <span className="text-xs text-muted-foreground">on day</span>
            <Select
              value={String(entry.dayOfMonth)}
              onValueChange={(v) =>
                onChange({ dayOfMonth: parseInt(v, 10) })
              }
              disabled={disabled}
            >
              <SelectTrigger className="w-[56px]" size="sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {DAYS_OF_MONTH.map((d) => (
                  <SelectItem key={d} value={String(d)}>
                    {d}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </>
        )}

        {/* Hour selector */}
        {needsHour && (
          <>
            <span className="text-xs text-muted-foreground">at</span>
            <Select
              value={String(entry.hour)}
              onValueChange={(v) => onChange({ hour: parseInt(v, 10) })}
              disabled={disabled}
            >
              <SelectTrigger className="w-[56px]" size="sm">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {HOURS.map((h) => (
                  <SelectItem key={h} value={String(h)}>
                    {pad2(h)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <span className="text-xs text-muted-foreground">:</span>
          </>
        )}

        {/* "at :" label for hourly (minute only) */}
        {needsMinute && !needsHour && (
          <span className="text-xs text-muted-foreground">at :</span>
        )}

        {/* Minute selector */}
        {needsMinute && (
          <Select
            value={String(entry.minute)}
            onValueChange={(v) => onChange({ minute: parseInt(v, 10) })}
            disabled={disabled}
          >
            <SelectTrigger className="w-[56px]" size="sm">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {MINUTES.map((m) => (
                <SelectItem key={m} value={String(m)}>
                  {pad2(m)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        {/* Custom cron input */}
        {isCustom && (
          <Input
            value={entry.raw}
            onChange={(e) => onChange({ raw: e.target.value })}
            placeholder="0 */2 * * 1-5"
            className="h-6 w-[160px] text-xs font-mono"
            disabled={disabled}
          />
        )}

        {/* Remove */}
        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={onRemove}
          disabled={disabled}
          className="h-6 w-6 p-0 text-muted-foreground hover:text-destructive"
        >
          <X className="size-3" />
        </Button>
      </div>

      {/* Cron expression + next run preview */}
      <div className="flex items-center gap-1.5 text-[11px] text-muted-foreground pl-0.5">
        {!isCustom && (
          <code className="font-mono opacity-60">{cronExpr}</code>
        )}
        {nextRun && (
          <span className={!isCustom ? 'before:content-["·"] before:mr-1.5' : ''}>
            next: {nextRun}
          </span>
        )}
      </div>
    </div>
  );
}
