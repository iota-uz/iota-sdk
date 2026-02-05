/**
 * InlineQuestionForm component
 * Handles HITL (Human-in-the-Loop) questions from the AI agent
 *
 * Supports multiple questions with multi-step navigation:
 * - SINGLE_CHOICE: Radio buttons + always-present "Other" text input
 * - MULTIPLE_CHOICE: Checkboxes + always-present "Other" text input
 */

import { useState, useCallback } from 'react'
import { CaretLeft, CaretRight, Question } from '@phosphor-icons/react'
import { PendingQuestion, QuestionAnswers } from '../types'
import { useChat } from '../context/ChatContext'

interface InlineQuestionFormProps {
  pendingQuestion: PendingQuestion
}

export function InlineQuestionForm({ pendingQuestion }: InlineQuestionFormProps) {
  const { handleSubmitQuestionAnswers, handleCancelPendingQuestion, loading } = useChat()
  const [currentStep, setCurrentStep] = useState(0)
  const [answers, setAnswers] = useState<QuestionAnswers>({})
  const [otherTexts, setOtherTexts] = useState<Record<string, string>>({})

  const questions = pendingQuestion.questions
  const currentQuestion = questions[currentStep]
  const isLastStep = currentStep === questions.length - 1
  const isFirstStep = currentStep === 0
  const totalSteps = questions.length

  // Get current answer for the current question
  const currentAnswer = answers[currentQuestion?.id]
  const currentOtherText = otherTexts[currentQuestion?.id] || ''

  const handleOptionChange = useCallback(
    (optionLabel: string, checked: boolean) => {
      if (!currentQuestion) return
      const questionId = currentQuestion.id
      const existingAnswer = answers[questionId] || { options: [] }
      const isOtherOption = optionLabel === '__other__'
      const isMultiSelect = currentQuestion.type === 'MULTIPLE_CHOICE'

      // "Other" is mutually exclusive with predefined options.
      if (isOtherOption) {
        setAnswers({
          ...answers,
          [questionId]: {
            options: [],
            customText: checked ? currentOtherText : undefined,
          },
        })
        return
      }

      let newOptions: string[]
      if (isMultiSelect) {
        // Multi-select: toggle option
        if (!checked) {
          newOptions = existingAnswer.options.filter((o) => o !== optionLabel)
        } else if (existingAnswer.options.includes(optionLabel)) {
          newOptions = existingAnswer.options
        } else {
          newOptions = [...existingAnswer.options, optionLabel]
        }
      } else {
        // Single-select: replace selection (radio)
        newOptions = checked ? [optionLabel] : []
      }

      setAnswers({
        ...answers,
        [questionId]: {
          options: newOptions,
          customText: undefined,
        },
      })
    },
    [currentQuestion, answers, currentOtherText]
  )

  const handleOtherTextChange = useCallback(
    (text: string) => {
      if (!currentQuestion) return
      const questionId = currentQuestion.id
      setOtherTexts({ ...otherTexts, [questionId]: text })

      // Update the answer with custom text ("Other" is selected when customText is set)
      setAnswers({
        ...answers,
        [questionId]: {
          options: [],
          customText: text,
        },
      })
    },
    [currentQuestion, answers, otherTexts]
  )

  const isCurrentAnswerValid = (): boolean => {
    if (!currentQuestion) return false

    const answer = answers[currentQuestion.id]
    const required = currentQuestion.required ?? true

    if (!answer) return !required

    const hasOptionSelection = answer.options.length > 0
    const hasOtherSelected = answer.customText !== undefined
    const hasOtherText = (answer.customText?.trim().length ?? 0) > 0

    if (!hasOptionSelection && !hasOtherSelected) {
      return !required
    }

    if (hasOptionSelection) {
      return true
    }

    // "Other" selected: require non-empty text if required
    return !required || hasOtherText
  }

  const handleNext = () => {
    if (!isCurrentAnswerValid()) return

    if (isLastStep) {
      handleSubmitQuestionAnswers(answers)
    } else {
      setCurrentStep(currentStep + 1)
    }
  }

  const handleBack = () => {
    if (!isFirstStep) {
      setCurrentStep(currentStep - 1)
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    handleNext()
  }

  if (!currentQuestion) return null

  const isMultiSelect = currentQuestion.type === 'MULTIPLE_CHOICE'
  const options = currentQuestion.options || []
  const isOtherSelected = currentAnswer?.customText !== undefined

  return (
    <div className="border border-amber-200 dark:border-amber-800 rounded-lg p-4 bg-amber-50 dark:bg-amber-900/20">
      <form onSubmit={handleSubmit}>
        {/* Header with progress */}
        <div className="flex items-start gap-2 mb-4">
          <Question
            className="w-5 h-5 text-amber-600 dark:text-amber-400 flex-shrink-0 mt-0.5"
            weight="fill"
          />
          <div className="flex-1">
            <div className="flex items-center justify-between mb-1">
              <h4 className="font-medium text-gray-900 dark:text-gray-100">Question from AI</h4>
              {totalSteps > 1 && (
                <span className="text-xs text-gray-500 dark:text-gray-400">
                  {currentStep + 1} of {totalSteps}
                </span>
              )}
            </div>
            <p className="text-gray-700 dark:text-gray-300">{currentQuestion.text}</p>
          </div>
        </div>

        {/* Progress bar for multi-step */}
        {totalSteps > 1 && (
          <div className="flex gap-1 mb-4">
            {questions.map((_, index) => (
              <div
                key={index}
                className={`h-1 flex-1 rounded-full transition-colors ${
                  index <= currentStep
                    ? 'bg-amber-500 dark:bg-amber-400'
                    : 'bg-gray-200 dark:bg-gray-700'
                }`}
              />
            ))}
          </div>
        )}

        {/* Question content */}
        <div className="space-y-2 mb-4">
          {options.map((option) => (
            <label
              key={option.id}
              className="flex items-center gap-2 p-2 hover:bg-amber-100 dark:hover:bg-amber-900/30 rounded cursor-pointer"
            >
              <input
                type={isMultiSelect ? 'checkbox' : 'radio'}
                name={`question-${currentQuestion.id}`}
                value={option.value}
                checked={currentAnswer?.options.includes(option.label) || false}
                onChange={(e) => handleOptionChange(option.label, e.target.checked)}
                className="w-4 h-4 text-amber-600 focus:ring-amber-500"
              />
              <span className="text-gray-900 dark:text-gray-100">{option.label}</span>
            </label>
          ))}

          {/* Always-present "Other" option */}
          <label className="flex items-center gap-2 p-2 hover:bg-amber-100 dark:hover:bg-amber-900/30 rounded cursor-pointer">
            <input
              type={isMultiSelect ? 'checkbox' : 'radio'}
              name={`question-${currentQuestion.id}`}
              value="__other__"
              checked={isOtherSelected}
              onChange={(e) => handleOptionChange('__other__', e.target.checked)}
              className="w-4 h-4 text-amber-600 focus:ring-amber-500"
            />
            <span className="text-gray-900 dark:text-gray-100">Other</span>
          </label>

          {/* Other text input */}
          {isOtherSelected && (
            <div className="ml-6">
              <input
                type="text"
                value={currentOtherText}
                onChange={(e) => handleOtherTextChange(e.target.value)}
                placeholder="Please specify..."
                className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-amber-500 focus:border-transparent"
              />
            </div>
          )}
        </div>

        {/* Navigation buttons */}
        <div className="flex items-center justify-between">
          <div className="flex gap-2">
            {!isFirstStep && (
              <button
                type="button"
                onClick={handleBack}
                className="flex items-center gap-1 px-3 py-2 text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-gray-100 transition-colors"
              >
                <CaretLeft size={16} weight="bold" />
                Back
              </button>
            )}
          </div>

          <div className="flex gap-2">
            <button
              type="button"
              onClick={handleCancelPendingQuestion}
              disabled={loading}
              className="px-4 py-2 border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-lg hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || !isCurrentAnswerValid()}
              className="flex items-center gap-1 px-4 py-2 bg-amber-600 text-white rounded-lg hover:bg-amber-700 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {isLastStep ? 'Submit' : 'Next'}
              {!isLastStep && <CaretRight size={16} weight="bold" />}
            </button>
          </div>
        </div>
      </form>
    </div>
  )
}
