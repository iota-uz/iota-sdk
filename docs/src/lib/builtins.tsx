import { createUniqueId, type JSX } from "solid-js";
type Children = { children?: JSX.Element };
// Visual vocabulary adapted from Nextra 4.6.1 (MIT; see styles/vendor/NEXTRA-LICENSE).
const calloutIcons: Record<string, string> = {
  default:
    "M8 1.5c-2.363 0-4 1.69-4 3.75 0 .984.424 1.625.984 2.304l.214.253c.223.264.47.556.673.848.284.411.537.896.621 1.49a.75.75 0 0 1-1.484.211c-.04-.282-.163-.547-.37-.847a8.456 8.456 0 0 0-.542-.68c-.084-.1-.173-.205-.268-.32C3.201 7.75 2.5 6.766 2.5 5.25 2.5 2.31 4.863 0 8 0s5.5 2.31 5.5 5.25c0 1.516-.701 2.5-1.328 3.259-.095.115-.184.22-.268.319-.207.245-.383.453-.541.681-.208.3-.33.565-.37.847a.751.751 0 0 1-1.485-.212c.084-.593.337-1.078.621-1.489.203-.292.45-.584.673-.848.075-.088.147-.173.213-.253.561-.679.985-1.32.985-2.304 0-2.06-1.637-3.75-4-3.75ZM5.75 12h4.5a.75.75 0 0 1 0 1.5h-4.5a.75.75 0 0 1 0-1.5ZM6 15.25a.75.75 0 0 1 .75-.75h2.5a.75.75 0 0 1 0 1.5h-2.5a.75.75 0 0 1-.75-.75Z",
  info: "M0 8a8 8 0 1 1 16 0A8 8 0 0 1 0 8Zm8-6.5a6.5 6.5 0 1 0 0 13 6.5 6.5 0 0 0 0-13ZM6.5 7.75A.75.75 0 0 1 7.25 7h1a.75.75 0 0 1 .75.75v2.75h.25a.75.75 0 0 1 0 1.5h-2a.75.75 0 0 1 0-1.5h.25v-2h-.25a.75.75 0 0 1-.75-.75ZM8 6a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z",
  warning:
    "M6.457 1.047c.659-1.234 2.427-1.234 3.086 0l6.082 11.378A1.75 1.75 0 0 1 14.082 15H1.918a1.75 1.75 0 0 1-1.543-2.575Zm1.763.707a.25.25 0 0 0-.44 0L1.698 13.132a.25.25 0 0 0 .22.368h12.164a.25.25 0 0 0 .22-.368Zm.53 3.996v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 11a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z",
  error:
    "M4.47.22A.749.749 0 0 1 5 0h6c.199 0 .389.079.53.22l4.25 4.25c.141.14.22.331.22.53v6a.749.749 0 0 1-.22.53l-4.25 4.25A.749.749 0 0 1 11 16H5a.749.749 0 0 1-.53-.22L.22 11.53A.749.749 0 0 1 0 11V5c0-.199.079-.389.22-.53Zm.84 1.28L1.5 5.31v5.38l3.81 3.81h5.38l3.81-3.81V5.31L10.69 1.5ZM8 4a.75.75 0 0 1 .75.75v3.5a.75.75 0 0 1-1.5 0v-3.5A.75.75 0 0 1 8 4Zm0 8a1 1 0 1 1 0-2 1 1 0 0 1 0 2Z",
  important:
    "M0 1.75C0 .784.784 0 1.75 0h12.5C15.216 0 16 .784 16 1.75v9.5A1.75 1.75 0 0 1 14.25 13H8.06l-2.573 2.573A1.458 1.458 0 0 1 3 14.543V13H1.75A1.75 1.75 0 0 1 0 11.25Zm1.75-.25a.25.25 0 0 0-.25.25v9.5c0 .138.112.25.25.25h2a.75.75 0 0 1 .75.75v2.19l2.72-2.72a.749.749 0 0 1 .53-.22h6.5a.25.25 0 0 0 .25-.25v-9.5a.25.25 0 0 0-.25-.25Zm7 2.25v2.5a.75.75 0 0 1-1.5 0v-2.5a.75.75 0 0 1 1.5 0ZM9 9a1 1 0 1 1-2 0 1 1 0 0 1 2 0Z",
};
const calloutClasses: Record<string, string> = {
  default:
    "x:bg-green-100 x:dark:bg-green-900/30 x:text-green-700 x:dark:text-green-500 x:border-green-700 x:dark:border-green-800",
  info: "x:bg-blue-100 x:dark:bg-blue-900/30 x:text-blue-700 x:dark:text-blue-400 x:border-blue-700 x:dark:border-blue-600",
  warning:
    "x:bg-yellow-50 x:dark:bg-yellow-700/30 x:text-yellow-700 x:dark:text-yellow-500 x:border-yellow-700",
  error:
    "x:bg-red-100 x:dark:bg-red-900/30 x:text-red-700 x:dark:text-red-500 x:border-red-700 x:dark:border-red-600",
  important:
    "x:bg-purple-100 x:dark:bg-purple-900/30 x:text-purple-600 x:dark:text-purple-400 x:border-purple-600",
};
export function Callout(props: Children & { type?: string; emoji?: string }) {
  const kind = props.type ?? "default";
  return (
    <div
      class={
        "nextra-callout x:overflow-x-auto x:not-first:mt-[1.25em] x:flex x:rounded-lg x:border x:py-[.5em] x:pe-[1em] x:contrast-more:border-current! " +
        (calloutClasses[kind] ?? "")
      }
    >
      <div
        class="x:select-none x:text-[1.25em] x:ps-[.6em] x:pe-[.4em]"
        style={{
          "font-family":
            '"Apple Color Emoji", "Segoe UI Emoji", "Segoe UI Symbol"',
        }}
        data-pagefind-ignore="all"
      >
        {props.emoji ?? (
          <svg
            class="x:mt-[.3em]"
            fill="currentColor"
            height=".8em"
            viewBox="0 0 16 16"
          >
            <path d={calloutIcons[kind]} />
          </svg>
        )}
      </div>
      <div class="x:w-full x:min-w-0">{props.children}</div>
    </div>
  );
}
export function Steps(props: Children) {
  return (
    <div
      class="nextra-steps x:ms-4 x:mb-12 x:border-s x:border-gray-200 x:ps-6 x:dark:border-neutral-800"
      style={{ "--counter-id": createUniqueId() }}
    >
      {props.children}
    </div>
  );
}
const Tab = (props: Children) => (
  <section data-doc-tab-panel>{props.children}</section>
);
export const Tabs = Object.assign(
  (props: Children & { items?: string[] }) => (
    <div data-doc-tabs data-labels={JSON.stringify(props.items ?? [])}>
      {props.children}
    </div>
  ),
  { Tab },
);
const Card = (props: Children & { title: string; href: string }) => (
  <a
    class="x:group x:focus-visible:nextra-focus nextra-card x:flex x:flex-col x:justify-start x:overflow-hidden x:rounded-lg x:border x:border-gray-200 x:text-current x:no-underline x:dark:shadow-none x:hover:shadow-gray-100 x:dark:hover:shadow-none x:shadow-gray-100 x:active:shadow-sm x:active:shadow-gray-200 x:transition-all x:duration-200 x:hover:border-gray-300 x:bg-transparent x:shadow-sm x:dark:border-neutral-800 x:hover:bg-slate-50 x:hover:shadow-md x:dark:hover:border-neutral-700 x:dark:hover:bg-neutral-900"
    href={
      props.href.startsWith("/") &&
      !props.href.startsWith("//") &&
      !props.href.startsWith(import.meta.env.BASE_URL.replace(/\/$/, ""))
        ? import.meta.env.BASE_URL.replace(/\/$/, "") + props.href
        : props.href
    }
  >
    {props.children}
    <span class="x:flex x:font-semibold x:items-center x:gap-2 x:p-4 x:text-gray-700 x:hover:text-gray-900 x:dark:text-neutral-200 x:dark:hover:text-neutral-50">
      <span class="_truncate">{props.title}</span>
    </span>
  </a>
);
export const Cards = Object.assign(
  (props: Children) => (
    <div
      class="nextra-cards x:mt-4 x:gap-4 x:grid not-prose"
      style={{ "--rows": 3 }}
    >
      {props.children}
    </div>
  ),
  { Card },
);
