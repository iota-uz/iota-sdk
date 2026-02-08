/**
 * Shared validation utilities for QuestionForm components
 */

import type { Question, QuestionAnswerData, QuestionAnswers } from '../types'

/**
 * Checks if a question has been answered.
 * A question is answered if it has at least one selected option OR custom text.
 */
export function isQuestionAnswered(data: QuestionAnswerData | undefined): boolean {
  if (!data) return false
  return (data.options?.length ?? 0) > 0 || !!data.customText
}

/**
 * Validates that all questions are answered and custom text is valid.
 * Returns null if valid, or an error message string if invalid.
 *
 * @param questions - Array of questions to validate
 * @param answers - Map of question IDs to answer data
 * @param t - Optional translation function for error messages
 */
export function validateAnswers(
  questions: Question[],
  answers: QuestionAnswers,
  t?: (key: string, params?: Record<string, any>) => string
): string | null {
  const allAnswered = questions.every((q) => isQuestionAnswered(answers[q.id]))
  if (!allAnswered) {
    return t ? t('error.allQuestionsRequired') : 'Please answer all questions before submitting'
  }

  for (const q of questions) {
    const data = answers[q.id]
    if (data && (data.options?.length ?? 0) === 0 && data.customText === '') {
      return t
        ? t('error.customTextRequired', { question: q.text })
        : `Please enter custom text for question: ${q.text}`
    }
  }

  return null
}
