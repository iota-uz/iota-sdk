export function docsMarkdown({ base }) {
  return (tree, file) => {
    const locale = /content[\\/](en|ru)[\\/]/.exec(file.path)?.[1];
    const href = (value) => {
      if (
        !value?.startsWith("/") ||
        value.startsWith("//") ||
        value === base ||
        value.startsWith(base + "/")
      )
        return value;
      if (
        locale &&
        !/^\/(en|ru)(\/|$)/.test(value) &&
        !/^\/(images|assets|evm-registration)(\/|$)/.test(value)
      )
        value = "/" + locale + value;
      return base + value;
    };
    const visit = (node) => {
      if (node.type === "link" || node.type === "image")
        node.url = href(node.url);
      if (
        node.type === "mdxJsxFlowElement" ||
        node.type === "mdxJsxTextElement"
      ) {
        if (node.name === "a" || node.name === "Cards.Card")
          for (const attribute of node.attributes ?? [])
            if (
              attribute.name === "href" &&
              typeof attribute.value === "string"
            )
              attribute.value = href(attribute.value);
      }
      if (node.type === "code" && node.lang === "mermaid") {
        node.type = "html";
        node.value =
          '<pre class="mermaid">' +
          node.value
            .replaceAll("&", "&amp;")
            .replaceAll("<", "&lt;")
            .replaceAll(">", "&gt;") +
          "</pre>";
        delete node.lang;
      }
      for (const child of node.children ?? []) visit(child);
    };
    visit(tree);
  };
}

