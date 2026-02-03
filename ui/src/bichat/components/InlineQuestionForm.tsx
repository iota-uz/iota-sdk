/**
 * InlineQuestionForm component
 * Handles HITL (Human-in-the-Loop) questions from the AI agent
 */

import { useState } from 'react'
import { PendingQuestion, QuestionAnswers } from '../types'
import { useChat } from '../context/ChatContext'

interface InlineQuestionFormProps {
  pendingQuestion: PendingQuestion
}

export function InlineQuestionForm({ pendingQuestion }: InlineQuestionFormProps) {
  const { handleSubmitQuestionAnswers, handleCancelPendingQuestion } = useChat()
  const [answers, setAnswers] = useState<QuestionAnswers>({})
  const [textInput, setTextInput] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()

    if (pendingQuestion.type === 'MULTIPLE_CHOICE' && !answers.choice) {
      return
    }

    if (pendingQuestion.type === 'FREE_TEXT' && !textInput.trim()) {
      return
    }

    const finalAnswers =
      pendingQuestion.type === 'MULTIPLE_CHOICE'
        ? answers
        : { answer: textInput }

    handleSubmitQuestionAnswers(finalAnswers)
  }

  return (
    <div className="border border-[var(--bichat-border)] rounded-lg p-4 bg-yellow-50">
      <form onSubmit={handleSubmit}>
        <div className="flex items-start gap-2 mb-4">
          <svg
            className="w-5 h-5 text-yellow-600 flex-shrink-0 mt-0.5"
            fill="currentColor"
            viewBox="0 0 20 20"
          >
            <path
              fillRule="evenodd"
              d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-3a1 1 0 00-.867.5 1 1 0 11-1.731-1A3 3 0 0113 8a3.001 3.001 0 01-2 2.83V11a1 1 0 11-2 0v-1a1 1 0 011-1 1 1 0 100-2zm0 8a1 1 0 100-2 1 1 0 000 2z"
              clipRule="evenodd"
            />
          </svg>
          <div className="flex-1">
            <h4 className="font-medium text-gray-900 mb-2">
              Question from AI
            </h4>
            <p className="text-gray-700">{pendingQuestion.question}</p>
          </div>
        </div>

        {pendingQuestion.type === 'MULTIPLE_CHOICE' && pendingQuestion.options && (
          <div className="space-y-2 mb-4">
            {pendingQuestion.options.map((option, index) => (
              <label
                key={index}
                className="flex items-center gap-2 p-2 hover:bg-yellow-100 rounded cursor-pointer"
              >
                <input
                  type="radio"
                  name="choice"
                  value={option}
                  checked={answers.choice === option}
                  onChange={(e) =>
                    setAnswers({ ...answers, choice: e.target.value })
                  }
                  className="w-4 h-4 text-[var(--bichat-primary)]"
                />
                <span className="text-gray-900">{option}</span>
              </label>
            ))}
          </div>
        )}

        {pendingQuestion.type === 'FREE_TEXT' && (
          <div className="mb-4">
            <textarea
              value={textInput}
              onChange={(e) => setTextInput(e.target.value)}
              placeholder="Type your answer..."
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-[var(--bichat-primary)] focus:border-transparent resize-none"
              rows={3}
            />
          </div>
        )}

        <div className="flex gap-2">
          <button
            type="submit"
            className="px-4 py-2 bg-[var(--bichat-primary)] text-white rounded-lg hover:opacity-90 transition-opacity"
          >
            Submit Answer
          </button>
          <button
            type="button"
            onClick={handleCancelPendingQuestion}
            className="px-4 py-2 border border-gray-300 rounded-lg hover:bg-gray-50 transition-colors"
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  )
}
