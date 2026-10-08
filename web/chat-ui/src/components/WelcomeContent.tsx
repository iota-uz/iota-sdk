import { createReducedMotion } from '../hooks/createReducedMotion';
import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { ChartBar, FileText, Lightbulb, type IconComponent } from '../icons';
import { useTranslation } from '../hooks/useTranslation';
interface ExamplePrompt {
    category: string;
    icon: IconComponent;
    text: string;
}
interface WelcomeContentProps {
    onPromptSelect?: (prompt: string) => void;
    title?: string;
    description?: string;
    disabled?: boolean;
    /** Custom prompts to replace the default i18n prompts. Icons cycle from defaults if not provided. */
    prompts?: Array<{
        category: string;
        text: string;
        icon?: IconComponent;
    }>;
}
/** Default prompt definitions with i18n keys and English fallbacks. */
const PROMPT_DEFS = [
    { categoryKey: 'BiChat.Welcome.Prompt1Category', textKey: 'BiChat.Welcome.Prompt1Text', icon: ChartBar, defaultCategory: 'Data Analysis', defaultText: 'Show me a summary of key metrics' },
    { categoryKey: 'BiChat.Welcome.Prompt2Category', textKey: 'BiChat.Welcome.Prompt2Text', icon: FileText, defaultCategory: 'Reports', defaultText: 'Generate a report for the current period' },
    { categoryKey: 'BiChat.Welcome.Prompt3Category', textKey: 'BiChat.Welcome.Prompt3Text', icon: Lightbulb, defaultCategory: 'Insights', defaultText: 'What trends can you identify in the data?' },
] as const;
const PROMPT_STYLES: {
    badge: string;
    icon: string;
}[] = [
    {
        badge: 'bg-sky-50 text-sky-600 ring-sky-600/10 dark:bg-sky-400/10 dark:text-sky-400 dark:ring-sky-400/20',
        icon: 'text-sky-500 dark:text-sky-400',
    },
    {
        badge: 'bg-teal-50 text-teal-600 ring-teal-600/10 dark:bg-teal-400/10 dark:text-teal-400 dark:ring-teal-400/20',
        icon: 'text-teal-500 dark:text-teal-400',
    },
    {
        badge: 'bg-amber-50 text-amber-600 ring-amber-600/10 dark:bg-amber-400/10 dark:text-amber-400 dark:ring-amber-400/20',
        icon: 'text-amber-500 dark:text-amber-400',
    },
];
const containerVariants = {
    hidden: { opacity: 0 },
    visible: {
        opacity: 1,
        transition: {
            staggerChildren: 0.08,
            delayChildren: 0.05
        }
    }
};
const reducedContainerVariants = {
    hidden: { opacity: 0 },
    visible: {
        opacity: 1,
        transition: {
            staggerChildren: 0,
            delayChildren: 0
        }
    }
};
const itemVariants = {
    hidden: { opacity: 0, y: 12 },
    visible: {
        opacity: 1,
        y: 0,
        transition: {
            duration: 0.3,
            ease: [0.4, 0, 0.2, 1]
        }
    }
};
const reducedItemVariants = {
    hidden: { opacity: 0 },
    visible: {
        opacity: 1,
        y: 0,
        transition: {
            duration: 0
        }
    }
};
/** Resolve a translation key, falling back to a default if the key is not translated. */
function tOr(t: (key: string) => string, key: string, defaultValue: string): string {
    const v = t(key);
    return v !== key ? v : defaultValue;
}
const DEFAULT_ICONS: IconComponent[] = PROMPT_DEFS.map((d) => d.icon);
function WelcomeContent(solidProps1Input: WelcomeContentProps) {
    const solidProps1 = mergeProps({ disabled: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const shouldReduceMotion = createReducedMotion();
    const resolvedTitle = createMemo(() => solidProps1.title || '');
    const resolvedDescription = createMemo(() => solidProps1.description || '');
    const prompts = createMemo<ExamplePrompt[]>(() => solidProps1.prompts?.length
        ? solidProps1.prompts.map((p, i) => ({
            category: p.category,
            text: p.text,
            icon: p.icon ?? DEFAULT_ICONS[i % DEFAULT_ICONS.length],
        }))
        : PROMPT_DEFS.map((def) => ({
            category: tOr(solidState2.t, def.categoryKey, def.defaultCategory),
            text: tOr(solidState2.t, def.textKey, def.defaultText),
            icon: def.icon,
        })));
    const handlePromptClick = (prompt: string) => {
        if (solidProps1.onPromptSelect && !solidProps1.disabled) {
            solidProps1.onPromptSelect?.(prompt);
        }
    };
    const activeContainerVariants = shouldReduceMotion() ? reducedContainerVariants : containerVariants;
    const activeItemVariants = shouldReduceMotion() ? reducedItemVariants : itemVariants;
    return (<div class="relative w-full max-w-5xl mx-auto px-6 text-center">
      {resolvedTitle() && (<h1 class="relative text-2xl sm:text-3xl font-semibold text-gray-900 dark:text-white mb-4">
          {resolvedTitle()}
        </h1>)}

      {resolvedDescription() && (<p class="text-base text-gray-500 dark:text-gray-400 mb-10 max-w-2xl mx-auto leading-relaxed">
          {resolvedDescription()}
        </p>)}

      {/* Example prompts */}
      <div>
        <div class="flex items-center gap-4 mb-5">
          <div class="h-px flex-1 bg-gradient-to-r from-transparent to-gray-200 dark:to-gray-700/70"/>
          <span class="text-[11px] font-semibold uppercase tracking-[0.12em] text-gray-400 dark:text-gray-500 select-none">
            {solidState2.t('BiChat.Welcome.QuickStart')}
          </span>
          <div class="h-px flex-1 bg-gradient-to-l from-transparent to-gray-200 dark:to-gray-700/70"/>
        </div>

        <div class={`grid gap-3 ${prompts().length <= 2 ? 'sm:grid-cols-2' : 'sm:grid-cols-2 lg:grid-cols-3'}`}>
          {prompts().map((prompt, index) => {
            const style = PROMPT_STYLES[index % PROMPT_STYLES.length];
            return (<button onClick={() => handlePromptClick(prompt.text)} disabled={solidProps1.disabled} class="cursor-pointer group flex flex-col items-start text-left p-4 rounded-xl bg-white dark:bg-gray-800/80 border border-gray-200/80 dark:border-gray-700/60 hover:border-gray-300 dark:hover:border-gray-600 hover:bg-gray-50/80 dark:hover:bg-gray-700/30 shadow-sm hover:shadow-md transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-primary-500/40 focus:ring-offset-2 dark:focus:ring-offset-gray-900 disabled:opacity-50 disabled:cursor-not-allowed" aria-label={`${prompt.category}: ${prompt.text}`}>
                <div class="mb-3 flex items-center gap-2">
                  <prompt.icon size={16} weight="duotone" class={style?.icon ?? 'text-gray-500'}/>
                  <span class={`inline-flex items-center rounded-full px-2.5 py-0.5 text-[11px] font-medium ${style?.badge ?? 'bg-gray-100 text-gray-600 ring-gray-500/10'}`}>
                    {prompt.category}
                  </span>
                </div>

                <p class="text-[13px] text-gray-700 dark:text-gray-200 leading-relaxed group-hover:text-gray-900 dark:group-hover:text-white transition-colors duration-200">
                  {prompt.text}
                </p>
              </button>);
        })}
        </div>
      </div>
    </div>);
}
export { WelcomeContent };
export default WelcomeContent;
