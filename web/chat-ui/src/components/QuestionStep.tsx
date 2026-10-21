import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * QuestionStep Component
 * Displays a single question with options to select from
 * Supports both single-select (radio) and multi-select (checkboxes)
 * Includes automatic "Other" option for custom text input
 */
import { type Question, type QuestionAnswers, type QuestionAnswerData } from '../types';
import { useTranslation } from '../hooks/useTranslation';
interface QuestionStepProps {
    question: Question;
    selectedAnswers: QuestionAnswers;
    onAnswer: (answerData: QuestionAnswerData) => void;
}
export default function QuestionStep(solidProps1: QuestionStepProps) {
    const solidState2 = useTranslation();
    const answerData = solidProps1.selectedAnswers[solidProps1.question.id] || { options: [] };
    const selectedOptions = answerData.options || [];
    const isMultiSelect = solidProps1.question.type === 'MULTIPLE_CHOICE';
    // Local state for "Other" text input
    const [otherText, setOtherText] = createSignal(answerData.customText || '');
    // Sync local state with props when switching questions
    createEffect(on(() => [solidProps1.question.id], () => {
        const cleanup = untrack(() => {
            const data = solidProps1.selectedAnswers[solidProps1.question.id] || { options: [] };
            setOtherText(data.customText || '');
            // eslint-disable-next-line react-hooks/exhaustive-deps
        });
        if (typeof cleanup === "function")
            onCleanup(cleanup);
    }));
    const handleOptionClick = (optionID: string) => {
        setOtherText('');
        if (isMultiSelect) {
            // Multi-select: toggle the option
            const newOptions = selectedOptions.includes(optionID)
                ? selectedOptions.filter((a) => a !== optionID)
                : [...selectedOptions, optionID];
            solidProps1.onAnswer?.({ options: newOptions, customText: undefined });
        }
        else {
            // Single-select: replace with new selection and clear custom text.
            solidProps1.onAnswer?.({ options: [optionID], customText: undefined });
        }
    };
    const handleOtherTextChange = (text: string) => {
        setOtherText(text);
        solidProps1.onAnswer?.({
            options: [],
            customText: text || undefined
        });
    };
    return (<div class="space-y-6">
      {/* Question Header */}
      <div>
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white mb-2">
          {solidProps1.question.text}
        </h3>
      </div>

      {/* Multi-select hint */}
      {isMultiSelect && (<p class="text-sm text-gray-500 dark:text-gray-500 italic">
          {solidState2.t('BiChat.Question.SelectMulti')}
        </p>)}

      {/* Options Grid */}
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {(solidProps1.question.options || []).map((option) => {
            const isSelected = selectedOptions.includes(option.id);
            return (<button onClick={() => handleOptionClick(option.id)} class={`
                cursor-pointer relative p-4 text-left border-2 rounded-lg transition-all
                ${isSelected
                    ? 'border-primary-500 bg-white dark:bg-gray-800'
                    : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 hover:border-gray-300 dark:hover:border-gray-600'}
                focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-gray-900
              `} type="button" aria-pressed={isSelected}>
              <div class="flex items-start gap-3">
                {/* Radio or Checkbox */}
                <div class="flex-shrink-0 mt-1">
                  {isMultiSelect ? (<input type="checkbox" checked={isSelected} readOnly class="w-5 h-5 text-primary-600 border-gray-300 rounded focus:ring-0 dark:bg-gray-700 dark:border-gray-600"/>) : (<input type="radio" checked={isSelected} readOnly class="w-5 h-5 text-primary-600 border-gray-300 focus:ring-0 dark:bg-gray-700 dark:border-gray-600"/>)}
                </div>

                {/* Label */}
                <div class="flex-1 min-w-0">
                  <p class="font-medium text-gray-900 dark:text-white">
                    {option.label}
                  </p>
                </div>
              </div>
            </button>);
        })}
      </div>

      {/* "Other" Text Input - always shown */}
      <div>
        <label for="other-input" class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          {solidState2.t('BiChat.Question.SpecifyOther')}:
        </label>
        <textarea id="other-input" value={otherText()} onInput={(e) => handleOtherTextChange(e.target.value)} placeholder={solidState2.t('BiChat.Question.OtherOption')} rows={3} class="w-full px-4 py-3 border-2 border-gray-200 dark:border-gray-700 rounded-lg
            bg-white dark:bg-gray-800 text-gray-900 dark:text-white
            placeholder-gray-400 dark:placeholder-gray-500
            focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500
            resize-none"/>
      </div>
    </div>);
}
