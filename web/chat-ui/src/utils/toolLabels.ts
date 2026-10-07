/**
 * Tool label resolution for the Activity Trace.
 *
 * Convention: `BiChat.Tools.{toolName}` for SDK-provided tools.
 * Upstream consumers can extend with their own prefix (e.g., `Ali.Tools.{customTool}`).
 */
type TranslateFn = (key: string, params?: Record<string, string | number | boolean>) => string;
const ACRONYMS = new Set(['sql', 'kb', 'pdf', 'api', 'csv', 'http', 'url', 'id']);
function humanizeToolName(name: string): string {
    return name
        .split('_')
        .map((word) => (ACRONYMS.has(word) ? word.toUpperCase() : word.charAt(0).toUpperCase() + word.slice(1)))
        .join(' ');
}
/**
 * Resolve a human-readable label for a tool invocation.
 *
 * Lookup order:
 * 1. Consumer prefix (if provided), e.g. `Ali.Tools.custom_tool`
 * 2. SDK default prefix `BiChat.Tools.{name}`
 * 3. Fallback: humanise the raw tool name
 */
export function getToolLabel(t: TranslateFn, name: string, args?: string, prefix?: string): string {
    // 1. Consumer prefix
    if (prefix) {
        const key = `${prefix}.${name}`;
        const label = t(key, parseToolParams(name, args));
        if (label !== key) {
            return label;
        }
    }
    // 2. SDK default
    const sdkKey = `BiChat.Tools.${name}`;
    const sdkLabel = t(sdkKey, parseToolParams(name, args));
    if (sdkLabel !== sdkKey) {
        return sdkLabel;
    }
    // 3. Humanise fallback
    return humanizeToolName(name);
}
/**
 * Extract interpolation params from tool arguments for specific tools.
 */
function parseToolParams(name: string, args?: string): Record<string, string> | undefined {
    if (name === 'task' && args) {
        try {
            const parsed = JSON.parse(args) as Record<string, unknown>;
            return { agent: String(parsed.description ?? parsed.subagent_type ?? 'specialist') };
        }
        catch {
            return { agent: 'specialist' };
        }
    }
    return undefined;
}
