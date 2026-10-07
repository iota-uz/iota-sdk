/**
 * Normalizes common LaTeX delimiters used by LLMs into remark-math compatible
 * delimiters while preserving fenced and inline code sections.
 *
 * Supported conversions:
 * - \[ ... \] -> $$ ... $$
 * - \( ... \) -> $ ... $
 */
export function normalizeLatexDelimiters(markdown: string): string {
    if (!markdown || !hasLatexDelimiters(markdown)) {
        return markdown;
    }
    const lines = markdown.split('\n');
    let inFence = false;
    let fenceMarker = '';
    const output: string[] = [];
    for (const line of lines) {
        const fence = parseFenceLine(line);
        if (fence) {
            if (!inFence) {
                inFence = true;
                fenceMarker = fence;
            }
            else if (fence[0] === fenceMarker[0] && fence.length >= fenceMarker.length) {
                inFence = false;
                fenceMarker = '';
            }
            output.push(line);
            continue;
        }
        if (inFence) {
            output.push(line);
            continue;
        }
        output.push(rewriteOutsideInlineCode(line));
    }
    return output.join('\n');
}
function hasLatexDelimiters(markdown: string): boolean {
    return markdown.includes('\\[') || markdown.includes('\\]') || markdown.includes('\\(') || markdown.includes('\\)');
}
function parseFenceLine(line: string): string | null {
    const match = line.match(/^\s*(`{3,}|~{3,})/);
    return match ? match[1] : null;
}
function rewriteOutsideInlineCode(line: string): string {
    const segments = line.split(/(`+[^`]*`+)/g);
    for (let i = 0; i < segments.length; i++) {
        const segment = segments[i];
        if (segment.startsWith('`') && segment.endsWith('`')) {
            continue;
        }
        segments[i] = segment
            .replace(/\\\[/g, () => '$$')
            .replace(/\\\]/g, () => '$$')
            .replace(/\\\(/g, '$')
            .replace(/\\\)/g, '$');
    }
    return segments.join('');
}
