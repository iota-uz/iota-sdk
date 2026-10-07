import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * ConfirmationStep Component
 * Displays a summary of all questions and selected answers for review before submission
 * Supports both predefined options and custom "Other" text
 */
import { type Question, type QuestionAnswers } from '../types';
import { useTranslation } from '../hooks/useTranslation';
interface ConfirmationStepProps {
    questions: Question[];
    answers: QuestionAnswers;
}
export default function ConfirmationStep(solidProps1: ConfirmationStepProps) {
    const solidState2 = useTranslation();
    return (<div class="space-y-6">
      {/* Header */}
      <div>
        <h3 class="text-lg font-semibold text-gray-900 dark:text-white">
          {solidState2.t('BiChat.QuestionForm.ReviewTitle')}
        </h3>
        <p class="text-gray-600 dark:text-gray-400 mt-1">
          {solidState2.t('BiChat.QuestionForm.ReviewDescription')}
        </p>
      </div>

      {/* Questions Summary */}
      <div class="space-y-4">
        {solidProps1.questions.map((question) => {
            const answerData = solidProps1.answers[question.id] || { options: [] };
            const selectedOptions = answerData.options || [];
            const customText = answerData.customText;
            const optionLabelByID = new Map((question.options || []).map((option) => [option.id, option.label]));
            const hasAnswer = selectedOptions.length > 0 || !!customText;
            return (<div class="p-4 border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-800">
              {/* Question Text */}
              <h4 class="font-medium text-gray-900 dark:text-white mb-2">
                {question.text}
              </h4>

              {/* Selected Answers as Tags */}
              {hasAnswer ? (<div class="flex flex-wrap gap-2">
                  {/* Predefined options */}
                  {selectedOptions.map((option) => (<span class="inline-flex items-center px-3 py-1 rounded-lg text-sm font-medium border border-primary-500 bg-primary-500/10 text-primary-600 dark:border-primary-400 dark:bg-primary-400/10 dark:text-primary-400">
                      {optionLabelByID.get(option) || option}
                    </span>))}

                  {/* Custom "Other" text - displayed with distinct styling */}
                  {customText && (<span class="inline-flex items-center px-3 py-1 rounded-lg text-sm font-medium border border-amber-500 bg-amber-500/10 text-amber-600 dark:border-amber-400 dark:bg-amber-400/10 dark:text-amber-400">
                      <span class="font-semibold mr-1">{solidState2.t('BiChat.Question.OtherOption')}:</span>
                      <span class="italic">{customText}</span>
                    </span>)}
                </div>) : (<p class="text-sm text-gray-400 dark:text-gray-500 italic">
                  {solidState2.t('BiChat.QuestionForm.Skip')}
                </p>)}
            </div>);
        })}
      </div>
    </div>);
}
