import React from 'react'

// Note: In Nextra 4 with App Router, most configuration is done via
// Layout props in app/[locale]/layout.tsx, not in theme.config.tsx.
// This file is kept for backward compatibility and fallback defaults.

export default {
  logo: <span style={{ fontWeight: 700 }}>IOTA SDK</span>,
  head: (
    <>
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta property="og:title" content="IOTA SDK Documentation" />
      <meta property="og:description" content="Multi-tenant business management platform SDK for Go" />
    </>
  ),
  sidebar: {
    defaultMenuCollapseLevel: 1,
    toggleButton: true,
  },
  toc: {
    backToTop: true,
  },
  footer: {
    content: `© ${new Date().getFullYear()} IOTA SDK. All rights reserved.`,
  },

}
