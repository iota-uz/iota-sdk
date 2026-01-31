import { generateStaticParamsFor, importPage } from 'nextra/pages'
import { useMDXComponents } from '../../mdx-components.js'

// Generate static params for content structure
const _generateStaticParams = generateStaticParamsFor('mdxPath')

export async function generateStaticParams() {
  return _generateStaticParams()
}

export async function generateMetadata(props: PageProps) {
  const params = await props.params
  const { metadata } = await importPage(params.mdxPath)
  return metadata
}

type PageProps = {
  params: Promise<{ mdxPath?: string[] }>
}

export default async function Page(props: PageProps) {
  const params = await props.params

  try {
    const result = await importPage(params.mdxPath)
    const { default: MDXContent, toc, metadata } = result
    const Wrapper = useMDXComponents().wrapper

    return (
      <Wrapper toc={toc} metadata={metadata} sourceCode={'sourceCode' in result ? result.sourceCode : ''}>
        <MDXContent {...props} params={params} />
      </Wrapper>
    )
  } catch (error) {
    console.error('Error importing page for path:', params.mdxPath, error)
    throw error
  }
}
