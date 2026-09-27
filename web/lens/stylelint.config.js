import stylelint from 'stylelint'

const ruleName = 'lens/no-raw-color-utilities'

// Every colour Tailwind ships under its own name. The list used to be the five
// families that happened to appear in the sheet, which let `lens-text-white`
// (and any family nobody had reached for yet) through — a named colour is a
// named colour whether or not it carries a numeric step.
const tailwindColorFamilies = [
  'slate', 'gray', 'zinc', 'neutral', 'stone', 'red', 'orange', 'amber', 'yellow',
  'lime', 'green', 'emerald', 'teal', 'cyan', 'sky', 'blue', 'indigo', 'violet',
  'purple', 'fuchsia', 'pink', 'rose',
].join('|')

// `transparent`, `current` and `inherit` are deliberately absent: they state an
// absence of colour or a deferral to context, which no token can express.
const rawColorUtility = new RegExp(
  String.raw`lens-(?:bg|text|border|divide|fill|stroke|ring|from|via|to)-(?:(?:${tailwindColorFamilies})-\d+|white|black)`,
  'g',
)

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
