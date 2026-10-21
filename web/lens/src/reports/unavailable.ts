import type { GroupUnavailable } from '../contract/types'

type Translate = (key: string, fallback: string, vars?: Record<string, string | number>) => string

/** The explanation shown, and printed, in place of an unavailable placement's figures. */
export function reportUnavailableCopy(reason: GroupUnavailable, translate: Translate): string {
  switch (reason) {
    case 'report-deleted':
      return translate('report.unavailable.deleted', 'This report was deleted. Remove the card or place another report here.')
    case 'report-forbidden':
      return translate('report.unavailable.forbidden', 'You do not have access to this report.')
    case 'report-incompatible':
      return translate('report.version.incompatible', 'This report is unavailable: its definition no longer matches the data source. Ask the report owner to update it.')
  }
}

/**
 * The heading of an unavailable placement. A forbidden one never carries its
 * caption on the wire, so it is named generically rather than by its id.
 */
export function reportUnavailableTitle(reason: GroupUnavailable, label: string | undefined, translate: Translate): string {
  if (reason !== 'report-forbidden' && label) return label
  return translate('report.unavailable.title', 'Unavailable report')
}

/** The data-state a placement card exposes for each reason. */
export function reportUnavailableState(reason: GroupUnavailable): 'deleted' | 'forbidden' | 'incompatible' {
  return reason === 'report-deleted' ? 'deleted' : reason === 'report-forbidden' ? 'forbidden' : 'incompatible'
}
