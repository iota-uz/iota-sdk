import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [{
    name: 'lens-visual-regression-mode',
    transformIndexHtml: {
      order: 'pre',
      handler: () => [{
        tag: 'script',
        children: "if (new URLSearchParams(location.search).get('lens-vr') === '1') document.documentElement.dataset.lensVr = 'true'",
        injectTo: 'head-prepend',
      }],
    },
  }],
  server: {
    watch: {
      usePolling: true,
      interval: 300,
    },
  },
})
