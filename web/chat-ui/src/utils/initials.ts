export interface InitialsSource {
    initials?: string | null;
    firstName?: string | null;
    lastName?: string | null;
}
/** Placeholder shown by avatars when no name data is available. */
export const UNKNOWN_INITIALS = '?';
/**
 * Derives avatar initials: explicit initials win, then the first letters of the
 * first and last name. Returns an empty string when nothing is known, so callers
 * can fall back to another source instead of a fabricated letter.
 */
export function deriveInitials(source: InitialsSource | null | undefined): string {
    const explicit = source?.initials?.trim();
    if (explicit) {
        return explicit.toUpperCase();
    }
    const first = Array.from(source?.firstName?.trim() ?? '')[0] ?? '';
    const last = Array.from(source?.lastName?.trim() ?? '')[0] ?? '';
    return `${first}${last}`.toUpperCase();
}
