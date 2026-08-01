import stylelint from 'stylelint'

const ruleName = 'lens/no-raw-color-utilities'
const rawColorUtility = /lens-(?:bg|text|border)-(?:slate|blue|red|green|amber)-\d+/g

const semanticColorPlugin = stylelint.createPlugin(ruleName, (enabled) => (root, result) => {
  if (!enabled) return
  root.walkAtRules('apply', (rule) => {
    for (const match of rule.params.matchAll(rawColorUtility)) {
      stylelint.utils.report({
        message: `Use a Lens semantic token instead of ${match[0]}`,
        node: rule,
        result,
        ruleName,
        word: match[0],
      })
    }
  })
})

export default {
  plugins: [semanticColorPlugin],
  rules: { [ruleName]: true },
}
