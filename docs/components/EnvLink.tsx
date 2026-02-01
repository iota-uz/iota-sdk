'use client'

import Link from 'next/link'
import { ReactNode } from 'react'
import { useEnvironment } from '../contexts/EnvironmentContext'

interface EnvLinkProps {
  href: string
  children: ReactNode
  type?: 'erp' | 'website' | 'auto'
  className?: string
}

export function EnvLink({ href, children, type = 'auto', className }: EnvLinkProps) {
  const { environment, getUrl } = useEnvironment()
  const translatedUrl = getUrl(href, type)

  // If pre-production and relative link, show disabled state
  const isDisabled = environment === 'pre-production' && translatedUrl === '#'

  if (isDisabled) {
    return (
      <span
        className={`${className || ''} opacity-50 cursor-not-allowed`}
        title="This feature is not yet available in pre-production"
      >
        {children}
      </span>
    )
  }

  // External or translated links
  if (translatedUrl.startsWith('http')) {
    return (
      <a
        href={translatedUrl}
        className={className}
        target="_blank"
        rel="noopener noreferrer"
      >
        {children}
      </a>
    )
  }

  // Internal Next.js links (shouldn't happen after translation, but fallback)
  return (
    <Link href={translatedUrl} className={className}>
      {children}
    </Link>
  )
}
