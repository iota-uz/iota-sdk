import type { DashboardDocument, LayoutGroup, LayoutItem, Panel } from '../contract'
import { resolveQuality, type QualityInput } from '../panels/QualityChip'
import type { PrintReport, PrintSection } from '../runtime/print'
import { chartKinds, frameSignature, sectionPanel } from './values'

/**
 * A figure printed in a chapter body: the dashboard's own reading of a panel,
 * numbered so the text can point at it.
 */
export interface PrintFigure {
  section: PrintSection
  /** "2.1" — chapter number, then position inside the chapter. */
  number: string
  /** Rendered as a chart, rather than as evidence rows only. */
  chart: boolean
  /**
   * A single value: printed as a metric tile. A chart of one point and a table
   * of one row are both worse than the number itself.
   */
  metric: boolean
  width: 'half' | 'full'
  /** The metric strip this reading was authored inside, when it was. */
  group?: { id: string; label?: string; caption?: string }
  /**
   * A headline reading the cover lifts into its "at a glance" strip. It is not
   * printed a second time in the chapter body.
   */
  kpi: boolean
  /**
   * What the dashboard shows when this reading is clicked: the numerator and
   * denominator of a ratio, the components of a total. It prints with the
   * figure it explains, not as loose fragments at the back of the report.
   */
  breakdown: Array<PrintSection>
}

/** A drill reading, printed as a dense table in the detail appendix. */
export interface PrintDetail {
  section: PrintSection
  /** "A2.3" — appendix, owning chapter, position. */
  number: string
  /** Where the reading sits in the dashboard's drill graph. */
  trail: string
}

export interface PrintChapter {
  id: string
  number: number
  title: string
  caption?: string
  figures: Array<PrintFigure>
  details: Array<PrintDetail>
}

/** An entry of the closing methodology appendix. */
export interface PrintNote {
  id: string
  label: string
  detail?: string
}

export interface PrintOutline {
  chapters: Array<PrintChapter>
  /** Chapters that carry detail readings, in chapter order. */
  appendix: Array<PrintChapter>
  /** Headline readings printed on the cover instead of inside a chapter. */
  kpis: Array<PrintFigure>
  notes: Array<PrintNote>
  /** Readings that resolved to nothing, disclosed instead of printed empty. */
  missing: Array<string>
  /**
   * Printed readings that are an estimate, a proxy or an unreconciled figure.
   * The cover states the count, because "how much of this is settled" is the
   * first thing a reader of a management report is entitled to know.
   */
  estimated: number
  /**
   * Every quality the document declares anywhere — panel, formula stage,
   * hierarchy row, relationship end. The closing appendix defines the words
   * this report actually used, and only those.
   */
  qualities: Array<QualityInput>
  figureCount: number
  detailCount: number
}

/** Every quality declaration in the document, wherever it was authored. */
function documentQualities(document: DashboardDocument): Array<QualityInput> {
  const qualities: Array<QualityInput> = []
  for (const panel of document.panels) {
    qualities.push(panel)
    for (const stage of panel.metricFlow?.stages ?? []) qualities.push(stage)
    for (const row of panel.metricHierarchy?.rows ?? []) qualities.push(row)
    if (panel.metricRelationship) {
      qualities.push(panel.metricRelationship.source, panel.metricRelationship.target)
    }
  }
  return qualities
}

/** Panel kinds whose single reading can stand as a cover headline. */
const kpiKinds = new Set<Panel['kind']>(['stat', 'metric_relationship'])

function widthFor(panel: Panel): 'half' | 'full' {
  if (panel.kind === 'stat' || panel.kind === 'metric_relationship') return 'half'
  if (panel.kind === 'pie' || panel.kind === 'donut' || panel.kind === 'radial') return 'half'
  return 'full'
}

/**
 * The strip a panel was authored inside — the one that names and explains this
 * run of metrics. A dashboard nests groups; the innermost one is the label the
 * reader sees directly above the numbers.
 */
function innermostGroup(item: LayoutItem): LayoutGroup | undefined {
  const groups = item.groups ?? (item.group ? [item.group] : [])
  return [...groups].reverse().find((group) => group.label || group.caption)
}

