'use client';

import { useState } from 'react';
import { MessagesSquare, Bot, MessageCircle } from 'lucide-react';
import ChatbotInterface, { type FAQItem } from '@/components/chatbot-interface';

export default function Home() {
  const [locale, setLocale] = useState<string>('ru');
  const [apiEndpoint, setApiEndpoint] = useState<string>('http://localhost:3200/website/ai-chat');
  const [title, setTitle] = useState<string>('AI Chat Bot');
  const [subtitle, setSubtitle] = useState<string>('Our AI bot is ready to help you 24/7');
  const [selectedIcon, setSelectedIcon] = useState<string>('message-circle');

  // Custom FAQ items for demo
  const [faqItems, setFaqItems] = useState<FAQItem[]>([
    { id: 'extend-policy', question: 'Как продлить полис?' },
    { id: 'find-contract', question: 'Где найти номер договора?' },
    { id: 'submit-claim', question: 'Как подать заявление на страховой случай?' },
  ]);

  return (
    <div className="min-h-screen bg-gray-100">
      <div className="container mx-auto py-8">
        <div className="mb-8 bg-white p-6 rounded-lg shadow">
          <h1 className="text-2xl font-bold mb-6">AI Chatbot Configuration Demo</h1>

          {/* Language Selection */}
          <div className="mb-6">
            <h2 className="text-lg font-medium mb-3">Language</h2>
            <div className="flex flex-wrap gap-3">
              <button
                onClick={() => setLocale('ru')}
                className={`px-6 py-3 rounded-lg text-base font-medium ${locale === 'ru' ? 'bg-blue-600 text-white shadow-md' : 'bg-gray-200 text-gray-800 hover:bg-gray-300'
                  }`}
              >
                Russian
              </button>
              <button
                onClick={() => setLocale('uz')}
                className={`px-6 py-3 rounded-lg text-base font-medium ${locale === 'uz' ? 'bg-blue-600 text-white shadow-md' : 'bg-gray-200 text-gray-800 hover:bg-gray-300'
                  }`}
              >
                Uzbek (Latin)
              </button>
              <button
                onClick={() => setLocale('uzCyrl')}
                className={`px-6 py-3 rounded-lg text-base font-medium ${locale === 'uzCyrl'
                  ? 'bg-blue-600 text-white shadow-md'
                  : 'bg-gray-200 text-gray-800 hover:bg-gray-300'
                  }`}
              >
                Uzbek (Cyrillic)
              </button>
              <button
                onClick={() => setLocale('en')}
                className={`px-6 py-3 rounded-lg text-base font-medium ${locale === 'en' ? 'bg-blue-600 text-white shadow-md' : 'bg-gray-200 text-gray-800 hover:bg-gray-300'
                  }`}
              >
                English
              </button>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6 mb-6">
            {/* Title and Subtitle Configuration */}
            <div>
              <h2 className="text-lg font-medium mb-3">Chatbot Title</h2>
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="Enter chatbot title"
              />
            </div>
            <div>
              <h2 className="text-lg font-medium mb-3">Chatbot Subtitle</h2>
              <input
                type="text"
                value={subtitle}
                onChange={(e) => setSubtitle(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="Enter chatbot subtitle"
              />
            </div>
          </div>

          {/* Chat Icon Selection */}
          <div className="mb-6">
            <h2 className="text-lg font-medium mb-3">Chat Icon</h2>
            <div className="flex flex-wrap gap-3">
              <button
                onClick={() => setSelectedIcon('message-circle')}
                className={`px-6 py-3 rounded-lg text-base font-medium flex items-center ${
                  selectedIcon === 'message-circle' ? 'bg-blue-600 text-white shadow-md' : 'bg-gray-200 text-gray-800 hover:bg-gray-300'
                }`}
              >
                <MessageCircle className="mr-2" size={20} />
                Default
              </button>
              <button
                onClick={() => setSelectedIcon('messages-square')}
                className={`px-6 py-3 rounded-lg text-base font-medium flex items-center ${
                  selectedIcon === 'messages-square' ? 'bg-blue-600 text-white shadow-md' : 'bg-gray-200 text-gray-800 hover:bg-gray-300'
                }`}
              >
                <MessagesSquare className="mr-2" size={20} />
                Messages Square
              </button>
              <button
                onClick={() => setSelectedIcon('bot')}
                className={`px-6 py-3 rounded-lg text-base font-medium flex items-center ${
                  selectedIcon === 'bot' ? 'bg-blue-600 text-white shadow-md' : 'bg-gray-200 text-gray-800 hover:bg-gray-300'
                }`}
              >
                <Bot className="mr-2" size={20} />
                Bot
              </button>
            </div>
          </div>

          <div className="grid grid-cols-1 gap-6">

            {/* API Endpoint Input */}
            <div>
              <h2 className="text-lg font-medium mb-3">API Endpoint</h2>
              <div className="flex gap-2">
                <input
                  type="text"
                  id="apiEndpoint"
                  value={apiEndpoint}
                  onChange={(e) => setApiEndpoint(e.target.value)}
                  className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                  placeholder="Enter API endpoint"
                />
                <button
                  onClick={() => setApiEndpoint('http://localhost:3200/website/ai-chat')}
                  className="px-4 py-2 bg-gray-200 text-gray-800 rounded-lg hover:bg-gray-300"
                >
                  Reset
                </button>
              </div>
              <p className="text-xs text-gray-500 mt-1">Enter the direct URL to your IOTA SDK server</p>
            </div>

            {/* FAQ Items Configuration */}
            <div>
              <h2 className="text-lg font-medium mb-3">FAQ Items</h2>
              <div className="space-y-3">
                {faqItems.map((faq, index) => (
                  <div key={faq.id} className="flex gap-2">
                    <input
                      type="text"
                      value={faq.question}
                      onChange={(e) => {
                        const newFaqItems = [...faqItems];
                        newFaqItems[index] = { ...faq, question: e.target.value };
                        setFaqItems(newFaqItems);
                      }}
                      className="flex-1 px-3 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                      placeholder={`FAQ Question ${index + 1}`}
                    />
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>

        <ChatbotInterface
          locale={locale}
          apiEndpoint={apiEndpoint}
          faqItems={faqItems}
          title={title}
          subtitle={subtitle}
          chatIcon={
            selectedIcon === 'messages-square' ? (
              <MessagesSquare className="text-white" size={24} />
            ) : selectedIcon === 'bot' ? (
              <Bot className="text-white" size={24} />
            ) : (
              <MessageCircle className="text-white" size={24} />
            )
          }
        />
      </div>
    </div>
  );
}
