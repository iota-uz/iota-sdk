import { StorybookConfig } from '@storybook/react-vite'
import { mergeConfig } from 'vite'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const config: StorybookConfig = {
  stories: ['../src/**/*.stories.@(ts|tsx)'],
  addons: [
    '@storybook/addon-essentials',
    '@storybook/addon-viewport',
  ],
  framework: {
    name: '@storybook/react-vite',
    options: {},
  },
  staticDirs: [
    {
      from: '../../modules/core/presentation/assets',
      to: '/assets',
    },
  ],
  viteFinal: async (viteConfig) => {
    return mergeConfig(viteConfig, {
      resolve: {
        alias: {
          '@ui': path.resolve(__dirname, '../src'),
          '@sb': path.resolve(__dirname),
          '@sb-helpers': path.resolve(__dirname, 'helpers'),
        },
      },
    })
  },
}

export default config
