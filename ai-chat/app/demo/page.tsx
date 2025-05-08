"use client"

import { useState } from "react"
import ChatbotInterface from "../page"

export default function ChatbotDemo() {
  const [locale, setLocale] = useState<string>("ru")

  return (
    <div className="min-h-screen bg-gray-100">
      <div className="container mx-auto py-8">
        <div className="mb-8 bg-white p-4 rounded-lg shadow">
          <h1 className="text-2xl font-bold mb-4">AI Chatbot Language Demo</h1>
          <div className="flex gap-4">
            <button
              onClick={() => setLocale("ru")}
              className={`px-4 py-2 rounded-lg ${
                locale === "ru" ? "bg-blue-600 text-white" : "bg-gray-200 text-gray-800"
              }`}
            >
              Russian
            </button>
            <button
              onClick={() => setLocale("uz")}
              className={`px-4 py-2 rounded-lg ${
                locale === "uz" ? "bg-blue-600 text-white" : "bg-gray-200 text-gray-800"
              }`}
            >
              Uzbek
            </button>
            <button
              onClick={() => setLocale("en")}
              className={`px-4 py-2 rounded-lg ${
                locale === "en" ? "bg-blue-600 text-white" : "bg-gray-200 text-gray-800"
              }`}
            >
              English
            </button>
          </div>
        </div>

        <ChatbotInterface locale={locale} />
      </div>
    </div>
  )
}
