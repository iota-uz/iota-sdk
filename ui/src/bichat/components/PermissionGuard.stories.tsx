import type { Meta, StoryObj } from '@storybook/react'

import { PermissionGuard } from './PermissionGuard'
import { ScenarioGrid } from '@sb-helpers/ScenarioGrid'

const meta: Meta<typeof PermissionGuard> = {
  title: 'BiChat/Components/PermissionGuard',
  component: PermissionGuard,
}

export default meta
type Story = StoryObj<typeof PermissionGuard>

const Content = ({ label }: { label: string }) => (
  <div className="p-4 bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-300 rounded border border-green-200">
    Visible Content: {label}
  </div>
)

const Fallback = () => (
  <div className="p-4 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 rounded border border-red-200">
    Access Denied
  </div>
)

export const Playground: Story = {
  args: {
    permissions: ['admin'],
    hasPermission: (p) => p === 'admin',
    children: <Content label="Admin Area" />,
    fallback: <Fallback />,
  },
}

export const Stress: Story = {
  render: () => (
    <ScenarioGrid
      scenarios={[
        {
          name: 'Mode: All (Requires both)',
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
                <Content label="Should not see this" />
              </PermissionGuard>
            </div>
          ),
        },
        {
          name: 'Mode: Any (Requires one)',
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
              <PermissionGuard
                mode="any"
                permissions={['admin', 'editor']}
                hasPermission={() => false}
                fallback={<Fallback />}
              >
                <Content label="Should not see this" />
              </PermissionGuard>
            </div>
          ),
        },
        {
          name: 'Empty Permissions (Always shows)',
          content: (
            <PermissionGuard permissions={[]} hasPermission={() => false}>
              <Content label="Public Access" />
            </PermissionGuard>
          ),
        },
      ]}
    />
  ),
}
