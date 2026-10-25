// Alpine's MutationObserver owns initialization and cleanup for normal, OOB,
// and history swaps. Manual initTree calls from HTMX events duplicate directive
// listeners in the bundled Alpine version. Keep this legacy asset as a no-op.
