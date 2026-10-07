import { describe, expect, it } from 'vitest';

import { bichatTailwindPreset } from './tailwind';

describe('bichatTailwindPreset', () => {
  it('exposes the semantic border contract to Tailwind consumers', () => {
    expect(bichatTailwindPreset.theme?.extend?.borderColor).toEqual({
      subtle: 'var(--bichat-color-border-subtle)',
      default: 'var(--bichat-color-border)',
      strong: 'var(--bichat-color-border-strong)',
      brand: 'var(--bichat-color-border-brand)',
      danger: 'var(--bichat-color-border-danger)',
      warning: 'var(--bichat-color-border-warning)',
      success: 'var(--bichat-color-border-success)',
      disabled: 'var(--bichat-color-border-disabled)',
    });
  });
});
