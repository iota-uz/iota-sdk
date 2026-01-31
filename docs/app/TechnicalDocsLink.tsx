'use client'

import { useParams, usePathname } from 'next/navigation'
import Link from 'next/link'

export function TechnicalDocsLink() {
  const params = useParams()
  const pathname = usePathname()
  const locale = params?.locale as string || 'en'

  const isTechnical = pathname?.includes('/technical')

  const labels = {
    en: { technical: 'Technical Docs', business: 'Business Docs' },
    ru: { technical: 'Техническая документация', business: 'Бизнес документация' }
  }

  const t = labels[locale as keyof typeof labels] || labels.en

  return (
    <Link
      href={isTechnical ? `/${locale}` : `/${locale}/technical`}
      className="hidden sm:flex items-center gap-1 rounded-lg px-3 py-1.5 text-sm font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
    >
      {isTechnical ? (
        <svg
          className="w-4 h-4"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
          />
        </svg>
      ) : (
        <svg
          className="w-4 h-4"
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4"
          />
        </svg>
      )}
      {isTechnical ? t.business : t.technical}
    </Link>
  )
}
