import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * InlineQuestionForm component
 * Handles HITL (Human-in-the-Loop) questions from the AI agent
 *
 * Supports multiple questions with multi-step navigation:
 * - SINGLE_CHOICE: Clickable option cards (radio) + always-present "Other" text input
 * - MULTIPLE_CHOICE: Clickable option cards (checkbox) + always-present "Other" text input
 */
import { ArrowLeft, ArrowRight, Check, ChatCircleDots, PencilSimpleLine, PaperPlaneTilt, X, } from '../icons';
import { PendingQuestion, QuestionAnswers } from "../types";
import { useChatMessaging } from "../context/ChatContext";
import { useTranslation } from "../hooks/useTranslation";
interface InlineQuestionFormProps {
    pendingQuestion: PendingQuestion;
}
export function InlineQuestionForm(solidProps1: InlineQuestionFormProps) {
    const solidState2 = useChatMessaging();
    const solidState3 = useTranslation();
    const [currentStep, setCurrentStep] = createSignal(0);
    const [answers, setAnswers] = createSignal<QuestionAnswers>({});
    const [otherTexts, setOtherTexts] = createSignal<Record<string, string>>({});
    const questions = Array.isArray(solidProps1.pendingQuestion.questions)
        ? solidProps1.pendingQuestion.questions
        : [];
    const currentQuestion = createMemo(() => questions[currentStep()]);
    const isLastStep = createMemo(() => currentStep() === questions.length - 1);
    const isFirstStep = createMemo(() => currentStep() === 0);
    const totalSteps = questions.length;
    const isFailedRetry = solidProps1.pendingQuestion.status === "ANSWER_RESUME_FAILED" ||
        solidProps1.pendingQuestion.status === "REJECT_RESUME_FAILED";
    // Get current answer for the current question
    const currentAnswer = createMemo(() => answers()[currentQuestion()?.id]);
    const currentOtherText = createMemo(() => otherTexts()[currentQuestion()?.id] || "");
    const handleOptionChange = (optionID: string, checked: boolean) => {
        if (!currentQuestion()) {
            return;
        }
        const questionId = currentQuestion().id;
        const existingAnswer = answers()[questionId] || { options: [] };
        const isOtherOption = optionID === "__other__";
        const isMultiSelect = currentQuestion().type === "MULTIPLE_CHOICE";
        // "Other" is mutually exclusive with predefined options.
        if (isOtherOption) {
            setAnswers({
                ...answers(),
                [questionId]: {
                    options: [],
                    customText: checked ? currentOtherText() : undefined,
                },
            });
            return;
        }
        let newOptions: string[];
        if (isMultiSelect) {
            // Multi-select: toggle option
            if (!checked) {
                newOptions = existingAnswer.options.filter((o) => o !== optionID);
            }
            else if (existingAnswer.options.includes(optionID)) {
                newOptions = existingAnswer.options;
            }
            else {
                newOptions = [...existingAnswer.options, optionID];
            }
        }
        else {
            // Single-select: replace selection (radio)
            newOptions = checked ? [optionID] : [];
        }
        setAnswers({
            ...answers(),
            [questionId]: {
                options: newOptions,
                customText: undefined,
            },
        });
    };
    const handleOtherTextChange = (text: string) => {
        if (!currentQuestion()) {
            return;
        }
        const questionId = currentQuestion().id;
        setOtherTexts({ ...otherTexts(), [questionId]: text });
        // Update the answer with custom text ("Other" is selected when customText is set)
        setAnswers({
            ...answers(),
            [questionId]: {
                options: [],
                customText: text,
            },
        });
    };
    const isCurrentAnswerValid = (): boolean => {
        if (!currentQuestion()) {
            return false;
        }
        const answer = answers()[currentQuestion().id];
        const required = currentQuestion().required ?? true;
        if (!answer) {
            return !required;
        }
        const hasOptionSelection = answer.options.length > 0;
        const hasOtherSelected = answer.customText !== undefined;
        const hasOtherText = (answer.customText?.trim().length ?? 0) > 0;
        if (!hasOptionSelection && !hasOtherSelected) {
            return !required;
        }
        if (hasOptionSelection) {
            return true;
        }
        // "Other" selected: require non-empty text if required
        return !required || hasOtherText;
    };
    const handleNext = () => {
        if (!isCurrentAnswerValid()) {
            return;
        }
        if (isLastStep()) {
            solidState2.handleSubmitQuestionAnswers(answers());
        }
        else {
            setCurrentStep(currentStep() + 1);
        }
    };
    const handleBack = () => {
        if (!isFirstStep()) {
            setCurrentStep(currentStep() - 1);
        }
    };
    const handleSubmit = (e: Event) => {
        e.preventDefault();
        handleNext();
    };
    return <>{createMemo(() => {
            if (!currentQuestion()) {
                return (<div class="animate-slide-up rounded-2xl border border-amber-200 dark:border-amber-700/50 bg-gradient-to-b from-amber-50/70 to-white dark:from-amber-950/20 dark:to-gray-900/80 shadow-sm overflow-hidden p-4">
        <p class="text-sm font-medium text-gray-800 dark:text-gray-200">
          {solidState3.t("BiChat.Error.SomethingWentWrong")}
        </p>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {solidState3.t("BiChat.Error.UnexpectedError")}
        </p>
        <div class="mt-3">
          <button type="button" onClick={solidState2.handleRejectPendingQuestion} disabled={solidState2.loading} class="cursor-pointer inline-flex items-center gap-1.5 px-3 py-1.5 text-sm rounded-lg bg-gray-100 dark:bg-gray-800 text-gray-700 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-700 disabled:opacity-40">
            <X size={14} weight="bold"/>
            {solidState3.t("BiChat.InlineQuestion.Dismiss")}
          </button>
        </div>
      </div>);
            }
            const isMultiSelect = createMemo(() => currentQuestion().type === "MULTIPLE_CHOICE");
            const options = createMemo(() => (currentQuestion().options || [])
                .filter((option) => Boolean(option && typeof option.label === "string"))
                .map((option, index) => ({
                id: option.id || `${currentQuestion().id}-option-${index}`,
                label: option.label,
                value: option.value || option.id || `${currentQuestion().id}-option-${index}`,
            })));
            const isOtherSelected = createMemo(() => currentAnswer()?.customText !== undefined);
            const canProceed = isCurrentAnswerValid();
            return (<div class="animate-slide-up rounded-2xl border border-gray-200 dark:border-gray-700/50 bg-gradient-to-b from-primary-50/80 to-white dark:from-primary-950/30 dark:to-gray-900/80 shadow-sm overflow-hidden">
      <form data-testid="bichat-question-form" onSubmit={handleSubmit}>
        {/* Header bar */}
        <div class="flex items-center gap-2.5 px-4 pt-4 pb-3">
          <div class="flex items-center justify-center w-7 h-7 rounded-lg bg-primary-100 dark:bg-primary-900/40">
            <ChatCircleDots className="w-4 h-4 text-primary-600 dark:text-primary-400" weight="fill"/>
          </div>
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2">
              <span class="text-xs font-semibold uppercase tracking-wide text-primary-600 dark:text-primary-400">
                {solidState3.t("BiChat.InlineQuestion.InputNeeded")}
              </span>
              {totalSteps > 1 && (<span class="text-[11px] tabular-nums text-gray-400 dark:text-gray-500">
                  {currentStep() + 1}/{totalSteps}
                </span>)}
            </div>
          </div>
          <button type="button" onClick={solidState2.handleRejectPendingQuestion} disabled={solidState2.loading} class="cursor-pointer p-1 rounded-md text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors disabled:opacity-40" aria-label={solidState3.t("BiChat.InlineQuestion.Dismiss")}>
            <X size={16} weight="bold"/>
          </button>
        </div>

        {isFailedRetry && (<div class="mx-4 mb-3 rounded-xl border border-amber-200 dark:border-amber-700/40 bg-amber-50/80 dark:bg-amber-950/20 px-3 py-2">
            <p class="text-xs font-medium text-amber-800 dark:text-amber-300">
              Continuing after your last response failed.
            </p>
            <p class="mt-1 text-xs text-amber-700/80 dark:text-amber-200/80">
              Review the answer and submit again, or dismiss the question if you
              want to skip it.
            </p>
          </div>)}

        {/* Progress dots for multi-step */}
        {totalSteps > 1 && (<div class="flex items-center gap-1.5 px-4 pb-3">
            {questions.map((_, index) => {
                        const isCompleted = createMemo(() => index < currentStep());
                        const isCurrent = createMemo(() => index === currentStep());
                        return (<div class={[
                                "h-1 rounded-full transition-all duration-300",
                                isCurrent() ? "flex-[2] bg-primary-500 dark:bg-primary-400"
                                    : "flex-1",
                                isCompleted() ? "bg-primary-400 dark:bg-primary-500"
                                    : !isCurrent()
                                        ? "bg-gray-200 dark:bg-gray-700"
                                        : "",
                            ].join(" ")}/>);
                    })}
          </div>)}

        {/* Question text */}
        <div class="px-4 pb-3">
          <p class="text-[15px] leading-relaxed text-gray-800 dark:text-gray-200">
            {currentQuestion().text}
          </p>
          {isMultiSelect() && (<p class="mt-1 text-xs text-gray-400 dark:text-gray-500">
              {solidState3.t("BiChat.InlineQuestion.SelectAllThatApply")}
            </p>)}
        </div>

        {/* Options as clickable cards */}
        <div class="px-4 pb-2 space-y-1.5">
          {options().map((option) => {
                    const isSelected = createMemo(() => currentAnswer()?.options.includes(option.id) || false);
                    return (<label data-testid={`bichat-question-option-${option.id}`} class={[
                            "group/opt flex items-center gap-3 px-3 py-2.5 rounded-xl cursor-pointer",
                            "border transition-all duration-150",
                            isSelected() ? "border-primary-300 dark:border-primary-600 bg-primary-50 dark:bg-primary-900/30 shadow-sm"
                                : "border-transparent hover:border-gray-200 dark:hover:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/50",
                        ].join(" ")}>
                {/* Custom indicator */}
                <span class={[
                            "flex-shrink-0 flex items-center justify-center w-5 h-5 transition-all duration-150",
                            isMultiSelect() ? "rounded-md" : "rounded-full",
                            isSelected() ? "bg-primary-600 dark:bg-primary-500 border-primary-600 dark:border-primary-500 text-white shadow-sm"
                                : "border-2 border-gray-300 dark:border-gray-600 group-hover/opt:border-gray-400 dark:group-hover/opt:border-gray-500",
                        ].join(" ")}>
                  {isSelected() && <Check size={12} weight="bold"/>}
                </span>
                <input type={isMultiSelect() ? "checkbox" : "radio"} name={`question-${currentQuestion().id}`} value={option.value} checked={isSelected()} onInput={(e) => handleOptionChange(option.id, e.target.checked)} class="sr-only"/>
                <span class={[
                            "text-sm transition-colors duration-150",
                            isSelected() ? "text-gray-900 dark:text-gray-100 font-medium"
                                : "text-gray-700 dark:text-gray-300",
                        ].join(" ")}>
                  {option.label}
                </span>
              </label>);
                })}

          {/* "Other" option */}
          <label class={[
                    "group/opt flex items-center gap-3 px-3 py-2.5 rounded-xl cursor-pointer",
                    "border transition-all duration-150",
                    isOtherSelected() ? "border-primary-300 dark:border-primary-600 bg-primary-50 dark:bg-primary-900/30 shadow-sm"
                        : "border-transparent hover:border-gray-200 dark:hover:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/50",
                ].join(" ")}>
            <span class={[
                    "flex-shrink-0 flex items-center justify-center w-5 h-5 transition-all duration-150",
                    isMultiSelect() ? "rounded-md" : "rounded-full",
                    isOtherSelected() ? "bg-primary-600 dark:bg-primary-500 border-primary-600 dark:border-primary-500 text-white shadow-sm"
                        : "border-2 border-gray-300 dark:border-gray-600 group-hover/opt:border-gray-400 dark:group-hover/opt:border-gray-500",
                ].join(" ")}>
              {isOtherSelected() && <PencilSimpleLine size={11} weight="bold"/>}
            </span>
            <input type={isMultiSelect() ? "checkbox" : "radio"} name={`question-${currentQuestion().id}`} value="__other__" checked={isOtherSelected()} onInput={(e) => handleOptionChange("__other__", e.target.checked)} class="sr-only"/>
            <span class={[
                    "text-sm transition-colors duration-150",
                    isOtherSelected() ? "text-gray-900 dark:text-gray-100 font-medium"
                        : "text-gray-700 dark:text-gray-300",
                ].join(" ")}>
              {solidState3.t("BiChat.InlineQuestion.OtherOption")}
            </span>
          </label>

          {/* Other text input — always visible, typing auto-selects "Other" */}
          <div class="pl-8 pr-1 pb-1">
            <input type="text" value={currentOtherText()} onInput={(e) => handleOtherTextChange(e.target.value)} placeholder={solidState3.t("BiChat.InlineQuestion.TypeYourAnswer")} class="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder:text-gray-400 dark:placeholder:text-gray-500 focus:outline-none focus:ring-2 focus:ring-primary-500/40 focus:border-primary-400 dark:focus:border-primary-600 transition-shadow"/>
          </div>
        </div>

        {/* Footer with navigation */}
        <div class="flex items-center justify-between gap-2 px-4 pt-2 pb-4">
          <div>
            {!isFirstStep() && (<button type="button" onClick={handleBack} class="cursor-pointer flex items-center gap-1 px-2.5 py-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors">
                <ArrowLeft size={14} weight="bold"/>
                {solidState3.t("BiChat.InlineQuestion.Back")}
              </button>)}
          </div>

          <button type="submit" data-testid="bichat-question-submit" disabled={solidState2.loading || !canProceed} class={[
                    "flex items-center gap-1.5 px-4 py-2 text-sm font-medium rounded-xl transition-all duration-150",
                    "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2",
                    canProceed
                        ? "cursor-pointer bg-primary-600 hover:bg-primary-700 active:bg-primary-800 text-white shadow-sm hover:shadow"
                        : "bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-600 cursor-not-allowed",
                ].join(" ")}>
            {isLastStep() ? (<>
                {solidState3.t("BiChat.Submit")}
                <PaperPlaneTilt size={14} weight="fill"/>
              </>) : (<>
                {solidState3.t("BiChat.InlineQuestion.Next")}
                <ArrowRight size={14} weight="bold"/>
              </>)}
          </button>
        </div>
      </form>
    </div>);
        })}</>;
}
