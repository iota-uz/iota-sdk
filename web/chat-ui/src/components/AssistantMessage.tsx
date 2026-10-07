import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
import { Check, Copy, ArrowsClockwise, CaretRight, Lightning, Brain, } from '../icons';
import { formatRelativeTime } from "../utils/dateFormatting";
import CodeOutputsPanel from "./CodeOutputsPanel";
import StreamingCursor from "./StreamingCursor";
import { ChartCard } from "./ChartCard";
import { InteractiveTableCard } from "./InteractiveTableCard";
import { TabbedTableGroup } from "./TabbedTableGroup";
import { TabbedChartGroup } from "./TabbedChartGroup";
import { SourcesPanel } from "./SourcesPanel";
import { DownloadCard } from "./DownloadCard";
import { InlineQuestionForm } from "./InlineQuestionForm";
import { RetryActionArea } from "./RetryActionArea";
import type { AssistantTurn, Citation, ChartData, Artifact, CodeOutput, PendingQuestion, RenderTableData, } from "../types";
import { DebugPanel } from "./DebugPanel";
import { useTranslation } from "../hooks/useTranslation";
import { shouldRenderInlineRetry } from "../utils/assistantTurnState";
const MarkdownRenderer = lazy(() => import("./MarkdownRenderer").then((module) => ({
    default: module.MarkdownRenderer,
})));
/* -------------------------------------------------------------------------------------------------
 * Slot Props Types
 * -----------------------------------------------------------------------------------------------*/
export interface AssistantMessageAvatarSlotProps {
    /** Default text */
    text: string;
}
export interface AssistantMessageContentSlotProps {
    /** Message content (markdown) */
    content: string;
    /** Citations */
    citations?: Citation[];
    /** Whether streaming is active */
    isStreaming: boolean;
}
export interface AssistantMessageSourcesSlotProps {
    /** Citations to display */
    citations: Citation[];
}
export interface AssistantMessageChartsSlotProps {
    /** Chart data array */
    charts: ChartData[];
}
export interface AssistantMessageCodeOutputsSlotProps {
    /** Code execution outputs */
    outputs: CodeOutput[];
}
export interface AssistantMessageTablesSlotProps {
    /** Interactive table payloads */
    tables: RenderTableData[];
}
export interface AssistantMessageArtifactsSlotProps {
    /** Downloadable artifacts */
    artifacts: Artifact[];
}
export interface AssistantMessageActionsSlotProps {
    /** Copy content to clipboard */
    onCopy: () => void;
    /** Regenerate response, optionally with a specific model id */
    onRegenerate?: (model?: string) => void;
    /** Available models that can be picked for regenerate */
    regenerateModels?: RegenerateModelOption[];
    /** Formatted timestamp */
    timestamp: string;
    /** Whether copy action is available */
    canCopy: boolean;
    /** Whether regenerate action is available */
    canRegenerate: boolean;
}
export interface RegenerateModelOption {
    /** Model id passed to onRegenerate */
    id: string;
    /** Translation key (or label) shown in the picker */
    label: string;
}
export interface AssistantMessageExplanationSlotProps {
    /** Explanation content (markdown) */
    explanation: string;
    /** Whether expanded */
    isExpanded: boolean;
    /** Toggle expansion */
    onToggle: () => void;
}
/* -------------------------------------------------------------------------------------------------
 * Component Types
 * -----------------------------------------------------------------------------------------------*/
