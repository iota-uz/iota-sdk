export const smallText = 'OK'

export const largeText =
  'Quarterly performance summary: Revenue grew 18% QoQ, driven by a 9% uplift in conversion and a 6% increase in AOV. ' +
  'The largest gains came from repeat customers and mid-market accounts. ' +
  'Risks: churn increased in SMB (notably in week 9), and delivery SLAs regressed by 0.6 days. ' +
  'Recommended actions: tighten onboarding, add proactive renewal nudges, and re-balance inventory for top SKUs.'

export const veryLargeText = Array.from({ length: 30 })
  .map(
    (_, i) =>
      `Paragraph ${i + 1}: ${largeText} ` +
      'This is intentionally long to stress wrapping, scrolling, and overflow behavior across layouts.'
  )
  .join('\n\n')

export const flowingMarkdown = [
  '# Heading 1',
  '',
  'This is a longer markdown message with **bold**, _italic_, `inline code`, and a link: [Example](https://example.com).',
  '',
  '## Lists',
  '',
  '- Item one with a slightly longer description to test wrapping',
  '- Item two',
  '- Item three',
  '',
  '## Code',
  '',
  '```ts',
  'export function sum(a: number, b: number) {',
  '  return a + b',
  '}',
  '```',
  '',
  '## Table',
  '',
  '| Metric | Value |',
  '|---|---:|',
  '| Orders | 12,345 |',
  '| Conversion | 3.21% |',
  '| Revenue | $987,654 |',
].join('\n')

