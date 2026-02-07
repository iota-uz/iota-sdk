import type {
  Attachment,
  AssistantTurn,
  Artifact,
  ChartData,
  Citation,
  CodeOutput,
  ConversationTurn,
  PendingQuestion,
  Question,
  Session,
  UserTurn,
} from '../../src/bichat/types'
import type { ImageAttachment } from '../../src/bichat/types'

import { base64FromDataUrl, largeImageDataUrl, smallImageDataUrl } from './imageFixtures'
import { flowingMarkdown, largeText, veryLargeText } from './textFixtures'

function isoNow(offsetMs = 0) {
  return new Date(Date.now() + offsetMs).toISOString()
}

export function makeSession(partial?: Partial<Session>): Session {
  return {
    id: partial?.id ?? 'session-1',
    title: partial?.title ?? 'Storybook Session',
    status: partial?.status ?? 'active',
    pinned: partial?.pinned ?? false,
    createdAt: partial?.createdAt ?? isoNow(-1000 * 60 * 60),
    updatedAt: partial?.updatedAt ?? isoNow(-1000 * 60 * 5),
  }
}

export function makeCitation(partial?: Partial<Citation>): Citation {
  return {
    id: partial?.id ?? `cit-${Math.random().toString(36).slice(2)}`,
    type: partial?.type ?? 'url_citation',
    title: partial?.title ?? 'Example source',
    url: partial?.url ?? 'https://example.com',
    startIndex: partial?.startIndex ?? 0,
    endIndex: partial?.endIndex ?? 10,
    excerpt: partial?.excerpt,
    source: partial?.source,
  }
}

export function makeChartData(overrides?: Partial<ChartData>): ChartData {
  return {
    chartType: overrides?.chartType ?? 'line',
    title: overrides?.title ?? 'Orders (last 12 weeks)',
    labels: overrides?.labels ?? Array.from({ length: 12 }).map((_, i) => `W${i + 1}`),
    series:
      overrides?.series ??
      [
        {
          name: 'Orders',
          data: Array.from({ length: 12 }).map((_, i) => 40 + i * 3 + (i % 3) * 5),
        },
      ],
    colors: overrides?.colors,
    height: overrides?.height,
  }
}

export function makeCodeOutputs(): CodeOutput[] {
  return [
    {
      type: 'text',
      content: 'Preview: generated 3 rows.\nOK.',
      filename: 'output.txt',
      mimeType: 'text/plain',
      sizeBytes: 128,
    },
    {
      type: 'image',
      content: smallImageDataUrl,
      filename: 'chart.png',
      mimeType: 'image/png',
      sizeBytes: 42_000,
    },
  ]
}

export function makeArtifacts(): Artifact[] {
  return [
    {
      type: 'excel',
      filename: 'export.xlsx',
      url: '#',
      sizeReadable: '184 KB',
      rowCount: 1234,
      description: 'Exported table',
    },
    {
      type: 'pdf',
      filename: 'report.pdf',
      url: '#',
      sizeReadable: '2.1 MB',
      description: 'Generated report',
    },
  ]
}

export function makeAttachment(partial?: Partial<Attachment>): Attachment {
  return {
    id: partial?.id ?? `att-${Math.random().toString(36).slice(2)}`,
    filename: partial?.filename ?? 'image.png',
    mimeType: partial?.mimeType ?? 'image/svg+xml',
    sizeBytes: partial?.sizeBytes ?? 12345,
    base64Data: partial?.base64Data,
  }
}

export function makeImageAttachment(partial?: Partial<ImageAttachment>): ImageAttachment {
  const preview = partial?.preview ?? smallImageDataUrl
  return {
    filename: partial?.filename ?? 'preview.svg',
    mimeType: partial?.mimeType ?? 'image/svg+xml',
    sizeBytes: partial?.sizeBytes ?? 12345,
    base64Data: partial?.base64Data ?? base64FromDataUrl(preview),
    preview,
  }
}

