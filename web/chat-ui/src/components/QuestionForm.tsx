import { type JSX, mergeProps, createSignal, createMemo, createEffect, on, onMount, onCleanup, untrack, createContext, useContext, lazy, Suspense, createUniqueId, Show, For } from "solid-js";
/**
 * QuestionForm Component
 * Multi-step modal wizard for answering pending questions
 * Includes question steps and confirmation step with progress indicator
 * Supports custom "Other" text input for all questions
 */
import { X } from '../icons';
import { type PendingQuestion, type QuestionAnswers, type QuestionAnswerData } from '../types';
import { useTranslation } from '../hooks/useTranslation';
import QuestionStep from './QuestionStep';
import ConfirmationStep from './ConfirmationStep';
import { LoadingSpinner } from './LoadingSpinner';
import { isQuestionAnswered, validateAnswers } from '../utils/questionFormUtils';
interface QuestionFormProps {
    pendingQuestion: PendingQuestion;
    sessionId: string;
    onSubmit: (answers: QuestionAnswers) => Promise<void>;
    onCancel: () => void;
}
export default function QuestionForm(solidProps1: QuestionFormProps) {
    const solidState2 = useTranslation();
    const [currentStep, setCurrentStep] = createSignal(0);
    const [answers, setAnswers] = createSignal<QuestionAnswers>({});
    const [isSubmitting, setIsSubmitting] = createSignal(false);
    const [error, setError] = createSignal<string | null>(null);
    const questions = solidProps1.pendingQuestion.questions;
    const isConfirmationStep = createMemo(() => currentStep() === questions.length);
    const isFirstStep = createMemo(() => currentStep() === 0);
    const isLastStep = createMemo(() => currentStep() === questions.length - 1);
    // Check if current question is answered
    const currentQuestionAnswered = createMemo(() => isConfirmationStep() ||
        (questions[currentStep()]?.id &&
            isQuestionAnswered(answers()[questions[currentStep()].id])));
    const handleAnswer = (answerData: QuestionAnswerData) => {
        const currentQuestion = questions[currentStep()];
        if (currentQuestion) {
            setAnswers((prev) => ({
                ...prev,
                [currentQuestion.id]: answerData,
            }));
        }
    };
    const handleNext = () => {
        if (!currentQuestionAnswered()) {
            return;
        }
        if (isLastStep()) {
            setCurrentStep(isConfirmationStep() ? currentStep() : currentStep() + 1);
        }
        else {
            setCurrentStep(currentStep() + 1);
        }
    };
    const handleBack = () => {
        if (currentStep() > 0) {
            setCurrentStep(currentStep() - 1);
        }
    };
    const handleSubmitAnswers = async () => {
        const validationError = validateAnswers(questions, answers(), solidState2.t);
        if (validationError) {
            setError(validationError);
            return;
        }
        setIsSubmitting(true);
        setError(null);
        try {
            await solidProps1.onSubmit?.(answers());
        }
        catch (err) {
            const errorMessage = err instanceof Error ? err.message : solidState2.t('BiChat.Error.Generic');
            setError(errorMessage);
            setIsSubmitting(false);
        }
    };
    // Calculate progress text
    const totalSteps = questions.length + 1;
    const progressText = createMemo(() => isConfirmationStep() ? solidState2.t('BiChat.QuestionForm.Step', { current: totalSteps, total: totalSteps })
        : solidState2.t('BiChat.QuestionForm.Step', { current: currentStep() + 1, total: totalSteps }));
    return (<div class="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50 p-4">
      {/* Modal Container */}
      <div class="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div class="border-b border-gray-200 dark:border-gray-700 p-6">
          <div class="flex items-center justify-between mb-4">
            <h2 class="text-2xl font-bold text-gray-900 dark:text-white">
              {solidState2.t('BiChat.QuestionForm.Title')}
            </h2>
            <button onClick={solidProps1.onCancel} disabled={isSubmitting()} class="cursor-pointer text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 disabled:opacity-50" aria-label={solidState2.t('BiChat.Common.Close')}>
              <X className="w-6 h-6"/>
            </button>
          </div>

          {/* Progress Indicator */}
          <div class="text-sm text-gray-600 dark:text-gray-400">
            {progressText()}
          </div>

          {/* Progress Bar */}
          <div class="mt-3 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
            <div class="h-full bg-primary-600 transition-all duration-300" style={{
            "width": `${((currentStep() + 1) / totalSteps) * 100}%`
        }}/>
          </div>
        </div>

        {/* Content */}
        <div class="p-6">
          {error() && (<div class="mb-6 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
              <p class="text-red-700 dark:text-red-400 text-sm">{error()}</p>
            </div>)}

          {isConfirmationStep() ? (<ConfirmationStep questions={questions} answers={answers()}/>) : (<QuestionStep question={questions[currentStep()]!} selectedAnswers={answers()} onAnswer={handleAnswer}/>)}
        </div>

        {/* Footer */}
        <div class="border-t border-gray-200 dark:border-gray-700 p-6 flex gap-3 justify-between">
          {/* Back Button */}
          {!isFirstStep() && (<button onClick={handleBack} disabled={isSubmitting()} class="cursor-pointer px-6 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 font-medium transition-colors">
              {solidState2.t('BiChat.QuestionForm.Back')}
            </button>)}

          <div class="flex-1"/>

          {isConfirmationStep() ? (<>
              {/* Cancel Button */}
              <button onClick={solidProps1.onCancel} disabled={isSubmitting()} class="cursor-pointer px-6 py-2 text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 font-medium transition-colors">
                {solidState2.t('BiChat.Message.Cancel')}
              </button>

              {/* Submit Button */}
              <button onClick={handleSubmitAnswers} disabled={isSubmitting()} class="cursor-pointer px-6 py-2 bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white rounded-lg font-medium transition-colors flex items-center gap-2">
                {isSubmitting() && <LoadingSpinner size="sm"/>}
                {isSubmitting() ? solidState2.t('BiChat.QuestionForm.Submitting') : solidState2.t('BiChat.QuestionForm.Confirm')}
              </button>
            </>) : (
        /* Next Button */
        <button onClick={handleNext} disabled={!currentQuestionAnswered() || isSubmitting()} class="cursor-pointer px-6 py-2 bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white rounded-lg font-medium transition-colors">
              {solidState2.t('BiChat.QuestionForm.Next')}
            </button>)}
        </div>
      </div>
    </div>);
}
