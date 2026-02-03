const sdkThemeExtend = {
  fontFamily: {
    sans: ['Gilroy'],
  },
  backgroundColor: {
    surface: {
      100: 'oklch(var(--clr-surface-100))',
      200: 'oklch(var(--clr-surface-200))',
      300: 'oklch(var(--clr-surface-300))',
      400: 'oklch(var(--clr-surface-400))',
      500: 'oklch(var(--clr-surface-500))',
      600: 'oklch(var(--clr-surface-600))',
    },
    avatar: 'oklch(var(--clr-avatar-bg))',
  },
  borderColor: {
    primary: 'oklch(var(--clr-border-primary))',
    secondary: 'oklch(var(--clr-border-secondary))',
    green: 'oklch(var(--clr-border-green))',
    pink: 'oklch(var(--clr-border-pink))',
    yellow: 'oklch(var(--clr-border-yellow))',
    blue: 'oklch(var(--clr-border-blue))',
    purple: 'oklch(var(--clr-border-purple))',
  },
  textColor: {
    100: 'oklch(var(--clr-text-100))',
    200: 'oklch(var(--clr-text-200))',
    300: 'oklch(var(--clr-text-300))',
    green: 'oklch(var(--clr-text-green))',
    pink: 'oklch(var(--clr-text-pink))',
    yellow: 'oklch(var(--clr-text-yellow))',
    blue: 'oklch(var(--clr-text-blue))',
    purple: 'oklch(var(--clr-text-purple))',
    avatar: 'oklch(var(--clr-avatar-text))',
  },
  colors: {
    100: 'oklch(var(--clr-text-100))',
    200: 'oklch(var(--clr-text-200))',
    300: 'oklch(var(--clr-text-300))',
    green: 'oklch(var(--clr-text-green))',
    pink: 'oklch(var(--clr-text-pink))',
    yellow: 'oklch(var(--clr-text-yellow))',
    blue: 'oklch(var(--clr-text-blue))',
    purple: 'oklch(var(--clr-text-purple))',
    avatar: 'oklch(var(--clr-avatar-text))',
    black: {
      DEFAULT: 'oklch(var(--black))',
      950: 'oklch(var(--black-950))',
    },
    brand: {
      500: 'oklch(var(--primary-500) / <alpha-value>)',
      600: 'oklch(var(--primary-600) / <alpha-value>)',
      700: 'oklch(var(--primary-700) / <alpha-value>)',
    },
    gray: {
      50: 'oklch(var(--gray-50) / <alpha-value>)',
      100: 'oklch(var(--gray-100) / <alpha-value>)',
      200: 'oklch(var(--gray-200) / <alpha-value>)',
      300: 'oklch(var(--gray-300) / <alpha-value>)',
      400: 'oklch(var(--gray-400) / <alpha-value>)',
      500: 'oklch(var(--gray-500) / <alpha-value>)',
      600: 'oklch(var(--gray-600) / <alpha-value>)',
      700: 'oklch(var(--gray-700) / <alpha-value>)',
      800: 'oklch(var(--gray-800) / <alpha-value>)',
      900: 'oklch(var(--gray-900) / <alpha-value>)',
      950: 'oklch(var(--gray-950) / <alpha-value>)',
    },
    green: {
      50: 'oklch(var(--green-50) / <alpha-value>)',
      100: 'oklch(var(--green-100) / <alpha-value>)',
      200: 'oklch(var(--green-200) / <alpha-value>)',
      500: 'oklch(var(--green-500) / <alpha-value>)',
      600: 'oklch(var(--green-600) / <alpha-value>)',
    },
    red: {
      100: 'oklch(var(--red-100))',
      200: 'oklch(var(--red-200))',
      500: 'oklch(var(--red-500) / <alpha-value>)',
      600: 'oklch(var(--red-600) / <alpha-value>)',
      700: 'oklch(var(--red-700) / <alpha-value>)',
    },
    badge: {
      pink: 'oklch(var(--clr-badge-pink))',
      yellow: 'oklch(var(--clr-badge-yellow))',
      green: 'oklch(var(--clr-badge-green))',
      blue: 'oklch(var(--clr-badge-blue))',
      purple: 'oklch(var(--clr-badge-purple))',
      gray: 'oklch(var(--clr-badge-gray))',
    },
    success: {
      DEFAULT: 'oklch(var(--green-500) / <alpha-value>)',
    },
    on: {
      success: 'oklch(var(--white))',
    },
  },
}

function isPlainObject(value) {
  return (
    value !== null &&
    typeof value === 'object' &&
    !Array.isArray(value) &&
    Object.prototype.toString.call(value) === '[object Object]'
  )
}

function mergeDeep(base, extra) {
  const out = { ...base }
  for (const [key, value] of Object.entries(extra ?? {})) {
    if (isPlainObject(out[key]) && isPlainObject(value)) {
      out[key] = mergeDeep(out[key], value)
      continue
    }
    out[key] = value
  }
  return out
}

function normalizeContent(content) {
  const list = Array.isArray(content) ? content.slice() : []
  const sdkGlob = './node_modules/@iotauz/iota-sdk/dist/**/*.{js,mjs,cjs}'
  if (!list.includes(sdkGlob)) list.push(sdkGlob)
  return list
}

function createIotaTailwindConfig(options = {}) {
  const content = normalizeContent(options.content)
  const extend = mergeDeep(sdkThemeExtend, options.extend ?? {})
  const plugins = Array.isArray(options.plugins) ? options.plugins : []

  return {
    darkMode: 'class',
    content,
    theme: { extend },
    plugins,
  }
}

module.exports = createIotaTailwindConfig
module.exports.createIotaTailwindConfig = createIotaTailwindConfig

