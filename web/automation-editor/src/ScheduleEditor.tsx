// Recurrence editor of a schedule trigger: cron with presets, time zone, missed_after and a preview
// of the next five runs. Params follow ScheduleDef of #1326: { cron, timezone?, missed_after? }.
import { Combobox } from '@iota-uz/sdk/solid-ui'
import { createEffect, createMemo, createSignal, createUniqueId, For, on, Show } from 'solid-js'
import {
  DEFAULT_CRON, detectPreset, isDuration, nextRuns, parseCron, presetCron, timeZones,
  type CronError, type CronFieldName, type Preset, type PresetKind,
} from './cron'
import { intlLocale, pick } from './locale'
import type { EditorLocale } from './types'

export interface ScheduleEditorProps {
  params: Record<string, unknown>
  onChange(params: Record<string, unknown>): void
  readOnly: boolean
  locale: EditorLocale
  now?: () => Date
}

type Strings = {
  repeats: string
  kinds: Record<PresetKind, string>
  time: string
  weekday: string
  weekdays: string[]
  day: string
  dayHint: string
  cron: string
  cronHint: string
  fields: Record<CronFieldName, string>
  cronError: (error: CronError, fields: Record<CronFieldName, string>) => string
  timezone: string
  tzNotFound: string
  missedAfter: string
  missedHint: string
  missedError: string
  preview: string
  noRuns: string
}

