/**
 * QuestionForm Component
 * Multi-step modal wizard for answering pending questions
 * Includes question steps and confirmation step with progress indicator
 * Supports custom "Other" text input for all questions
 */

import { useState } from 'react'
import { X } from '@phosphor-icons/react'
import { type PendingQuestion, type QuestionAnswers, type QuestionAnswerData } from '../types'
import { useTranslation } from '../hooks/useTranslation'
import QuestionStep from './QuestionStep'
import ConfirmationStep from './ConfirmationStep'
import { LoadingSpinner } from './LoadingSpinner'
import { isQuestionAnswered, validateAnswers } from '../utils/questionFormUtils'

interface QuestionFormProps {
  pendingQuestion: PendingQuestion
  sessionId: string
  onSubmit: (answers: QuestionAnswers) => Promise<void>
  onCancel: () => void
}

export default function QuestionForm({
  pendingQuestion,
  onSubmit,
  onCancel,
}: QuestionFormProps) {
  const { t } = useTranslation()
  const [currentStep, setCurrentStep] = useState(0)
  const [answers, setAnswers] = useState<QuestionAnswers>({})
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const questions = pendingQuestion.questions
  const isConfirmationStep = currentStep === questions.length
  const isFirstStep = currentStep === 0
  const isLastStep = currentStep === questions.length - 1

  // Check if current question is answered
  const currentQuestionAnswered =
    isConfirmationStep ||
    (questions[currentStep]?.id &&
      isQuestionAnswered(answers[questions[currentStep].id]))

  const handleAnswer = (answerData: QuestionAnswerData) => {
    const currentQuestion = questions[currentStep]
    if (currentQuestion) {
      setAnswers((prev) => ({
        ...prev,
        [currentQuestion.id]: answerData,
      }))
    }
  }

  const handleNext = () => {
    if (!currentQuestionAnswered) return
    if (isLastStep) {
      setCurrentStep(isConfirmationStep ? currentStep : currentStep + 1)
    } else {
      setCurrentStep(currentStep + 1)
    }
  }

  const handleBack = () => {
    if (currentStep > 0) {
      setCurrentStep(currentStep - 1)
    }
  }

  const handleSubmitAnswers = async () => {
    const validationError = validateAnswers(questions, answers, t)
    if (validationError) {
      setError(validationError)
      return
    }

    setIsSubmitting(true)
    setError(null)

    try {
      await onSubmit(answers)
    } catch (err) {
      const errorMessage =
        err instanceof Error ? err.message : t('error.generic')
      setError(errorMessage)
      setIsSubmitting(false)
    }
  }

  // Calculate progress text
  const totalSteps = questions.length + 1
  const progressText = isConfirmationStep
    ? t('questionForm.step', { current: totalSteps, total: totalSteps })
    : t('questionForm.step', { current: currentStep + 1, total: totalSteps })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-50 p-4">
      {/* Modal Container */}
      <div className="bg-white dark:bg-gray-800 rounded-lg shadow-xl max-w-2xl w-full max-h-[90vh] overflow-y-auto">
        {/* Header */}
        <div className="border-b border-gray-200 dark:border-gray-700 p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
              {t('questionForm.title')}
            </h2>
            <button
              onClick={onCancel}
              disabled={isSubmitting}
              className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 disabled:opacity-50"
              aria-label={t('common.close')}
            >
              <X className="w-6 h-6" />
            </button>
          </div>

          {/* Progress Indicator */}
          <div className="text-sm text-gray-600 dark:text-gray-400">
            {progressText}
          </div>

          {/* Progress Bar */}
          <div className="mt-3 h-2 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-primary-600 transition-all duration-300"
              style={{
                width: `${((currentStep + 1) / totalSteps) * 100}%`,
              }}
            />
          </div>
        </div>

        {/* Content */}
        <div className="p-6">
          {error && (
            <div className="mb-6 p-4 bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg">
              <p className="text-red-700 dark:text-red-400 text-sm">{error}</p>
            </div>
          )}

          {isConfirmationStep ? (
            <ConfirmationStep questions={questions} answers={answers} />
          ) : (
            <QuestionStep
              question={questions[currentStep]!}
              selectedAnswers={answers}
              onAnswer={handleAnswer}
            />
          )}
        </div>

        {/* Footer */}
        <div className="border-t border-gray-200 dark:border-gray-700 p-6 flex gap-3 justify-between">
          {/* Back Button */}
          {!isFirstStep && (
            <button
              onClick={handleBack}
              disabled={isSubmitting}
              className="px-6 py-2 text-gray-700 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 disabled:opacity-50 font-medium transition-colors"
            >
              {t('questionForm.back')}
            </button>
          )}

          <div className="flex-1" />

          {isConfirmationStep ? (
            <>
              {/* Cancel Button */}
              <button
                onClick={onCancel}
                disabled={isSubmitting}
                className="px-6 py-2 text-gray-700 dark:text-gray-300 border border-gray-300 dark:border-gray-600 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-700 disabled:opacity-50 font-medium transition-colors"
              >
                {t('message.cancel')}
              </button>

              {/* Submit Button */}
              <button
                onClick={handleSubmitAnswers}
                disabled={isSubmitting}
                className="px-6 py-2 bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white rounded-lg font-medium transition-colors flex items-center gap-2"
              >
                {isSubmitting && <LoadingSpinner size="sm" />}
                {isSubmitting ? t('questionForm.submitting') : t('questionForm.confirm')}
              </button>
            </>
          ) : (
            /* Next Button */
            <button
              onClick={handleNext}
              disabled={!currentQuestionAnswered || isSubmitting}
              className="px-6 py-2 bg-primary-600 hover:bg-primary-700 disabled:opacity-50 text-white rounded-lg font-medium transition-colors"
            >
              {t('questionForm.next')}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
