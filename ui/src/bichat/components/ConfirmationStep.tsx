/**
 * ConfirmationStep Component
 * Displays a summary of all questions and selected answers for review before submission
 * Supports both predefined options and custom "Other" text
 */

import { type Question, type QuestionAnswers } from '../types'
import { useTranslation } from '../hooks/useTranslation'

interface ConfirmationStepProps {
  questions: Question[]
  answers: QuestionAnswers
}

export default function ConfirmationStep({
  questions,
  answers,
}: ConfirmationStepProps) {
  const { t } = useTranslation()

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
          {t('questionForm.reviewTitle')}
        </h3>
        <p className="text-gray-600 dark:text-gray-400 mt-1">
          {t('questionForm.reviewDescription')}
        </p>
      </div>

      {/* Questions Summary */}
      <div className="space-y-4">
        {questions.map((question) => {
          const answerData = answers[question.id] || { options: [] }
          const selectedOptions = answerData.options || []
          const customText = answerData.customText

          const hasAnswer = selectedOptions.length > 0 || !!customText

          return (
            <div
              key={question.id}
              className="p-4 border border-gray-200 dark:border-gray-700 rounded-lg bg-gray-50 dark:bg-gray-800"
            >
              {/* Question Text */}
              <h4 className="font-medium text-gray-900 dark:text-white mb-2">
                {question.text}
              </h4>

              {/* Selected Answers as Tags */}
              {hasAnswer ? (
                <div className="flex flex-wrap gap-2">
                  {/* Predefined options */}
                  {selectedOptions.map((option) => (
                    <span
                      key={option}
                      className="inline-flex items-center px-3 py-1 rounded-lg text-sm font-medium border border-primary-500 bg-primary-500/10 text-primary-600 dark:border-primary-400 dark:bg-primary-400/10 dark:text-primary-400"
                    >
                      {option}
                    </span>
                  ))}

                  {/* Custom "Other" text - displayed with distinct styling */}
                  {customText && (
                    <span className="inline-flex items-center px-3 py-1 rounded-lg text-sm font-medium border border-amber-500 bg-amber-500/10 text-amber-600 dark:border-amber-400 dark:bg-amber-400/10 dark:text-amber-400">
                      <span className="font-semibold mr-1">{t('question.other')}:</span>
                      <span className="italic">{customText}</span>
                    </span>
                  )}
                </div>
              ) : (
                <p className="text-sm text-gray-400 dark:text-gray-500 italic">
                  {t('questionForm.skip')}
                </p>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