export interface AssistantMessageSlots {
    /** Custom avatar renderer */
    avatar?: JSX.Element | ((props: AssistantMessageAvatarSlotProps) => JSX.Element);
    /** Custom content renderer */
    content?: JSX.Element | ((props: AssistantMessageContentSlotProps) => JSX.Element);
    /** Custom sources renderer */
    sources?: JSX.Element | ((props: AssistantMessageSourcesSlotProps) => JSX.Element);
    /** Custom charts renderer */
    charts?: JSX.Element | ((props: AssistantMessageChartsSlotProps) => JSX.Element);
    /** Custom code outputs renderer */
    codeOutputs?: JSX.Element | ((props: AssistantMessageCodeOutputsSlotProps) => JSX.Element);
    /** Custom table renderer */
    tables?: JSX.Element | ((props: AssistantMessageTablesSlotProps) => JSX.Element);
    /** Custom artifacts renderer */
    artifacts?: JSX.Element | ((props: AssistantMessageArtifactsSlotProps) => JSX.Element);
    /** Custom actions renderer */
    actions?: JSX.Element | ((props: AssistantMessageActionsSlotProps) => JSX.Element);
    /** Custom explanation renderer */
    explanation?: JSX.Element | ((props: AssistantMessageExplanationSlotProps) => JSX.Element);
}
export interface AssistantMessageClassNames {
    /** Root container */
    root?: string;
    /** Inner content wrapper */
    wrapper?: string;
    /** Avatar container */
    avatar?: string;
    /** Message bubble */
    bubble?: string;
    /** Code outputs container */
    codeOutputs?: string;
    /** Charts container */
    charts?: string;
    /** Tables container */
    tables?: string;
    /** Artifacts container */
    artifacts?: string;
    /** Sources container */
    sources?: string;
    /** Explanation container */
    explanation?: string;
    /** Actions container */
    actions?: string;
    /** Action button */
    actionButton?: string;
    /** Timestamp */
    timestamp?: string;
}
export interface AssistantMessageProps {
    /** Assistant turn data */
    turn: AssistantTurn;
    /** Turn ID for regenerate operations */
    turnId?: string;
    /** When true, this is the last turn (Regenerate button shown only on last assistant message) */
    isLastTurn?: boolean;
    /** Whether response is being streamed */
    isStreaming?: boolean;
    /** Pending question for HITL */
    pendingQuestion?: PendingQuestion | null;
    /** Slot overrides */
    slots?: AssistantMessageSlots;
    /** Class name overrides */
    classNames?: AssistantMessageClassNames;
    /** Copy handler */
    onCopy?: (content: string) => Promise<void> | void;
    /** Regenerate handler. The optional `model` argument is the id chosen from the
     *  Fast/Deep picker; when omitted the current session model is used. */
    onRegenerate?: (turnId: string, model?: string) => Promise<void> | void;
    /** Models offered when the user clicks the regenerate button. When two or more
     *  options are provided a Fast/Deep picker is shown; otherwise regenerate is
     *  triggered immediately with the active model. */
    regenerateModels?: RegenerateModelOption[];
    /** Send message handler (for markdown links) */
    onSendMessage?: (content: string) => void;
    /** Whether sending is disabled */
    sendDisabled?: boolean;
    /** Hide avatar */
    hideAvatar?: boolean;
    /** Hide actions */
    hideActions?: boolean;
    /** Hide timestamp */
    hideTimestamp?: boolean;
    /** Show debug panel */
    showDebug?: boolean;
}
type AssistantRenderMode = "content" | "hitl_form" | "hitl_resuming" | "hitl_waiting" | "retry" | "empty";
const COPY_FEEDBACK_MS = 2000;
/* -------------------------------------------------------------------------------------------------
 * Default Styles
 * -----------------------------------------------------------------------------------------------*/
const defaultClassNames: Required<AssistantMessageClassNames> = {
    root: "flex min-w-0 gap-3 group",
    wrapper: "flex-1 w-full min-w-0 flex flex-col gap-3 max-w-[var(--bichat-bubble-assistant-max-width,85%)]",
    avatar: "flex-shrink-0 w-8 h-8 rounded-full bg-primary-600 flex items-center justify-center text-white font-medium text-xs",
    bubble: "bg-white dark:bg-gray-800 rounded-2xl rounded-bl-sm px-4 py-3 shadow-sm",
    codeOutputs: "",
    charts: "mb-1 w-full",
    tables: "mb-1 flex flex-col gap-3 min-w-0",
    artifacts: "mb-1 flex flex-wrap gap-2",
    sources: "",
    explanation: "mt-4 border-t border-gray-100 dark:border-gray-700 pt-4",
    actions: "flex items-center gap-1",
    actionButton: "cursor-pointer p-2 min-h-[44px] min-w-[44px] flex items-center justify-center text-gray-500 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 active:bg-gray-200 dark:active:bg-gray-700 rounded-md transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50",
    timestamp: "text-xs text-gray-400 dark:text-gray-500 mr-1",
};
function mergeClassNames(defaults: Required<AssistantMessageClassNames>, overrides?: AssistantMessageClassNames): Required<AssistantMessageClassNames> {
    if (!overrides) {
        return defaults;
    }
    return {
        root: overrides.root ?? defaults.root,
        wrapper: overrides.wrapper ?? defaults.wrapper,
        avatar: overrides.avatar ?? defaults.avatar,
        bubble: overrides.bubble ?? defaults.bubble,
        codeOutputs: overrides.codeOutputs ?? defaults.codeOutputs,
        charts: overrides.charts ?? defaults.charts,
        tables: overrides.tables ?? defaults.tables,
        artifacts: overrides.artifacts ?? defaults.artifacts,
        sources: overrides.sources ?? defaults.sources,
        explanation: overrides.explanation ?? defaults.explanation,
        actions: overrides.actions ?? defaults.actions,
        actionButton: overrides.actionButton ?? defaults.actionButton,
        timestamp: overrides.timestamp ?? defaults.timestamp,
    };
}
/* -------------------------------------------------------------------------------------------------
 * Component
 * -----------------------------------------------------------------------------------------------*/
