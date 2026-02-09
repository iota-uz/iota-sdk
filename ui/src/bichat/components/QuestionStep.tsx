/**
 * QuestionStep Component
 * Displays a single question with options to select from
 * Supports both single-select (radio) and multi-select (checkboxes)
 * Includes automatic "Other" option for custom text input
 */

import { useState, useEffect } from 'react'
import { type Question, type QuestionAnswers, type QuestionAnswerData } from '../types'
import { useTranslation } from '../hooks/useTranslation'

interface QuestionStepProps {
  question: Question
  selectedAnswers: QuestionAnswers
  onAnswer: (answerData: QuestionAnswerData) => void
}

export default function QuestionStep({
  question,
  selectedAnswers,
  onAnswer,
}: QuestionStepProps) {
  const { t } = useTranslation()
  const answerData = selectedAnswers[question.id] || { options: [] }
  const selectedOptions = answerData.options || []
  const isMultiSelect = question.type === 'MULTIPLE_CHOICE'

  // Local state for "Other" text input
  const [otherText, setOtherText] = useState(answerData.customText || '')

  // Sync local state with props when switching questions
  useEffect(() => {
    const data = selectedAnswers[question.id] || { options: [] }
    setOtherText(data.customText || '')
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [question.id])

  const handleOptionClick = (optionLabel: string) => {
    if (isMultiSelect) {
      // Multi-select: toggle the option
      const newOptions = selectedOptions.includes(optionLabel)
        ? selectedOptions.filter((a) => a !== optionLabel)
        : [...selectedOptions, optionLabel]
      onAnswer({ options: newOptions, customText: otherText || undefined })
    } else {
      // Single-select: replace with new selection, preserve custom text
      onAnswer({ options: [optionLabel], customText: otherText || undefined })
    }
  }

  const handleOtherTextChange = (text: string) => {
    setOtherText(text)
    onAnswer({
      options: selectedOptions,
      customText: text || undefined
    })
  }

  return (
    <div className="space-y-6">
      {/* Question Header */}
      <div>
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
          {question.text}
        </h3>
      </div>

      {/* Multi-select hint */}
      {isMultiSelect && (
        <p className="text-sm text-gray-500 dark:text-gray-500 italic">
          {t('question.selectMulti')}
        </p>
      )}

      {/* Options Grid */}
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {(question.options || []).map((option) => {
          const isSelected = selectedOptions.includes(option.label)

          return (
            <button
              key={option.id}
              onClick={() => handleOptionClick(option.label)}
              className={`
                cursor-pointer relative p-4 text-left border-2 rounded-lg transition-all
                ${
                  isSelected
                    ? 'border-primary-500 bg-white dark:bg-gray-800'
                    : 'border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800 hover:border-gray-300 dark:hover:border-gray-600'
                }
                focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:focus:ring-offset-gray-900
              `}
              type="button"
              aria-pressed={isSelected}
            >
              <div className="flex items-start gap-3">
                {/* Radio or Checkbox */}
                <div className="flex-shrink-0 mt-1">
                  {isMultiSelect ? (
                    <input
                      type="checkbox"
                      checked={isSelected}
                      readOnly
                      className="w-5 h-5 text-primary-600 border-gray-300 rounded focus:ring-0 dark:bg-gray-700 dark:border-gray-600"
                    />
                  ) : (
                    <input
                      type="radio"
                      checked={isSelected}
                      readOnly
                      className="w-5 h-5 text-primary-600 border-gray-300 focus:ring-0 dark:bg-gray-700 dark:border-gray-600"
                    />
                  )}
                </div>

                {/* Label */}
                <div className="flex-1 min-w-0">
                  <p className="font-medium text-gray-900 dark:text-white">
                    {option.label}
                  </p>
                </div>
              </div>
            </button>
          )
        })}
      </div>

      {/* "Other" Text Input - always shown */}
      <div>
        <label htmlFor="other-input" className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
          {t('question.specifyOther')}:
        </label>
        <textarea
          id="other-input"
          value={otherText}
          onChange={(e) => handleOtherTextChange(e.target.value)}
          placeholder={t('question.other')}
          rows={3}
          className="w-full px-4 py-3 border-2 border-gray-200 dark:border-gray-700 rounded-lg
            bg-white dark:bg-gray-800 text-gray-900 dark:text-white
            placeholder-gray-400 dark:placeholder-gray-500
            focus:outline-none focus:ring-2 focus:ring-primary-500 focus:border-primary-500
            resize-none"
        />
      </div>
    </div>
  )
}
