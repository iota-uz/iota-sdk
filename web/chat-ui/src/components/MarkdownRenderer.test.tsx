// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
vi.mock('../hooks/useTranslation', () => ({ useTranslation: () => ({ t: (key: string) => key, locale: 'en' }) }));
vi.mock('./ChartCard', () => ({ ChartCard: () => <div data-testid="chart-preview" /> }));
import { MarkdownRenderer } from './MarkdownRenderer';
afterEach(cleanup);
describe('Solid markdown renderer', () => {
  it('sanitizes links and raw HTML while preserving escaped code', () => {
    // False green: checking text alone would miss executable attributes in the DOM.
    const { container } = render(() => <MarkdownRenderer content={'[bad](javascript:alert(1))\n\n<script>alert(1)</script>\n\n```html\n<img src=x onerror=alert(1)>\n```'} />);
    expect(container.querySelector('script')).toBeNull();
    expect(container.querySelector('[onerror]')).toBeNull();
    expect(container.querySelector('a')?.getAttribute('href')).toBeNull();
    expect(container.querySelector('pre')?.textContent).toContain('<img src=x onerror=alert(1)>');
  });
  it('updates a streaming code block without replacing its DOM owner', () => {
    // False green: one static render would hide captured props and lost clipboard state.
    const [content, setContent] = createSignal('```ts\nconst answer = 1;\n```');
    const { container } = render(() => <MarkdownRenderer content={content()} />);
    const block = container.querySelector('pre');
    setContent('```ts\nconst answer = 42;\n```');
    expect(container.querySelector('pre')).toBe(block);
    expect(block?.textContent).toContain('42');
  });
  it('renders math in the MathML namespace and exports GFM tables', () => {
    // False green: serialized text would accept HTML elements with the wrong math namespace.
    const { container } = render(() => <MarkdownRenderer sendMessage={() => {}} content={'$x^2$\n\n| Name | Value |\n| --- | --- |\n| A | 42 |'} />);
    expect(container.querySelector('math')?.namespaceURI).toBe('http://www.w3.org/1998/Math/MathML');
    expect(container.querySelector('table')?.textContent).toContain('42');
    expect(container.querySelector('button')).not.toBeNull();
  });
});