function chapterTitle(index: number, heading: string | undefined, fallback: string): string {
  const title = heading?.trim()
  if (title) return title
  return index === 0 ? fallback : ''
}

/**
 * Groups the flat walk of the drill graph into the document's own chapters.
 *
 * `buildPrintReport` answers "what readings exist"; the printed document also
 * needs "in what order does a person read them". That order is already
 * authored: `layout.rows` carries the headings, spans and group captions the
 * dashboard itself is read by. Following it turns a numbered dump into a
 * document — and keeps the two in step, because a re-authored dashboard
 * re-authors its report.
 */
export function buildOutline(
  report: PrintReport,
  fallbackTitle: string,
  otherTitle: string,
): PrintOutline {
  const document: DashboardDocument = report.document
  const chapters: Array<PrintChapter> = []
  const missing: Array<string> = []
  const printed = new Map<string, string>()
  const claimed = new Set<PrintSection>()
  let liftedKpis = 0

  const rootSections = new Map<string, Array<PrintSection>>()
  const detailSections = new Map<string, Array<PrintSection>>()
  // A reading reached by clicking a KPI is that KPI's explanation. Kept with
  // the tile it explains it is a formula; filed at the back of the report it is
  // five one-row tables called «Итог» and «Числитель — Разбивка».
  const ownedSections = new Map<string, Array<PrintSection>>()
  for (const section of report.sections) {
    if (section.owner) {
      const owned = ownedSections.get(section.owner) ?? []
      owned.push(section)
      ownedSections.set(section.owner, owned)
      continue
    }
    const bucket = section.root ? rootSections : detailSections
    const list = bucket.get(section.panel.id) ?? []
    list.push(section)
    bucket.set(section.panel.id, list)
  }

  const accept = (section: PrintSection, kind: 'figure' | 'detail'): boolean => {
    const panel = sectionPanel(section)
    const signature = frameSignature(section.frame)
    if (!signature) {
      // An empty reading is disclosed once by name instead of printing a card
      // that says "no data" in the middle of the argument.
      missing.push(panel.title)
      return false
    }
    const owner = printed.get(signature)
    // A drill level that restates numbers already printed anywhere adds only
    // pages. Two authored panels may legitimately agree on a figure, so a body
    // figure only yields to its own panel's earlier reading.
    if (owner !== undefined && (kind === 'detail' || owner === section.panel.id)) return false
    if (owner === undefined) printed.set(signature, section.panel.id)
    return true
  }

  const chapterFor = (title: string, caption: string | undefined): PrintChapter => {
    const last = chapters[chapters.length - 1]
    if (!title && last) return last
    if (last && last.title === title) {
      if (!last.caption && caption) last.caption = caption
      return last
    }
    const chapter: PrintChapter = {
      id: `chapter-${chapters.length + 1}`,
      number: chapters.length + 1,
      title,
      ...(caption ? { caption } : {}),
      figures: [],
      details: [],
    }
    chapters.push(chapter)
    return chapter
  }

  const addFigure = (chapter: PrintChapter, section: PrintSection, group?: LayoutGroup) => {
    if (!accept(section, 'figure')) return
    const panel = sectionPanel(section)
    const single = (section.frame?.rows.length ?? 0) <= 1
    const breakdown = (ownedSections.get(section.panel.id) ?? []).filter((owned) => accept(owned, 'detail'))
    chapter.figures.push({
      section,
      number: `${chapter.number}.${chapter.figures.length + 1}`,
      chart: chartKinds.has(panel.kind) && !single,
      metric: single,
      // A reading that prints its calculation needs the width to hold it; half
      // a page beside another figure is where the two collide.
      width: breakdown.length > 0 ? 'full' : single ? 'half' : widthFor(panel),
      ...(group && (group.label || group.caption)
        ? { group: { id: group.id, ...(group.label ? { label: group.label } : {}), ...(group.caption ? { caption: group.caption } : {}) } }
        : {}),
      // The opening chapter's single-value readings are the dashboard's
      // headline; the cover carries them so page one is already informative.
      kpi: chapter.number === 1 && single && kpiKinds.has(panel.kind) && liftedKpis++ < 6,
      breakdown,
    })
  }

  const addDetail = (chapter: PrintChapter, section: PrintSection) => {
    if (!accept(section, 'detail')) return
    chapter.details.push({
      section,
      number: `A${chapter.number}.${chapter.details.length + 1}`,
      trail: section.breadcrumb
        .filter((part, index, all) => part && part !== all[index - 1])
        .join(' › '),
    })
  }

  const addPanel = (chapter: PrintChapter, panelId: string, group?: LayoutGroup) => {
    for (const section of rootSections.get(panelId) ?? []) {
      claimed.add(section)
      addFigure(chapter, section, group)
    }
    for (const section of detailSections.get(panelId) ?? []) {
      claimed.add(section)
      addDetail(chapter, section)
    }
  }

  for (const row of document.layout.rows) {
    const heading = chapterTitle(chapters.length, row.heading, fallbackTitle)
    const caption = row.panels
      .flatMap((item) => item.groups ?? (item.group ? [item.group] : []))
      .map((group) => group.caption?.trim())
      .find((value) => Boolean(value))
    const chapter = chapterFor(heading, caption)
    for (const item of row.panels) addPanel(chapter, item.panelId, innermostGroup(item))
  }

  // Panels the layout never places — and readings reached through a drawer,
  // which belong to no row — still have to appear somewhere.
  const orphans = report.sections.filter((section) => !claimed.has(section))
  if (orphans.length > 0) {
    const chapter = chapterFor(otherTitle, undefined)
    for (const section of orphans) {
      if (section.root) addFigure(chapter, section)
      else addDetail(chapter, section)
    }
  }

  // Headline readings are printed on the cover, so a chapter that held nothing
  // else is not a chapter of the report. Numbering is assigned only now, over
  // what will actually be printed, so the contents page and the pages agree.
  const kpis = chapters.flatMap((chapter) => chapter.figures.filter(({ kpi }) => kpi))
  const populated = chapters
    .map((chapter) => ({ ...chapter, figures: chapter.figures.filter(({ kpi }) => !kpi) }))
    .filter((chapter) => chapter.figures.length > 0 || chapter.details.length > 0)
  populated.forEach((chapter, index) => {
    chapter.number = index + 1
    chapter.figures.forEach((figure, position) => {
      figure.number = `${chapter.number}.${position + 1}`
    })
    chapter.details.forEach((detail, position) => {
      detail.number = `A${chapter.number}.${position + 1}`
    })
  })
  const qualified = [...kpis, ...populated.flatMap((chapter) => chapter.figures)]
    .filter(({ section }) => {
      const panel = sectionPanel(section)
      const quality = resolveQuality(panel)
      return quality !== undefined && quality.value !== 'verified' && quality.value !== 'calculated'
    })
  return {
    chapters: populated,
    appendix: populated.filter((chapter) => chapter.details.length > 0),
    kpis,
    notes: buildNotes(document),
    missing,
    estimated: qualified.length,
    qualities: documentQualities(document),
    figureCount: populated.reduce((sum, chapter) => sum + chapter.figures.length, 0),
    detailCount: populated.reduce((sum, chapter) => sum + chapter.details.length, 0),
  }
}

/**
 * The methodology appendix: what the dashboard itself says about how a number
 * was obtained — panel status chips, relationship notes and stage captions.
 * On screen these are chips and tooltips; on paper they are the only place a
 * reader can check what a figure is allowed to mean.
 */
function buildNotes(document: DashboardDocument): Array<PrintNote> {
  const notes: Array<PrintNote> = []
  const seen = new Set<string>()
  const add = (label: string, detail?: string) => {
    const key = `${label}::${detail ?? ''}`
    if (!label.trim() || seen.has(key)) return
    seen.add(key)
    notes.push({ id: `note-${notes.length + 1}`, label, ...(detail ? { detail } : {}) })
  }
  for (const panel of document.panels) {
    if (panel.status?.label) add(panel.title, panel.status.label)
    if (panel.metricRelationship?.note) add(panel.title, panel.metricRelationship.note)
    for (const stage of panel.metricFlow?.stages ?? []) {
      if (stage.caption) add(`${panel.title} · ${stage.label}`, stage.caption)
    }
  }
  return notes
}