export function AssistantMessage(solidProps1Input: AssistantMessageProps) {
    const solidProps1 = mergeProps({ isLastTurn: false, isStreaming: false, sendDisabled: false, hideAvatar: false, hideActions: false, hideTimestamp: false, showDebug: false } as const, solidProps1Input);
    const solidState2 = useTranslation();
    const [explanationExpanded, setExplanationExpanded] = createSignal(false);
    const [isCopied, setIsCopied] = createSignal(false);
    const [showRegenPicker, setShowRegenPicker] = createSignal(false);
    const regenPickerRef = { current: null } as {
        current: (HTMLDivElement | null) | null;
    };
    const copyFeedbackTimeoutRef = { current: null } as {
        current: (ReturnType<typeof setTimeout> | null) | null;
    };
    const classes = createMemo(() => mergeClassNames(defaultClassNames, solidProps1.classNames));
    const isSystemMessage = createMemo(() => solidProps1.turn.role === "system");
    const avatarClassName = createMemo(() => isSystemMessage() ? "flex-shrink-0 w-8 h-8 rounded-full bg-gray-500 dark:bg-gray-600 flex items-center justify-center text-white font-medium text-xs"
        : classes().avatar);
    const bubbleClassName = createMemo(() => isSystemMessage() ? "bg-gray-50 dark:bg-gray-900/40 rounded-2xl px-4 py-3 shadow-sm"
        : classes().bubble);
    onMount(() => {
        const cleanup = untrack(() => {
            return () => {
                if (copyFeedbackTimeoutRef.current) {
                    clearTimeout(copyFeedbackTimeoutRef.current);
                    copyFeedbackTimeoutRef.current = null;
                }
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    });
    const hasContent = createMemo(() => solidProps1.turn.content?.trim().length > 0);
    const hasExplanation = createMemo(() => !!solidProps1.turn.explanation?.trim());
    const isAwaitingHumanInput = createMemo(() => solidProps1.turn.lifecycle === "waiting_for_human_input");
    const pendingQuestionStatus = createMemo(() => solidProps1.pendingQuestion?.status);
    const pendingQuestionMatchesTurn = createMemo(() => !!solidProps1.pendingQuestion &&
        (pendingQuestionStatus() === "PENDING" ||
            pendingQuestionStatus() === "ANSWER_SUBMITTED" ||
            pendingQuestionStatus() === "REJECT_SUBMITTED" ||
            pendingQuestionStatus() === "ANSWER_RESUME_FAILED" ||
            pendingQuestionStatus() === "REJECT_RESUME_FAILED") &&
        (solidProps1.pendingQuestion.turnId === solidProps1.turnId ||
            solidProps1.pendingQuestion.turnId === solidProps1.turn.id ||
            (!solidProps1.pendingQuestion.turnId && solidProps1.isLastTurn)));
    const hasPendingQuestion = createMemo(() => pendingQuestionMatchesTurn() && !!solidProps1.pendingQuestion);
    const showQuestionForm = createMemo(() => hasPendingQuestion() &&
        (pendingQuestionStatus() === "PENDING" ||
            pendingQuestionStatus() === "ANSWER_RESUME_FAILED" ||
            pendingQuestionStatus() === "REJECT_RESUME_FAILED"));
    const showResumeState = createMemo(() => hasPendingQuestion() &&
        (pendingQuestionStatus() === "ANSWER_SUBMITTED" ||
            pendingQuestionStatus() === "REJECT_SUBMITTED"));
    const hasCodeOutputs = createMemo(() => !!solidProps1.turn.codeOutputs?.length);
    const hasChart = createMemo(() => !!solidProps1.turn.charts?.length);
    const hasTables = createMemo(() => !!solidProps1.turn.renderTables?.length);
    const hasArtifacts = createMemo(() => !!solidProps1.turn.artifacts?.length);
    const hasDebug = createMemo(() => solidProps1.showDebug && !!solidProps1.turn.debug);
    const hasAnyRenderedContent = createMemo(() => hasContent() || hasExplanation() || hasCodeOutputs() || hasChart() || hasTables() || hasArtifacts() || hasDebug());
    const canRegenerate = createMemo(() => !!solidProps1.onRegenerate && !!solidProps1.turnId && !isSystemMessage() && solidProps1.isLastTurn);
    const showInlineRetry = createMemo(() => shouldRenderInlineRetry(solidProps1.turn, canRegenerate()) && !hasAnyRenderedContent());
    const renderMode = createMemo<AssistantRenderMode>(() => showQuestionForm() ? "hitl_form"
        : showResumeState() ? "hitl_resuming"
            : isAwaitingHumanInput() ? "hitl_waiting"
                : hasAnyRenderedContent() ? "content"
                    : showInlineRetry() ? "retry"
                        : "empty");
    const handleCopyClick = async () => {
        try {
            if (solidProps1.onCopy) {
                await solidProps1.onCopy?.(solidProps1.turn.content);
            }
            else {
                await navigator.clipboard.writeText(solidProps1.turn.content);
            }
            setIsCopied(true);
            if (copyFeedbackTimeoutRef.current) {
                clearTimeout(copyFeedbackTimeoutRef.current);
            }
            copyFeedbackTimeoutRef.current = setTimeout(() => {
                setIsCopied(false);
                copyFeedbackTimeoutRef.current = null;
            }, COPY_FEEDBACK_MS);
        }
        catch (err) {
            setIsCopied(false);
            console.error("Failed to copy:", err);
        }
    };
    const hasRegenChoices = createMemo(() => (solidProps1.regenerateModels?.length ?? 0) >= 2);
    const handleRegenerateClick = async () => {
        if (!solidProps1.onRegenerate || !solidProps1.turnId) {
            return;
        }
        if (hasRegenChoices()) {
            setShowRegenPicker((prev) => !prev);
            return;
        }
        await solidProps1.onRegenerate?.(solidProps1.turnId);
    };
    const handleRegenerateWithModel = async (modelId: string) => {
        setShowRegenPicker(false);
        if (solidProps1.onRegenerate && solidProps1.turnId) {
            await solidProps1.onRegenerate?.(solidProps1.turnId, modelId);
        }
    };
    // Close picker on outside click / Escape.
    createEffect(on(() => [showRegenPicker()], () => {
        const cleanup = untrack(() => {
            if (!showRegenPicker()) {
                return;
            }
            const handlePointer = (e: MouseEvent | TouchEvent) => {
                const root = regenPickerRef.current;
                if (!root) {
                    return;
                }
                // Use composedPath so the check works across shadow DOM boundaries
                // (BiChat is hosted inside a shadow root; e.target gets retargeted to
                // the shadow host and `root.contains(target)` is always false).
                const path = typeof e.composedPath === "function" ? e.composedPath() : [];
                const insideViaPath = path.includes(root);
                const insideViaContains = root.contains(e.target as Node);
                if (!insideViaPath && !insideViaContains) {
                    setShowRegenPicker(false);
                }
            };
            const handleKey = (e: KeyboardEvent) => {
                if (e.key === "Escape") {
                    setShowRegenPicker(false);
                }
            };
            document.addEventListener("mousedown", handlePointer);
            document.addEventListener("touchstart", handlePointer);
            document.addEventListener("keydown", handleKey);
            return () => {
                document.removeEventListener("mousedown", handlePointer);
                document.removeEventListener("touchstart", handlePointer);
                document.removeEventListener("keydown", handleKey);
            };
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const timestamp = createMemo(() => formatRelativeTime(solidProps1.turn.createdAt, solidState2.t));
    // Slot props
    const avatarSlotProps = createMemo<AssistantMessageAvatarSlotProps>(() => ({
        text: isSystemMessage() ? "SYS" : "AI",
    }));
    const contentSlotProps: AssistantMessageContentSlotProps = {
        content: solidProps1.turn.content,
        citations: solidProps1.turn.citations,
        get isStreaming() {
            return solidProps1.isStreaming;
        },
    };
    const sourcesSlotProps = createMemo<AssistantMessageSourcesSlotProps>(() => ({
        citations: solidProps1.turn.citations || [],
    }));
    const chartsSlotProps = createMemo<AssistantMessageChartsSlotProps>(() => ({
        charts: solidProps1.turn.charts || [],
    }));
    const codeOutputsSlotProps = createMemo<AssistantMessageCodeOutputsSlotProps>(() => ({
        outputs: solidProps1.turn.codeOutputs || [],
    }));
    const tablesSlotProps = createMemo<AssistantMessageTablesSlotProps>(() => ({
        tables: solidProps1.turn.renderTables || [],
    }));
    const artifactsSlotProps = createMemo<AssistantMessageArtifactsSlotProps>(() => ({
        artifacts: solidProps1.turn.artifacts || [],
    }));
    const actionsSlotProps: AssistantMessageActionsSlotProps = {
        onCopy: handleCopyClick,
        onRegenerate: canRegenerate() ? (model) => {
            if (!solidProps1.onRegenerate || !solidProps1.turnId) {
                return;
            }
            if (model) {
                void handleRegenerateWithModel(model);
            }
            else {
                void handleRegenerateClick();
            }
        }
            : undefined,
        get regenerateModels() {
            return solidProps1.regenerateModels;
        },
        get timestamp() {
            return timestamp();
        },
        canCopy: hasContent(),
        get canRegenerate() {
            return canRegenerate();
        },
    };
    const explanationSlotProps = createMemo<AssistantMessageExplanationSlotProps>(() => ({
        explanation: solidProps1.turn.explanation || "",
        isExpanded: explanationExpanded(),
        onToggle: () => setExplanationExpanded(!explanationExpanded()),
    }));
    // Render helpers
    const renderSlot = <T,>(slot: JSX.Element | ((props: T) => JSX.Element) | undefined, props: T, defaultContent: JSX.Element): JSX.Element => {
        if (slot === undefined) {
            return defaultContent;
        }
        if (typeof slot === "function") {
            return slot(props);
        }
        return slot;
    };
    return (<div class={classes().root}>
      {/* Avatar */}
      {!solidProps1.hideAvatar && (<div class={avatarClassName()}>
          {renderSlot(solidProps1.slots?.avatar, avatarSlotProps(), isSystemMessage() ? "SYS" : "AI")}
        </div>)}

      <div class={classes().wrapper}>
        {/* Inline recovery for empty assistant responses */}
        
          {showInlineRetry() && (<RetryActionArea onRetry={() => {
                void handleRegenerateClick();
            }}/>)}
        

        {/* Code outputs */}
        {solidProps1.turn.codeOutputs && solidProps1.turn.codeOutputs.length > 0 && (<div class={classes().codeOutputs}>
            {renderSlot(solidProps1.slots?.codeOutputs, codeOutputsSlotProps(), <CodeOutputsPanel outputs={solidProps1.turn.codeOutputs}/>)}
          </div>)}

        {/* Charts */}
        {solidProps1.turn.charts && solidProps1.turn.charts.length > 0 && (<div class={classes().charts}>
            {renderSlot(solidProps1.slots?.charts, chartsSlotProps(), solidProps1.turn.charts.length === 1 ? (<ChartCard chartData={solidProps1.turn.charts[0]}/>) : (<TabbedChartGroup charts={solidProps1.turn.charts}/>))}
          </div>)}

        {/* Interactive tables */}
        {solidProps1.turn.renderTables && solidProps1.turn.renderTables.length > 0 && (<div class={classes().tables}>
            {renderSlot(solidProps1.slots?.tables, tablesSlotProps(), solidProps1.turn.renderTables.length === 1 ? (<InteractiveTableCard table={solidProps1.turn.renderTables[0]} onSendMessage={solidProps1.onSendMessage} sendDisabled={solidProps1.sendDisabled || solidProps1.isStreaming}/>) : (<TabbedTableGroup tables={solidProps1.turn.renderTables} onSendMessage={solidProps1.onSendMessage} sendDisabled={solidProps1.sendDisabled || solidProps1.isStreaming}/>))}
          </div>)}

        {/* Message bubble */}
        {hasContent() && (<div class={bubbleClassName()}>
            {renderSlot(solidProps1.slots?.content, contentSlotProps, <Suspense fallback={<div class="flex items-center gap-2 text-sm text-gray-400 dark:text-gray-500">
                    <div class="w-4 h-4 border-2 border-gray-300 dark:border-gray-600 border-t-transparent rounded-full animate-spin"/>
                    {solidState2.t("BiChat.Common.Loading")}
                  </div>}>
                <MarkdownRenderer content={solidProps1.turn.content} citations={solidProps1.turn.citations} sendMessage={solidProps1.onSendMessage} sendDisabled={solidProps1.sendDisabled || solidProps1.isStreaming}/>
              </Suspense>)}

            {/* Streaming cursor */}
            {solidProps1.isStreaming && <StreamingCursor />}

            {/* Sources panel */}
            {solidProps1.turn.citations && solidProps1.turn.citations.length > 0 && (<div class={classes().sources}>
                {renderSlot(solidProps1.slots?.sources, sourcesSlotProps(), <SourcesPanel citations={solidProps1.turn.citations}/>)}
              </div>)}

            {/* Explanation section */}
            {hasExplanation() && (<div class={classes().explanation}>
                {renderSlot(solidProps1.slots?.explanation, explanationSlotProps(), <>
                    <button type="button" onClick={() => setExplanationExpanded(!explanationExpanded())} class="cursor-pointer flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300 transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 rounded-md p-1 -m-1" aria-expanded={explanationExpanded()}>
                      <CaretRight size={16} weight="bold" className={`transition-transform duration-150 ${explanationExpanded() ? "rotate-90" : ""}`}/>
                      <span class="font-medium">
                        {solidState2.t("BiChat.Assistant.Explanation")}
                      </span>
                    </button>
                    {explanationExpanded() && (<div class="pt-3 text-sm text-gray-600 dark:text-gray-400">
                        <Suspense fallback={<div>{solidState2.t("BiChat.Common.Loading")}</div>}>
                          <MarkdownRenderer content={solidProps1.turn.explanation!}/>
                        </Suspense>
                      </div>)}
                  </>)}
              </div>)}

            {solidProps1.showDebug && <DebugPanel trace={solidProps1.turn.debug}/>}
          </div>)}

        {/* Artifacts */}
        {solidProps1.turn.artifacts && solidProps1.turn.artifacts.length > 0 && (<div class={classes().artifacts}>
            {renderSlot(solidProps1.slots?.artifacts, artifactsSlotProps(), solidProps1.turn.artifacts.map((artifact, index) => (<DownloadCard artifact={artifact}/>)))}
          </div>)}

        {/* HITL waiting state */}
        {renderMode() === "hitl_waiting" && (<div class="animate-slide-up rounded-2xl border border-primary-200 dark:border-primary-700/40 bg-gradient-to-b from-primary-50/70 to-white dark:from-primary-900/20 dark:to-gray-900/80 shadow-sm p-4">
            <p class="text-sm font-medium text-primary-700 dark:text-primary-300">
              {solidState2.t("BiChat.InlineQuestion.InputNeeded")}
            </p>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {solidState2.t("BiChat.InlineQuestion.WaitingForDetails")}
            </p>
          </div>)}

        {renderMode() === "hitl_resuming" && (<div role="status" aria-live="polite" class="animate-slide-up flex items-center gap-2 rounded-xl border border-emerald-100/80 bg-emerald-50/60 px-3 py-2 text-xs text-emerald-800 dark:border-emerald-900/60 dark:bg-emerald-950/20 dark:text-emerald-200">
            <span class="inline-flex h-2 w-2 flex-shrink-0 rounded-full bg-emerald-500 animate-pulse" aria-hidden="true"/>
            <span class="font-medium">
              {solidProps1.pendingQuestion?.status === "REJECT_SUBMITTED"
                ? solidState2.t("BiChat.InlineQuestion.DismissalSubmitted")
                : solidState2.t("BiChat.InlineQuestion.AnswerSubmitted")}
            </span>
            <span class="text-emerald-400/80 dark:text-emerald-500/80" aria-hidden="true">
              •
            </span>
            <span class="text-emerald-700/80 dark:text-emerald-300/80">
              {solidState2.t("BiChat.InlineQuestion.ResumeInProgress")}
            </span>
          </div>)}

        {/* HITL question form */}
        {renderMode() === "hitl_form" && solidProps1.pendingQuestion && (<InlineQuestionForm pendingQuestion={solidProps1.pendingQuestion}/>)}

        {/* Actions */}
        {hasContent() && !solidProps1.hideActions && (<div class={`${classes().actions} ${isCopied() ? "opacity-100" : ""}`}>
            {renderSlot(solidProps1.slots?.actions, actionsSlotProps, <>
                {!solidProps1.hideTimestamp && (<span class={classes().timestamp}>{timestamp()}</span>)}

                {solidProps1.showDebug && solidProps1.turn.debug?.attempts?.[0]?.model && (<span class="inline-flex items-center gap-0.5 rounded px-1 py-0.5 text-[10px] font-medium leading-none text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-800">
                    {solidProps1.turn.debug.attempts[0].model}
                  </span>)}

                <button onClick={handleCopyClick} class={`cursor-pointer ${classes().actionButton} ${isCopied() ? "text-green-600 dark:text-green-400" : ""}`} aria-label={solidState2.t("BiChat.Message.CopyMessage")} title={isCopied() ? solidState2.t("BiChat.Message.Copied")
                    : solidState2.t("BiChat.Message.Copy")}>
                  {isCopied() ? (<Check size={14} weight="bold"/>) : (<Copy size={14} weight="regular"/>)}
                </button>

                {canRegenerate() && (<div ref={element => regenPickerRef.current = element} class="relative inline-flex">
                    <button onClick={handleRegenerateClick} class={`cursor-pointer ${classes().actionButton} ${showRegenPicker() ? "bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-200"
                        : ""}`} aria-label={solidState2.t("BiChat.Message.Regenerate")} title={solidState2.t("BiChat.Message.Regenerate")} aria-haspopup={hasRegenChoices() ? "menu" : undefined} aria-expanded={hasRegenChoices() ? showRegenPicker() : undefined}>
                      <ArrowsClockwise size={14} weight="regular"/>
                    </button>
                    {hasRegenChoices() && showRegenPicker() && (<div role="menu" aria-label={solidState2.t("BiChat.Message.Regenerate")} class="animate-slide-up absolute left-0 top-full z-20 mt-1 flex flex-col gap-0.5 rounded-lg border border-gray-200 bg-white p-1 shadow-lg dark:border-gray-700 dark:bg-gray-800">
                        {solidProps1.regenerateModels!.map((m, i) => {
                            const isFast = i === 0;
                            const Icon = isFast ? Lightning : Brain;
                            const accent = isFast
                                ? "text-amber-600 dark:text-amber-400"
                                : "text-blue-600 dark:text-blue-400";
                            return (<button role="menuitem" type="button" onClick={() => {
                                    void handleRegenerateWithModel(m.id);
                                }} class="flex items-center gap-2 whitespace-nowrap rounded-md px-2.5 py-1.5 text-xs font-medium text-gray-700 transition-colors duration-150 hover:bg-gray-100 dark:text-gray-200 dark:hover:bg-gray-700">
                              <Icon size={14} weight="fill" className={accent}/>
                              <span>{solidState2.t(m.label)}</span>
                            </button>);
                        })}
                      </div>)}
                  </div>)}
              </>)}
          </div>)}
      </div>
    </div>);
}
export default AssistantMessage;
