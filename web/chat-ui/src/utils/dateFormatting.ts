/**
 * Date formatting utilities using date-fns
 */
import { differenceInMinutes, differenceInHours, differenceInDays, format } from 'date-fns';
/**
 * Format a date as relative time (e.g., "5m ago", "2h ago")
 * Falls back to HH:mm format for dates older than 24 hours
 *
 * Accepts an optional `t` function for i18n. Translation keys used:
 * - BiChat.RelativeTime.JustNow
 * - BiChat.RelativeTime.MinutesAgo (receives `{count}`)
 * - BiChat.RelativeTime.HoursAgo   (receives `{count}`)
 * - BiChat.RelativeTime.DaysAgo    (receives `{count}`)
 *
 * If no `t` function is provided, falls back to English defaults.
 */
export function formatRelativeTime(date: string | Date, t?: (key: string, params?: Record<string, string | number>) => string): string {
    const messageDate = new Date(date);
    const now = new Date();
    const diffMins = differenceInMinutes(now, messageDate);
    const diffHours = differenceInHours(now, messageDate);
    const diffDays = differenceInDays(now, messageDate);
    if (diffMins < 1) {
        return t ? t('BiChat.RelativeTime.JustNow') : 'Just now';
    }
    if (diffMins < 60) {
        return t ? t('BiChat.RelativeTime.MinutesAgo', { count: diffMins }) : `${diffMins}m ago`;
    }
    if (diffHours < 24) {
        return t ? t('BiChat.RelativeTime.HoursAgo', { count: diffHours }) : `${diffHours}h ago`;
    }
    if (diffDays <= 7) {
        return t ? t('BiChat.RelativeTime.DaysAgo', { count: diffDays }) : `${diffDays}d ago`;
    }
    return format(messageDate, 'HH:mm');
}
