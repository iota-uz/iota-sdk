import { Layout, Navbar, LastUpdated } from 'nextra-theme-docs'
import { Head } from 'nextra/components'
import { getPageMap } from 'nextra/page-map'
import type { Metadata } from 'next'
import type { ReactNode } from 'react'
import { Logo } from './Logo'
import { EnvironmentProvider } from '../contexts/EnvironmentContext'
import { EnvironmentSelector } from './EnvironmentSelector'
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
    <div className="flex items-center gap-2">
      <EnvironmentSelector />
    </div>
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
        <EnvironmentProvider>
          <Layout
            navbar={navbar}
            sidebar={{ defaultMenuCollapseLevel: 1, toggleButton: true }}
            toc={{ backToTop: true }}
            pageMap={pageMap}
            
            lastUpdated={<LastUpdated>Last updated on</LastUpdated>}
          >
            {children}
          </Layout>
        </EnvironmentProvider>
      </body>
    </html>
  )
}
