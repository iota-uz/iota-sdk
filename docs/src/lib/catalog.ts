type PageModule = {
  default: any;
  frontmatter: { title?: string; description?: string };
  getHeadings?: () => Array<{ depth: number; slug: string; text: string }>;
};
const modules = import.meta.glob("../../content/**/*.mdx", {
  eager: true,
}) as Record<string, PageModule>;
const rawModules = import.meta.glob("../../content/**/*.mdx", {
  eager: true,
  query: "?raw",
  import: "default",
}) as Record<string, string>;
const metadata = import.meta.glob("../../content/**/_meta.{js,jsx,ts,tsx}", {
  eager: true,
}) as Record<string, { default: Record<string, any> }>;
export const pages = Object.entries(modules).map(([file, module]) => ({
  slug: file
    .replace("../../content/", "")
    .replace(/(^|\/)index\.mdx$/, "")
    .replace(/\.mdx$/, ""),
  module,
  sourceCode: rawModules[file],
  sourcePath: file.replace("../../", ""),
}));
export type NavItem = {
  title: string;
  href?: string;
  children: NavItem[];
  separator?: boolean;
  menu?: boolean;
  page?: boolean;
  route?: string;
};
export function navigation(prefix = ""): NavItem[] {
  const meta =
    metadata["../../content/" + (prefix ? prefix + "/" : "") + "_meta.js"]
      ?.default ?? {};
  const childNames = [
    ...new Set(
      pages
        .filter((page) => page.slug.startsWith(prefix ? prefix + "/" : ""))
        .map(
          (page) =>
            page.slug.slice(prefix ? prefix.length + 1 : 0).split("/")[0],
        )
        .filter(Boolean),
    ),
  ];
  const keys = [...new Set([...Object.keys(meta), ...childNames])];
  return keys.flatMap<NavItem>((key) => {
    const value = meta[key];
    if (value?.display === "hidden") return [];
    if (value?.type === "separator")
      return [{ title: value.title ?? key, children: [], separator: true }];
    if (key === "index")
      return [
        {
          title:
            typeof value === "string" ? value : (value?.title ?? "Overview"),
          href: "/" + prefix,
          children: [],
        },
      ];
    const slug = (prefix ? prefix + "/" : "") + key;
    const page = pages.find((page) => page.slug === slug);
    if (value?.items)
      return [
        {
          title: value.title ?? key,
          menu: true,
          children: Object.values(value.items).map((item: any) => ({
            title: item.title,
            href: item.href,
            children: [],
          })),
        },
      ];
    const children = navigation(slug);
    if (!page && !children.length) return [];
    return [
      {
        title:
          typeof value === "string"
            ? value
            : (value?.title ?? page?.module.frontmatter.title ?? key),
        page: value?.type === "page",
        route: "/" + slug,
        href: page ? "/" + slug : undefined,
        children,
      },
    ];
  });
}

export function breadcrumbs(
  items: NavItem[],
  current: string,
  base: string,
): Array<{ title: string; href?: string }> {
  for (const item of items) {
    if (item.separator || item.menu) continue;
    if (item.children.length) {
      const nested = breadcrumbs(item.children, current, base);
      if (nested.length)
        return [
          { title: item.title, href: item.href ? base + item.href : undefined },
          ...nested,
        ];
    } else if (
      item.href &&
      (base + item.href).replace(/\/$/, "") === current.replace(/\/$/, "")
    )
      return [{ title: item.title }];
  }
  return [];
}