export function makeUserTurn(partial?: Partial<UserTurn>): UserTurn {
  const now = isoNow(-1000 * 60 * 3)
  return {
    id: partial?.id ?? `user-${Math.random().toString(36).slice(2)}`,
    content: partial?.content ?? 'Show me the revenue breakdown by region for the last quarter.',
    attachments: partial?.attachments ?? [],
    createdAt: partial?.createdAt ?? now,
  }
}

export function makeAssistantTurn(partial?: Partial<AssistantTurn>): AssistantTurn {
  const now = isoNow(-1000 * 60 * 2)
  return {
    id: partial?.id ?? `asst-${Math.random().toString(36).slice(2)}`,
    content: partial?.content ?? flowingMarkdown,
    explanation: partial?.explanation,
    citations: partial?.citations ?? [makeCitation(), makeCitation({ title: 'Internal dashboard', url: '#' })],
    chartData: partial?.chartData,
    artifacts: partial?.artifacts ?? [],
    codeOutputs: partial?.codeOutputs ?? [],
    createdAt: partial?.createdAt ?? now,
  }
}

export function makeConversationTurn(partial?: Partial<ConversationTurn>): ConversationTurn {
  const now = isoNow(-1000 * 60 * 4)
  const sessionId = partial?.sessionId ?? 'session-1'
  return {
    id: partial?.id ?? `turn-${Math.random().toString(36).slice(2)}`,
    sessionId,
    userTurn: partial?.userTurn ?? makeUserTurn(),
    assistantTurn: partial?.assistantTurn,
    createdAt: partial?.createdAt ?? now,
  }
}

export function makePendingQuestion(partial?: Partial<PendingQuestion>): PendingQuestion {
  const q: Question = {
    id: 'q-1',
    text: 'Which regions should the report include?',
    type: 'MULTIPLE_CHOICE',
    required: true,
    options: [
      { id: 'o-1', label: 'EMEA', value: 'EMEA' },
      { id: 'o-2', label: 'APAC', value: 'APAC' },
      { id: 'o-3', label: 'AMER', value: 'AMER' },
    ],
  }
  return {
    id: partial?.id ?? 'pending-1',
    turnId: partial?.turnId ?? 'turn-1',
    questions: partial?.questions ?? [q],
    status: partial?.status ?? 'PENDING',
  }
}

export const turnsShort: ConversationTurn[] = [
  makeConversationTurn({
    id: 'turn-1',
    assistantTurn: makeAssistantTurn({
      content: largeText,
      chartData: makeChartData(),
      artifacts: makeArtifacts(),
      codeOutputs: makeCodeOutputs(),
    }),
  }),
]

export const turnsLong: ConversationTurn[] = Array.from({ length: 24 }).map((_, i) => {
  const hasChart = i % 6 === 0
  const hasArtifacts = i % 8 === 0
  const hasCode = i % 7 === 0
  return makeConversationTurn({
    id: `turn-${i + 1}`,
    createdAt: isoNow(-1000 * 60 * (60 - i)),
    userTurn: makeUserTurn({
      id: `user-${i + 1}`,
      createdAt: isoNow(-1000 * 60 * (60 - i) - 1000 * 10),
      content: i % 4 === 0 ? veryLargeText.slice(0, 220) : `Question ${i + 1}: ${largeText}`,
      attachments:
        i % 5 === 0
          ? [
              makeAttachment({
                filename: `image-${i + 1}.svg`,
                mimeType: 'image/svg+xml',
                base64Data: base64FromDataUrl(i % 10 === 0 ? largeImageDataUrl : smallImageDataUrl),
              }),
            ]
          : [],
    }),
    assistantTurn: makeAssistantTurn({
      id: `asst-${i + 1}`,
      createdAt: isoNow(-1000 * 60 * (60 - i) + 1000 * 10),
      content: i % 3 === 0 ? flowingMarkdown : largeText,
      chartData: hasChart ? makeChartData({ title: `Chart ${i + 1}` }) : undefined,
      artifacts: hasArtifacts ? makeArtifacts() : [],
      codeOutputs: hasCode ? makeCodeOutputs() : [],
    }),
  })
})

