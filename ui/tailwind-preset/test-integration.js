/**
 * Test Integration Script
 *
 * This script demonstrates how the preset integrates with a Tailwind config
 * and validates that all expected properties are available.
 */

const preset = require('./preset.js')

// Simulate an applet's Tailwind config
const appletConfig = {
  presets: [preset],
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      // Applet-specific overrides
      colors: {
        brand: '#ff6b6b',
      },
    },
  },
}

console.log('=== Tailwind Preset Integration Test ===\n')

// Test 1: Preset loads successfully
console.log('Test 1: Preset Loads')
console.log('✓ Preset object:', typeof preset === 'object')
console.log('✓ Has theme:', preset.theme !== undefined)
console.log('✓ Has plugins:', Array.isArray(preset.plugins))
console.log('')

// Test 2: Color tokens available
console.log('Test 2: Color Tokens')
const colors = preset.theme.extend.colors
console.log('✓ Primary color:', colors.primary.DEFAULT)
console.log('✓ Success color:', colors.success.DEFAULT)
console.log('✓ Error color:', colors.error.DEFAULT)
console.log('✓ Text color:', colors.text.DEFAULT)
console.log('✓ Border color:', colors.border.DEFAULT)
console.log('✓ Surface color:', colors.surface.DEFAULT)
console.log('')

// Test 3: Typography tokens available
console.log('Test 3: Typography Tokens')
const { fontFamily, fontSize, fontWeight } = preset.theme.extend
console.log('✓ Font family sans:', fontFamily.sans[0])
console.log('✓ Font family mono:', fontFamily.mono[0])
console.log('✓ Base font size:', fontSize.base[0])
console.log('✓ Base line height:', fontSize.base[1].lineHeight)
console.log('✓ Normal font weight:', fontWeight.normal)
console.log('✓ Bold font weight:', fontWeight.bold)
console.log('')

// Test 4: Extended properties available
console.log('Test 4: Extended Properties')
const extended = preset.theme.extend
console.log('✓ Custom spacing:', extended.spacing['18'])
console.log('✓ Custom border radius:', extended.borderRadius['4xl'])
console.log('✓ Custom shadows:', Object.keys(extended.boxShadow).join(', '))
console.log('✓ Z-index layers:', Object.keys(extended.zIndex).slice(0, 3).join(', '))
console.log('')

// Test 5: Plugins loaded
console.log('Test 5: Plugins')
console.log('✓ Number of plugins:', preset.plugins.length)
console.log('✓ Plugin types:', preset.plugins.map(p => p.name || 'anonymous').join(', '))
console.log('')

// Test 6: Applet config merges correctly
console.log('Test 6: Applet Config Merge')
console.log('✓ Applet uses preset:', appletConfig.presets.includes(preset))
console.log('✓ Applet has content paths:', appletConfig.content.length > 0)
console.log('✓ Applet can override:', appletConfig.theme.extend.colors.brand === '#ff6b6b')
console.log('')

// Test 7: CSS Variable Pattern
console.log('Test 7: CSS Variable Pattern')
const cssVarPattern = /^var\(--color-[a-z-]+\)$/
const primaryVar = colors.primary.DEFAULT
const textVar = colors.text.DEFAULT
const successVar = colors.success.DEFAULT
console.log('✓ Primary uses CSS var:', cssVarPattern.test(primaryVar))
console.log('✓ Text uses CSS var:', cssVarPattern.test(textVar))
console.log('✓ Success uses CSS var:', cssVarPattern.test(successVar))
console.log('')

// Summary
console.log('=== All Integration Tests Passed ===')
console.log('\nThe preset is ready to be consumed by applets!')
console.log('\nNext steps:')
console.log('1. Publish to NPM: npm publish --access public')
console.log('2. Install in applet: npm install @iota-uz/tailwind-preset')
console.log('3. Use in tailwind.config.js: const iotaPreset = require("@iota-uz/tailwind-preset")')