const STRINGS: Record<EditorLocale, Strings> = {
  en: {
    repeats: 'Repeats',
    kinds: { daily: 'Every day', weekdays: 'Weekdays', weekly: 'Weekly', monthly: 'Monthly', custom: 'Custom cron' },
    time: 'Time',
    weekday: 'Day of the week',
    weekdays: ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'],
    day: 'Day of the month',
    dayHint: 'Months without this day are skipped.',
    cron: 'Cron expression',
    cronHint: 'Five fields: minute hour day month weekday.',
    fields: { minute: 'minute', hour: 'hour', day: 'day', month: 'month', weekday: 'weekday' },
    cronError: (e, f) => e.code === 'fields'
      ? `Expected 5 fields, got ${e.count}.`
      : e.code === 'range'
        ? `${e.value} is out of range for the ${f[e.field]} field (${e.min}–${e.max}).`
        : `“${e.value}” is not a valid ${f[e.field]} value.`,
    timezone: 'Time zone',
    tzNotFound: 'No time zone matches.',
    missedAfter: 'Skip a missed run after',
    missedHint: '30s, 20m, 2h or 7d, at most 30 days. Empty: the scheduler default.',
    missedError: 'Use a number with s, m, h or d, at most 30 days: 30s, 20m, 2h, 7d.',
    preview: 'Next runs',
    noRuns: 'This schedule has no runs in the coming years.',
  },
  ru: {
    repeats: 'Повтор',
    kinds: { daily: 'Каждый день', weekdays: 'По будням', weekly: 'Раз в неделю', monthly: 'Раз в месяц', custom: 'Свой cron' },
    time: 'Время',
    weekday: 'День недели',
    weekdays: ['Вс', 'Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб'],
    day: 'День месяца',
    dayHint: 'Месяцы без этого дня пропускаются.',
    cron: 'Выражение cron',
    cronHint: 'Пять полей: минута час день месяц день-недели.',
    fields: { minute: 'минуты', hour: 'часа', day: 'дня', month: 'месяца', weekday: 'дня недели' },
    cronError: (e, f) => e.code === 'fields'
      ? `Нужно 5 полей, указано ${e.count}.`
      : e.code === 'range'
        ? `${e.value} вне диапазона ${f[e.field]} (${e.min}–${e.max}).`
        : `«${e.value}» — недопустимое значение ${f[e.field]}.`,
    timezone: 'Часовой пояс',
    tzNotFound: 'Такого часового пояса нет.',
    missedAfter: 'Пропускать запуск, опоздавший больше чем на',
    missedHint: '30s, 20m, 2h или 7d, не больше 30 дней. Пусто — по умолчанию планировщика.',
    missedError: 'Число с s, m, h или d, не больше 30 дней: 30s, 20m, 2h, 7d.',
    preview: 'Ближайшие запуски',
    noRuns: 'В ближайшие годы по этому расписанию запусков нет.',
  },
  uz: {
    repeats: 'Takrorlash',
    kinds: { daily: 'Har kuni', weekdays: 'Ish kunlari', weekly: 'Haftada bir', monthly: 'Oyda bir', custom: 'Oʻz cron' },
    time: 'Vaqt',
    weekday: 'Hafta kuni',
    weekdays: ['Ya', 'Du', 'Se', 'Ch', 'Pa', 'Ju', 'Sh'],
    day: 'Oy kuni',
    dayHint: 'Bu kuni boʻlmagan oylar oʻtkazib yuboriladi.',
    cron: 'Cron ifodasi',
    cronHint: 'Beshta maydon: daqiqa soat kun oy hafta-kuni.',
    fields: { minute: 'daqiqa', hour: 'soat', day: 'kun', month: 'oy', weekday: 'hafta kuni' },
    cronError: (e, f) => e.code === 'fields'
      ? `5 ta maydon kerak, ${e.count} ta berilgan.`
      : e.code === 'range'
        ? `${e.value} ${f[e.field]} oraligʻidan tashqarida (${e.min}–${e.max}).`
        : `«${e.value}» — ${f[e.field]} uchun notoʻgʻri qiymat.`,
    timezone: 'Vaqt mintaqasi',
    tzNotFound: 'Bunday vaqt mintaqasi yoʻq.',
    missedAfter: 'Kechikkan ishga tushirishni oʻtkazib yuborish',
    missedHint: '30s, 20m, 2h yoki 7d, koʻpi bilan 30 kun. Boʻsh — rejalashtiruvchi standarti.',
    missedError: 's, m, h yoki d bilan son, koʻpi bilan 30 kun: 30s, 20m, 2h, 7d.',
    preview: 'Keyingi ishga tushirishlar',
    noRuns: 'Bu jadval boʻyicha yaqin yillarda ishga tushirish yoʻq.',
  },
  'uz-Cyrl': {
    repeats: 'Такрорлаш',
    kinds: { daily: 'Ҳар куни', weekdays: 'Иш кунлари', weekly: 'Ҳафтада бир', monthly: 'Ойда бир', custom: 'Ўз cron' },
    time: 'Вақт',
    weekday: 'Ҳафта куни',
    weekdays: ['Якш', 'Душ', 'Сеш', 'Чор', 'Пай', 'Жум', 'Шан'],
    day: 'Ой куни',
    dayHint: 'Бу куни бўлмаган ойлар ўтказиб юборилади.',
    cron: 'Cron ифодаси',
    cronHint: 'Бешта майдон: дақиқа соат кун ой ҳафта-куни.',
    fields: { minute: 'дақиқа', hour: 'соат', day: 'кун', month: 'ой', weekday: 'ҳафта куни' },
    cronError: (e, f) => e.code === 'fields'
      ? `5 та майдон керак, ${e.count} та берилган.`
      : e.code === 'range'
        ? `${e.value} ${f[e.field]} оралиғидан ташқарида (${e.min}–${e.max}).`
        : `«${e.value}» — ${f[e.field]} учун нотўғри қиймат.`,
    timezone: 'Вақт минтақаси',
    tzNotFound: 'Бундай вақт минтақаси йўқ.',
    missedAfter: 'Кечиккан ишга туширишни ўтказиб юбориш',
    missedHint: '30s, 20m, 2h ёки 7d, кўпи билан 30 кун. Бўш — режалаштирувчи стандарти.',
    missedError: 's, m, h ёки d билан сон, кўпи билан 30 кун: 30s, 20m, 2h, 7d.',
    preview: 'Кейинги ишга туширишлар',
    noRuns: 'Бу жадвал бўйича яқин йилларда ишга тушириш йўқ.',
  },
}

const KINDS: PresetKind[] = ['daily', 'weekdays', 'weekly', 'monthly', 'custom']
/** Monday first in the picker; values stay cron weekdays (0 = Sunday). */
const WEEK_ORDER = [1, 2, 3, 4, 5, 6, 0]

const pad = (n: number) => String(n).padStart(2, '0')

