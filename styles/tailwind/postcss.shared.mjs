export function createTailwindPostcssConfig(options = {}) {
  const { autoprefixer = true } = options;
  const plugins = {
    "@tailwindcss/postcss": {},
  };

  if (autoprefixer) {
    plugins.autoprefixer = {};
  }

  return { plugins };
}

export default createTailwindPostcssConfig();
