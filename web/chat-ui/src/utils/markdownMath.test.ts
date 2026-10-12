import { describe, expect, it } from 'vitest';
import { normalizeLatexDelimiters } from './markdownMath';

describe('normalizeLatexDelimiters', () => {
  it('converts display math delimiters', () => {
    const input = 'Формула:\n\\[ a = b + c \\]';
    const output = normalizeLatexDelimiters(input);
    expect(output).toContain('$$ a = b + c $$');
  });

  it('converts inline math delimiters', () => {
    const input = 'База: \\( premium - commission \\)';
    const output = normalizeLatexDelimiters(input);
    expect(output).toBe('База: $ premium - commission $');
  });

  it('does not rewrite fenced code blocks', () => {
    const input = ['```sql', "select '\\\\[' as marker", '```'].join('\n');
    const output = normalizeLatexDelimiters(input);
    expect(output).toBe(input);
  });

  it('does not rewrite inline code', () => {
    const input = 'Используй `\\[literal\\]` и формулу \\(x+y\\)';
    const output = normalizeLatexDelimiters(input);
    expect(output).toContain('`\\[literal\\]`');
    expect(output).toContain('формулу $x+y$');
  });
});
