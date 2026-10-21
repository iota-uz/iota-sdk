/**
 * Error display utilities for structured RPC error handling.
 * Distinguishes permission-denied errors (amber styling) from generic errors (red styling).
 */
import { AppletRPCException } from '../data/applet-rpc';
export interface RPCErrorDisplay {
    title: string;
    description: string;
    isPermissionDenied: boolean;
}
export interface NormalizedRPCError extends RPCErrorDisplay {
    code: string;
    retryable: boolean;
    userMessage: string;
    isTimeout: boolean;
    isOffline: boolean;
    isCanceled: boolean;
    isNotFound: boolean;
}
/**
 * Check whether an error represents a permission-denied / forbidden response.
 */
export function isPermissionDeniedError(error: unknown): boolean {
    if (!error) {
        return false;
    }
    if (error instanceof Error) {
        const msg = error.message.toLowerCase();
        if (msg.includes('forbidden') || msg.includes('permission denied')) {
            return true;
        }
    }
    if (typeof error === 'object' && error !== null) {
        const obj = error as Record<string, unknown>;
        // RPC error shape: { code: 'forbidden' } or { status: 403 }
        if (obj.code === 'forbidden' || obj.code === 403) {
            return true;
        }
        if (obj.status === 403) {
            return true;
        }
        if (obj.statusCode === 403) {
            return true;
        }
        // Nested response: { response: { status: 403 } }
        if (typeof obj.response === 'object' && obj.response !== null) {
            const resp = obj.response as Record<string, unknown>;
            if (resp.status === 403) {
                return true;
            }
        }
    }
    if (typeof error === 'string') {
        const lower = error.toLowerCase();
        if (lower.includes('forbidden') || lower.includes('permission denied')) {
            return true;
        }
    }
    return false;
}
function extractStatus(error: unknown): number | undefined {
    if (!error || typeof error !== 'object') {
        return undefined;
    }
    const obj = error as Record<string, unknown>;
    if (typeof obj.status === 'number') {
        return obj.status;
    }
    if (typeof obj.statusCode === 'number') {
        return obj.statusCode;
    }
    if (typeof obj.details === 'object' && obj.details !== null) {
        const details = obj.details as Record<string, unknown>;
        if (typeof details.status === 'number') {
            return details.status;
        }
        if (typeof details.statusCode === 'number') {
            return details.statusCode;
        }
    }
    if (typeof obj.response === 'object' && obj.response !== null) {
        const response = obj.response as Record<string, unknown>;
        if (typeof response.status === 'number') {
            return response.status;
        }
    }
    return undefined;
}
function isOfflineNow(): boolean {
    return typeof navigator !== 'undefined' && navigator.onLine === false;
}
function inferErrorCode(error: unknown): string {
    if (isOfflineNow()) {
        return 'offline';
    }
    if (error instanceof AppletRPCException) {
        const code = String(error.code || '').toLowerCase().trim();
        if (code) {
            return code;
        }
    }
    if (typeof error === 'object' && error !== null) {
        const obj = error as Record<string, unknown>;
        if (typeof obj.code === 'string' && obj.code.trim() !== '') {
            return obj.code.toLowerCase();
        }
        if (typeof obj.error === 'string' && obj.error.trim() !== '') {
            return obj.error.toLowerCase();
        }
    }
    const status = extractStatus(error);
    if (status === 401 || status === 403) {
        return 'forbidden';
    }
    if (status === 404) {
        return 'not_found';
    }
    if (status === 408) {
        return 'timeout';
    }
    if (status === 413) {
        return 'payload_too_large';
    }
    if (status === 429) {
        return 'rate_limited';
    }
    if (status && status >= 500) {
        return 'server_error';
    }
    if (status && status >= 400) {
        return 'bad_request';
    }
    let message = '';
    if (error instanceof Error) {
        message = error.message;
    }
    if (!message && typeof error === 'string') {
        message = error;
    }
    const lower = message.toLowerCase();
    if (lower.includes('timeout') || lower.includes('timed out')) {
        return 'timeout';
    }
    if (lower.includes('abort') || lower.includes('cancel')) {
        return 'aborted';
    }
    if (lower.includes('forbidden') || lower.includes('permission denied')) {
        return 'forbidden';
    }
    if (lower.includes('not found') || lower.includes('session not found')) {
        return 'not_found';
    }
    if (lower.includes('payload too large') || lower.includes('request too large') || lower.includes('413')) {
        return 'payload_too_large';
    }
    if (lower.includes('network') || lower.includes('failed to fetch')) {
        return 'network_error';
    }
    return 'unknown';
}
function describeCode(code: string, fallbackTitle: string): {
    title: string;
    description: string;
    retryable: boolean;
} {
    switch (code) {
        case 'offline':
            return {
                title: 'You are offline',
                description: 'Check your internet connection and try again.',
                retryable: true,
            };
        case 'timeout':
            return {
                title: 'Request timed out',
                description: 'The request took too long. Please try again.',
                retryable: true,
            };
        case 'aborted':
            return {
                title: 'Request canceled',
                description: 'The request was canceled before completion.',
                retryable: true,
            };
        case 'forbidden':
            return {
                title: 'Access denied',
                description: 'Your account does not have permission for this action.',
                retryable: false,
            };
        case 'not_found':
            return {
                title: 'Not found',
                description: 'The requested resource could not be found.',
                retryable: false,
            };
        case 'payload_too_large':
            return {
                title: 'Attachment too large',
                description: 'The uploaded payload exceeds allowed limits. Reduce file size and retry.',
                retryable: false,
            };
        case 'invalid_request':
        case 'validation':
        case 'bad_request':
            return {
                title: 'Invalid request',
                description: 'The request could not be processed. Review the input and try again.',
                retryable: false,
            };
        case 'rate_limited':
            return {
                title: 'Too many requests',
                description: 'Please wait a moment before trying again.',
                retryable: true,
            };
        case 'http_error':
        case 'server_error':
            return {
                title: 'Server error',
                description: 'The server failed to process this request. Please retry shortly.',
                retryable: true,
            };
        case 'network_error':
            return {
                title: 'Network error',
                description: 'A network issue interrupted the request. Please try again.',
                retryable: true,
            };
        default:
            return {
                title: fallbackTitle,
                description: 'Something went wrong. Please try again.',
                retryable: true,
            };
    }
}
export function normalizeRPCError(error: unknown, fallbackTitle: string): NormalizedRPCError {
    const code = inferErrorCode(error);
    const base = describeCode(code, fallbackTitle);
    const permissionDenied = code === 'forbidden' || isPermissionDeniedError(error);
    let description = base.description;
    if (error instanceof AppletRPCException && typeof error.message === 'string' && error.message.trim() !== '' && base.title === fallbackTitle) {
        description = error.message;
    }
    else if (error instanceof Error && error.message && code === 'unknown') {
        description = error.message;
    }
    return {
        code,
        title: base.title,
        description,
        userMessage: description || base.title,
        retryable: base.retryable,
        isPermissionDenied: permissionDenied,
        isTimeout: code === 'timeout',
        isOffline: code === 'offline',
        isCanceled: code === 'aborted',
        isNotFound: code === 'not_found',
    };
}
/**
 * Convert an unknown error into a structured display object.
 */
export function toErrorDisplay(error: unknown, fallbackTitle: string): RPCErrorDisplay {
    const normalized = normalizeRPCError(error, fallbackTitle);
    return {
        title: normalized.title,
        description: normalized.description,
        isPermissionDenied: normalized.isPermissionDenied,
    };
}
