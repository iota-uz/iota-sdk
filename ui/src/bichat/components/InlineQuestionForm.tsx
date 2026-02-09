/**
 * InlineQuestionForm component
 * Handles HITL (Human-in-the-Loop) questions from the AI agent
 *
 * Supports multiple questions with multi-step navigation:
 * - SINGLE_CHOICE: Clickable option cards (radio) + always-present "Other" text input
 * - MULTIPLE_CHOICE: Clickable option cards (checkbox) + always-present "Other" text input
 */

import { useState, useCallback } from 'react'
import {
  ArrowLeft,
  ArrowRight,
  Check,
  ChatCircleDots,
  PencilSimpleLine,
  PaperPlaneTilt,
  X,
} from '@phosphor-icons/react'
import { PendingQuestion, QuestionAnswers } from '../types'
import { useChat } from '../context/ChatContext'

interface InlineQuestionFormProps {
  pendingQuestion: PendingQuestion
}

export function InlineQuestionForm({ pendingQuestion }: InlineQuestionFormProps) {
  const { handleSubmitQuestionAnswers, handleRejectPendingQuestion, loading } = useChat()
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
  const canProceed = isCurrentAnswerValid()

  return (
    <div className="animate-slide-up rounded-2xl border border-primary-200 dark:border-primary-800/50 bg-gradient-to-b from-primary-50/80 to-white dark:from-primary-950/30 dark:to-gray-900/80 shadow-sm overflow-hidden">
      <form onSubmit={handleSubmit}>
        {/* Header bar */}
        <div className="flex items-center gap-2.5 px-4 pt-4 pb-3">
          <div className="flex items-center justify-center w-7 h-7 rounded-lg bg-primary-100 dark:bg-primary-900/40">
            <ChatCircleDots className="w-4 h-4 text-primary-600 dark:text-primary-400" weight="fill" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <span className="text-xs font-semibold uppercase tracking-wide text-primary-600 dark:text-primary-400">
                Input needed
              </span>
              {totalSteps > 1 && (
                <span className="text-[11px] tabular-nums text-gray-400 dark:text-gray-500">
                  {currentStep + 1}/{totalSteps}
                </span>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={handleRejectPendingQuestion}
            disabled={loading}
            className="cursor-pointer p-1 rounded-md text-gray-400 dark:text-gray-500 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors disabled:opacity-40"
            aria-label="Dismiss"
          >
            <X size={16} weight="bold" />
          </button>
        </div>

        {/* Progress dots for multi-step */}
        {totalSteps > 1 && (
          <div className="flex items-center gap-1.5 px-4 pb-3">
            {questions.map((_, index) => {
              const isCompleted = index < currentStep
              const isCurrent = index === currentStep
              return (
                <div
                  key={index}
                  className={[
                    'h-1 rounded-full transition-all duration-300',
                    isCurrent ? 'flex-[2] bg-primary-500 dark:bg-primary-400' : 'flex-1',
                    isCompleted
                      ? 'bg-primary-400 dark:bg-primary-500'
                      : !isCurrent
                        ? 'bg-gray-200 dark:bg-gray-700'
                        : '',
                  ].join(' ')}
                />
              )
            })}
          </div>
        )}

        {/* Question text */}
        <div className="px-4 pb-3">
          <p className="text-[15px] leading-relaxed text-gray-800 dark:text-gray-200">
            {currentQuestion.text}
          </p>
          {isMultiSelect && (
            <p className="mt-1 text-xs text-gray-400 dark:text-gray-500">
              Select all that apply
            </p>
          )}
        </div>

        {/* Options as clickable cards */}
        <div className="px-4 pb-2 space-y-1.5">
          {options.map((option) => {
            const isSelected = currentAnswer?.options.includes(option.label) || false
            return (
              <label
                key={option.id}
                className={[
                  'group/opt flex items-center gap-3 px-3 py-2.5 rounded-xl cursor-pointer',
                  'border transition-all duration-150',
                  isSelected
                    ? 'border-primary-300 dark:border-primary-600 bg-primary-50 dark:bg-primary-900/30 shadow-sm'
                    : 'border-transparent hover:border-gray-200 dark:hover:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/50',
                ].join(' ')}
              >
                {/* Custom indicator */}
                <span
                  className={[
                    'flex-shrink-0 flex items-center justify-center w-5 h-5 transition-all duration-150',
                    isMultiSelect ? 'rounded-md' : 'rounded-full',
                    isSelected
                      ? 'bg-primary-600 dark:bg-primary-500 border-primary-600 dark:border-primary-500 text-white shadow-sm'
                      : 'border-2 border-gray-300 dark:border-gray-600 group-hover/opt:border-gray-400 dark:group-hover/opt:border-gray-500',
                  ].join(' ')}
                >
                  {isSelected && <Check size={12} weight="bold" />}
                </span>
                <input
                  type={isMultiSelect ? 'checkbox' : 'radio'}
                  name={`question-${currentQuestion.id}`}
                  value={option.value}
                  checked={isSelected}
                  onChange={(e) => handleOptionChange(option.label, e.target.checked)}
                  className="sr-only"
                />
                <span className={[
                  'text-sm transition-colors duration-150',
                  isSelected
                    ? 'text-gray-900 dark:text-gray-100 font-medium'
                    : 'text-gray-700 dark:text-gray-300',
                ].join(' ')}>
                  {option.label}
                </span>
              </label>
            )
          })}

          {/* "Other" option */}
          <label
            className={[
              'group/opt flex items-center gap-3 px-3 py-2.5 rounded-xl cursor-pointer',
              'border transition-all duration-150',
              isOtherSelected
                ? 'border-primary-300 dark:border-primary-600 bg-primary-50 dark:bg-primary-900/30 shadow-sm'
                : 'border-transparent hover:border-gray-200 dark:hover:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800/50',
            ].join(' ')}
          >
            <span
              className={[
                'flex-shrink-0 flex items-center justify-center w-5 h-5 transition-all duration-150',
                isMultiSelect ? 'rounded-md' : 'rounded-full',
                isOtherSelected
                  ? 'bg-primary-600 dark:bg-primary-500 border-primary-600 dark:border-primary-500 text-white shadow-sm'
                  : 'border-2 border-gray-300 dark:border-gray-600 group-hover/opt:border-gray-400 dark:group-hover/opt:border-gray-500',
              ].join(' ')}
            >
              {isOtherSelected && <PencilSimpleLine size={11} weight="bold" />}
            </span>
            <input
              type={isMultiSelect ? 'checkbox' : 'radio'}
              name={`question-${currentQuestion.id}`}
              value="__other__"
              checked={isOtherSelected}
              onChange={(e) => handleOptionChange('__other__', e.target.checked)}
              className="sr-only"
            />
            <span className={[
              'text-sm transition-colors duration-150',
              isOtherSelected
                ? 'text-gray-900 dark:text-gray-100 font-medium'
                : 'text-gray-700 dark:text-gray-300',
            ].join(' ')}>
              Other
            </span>
          </label>

          {/* Other text input — slides open */}
          {isOtherSelected && (
            <div className="pl-8 pr-1 pb-1 animate-slide-up">
              <input
                type="text"
                value={currentOtherText}
                onChange={(e) => handleOtherTextChange(e.target.value)}
                placeholder="Type your answer..."
                autoFocus
                className="w-full px-3 py-2 text-sm border border-gray-200 dark:border-gray-700 rounded-lg bg-white dark:bg-gray-800 text-gray-900 dark:text-gray-100 placeholder:text-gray-400 dark:placeholder:text-gray-500 focus:outline-none focus:ring-2 focus:ring-primary-500/40 focus:border-primary-400 dark:focus:border-primary-600 transition-shadow"
              />
            </div>
          )}
        </div>

        {/* Footer with navigation */}
        <div className="flex items-center justify-between gap-2 px-4 pt-2 pb-4">
          <div>
            {!isFirstStep && (
              <button
                type="button"
                onClick={handleBack}
                className="cursor-pointer flex items-center gap-1 px-2.5 py-1.5 text-sm text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-200 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
              >
                <ArrowLeft size={14} weight="bold" />
                Back
              </button>
            )}
          </div>

          <button
            type="submit"
            disabled={loading || !canProceed}
            className={[
              'flex items-center gap-1.5 px-4 py-2 text-sm font-medium rounded-xl transition-all duration-150',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/50 focus-visible:ring-offset-2',
              canProceed
                ? 'cursor-pointer bg-primary-600 hover:bg-primary-700 active:bg-primary-800 text-white shadow-sm hover:shadow'
                : 'bg-gray-100 dark:bg-gray-800 text-gray-400 dark:text-gray-600 cursor-not-allowed',
            ].join(' ')}
          >
            {isLastStep ? (
              <>
                Submit
                <PaperPlaneTilt size={14} weight="fill" />
              </>
            ) : (
              <>
                Next
                <ArrowRight size={14} weight="bold" />
              </>
            )}
          </button>
        </div>
      </form>
    </div>
  )
}