export function ScheduleEditor(props: ScheduleEditorProps) {
  const t = () => pick(STRINGS, props.locale)
  const uid = createUniqueId()
  const cron = () => (typeof props.params.cron === 'string' ? props.params.cron : DEFAULT_CRON)
  const zone = () => (typeof props.params.timezone === 'string' && props.params.timezone ? props.params.timezone : 'UTC')
  const missed = () => (typeof props.params.missed_after === 'string' ? props.params.missed_after : '')

  const [kind, setKind] = createSignal<PresetKind>(detectPreset(cron()).kind)
  // An outside change (undo, another field) re-detects the preset; choosing "custom" keeps it custom.
  createEffect(on(cron, (value) => {
    const detected = detectPreset(value).kind
    if (kind() !== 'custom' || detected === 'custom') setKind(detected)
  }, { defer: true }))

  const preset = createMemo(() => detectPreset(cron()))
  const parsed = createMemo(() => parseCron(cron()))
  const cronError = () => {
    const result = parsed()
    return result.ok ? null : t().cronError(result.error, t().fields)
  }
  const [missedDraft, setMissedDraft] = createSignal(missed())
  createEffect(on(missed, (value) => setMissedDraft(value), { defer: true }))
  const missedInvalid = () => missedDraft() !== '' && !isDuration(missedDraft())

  const runs = createMemo(() => nextRuns(cron(), zone(), props.now?.() ?? new Date(), 5))
  const runFormat = createMemo(() => new Intl.DateTimeFormat(intlLocale(props.locale), {
    timeZone: zone(), weekday: 'short', day: 'numeric', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hourCycle: 'h23',
  }))

  const zoneOptions = createMemo(() => timeZones().map((value) => ({ value, label: value.replaceAll('_', ' ') })))

  const emit = (patch: Record<string, unknown>) => {
    const next: Record<string, unknown> = { ...props.params, ...patch }
    for (const [key, value] of Object.entries(next)) if (value === undefined) delete next[key]
    props.onChange(next)
  }
  const emitPreset = (next: Partial<Preset> & { kind: PresetKind }) => {
    const base = preset()
    const value = presetCron({ hour: base.hour, minute: base.minute, weekday: base.weekday ?? 1, day: base.day ?? 1, ...next })
    if (value !== null && value !== cron()) emit({ cron: value })
  }
  const chooseKind = (next: PresetKind) => {
    if (props.readOnly) return
    setKind(next)
    if (next !== 'custom') emitPreset({ kind: next })
  }
  const onKindKey = (event: KeyboardEvent, index: number) => {
    const delta = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : event.key === 'ArrowLeft' || event.key === 'ArrowUp' ? -1 : 0
    if (!delta) return
    event.preventDefault()
    const next = KINDS[(index + delta + KINDS.length) % KINDS.length]!
    chooseKind(next)
    const group = (event.currentTarget as HTMLElement).parentElement
    group?.querySelectorAll<HTMLButtonElement>('[role="radio"]')[KINDS.indexOf(next)]?.focus()
  }
  const onTime = (value: string) => {
    const match = /^(\d{1,2}):(\d{2})$/.exec(value)
    if (!match) return
    emitPreset({ kind: kind(), hour: Math.min(23, Number(match[1])), minute: Math.min(59, Number(match[2])) })
  }

  return (
    <div class="ae-sched" data-readonly={props.readOnly || undefined}>
      <fieldset class="ae-sched-group" disabled={props.readOnly}>
        <legend class="ae-sched-label">{t().repeats}</legend>
        <div class="ae-sched-segments" role="radiogroup" aria-label={t().repeats}>
          <For each={KINDS}>{(item, index) => (
            <button
              type="button"
              role="radio"
              class="ae-sched-segment"
              aria-checked={kind() === item}
              tabindex={kind() === item ? 0 : -1}
              disabled={props.readOnly}
              onClick={() => chooseKind(item)}
              onKeyDown={(event) => onKindKey(event, index())}
            >{t().kinds[item]}</button>
          )}</For>
        </div>
      </fieldset>

      <Show when={kind() !== 'custom'}>
        <div class="ae-sched-row">
          <div class="ae-sched-field">
            <label class="ae-sched-label" for={`${uid}-time`}>{t().time}</label>
            <input
              id={`${uid}-time`}
              class="ae-sched-input ae-sched-time"
              type="time"
              step="60"
              value={`${pad(preset().hour)}:${pad(preset().minute)}`}
              disabled={props.readOnly}
              onChange={(event) => onTime(event.currentTarget.value)}
            />
          </div>
          <Show when={kind() === 'monthly'}>
            <div class="ae-sched-field">
              <label class="ae-sched-label" for={`${uid}-day`}>{t().day}</label>
              <input
                id={`${uid}-day`}
                class="ae-sched-input ae-sched-day"
                type="number"
                min="1"
                max="31"
                inputmode="numeric"
                value={preset().day ?? 1}
                disabled={props.readOnly}
                aria-describedby={(preset().day ?? 1) > 28 ? `${uid}-day-hint` : undefined}
                onChange={(event) => {
                  const day = Math.round(Number(event.currentTarget.value))
                  if (day >= 1 && day <= 31) emitPreset({ kind: 'monthly', day })
                }}
              />
            </div>
          </Show>
        </div>
        <Show when={kind() === 'monthly' && (preset().day ?? 1) > 28}>
          <p id={`${uid}-day-hint`} class="ae-sched-hint">{t().dayHint}</p>
        </Show>
        <Show when={kind() === 'weekly'}>
          <div class="ae-sched-group">
            <span class="ae-sched-label" id={`${uid}-weekday`}>{t().weekday}</span>
            <div class="ae-sched-days" role="radiogroup" aria-labelledby={`${uid}-weekday`}>
              <For each={WEEK_ORDER}>{(day) => (
                <button
                  type="button"
                  role="radio"
                  class="ae-sched-day-option"
                  aria-checked={(preset().weekday ?? 1) === day}
                  disabled={props.readOnly}
                  onClick={() => emitPreset({ kind: 'weekly', weekday: day })}
                >{t().weekdays[day]}</button>
              )}</For>
            </div>
          </div>
        </Show>
      </Show>

      <Show when={kind() === 'custom'}>
        <div class="ae-sched-field">
          <label class="ae-sched-label" for={`${uid}-cron`}>{t().cron}</label>
          <input
            id={`${uid}-cron`}
            class="ae-sched-input ae-sched-cron"
            type="text"
            spellcheck={false}
            autocomplete="off"
            value={cron()}
            disabled={props.readOnly}
            aria-invalid={cronError() ? true : undefined}
            aria-describedby={`${uid}-cron-hint${cronError() ? ` ${uid}-cron-error` : ''}`}
            onInput={(event) => emit({ cron: event.currentTarget.value })}
          />
          <span id={`${uid}-cron-hint`} class="ae-sched-hint">{t().cronHint}</span>
        </div>
      </Show>
      <Show when={cronError()}>{(message) => (
        <p id={`${uid}-cron-error`} class="ae-sched-error" role="alert">{message()}</p>
      )}</Show>

      <div class="ae-sched-field">
        <span class="ae-sched-label" id={`${uid}-tz`}>{t().timezone}</span>
        <Combobox
          aria-labelledby={`${uid}-tz`}
          class="ae-sched-zone"
          options={zoneOptions()}
          value={zone()}
          searchable
          readOnly={props.readOnly}
          labels={{ notFound: t().tzNotFound }}
          onValueChange={(value) => {
            const next = Array.isArray(value) ? value[0] : value
            if (next && next !== zone()) emit({ timezone: next })
          }}
        />
      </div>

      <div class="ae-sched-field">
        <label class="ae-sched-label" for={`${uid}-missed`}>{t().missedAfter}</label>
        <input
          id={`${uid}-missed`}
          class="ae-sched-input ae-sched-missed"
          type="text"
          spellcheck={false}
          autocomplete="off"
          placeholder="2h"
          value={missedDraft()}
          disabled={props.readOnly}
          aria-invalid={missedInvalid() ? true : undefined}
          aria-describedby={`${uid}-missed-hint${missedInvalid() ? ` ${uid}-missed-error` : ''}`}
          onInput={(event) => {
            const value = event.currentTarget.value.trim()
            setMissedDraft(value)
            emit({ missed_after: value === '' ? undefined : value })
          }}
        />
        <span id={`${uid}-missed-hint`} class="ae-sched-hint">{t().missedHint}</span>
      </div>
      <Show when={missedInvalid()}>
        <p id={`${uid}-missed-error`} class="ae-sched-error" role="alert">{t().missedError}</p>
      </Show>

      <section class="ae-sched-preview" aria-labelledby={`${uid}-preview`}>
        <h4 class="ae-sched-label" id={`${uid}-preview`}>{t().preview}</h4>
        <Show when={parsed().ok}>
          <Show when={runs().length > 0} fallback={<p class="ae-sched-hint">{t().noRuns}</p>}>
            <ol class="ae-sched-runs">
              <For each={runs()}>{(run) => (
                <li class="ae-sched-run"><time datetime={run.toISOString()}>{runFormat().format(run)}</time></li>
              )}</For>
            </ol>
          </Show>
        </Show>
      </section>
    </div>
  )
}
