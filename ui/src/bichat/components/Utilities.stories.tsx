import type { Meta, StoryObj } from '@storybook/react'
import { MagnifyingGlass } from '@phosphor-icons/react'

import { LoadingSpinner } from './LoadingSpinner'
import { Skeleton, SkeletonText, SkeletonAvatar, SkeletonCard } from './Skeleton'
import { TypingIndicator } from './TypingIndicator'
import { StreamingCursor } from './StreamingCursor'
import { Toast } from './Toast'
import { ToastContainer } from './ToastContainer'
import { ScrollToBottomButton } from './ScrollToBottomButton'
import { EmptyState } from './EmptyState'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta = {
  title: 'BiChat/Utilities',
  parameters: { layout: 'centered' },
}

export default meta

type Story = StoryObj

export const Loaders: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'LoadingSpinner',
          content: (
            <div className="flex flex-col gap-6">
              <div className="flex items-end gap-8">
                <LoadingSpinner size="sm" />
                <LoadingSpinner size="md" message="Loading..." />
                <LoadingSpinner size="lg" />
              </div>
              <div className="flex flex-col gap-4">
                <LoadingSpinner variant="spinner" message="Spinner" />
                <LoadingSpinner variant="dots" message="Dots" />
                <LoadingSpinner variant="pulse" message="Pulse" />
              </div>
            </div>
          ),
        },
        {
          name: 'Skeleton',
          content: (
            <div className="space-y-4 w-64">
              <Skeleton variant="text" width="60%" />
              <div className="flex items-center gap-3">
                <SkeletonAvatar size={40} />
                <SkeletonText lines={2} />
              </div>
              <Skeleton variant="rounded" width="100%" height={80} />
              <SkeletonCard height={60} />
            </div>
          ),
        },
        {
          name: 'TypingIndicator',
          content: (
            <div className="space-y-4">
              <TypingIndicator />
              <TypingIndicator verbs={['Analyzing data']} />
            </div>
          ),
        },
        {
          name: 'StreamingCursor',
          content: (
            <div className="space-y-2">
              <div className="flex items-center gap-1">
                <span>AI is typing</span>
                <StreamingCursor />
              </div>
              <div className="dark p-3 bg-gray-900 text-white rounded">
                Dark mode<StreamingCursor />
              </div>
            </div>
          ),
        },
      ]}
    />
  ),
}

export const Toasts: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Toast types',
          content: (
            <div className="flex flex-col gap-2 w-80">
              <Toast id="s" type="success" message="Success message." onDismiss={() => {}} />
              <Toast id="e" type="error" message="Error message." onDismiss={() => {}} />
              <Toast id="i" type="info" message="Info message." onDismiss={() => {}} />
              <Toast id="w" type="warning" message="Warning message." onDismiss={() => {}} />
            </div>
          ),
        },
        {
          name: 'ToastContainer (stacking)',
          content: (
            <div className="h-[220px] relative border border-dashed rounded-lg overflow-hidden w-80">
              <ToastContainer
                toasts={[
                  { id: '1', type: 'error', message: 'Failed to save.' },
                  { id: '2', type: 'success', message: 'File uploaded.' },
                  { id: '3', type: 'info', message: 'Welcome back!' },
                ]}
                onDismiss={() => {}}
              />
            </div>
          ),
        },
      ]}
    />
  ),
}

export const ScrollAndEmpty: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'ScrollToBottomButton',
          content: (
            <div className="flex gap-8 h-24 items-center relative border rounded bg-gray-50 overflow-hidden">
              <ScrollToBottomButton show onClick={() => {}} unreadCount={0} />
              <ScrollToBottomButton show onClick={() => {}} unreadCount={9} />
              <ScrollToBottomButton show onClick={() => {}} unreadCount={123} />
            </div>
          ),
        },
        {
          name: 'EmptyState',
          content: (
            <div className="space-y-6">
              <EmptyState
                size="sm"
                title="No results"
                description="Try adjusting filters."
                icon={<MagnifyingGlass size={32} className="text-gray-300" />}
              />
              <EmptyState size="md" title="Medium empty state" description="Standard page." />
              <EmptyState size="lg" title="Large hero" description="Landing empty state." />
            </div>
          ),
        },
      ]}
    />
  ),
}
