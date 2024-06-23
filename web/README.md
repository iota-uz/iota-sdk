# IOTA-ERP frontend

This is a [Next.js](https://nextjs.org/) project bootstrapped
with [`create-next-app`](https://github.com/vercel/next.js/tree/canary/packages/create-next-app).

## Getting Started

First, run the development server:

```bash
pnpm dev
```

Open [http://localhost:3000](http://localhost:3000) with your browser to see the result.

You can start editing the page by modifying `app/page.tsx`. The page auto-updates as you edit the file.

This project uses [`next/font`](https://nextjs.org/docs/basic-features/font-optimization) to automatically optimize and
load Inter, a custom Google Font.

## Installing components

Use shadcn CLI to install components

```bash
pnpm dlx shadcn-ui@latest add alert
```

## GraphQL codegen

Use GraphQL codegen to generate types

```bash
pnpm compile

# or in watch mode
pnpm watch
```

## Generating pages

Use https://v0.dev - a LLM based code generator to generate pages

## Learn More

To learn more about Next.js, take a look at the following resources:

- [Next.js Documentation](https://nextjs.org/docs) - learn about Next.js features and API.
- [Learn Next.js](https://nextjs.org/learn) - an interactive Next.js tutorial.
- [Localization](https://phrase.com/blog/posts/next-js-app-router-localization-next-intl/#how-do-i-localize-client-components) -
  a super useful blog post about internationalization in Next.js and next-intl.
