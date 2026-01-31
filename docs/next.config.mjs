import nextra from 'nextra'
import { remarkMermaid } from '@theguild/remark-mermaid'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

const withNextra = nextra({
  defaultShowCopyCode: true,
  search: {
    codeblocks: true
  },
  mdxOptions: {
    remarkPlugins: [
      [remarkMermaid, {
        // Pre-render Mermaid diagrams at build time
        // This validates syntax and improves performance
      }]
    ]
  }
})

/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  basePath: '/iota-sdk',
  images: {
    unoptimized: true
  },
  reactStrictMode: true,
  // Required for Turbopack to find mdx-components
  turbopack: {
    resolveAlias: {
      'next-mdx-import-source-file': './mdx-components.js'
    }
  },
  // Required for Webpack to find mdx-components
  webpack: (config) => {
    config.resolve.alias['next-mdx-import-source-file'] = path.resolve(__dirname, 'mdx-components.js')
    return config
  }
}

export default withNextra(nextConfig)