// Preserve the previous documentation typography without a React renderer.
export function docsTypography() {
  const classes = {
    h1: [
      "x:tracking-tight",
      "x:text-slate-900",
      "x:dark:text-slate-100",
      "x:font-bold",
      "x:mt-2",
      "x:text-4xl",
    ],
    h2: [
      "x:tracking-tight",
      "x:text-slate-900",
      "x:dark:text-slate-100",
      "x:font-semibold",
      "x:target:animate-[fade-in_1.5s]",
      "x:mt-10",
      "x:border-b",
      "x:pb-1",
      "x:text-3xl",
      "nextra-border",
    ],
    p: ["x:not-first:mt-[1.25em]", "x:leading-7"],
    ul: [
      "x:[:is(ol,ul)_&]:my-[.75em]",
      "x:not-first:mt-[1.25em]",
      "x:list-disc",
      "x:ms-[1.5em]",
    ],
    li: ["x:my-[.5em]"],
    table: [
      "x:block",
      "x:overflow-x-auto",
      "nextra-scrollbar",
      "x:not-first:mt-[1.25em]",
      "x:p-0",
    ],
    tr: [
      "x:m-0",
      "x:border-t",
      "x:border-gray-300",
      "x:p-0",
      "x:dark:border-gray-600",
      "x:even:bg-gray-100",
      "x:even:dark:bg-gray-600/20",
    ],
    th: [
      "x:m-0",
      "x:border",
      "x:border-gray-300",
      "x:px-4",
      "x:py-2",
      "x:font-semibold",
      "x:dark:border-gray-600",
    ],
    td: [
      "x:m-0",
      "x:border",
      "x:border-gray-300",
      "x:px-4",
      "x:py-2",
      "x:dark:border-gray-600",
    ],
    hr: ["x:my-[2em]", "nextra-border"],
    ol: [
      "x:[:is(ol,ul)_&]:my-[.75em]",
      "x:not-first:mt-[1.25em]",
      "x:list-decimal",
      "x:ms-[1.5em]",
    ],
    a: [
      "x:focus-visible:nextra-focus",
      "x:text-primary-600",
      "x:underline",
      "x:hover:no-underline",
      "x:decoration-from-font",
      "x:[text-underline-position:from-font]",
    ],
  };
  Object.assign(classes, {
    h3: [
      "x:tracking-tight",
      "x:text-slate-900",
      "x:dark:text-slate-100",
      "x:font-semibold",
      "x:target:animate-[fade-in_1.5s]",
      "x:mt-8",
      "x:text-2xl",
    ],
    h4: [
      "x:tracking-tight",
      "x:text-slate-900",
      "x:dark:text-slate-100",
      "x:font-semibold",
      "x:target:animate-[fade-in_1.5s]",
      "x:mt-8",
      "x:text-xl",
    ],
    code: ["nextra-code"],
  });
  Object.assign(classes, {
    h5: [
      "x:tracking-tight",
      "x:text-slate-900",
      "x:dark:text-slate-100",
      "x:font-semibold",
      "x:target:animate-[fade-in_1.5s]",
      "x:mt-8",
      "x:text-lg",
    ],
    h6: [
      "x:tracking-tight",
      "x:text-slate-900",
      "x:dark:text-slate-100",
      "x:font-semibold",
      "x:target:animate-[fade-in_1.5s]",
      "x:mt-8",
      "x:text-base",
    ],
    blockquote: [
      "x:not-first:mt-[1.25em]",
      "x:border-gray-300",
      "x:italic",
      "x:text-gray-700",
      "x:dark:border-gray-700",
      "x:dark:text-gray-400",
      "x:border-s-2",
      "x:ps-[1.5em]",
    ],
  });
  const glyph = {
    type: "element",
    tagName: "svg",
    properties: {
      className: ["x:inline", "x:align-baseline", "x:shrink-0"],
      fill: "none",
      height: "1em",
      stroke: "currentColor",
      strokeLinecap: "round",
      strokeLinejoin: "round",
      strokeWidth: "1.7",
      viewBox: "0 0 24 24",
    },
    children: [
      {
        type: "element",
        tagName: "path",
        properties: { d: "M7 17L17 7" },
        children: [],
      },
      {
        type: "element",
        tagName: "path",
        properties: { d: "M7 7h10v10" },
        children: [],
      },
    ],
  };
  const codeToolbar = {
    type: "element",
    tagName: "div",
    properties: {
      className: [
        "x:group-hover:opacity-100",
        "x:group-focus:opacity-100",
        "x:opacity-0",
        "x:transition",
        "x:focus-within:opacity-100",
        "x:flex",
        "x:gap-1",
        "x:absolute",
        "x:right-4",
        "x:top-2",
      ],
    },
    children: [
      {
        type: "element",
        tagName: "button",
        properties: {
          "data-docs-code-control": "",
          className: [
            "x:transition",
            "x:cursor-pointer",
            "x:border",
            "x:border-gray-300",
            "x:dark:border-neutral-700",
            "x:contrast-more:border-gray-900",
            "x:contrast-more:dark:border-gray-50",
            "x:rounded-md",
            "x:p-1.5",
            "x:md:hidden",
          ],
          title: "Toggle word wrap",
          type: "button",
        },
        children: [
          {
            type: "element",
            tagName: "svg",
            properties: {
              fill: "currentColor",
              height: "1em",
              viewbox: "0 0 24 24",
            },
            children: [
              {
                type: "element",
                tagName: "path",
                properties: {
                  d: "M4 19h6v-2H4v2zM20 5H4v2h16V5zm-3 6H4v2h13.25c1.1 0 2 .9 2 2s-.9 2-2 2H15v-2l-3 3l3 3v-2h2c2.21 0 4-1.79 4-4s-1.79-4-4-4z",
                },
                children: [],
              },
            ],
          },
        ],
      },
      {
        type: "element",
        tagName: "button",
        properties: {
          "data-docs-code-control": "",
          className: [
            "x:transition",
            "x:cursor-pointer",
            "x:border",
            "x:border-gray-300",
            "x:dark:border-neutral-700",
            "x:contrast-more:border-gray-900",
            "x:contrast-more:dark:border-gray-50",
            "x:rounded-md",
            "x:p-1.5",
          ],
          title: "Copy code",
          type: "button",
        },
        children: [
          {
            type: "element",
            tagName: "svg",
            properties: {
              className: ["nextra-copy-icon"],
              fill: "none",
              height: "1em",
              stroke: "currentColor",
              "stroke-width": "2",
              viewbox: "0 0 24 24",
            },
            children: [
              {
                type: "element",
                tagName: "rect",
                properties: {
                  height: "13",
                  rx: "2",
                  width: "13",
                  x: "9",
                  y: "9",
                },
                children: [],
              },
              {
                type: "element",
                tagName: "path",
                properties: {
                  d: "M5 15H4C2.89543 15 2 14.1046 2 13V4C2 2.89543 2.89543 2 4 2H13C14.1046 2 15 2.89543 15 4V5",
                },
                children: [],
              },
            ],
          },
        ],
      },
    ],
  };
  return (tree) => {
    const visit = (node) => {
      if (node.type === "element") {
        if (
          node.tagName === "pre" &&
          !node.properties?.className?.includes("mermaid")
        ) {
          const pre = {
            ...node,
            properties: {
              className: [
                "x:group",
                "x:focus-visible:nextra-focus",
                "x:overflow-x-auto",
                "x:subpixel-antialiased",
                "x:text-[.9em]",
                "x:bg-white",
                "x:dark:bg-black",
                "x:py-4",
                "x:ring-1",
                "x:ring-inset",
                "x:ring-gray-300",
                "x:dark:ring-neutral-700",
                "x:contrast-more:ring-gray-900",
                "x:contrast-more:dark:ring-gray-50",
                "x:contrast-more:contrast-150",
                "x:rounded-md",
                "not-prose",
              ],
              tabIndex: 0,
            },
            children: [structuredClone(codeToolbar), ...node.children],
          };
          node.tagName = "div";
          node.properties = {
            className: ["nextra-code", "x:relative", "x:not-first:mt-[1.25em]"],
          };
          node.children = [pre];
          for (const child of pre.children.slice(1)) visit(child);
          return;
        }
        if (node.tagName === "code") {
          node.properties = { ...node.properties, dir: "ltr" };
          for (const line of node.children ?? [])
            if (
              line.type === "element" &&
              line.tagName === "span" &&
              !line.children.length
            )
              line.children = [{ type: "text", value: " " }];
        }

        if (classes[node.tagName])
          node.properties = {
            ...node.properties,
            className: [
              ...(node.properties?.className ?? []),
              ...classes[node.tagName],
            ],
          };
        if (
          node.tagName === "a" &&
          /^https?:/.test(node.properties?.href ?? "")
        ) {
          node.properties.target = "_blank";
          node.properties.rel = ["noreferrer"];
          node.children.push(
            { type: "comment", value: "" },
            { type: "text", value: " " },
            structuredClone(glyph),
          );
        }
      }
      for (const child of [...(node.children ?? [])]) visit(child);
    };
    visit(tree);
  };
}
