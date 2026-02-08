import { useState } from 'react'
import type { Meta, StoryObj } from '@storybook/react'

import { SearchInput } from './SearchInput'
import { EditableText } from './EditableText'
import { UserAvatar } from './UserAvatar'
import { PermissionGuard } from './PermissionGuard'
import { ErrorBoundary } from './ErrorBoundary'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'
import { largeText } from '@sb-helpers/textFixtures'

const meta: Meta = {
  title: 'BiChat/InputsAndGuards',
  parameters: { layout: 'centered' },
}

export default meta

type Story = StoryObj

const Content = ({ label }: { label: string }) => (
  <div className="p-4 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded border border-green-200">
    Visible: {label}
  </div>
)

const Fallback = () => (
  <div className="p-4 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 rounded border border-red-200">
    Access Denied
  </div>
)

const BuggyComponent = ({ message = 'Crash!' }: { message?: string }) => {
  throw new Error(message)
}

export const SearchInputDemo: Story = {
  render: function SearchInputStory() {
    const [value, setValue] = useState('')
    return (
      <ScenarioGrid
        scenarios={[
          {
            name: 'Sizes',
            content: (
              <div className="space-y-4 w-64">
                <SearchInput value="" onChange={() => {}} size="sm" placeholder="Small" />
                <SearchInput value="" onChange={() => {}} size="md" placeholder="Medium" />
                <SearchInput value="" onChange={() => {}} size="lg" placeholder="Large" />
              </div>
            ),
          },
          {
            name: 'Controlled',
            content: (
              <div className="w-64">
                <SearchInput value={value} onChange={setValue} placeholder="Search..." />
              </div>
            ),
          },
          {
            name: 'States',
            content: (
              <div className="space-y-4 w-64">
                <SearchInput value="With content" onChange={() => {}} />
                <SearchInput value="" onChange={() => {}} disabled placeholder="Disabled" />
              </div>
            ),
          },
        ]}
      />
    )
  },
}

export const EditableTextDemo: Story = {
  render: function EditableTextStory() {
    const [v1, setV1] = useState('Double click to edit')
    const [v2, setV2] = useState('')
    return (
      <ScenarioGrid
        scenarios={[
          {
            name: 'Sizes',
            content: (
              <div className="space-y-4">
                <EditableText size="sm" value="Small" onSave={() => {}} />
                <EditableText size="md" value="Medium" onSave={() => {}} />
                <EditableText size="lg" value="Large title" onSave={() => {}} />
              </div>
            ),
          },
          {
            name: 'Controlled',
            content: <EditableText value={v1} onSave={setV1} />,
          },
          {
            name: 'Loading & placeholder',
            content: (
              <div className="space-y-4">
                <EditableText value="Saving..." isLoading onSave={() => {}} />
                <EditableText value={v2} placeholder="Untitled" onSave={setV2} />
              </div>
            ),
          },
          {
            name: 'Long text (truncation)',
            content: <EditableText value={largeText} onSave={() => {}} className="max-w-[300px]" />,
          },
        ]}
      />
    )
  },
}

export const UserAvatarDemo: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Sizes',
          content: (
            <div className="flex items-end gap-4">
              <UserAvatar firstName="Small" lastName="S" size="sm" />
              <UserAvatar firstName="Medium" lastName="M" size="md" />
              <UserAvatar firstName="Large" lastName="L" size="lg" />
            </div>
          ),
        },
        {
          name: 'Initials & determinism',
          content: (
            <div className="flex gap-2">
              <UserAvatar firstName="Alice" lastName="Smith" />
              <UserAvatar firstName="Alice" lastName="Smith" />
              <UserAvatar firstName="Very Long" lastName="Name" initials="VL" />
            </div>
          ),
        },
      ]}
    />
  ),
}

export const PermissionGuardDemo: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Mode: all (requires both)',
          content: (
            <div className="space-y-4">
              <PermissionGuard
                mode="all"
                permissions={['read', 'write']}
                hasPermission={(p) => ['read', 'write'].includes(p)}
                fallback={<Fallback />}
              >
                <Content label="Has read & write" />
              </PermissionGuard>
              <PermissionGuard
                mode="all"
                permissions={['read', 'write']}
                hasPermission={(p) => p === 'read'}
                fallback={<Fallback />}
              >
                <Content label="Hidden (no write)" />
              </PermissionGuard>
            </div>
          ),
        },
        {
          name: 'Mode: any (requires one)',
          content: (
            <div className="space-y-4">
              <PermissionGuard
                mode="any"
                permissions={['admin', 'editor']}
                hasPermission={(p) => p === 'editor'}
                fallback={<Fallback />}
              >
                <Content label="Is editor" />
              </PermissionGuard>
              <PermissionGuard mode="any" permissions={['admin']} hasPermission={() => false} fallback={<Fallback />}>
                <Content label="Hidden" />
              </PermissionGuard>
            </div>
          ),
        },
        {
          name: 'Empty permissions (always show)',
          content: (
            <PermissionGuard permissions={[]} hasPermission={() => false}>
              <Content label="Public" />
            </PermissionGuard>
          ),
        },
      ]}
    />
  ),
}

export const ErrorBoundaryDemo: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Default fallback',
          content: (
            <ErrorBoundary>
              <BuggyComponent message="Controlled crash" />
            </ErrorBoundary>
          ),
        },
        {
          name: 'Custom fallback',
          content: (
            <ErrorBoundary
              fallback={(error, reset) => (
                <div className="p-4 border border-red-500 rounded bg-red-50 text-red-700">
                  <p className="font-bold">{error?.message}</p>
                  <button type="button" onClick={reset} className="mt-2 underline">
                    Retry
                  </button>
                </div>
              )}
            >
              <BuggyComponent />
            </ErrorBoundary>
          ),
        },
      ]}
    />
  ),
}
