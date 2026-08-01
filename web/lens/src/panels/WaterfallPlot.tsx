import type { CSSProperties, ReactNode } from 'react'
import type { WaterfallItem, WaterfallModel } from './CascadePanel'

/** Chrome a host wraps around a single column, today: activation for a drill. */
export type WaterfallInteraction = (item: WaterfallItem, index: number) => Record<string, unknown> | undefined

/**
 * When the split band names itself. The band's own colour already says a part of
 * the movement differs in kind; its chip answers a follow-up question — how much,
 * and called what — and standing permanently beside the bar it spends the plot's
 * scarcest space (the gutter the neighbouring columns print their totals in) on
 * an answer nobody asked for yet. `hover` holds it until the reader points at
 * that column. A sheet of paper has no pointer, so the printed report keeps it
 * standing.
 */
export type WaterfallSplitCallout = 'hover' | 'always'

export interface WaterfallPlotProps {
  model: WaterfallModel
  label: string
  interaction?: WaterfallInteraction
  /** A grouping role when the columns are activatable; an image otherwise. */
  role?: 'group' | 'img'
  /** Defaults to `hover`; a static rendering must pass `always`. */
  splitCallout?: WaterfallSplitCallout
  children?: ReactNode
}

/**
 * The waterfall's markup, shared by the interactive panel and the printed
 * report. Everything positional lives in the model, so the same DOM serves a
 * clickable dashboard column and an inert printed one — the printed report gets
 * the real bridge instead of a table of the same numbers.
 */
export function WaterfallPlot({
  model,
  label,
  interaction,
  role = 'img',
  splitCallout = 'hover',
  children,
}: WaterfallPlotProps) {
  return (
    <div
      aria-label={label}
      className="lens-waterfall"
      data-lens-waterfall
      role={role}
      style={{
        '--lens-waterfall-count': model.items.length,
        '--lens-waterfall-zero': `${model.zero}%`,
      } as CSSProperties}
    >
      <div className="lens-waterfall-chart">
        <div className="lens-waterfall-axis" aria-hidden="true">
          {model.ticks.map((tick) => (
            <span key={tick.value} style={{ top: `${tick.top}%` }}>{tick.label}</span>
          ))}
        </div>
        <div className="lens-waterfall-plot">
          {model.ticks.map((tick) => (
            <span
              className="lens-waterfall-gridline"
              key={`grid-${tick.value}`}
              style={{ top: `${tick.top}%` }}
            />
          ))}
          <div className="lens-waterfall-zero" />
          {model.items.map((item, index) => {
            const chrome = interaction?.(item, index)
            // The callout leans away from the nearer plot edge, so a split
            // on the last columns does not run off the chart.
            const calloutSide = index * 2 >= model.items.length ? 'start' : 'end'
            return (
              <div className="lens-waterfall-column" key={`${item.label}-${index}`}>
                {item.underlayHeight !== undefined && (
                  <span
                    aria-hidden="true"
                    className="lens-waterfall-underlay"
                    style={{
                      top: `${item.top + item.height}%`,
                      height: `${item.underlayHeight}%`,
                    }}
                  />
                )}
                {index < model.items.length - 1 && (
                  <span
                    className="lens-waterfall-connector"
                    style={{ top: `${item.connectorTop}%` }}
                  />
                )}
                <div
                  className="lens-waterfall-bar"
                  data-checkpoint={item.checkpoint}
                  data-kind={item.kind}
                  data-label-row={index % 2}
                  data-terminal={!chrome || undefined}
                  data-tone={item.tone}
                  style={{
                    top: `${item.top}%`,
                    height: `${item.height}%`,
                    ...(chrome ? { cursor: 'pointer' } : {}),
                  }}
                  {...chrome}
                >
                  <strong>{item.formattedValue}</strong>
                  {item.splitHeight !== undefined && (
                    <span
                      className="lens-waterfall-bar-split"
                      style={{ height: `${item.splitHeight}%` }}
                    >
                      <span
                        className="lens-waterfall-split-callout"
                        data-reveal={splitCallout}
                        data-side={calloutSide}
                      >
                        {item.splitLabel ? `${item.splitLabel} ` : ''}
                        {item.formattedSplit}
                      </span>
                    </span>
                  )}
                </div>
              </div>
            )
          })}
        </div>
      </div>
      <div className="lens-waterfall-labels">
        {model.items.map((item, index) => (
          <span className="lens-waterfall-label" key={`${item.label}-label-${index}`}>
            <span>{item.label}</span>
            {item.annotation && (
              <small className="lens-waterfall-annotation">{item.annotation}</small>
            )}
          </span>
        ))}
      </div>
      {children}
    </div>
  )
}
