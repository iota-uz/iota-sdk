import { Layout, Navbar, LastUpdated } from 'nextra-theme-docs'
import { Head } from 'nextra/components'
import { getPageMap } from 'nextra/page-map'
import type { Metadata } from 'next'
import Link from 'next/link'
import type { ReactNode } from 'react'
import { Logo } from './Logo'
import '../styles/globals.css'
import 'nextra-theme-docs/style.css'

export const metadata: Metadata = {
  title: {
    default: 'IOTA SDK Documentation',
    template: '%s – IOTA SDK Docs'
  },
  description: 'Multi-tenant business management platform SDK for Go',
  metadataBase: new URL('https://iota-uz.github.io/iota-sdk')
}

const navbar = (
  <Navbar logo={<Logo />}>
    <nav className="hidden md:flex items-center gap-5 text-sm">
      <Link className="text-gray-300 hover:text-white transition-colors" href="/getting-started">
        Getting Started
      </Link>
      <Link className="text-gray-300 hover:text-white transition-colors" href="/architecture">
        Architecture
      </Link>
      <Link className="text-gray-300 hover:text-white transition-colors" href="/api">
        API Reference
      </Link>
    </nav>
  </Navbar>
)



type LayoutProps = {
  children: ReactNode
}

export default async function RootLayout({ children }: LayoutProps) {
  const pageMap = await getPageMap('/')

  return (
    <html lang="en" dir="ltr" suppressHydrationWarning>
      <Head />
      <body>
        <Layout
          navbar={navbar}
          sidebar={{ defaultMenuCollapseLevel: 1, toggleButton: true }}
          toc={{ backToTop: true }}
          pageMap={pageMap}
          lastUpdated={<LastUpdated>Last updated on</LastUpdated>}
        >
          {children}
        </Layout>
      </body>
    </html>
  )
}
