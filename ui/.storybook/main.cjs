const path = require('node:path')

/** @type { import('@storybook/react-vite').StorybookConfig } */
const config = {
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
  async viteFinal(viteConfig) {
    const { mergeConfig } = await import('vite')
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

module.exports = config
