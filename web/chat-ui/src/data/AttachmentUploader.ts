/**
 * Attachment file processing and upload logic.
 *
 * Handles MIME detection from file signatures, filename normalization,
 * base64/URL decoding, and upload to the core upload endpoint.
 *
 * @internal — Not part of the public API. Consumed by HttpDataSource.
 */
import type { Attachment } from '../types';
import { isRecord } from '../utils/chartSpec';
import { validateAttachmentFile } from '../utils/fileUtils';
// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
export interface CoreUploadResponse {
    id: number;
    url: string;
    path: string;
    name: string;
    mimetype: string;
    size: number;
}
type AttachmentLifecycleEvent = 'attachment_decode_start' | 'attachment_decode_success' | 'attachment_decode_fail' | 'attachment_upload_start' | 'attachment_upload_success' | 'attachment_upload_fail' | 'stream_send_with_upload_ids';
// ---------------------------------------------------------------------------
// MIME detection constants
// ---------------------------------------------------------------------------
const MIME_TO_EXTENSION: Record<string, string> = {
    'image/jpeg': 'jpg',
    'image/png': 'png',
    'image/gif': 'gif',
    'application/pdf': 'pdf',
};
const SAFE_AUTOCORRECT_MIME_TYPES = new Set(Object.keys(MIME_TO_EXTENSION));
// ---------------------------------------------------------------------------
// MIME detection from file signature
// ---------------------------------------------------------------------------
function detectMimeFromSignature(bytes: Uint8Array): string | undefined {
    if (bytes.length >= 8) {
        const isPng = bytes[0] === 0x89 &&
            bytes[1] === 0x50 &&
            bytes[2] === 0x4e &&
            bytes[3] === 0x47 &&
            bytes[4] === 0x0d &&
            bytes[5] === 0x0a &&
            bytes[6] === 0x1a &&
            bytes[7] === 0x0a;
        if (isPng) {
            return 'image/png';
        }
    }
    if (bytes.length >= 3) {
        const isJpeg = bytes[0] === 0xff && bytes[1] === 0xd8 && bytes[2] === 0xff;
        if (isJpeg) {
            return 'image/jpeg';
        }
    }
    if (bytes.length >= 6) {
        const isGif = bytes[0] === 0x47 &&
            bytes[1] === 0x49 &&
            bytes[2] === 0x46 &&
            bytes[3] === 0x38 &&
            (bytes[4] === 0x37 || bytes[4] === 0x39) &&
            bytes[5] === 0x61;
        if (isGif) {
            return 'image/gif';
        }
    }
    if (bytes.length >= 4) {
        const isPdf = bytes[0] === 0x25 && bytes[1] === 0x50 && bytes[2] === 0x44 && bytes[3] === 0x46;
        if (isPdf) {
            return 'application/pdf';
        }
    }
    return undefined;
}
// ---------------------------------------------------------------------------
// Filename normalization
// ---------------------------------------------------------------------------
function normalizeFilenameForMime(filename: string, mimeType: string): string {
    const expectedExt = MIME_TO_EXTENSION[mimeType];
    if (!expectedExt) {
        return filename;
    }
    const lower = filename.toLowerCase();
    if (mimeType === 'image/jpeg' && (lower.endsWith('.jpg') || lower.endsWith('.jpeg'))) {
        return filename;
    }
    if (lower.endsWith(`.${expectedExt}`)) {
        return filename;
    }
    const dotIndex = filename.lastIndexOf('.');
    const baseName = dotIndex > 0 ? filename.slice(0, dotIndex) : filename;
    return `${baseName}.${expectedExt}`;
}
// ---------------------------------------------------------------------------
// Lifecycle logging
// ---------------------------------------------------------------------------
function logAttachmentLifecycle(event: AttachmentLifecycleEvent, details: Record<string, unknown>): void {
    const payload = {
        source: 'HttpDataSource',
        event,
        ...details,
    };
    if (event.endsWith('_fail')) {
        console.warn('[bichat.attachments]', payload);
        return;
    }
    // Non-failure lifecycle events: no console output (ESLint allows only warn/error)
}
// ---------------------------------------------------------------------------
// File normalization (MIME correction, extension fix)
// ---------------------------------------------------------------------------
async function normalizeAttachmentFile(attachment: Attachment, file: File): Promise<File> {
    const signatureBytes = new Uint8Array(await file.slice(0, 16).arrayBuffer());
    const detectedMimeType = detectMimeFromSignature(signatureBytes);
    const declaredMimeType = (attachment.mimeType || file.type || '').trim().toLowerCase();
    let resolvedMimeType = declaredMimeType || detectedMimeType || 'application/octet-stream';
    let correctedFromDeclared = false;
    if (detectedMimeType && declaredMimeType && detectedMimeType !== declaredMimeType) {
        const safeToCorrect = SAFE_AUTOCORRECT_MIME_TYPES.has(detectedMimeType) &&
            SAFE_AUTOCORRECT_MIME_TYPES.has(declaredMimeType);
        if (!safeToCorrect) {
            throw new Error(`Attachment "${attachment.filename}" MIME mismatch: declared "${declaredMimeType}", detected "${detectedMimeType}"`);
        }
        resolvedMimeType = detectedMimeType;
        correctedFromDeclared = true;
    }
    else if (detectedMimeType && !declaredMimeType) {
        resolvedMimeType = detectedMimeType;
    }
    const normalizedName = normalizeFilenameForMime(attachment.filename, resolvedMimeType);
    const normalized = new File([file], normalizedName, {
        type: resolvedMimeType,
        lastModified: file.lastModified,
    });
    logAttachmentLifecycle('attachment_decode_success', {
        attachmentKey: attachment.clientKey,
        filename: attachment.filename,
        normalizedFilename: normalized.name,
        declaredMimeType: declaredMimeType || undefined,
        detectedMimeType,
        resolvedMimeType,
        correctedFromDeclared,
        sizeBytes: normalized.size,
    });
    return normalized;
}
// ---------------------------------------------------------------------------
// Convert Attachment to File (from base64 or URL)
// ---------------------------------------------------------------------------
async function attachmentToFile(attachment: Attachment): Promise<File> {
    if (attachment.base64Data && attachment.base64Data.trim().length > 0) {
        try {
            const base64Data = attachment.base64Data.trim();
            const dataUrl = base64Data.startsWith('data:')
                ? base64Data
                : `data:${attachment.mimeType || 'application/octet-stream'};base64,${base64Data}`;
            const blob = await fetch(dataUrl).then((response) => response.blob());
            return new File([blob], attachment.filename, {
                type: attachment.mimeType || blob.type || 'application/octet-stream',
            });
        }
        catch (err) {
            const message = err instanceof Error ? err.message : 'Unknown decode error';
            throw new Error(`Attachment "${attachment.filename}" decode failed: ${message}`);
        }
    }
    if (attachment.url) {
        let parsed: URL;
        try {
            parsed = new URL(attachment.url, window.location?.origin ?? 'https://localhost');
            if (!['http:', 'https:'].includes(parsed.protocol)) {
                throw new Error(`Attachment "${attachment.filename}" URL has disallowed protocol: ${parsed.protocol}`);
            }
        }
        catch (err) {
            if (err instanceof Error && err.message.includes('Attachment')) {
                throw err;
            }
            throw new Error(`Attachment "${attachment.filename}" has invalid or malformed URL`);
        }
        const response = await fetch(parsed.href);
        if (!response.ok) {
            throw new Error(`Attachment "${attachment.filename}" decode failed: source HTTP ${response.status}`);
        }
        const blob = await response.blob();
        return new File([blob], attachment.filename, {
            type: attachment.mimeType || blob.type || 'application/octet-stream',
        });
    }
    throw new Error(`Attachment "${attachment.filename}" has no uploadable data`);
}
// ---------------------------------------------------------------------------
// Upload a single file
// ---------------------------------------------------------------------------
export async function uploadFile(file: File, baseUrl: string, uploadEndpoint: string, createUploadHeaders: () => Headers, fetcher: typeof fetch = fetch): Promise<CoreUploadResponse> {
    const formData = new FormData();
    formData.append('file', file);
    const response = await fetcher(`${baseUrl}${uploadEndpoint}`, {
        method: 'POST',
        headers: createUploadHeaders(),
        body: formData,
    });
    let payload: unknown = null;
    try {
        payload = await response.json();
    }
    catch {
        payload = null;
    }
    if (!response.ok) {
        const errorMessage = isRecord(payload) && typeof payload.error === 'string'
            ? payload.error
            : `Upload failed: HTTP ${response.status}`;
        throw new Error(errorMessage);
    }
    if (!isRecord(payload) || typeof payload.id !== 'number' || payload.id <= 0) {
        throw new Error('Upload failed: invalid response payload');
    }
    return {
        id: payload.id,
        url: typeof payload.url === 'string' ? payload.url : '',
        path: typeof payload.path === 'string' ? payload.path : '',
        name: typeof payload.name === 'string' ? payload.name : file.name,
        mimetype: typeof payload.mimetype === 'string' ? payload.mimetype : file.type,
        size: typeof payload.size === 'number' && Number.isFinite(payload.size) ? payload.size : file.size,
    };
}
// ---------------------------------------------------------------------------
// Assert upload references are valid
// ---------------------------------------------------------------------------
export function assertUploadReferences(uploads: CoreUploadResponse[]): Array<{
    uploadId: number;
}> {
    return uploads.map((upload, index) => {
        if (typeof upload.id !== 'number' || !Number.isFinite(upload.id) || upload.id <= 0) {
            throw new Error(`Attachment upload reference is invalid at index ${index}`);
        }
        return { uploadId: upload.id };
    });
}
// ---------------------------------------------------------------------------
// Ensure attachment is uploaded (reuse existing uploadId or upload new)
// ---------------------------------------------------------------------------
export async function ensureAttachmentUpload(attachment: Attachment, context: {
    sessionId: string;
    attachmentIndex: number;
}, uploadFileFn: (file: File) => Promise<CoreUploadResponse>): Promise<CoreUploadResponse> {
    if (typeof attachment.uploadId === 'number' && attachment.uploadId > 0) {
        logAttachmentLifecycle('attachment_upload_success', {
            sessionId: context.sessionId,
            attachmentIndex: context.attachmentIndex,
            attachmentKey: attachment.clientKey,
            filename: attachment.filename,
            uploadId: attachment.uploadId,
            reusedUploadId: true,
        });
        return {
            id: attachment.uploadId,
            url: attachment.url || '',
            path: '',
            name: attachment.filename,
            mimetype: attachment.mimeType,
            size: attachment.sizeBytes,
        };
    }
    logAttachmentLifecycle('attachment_decode_start', {
        sessionId: context.sessionId,
        attachmentIndex: context.attachmentIndex,
        attachmentKey: attachment.clientKey,
        filename: attachment.filename,
        hasBase64Data: Boolean(attachment.base64Data && attachment.base64Data.trim().length > 0),
        hasURL: Boolean(attachment.url),
    });
    let file: File;
    try {
        const rawFile = await attachmentToFile(attachment);
        file = await normalizeAttachmentFile(attachment, rawFile);
        validateAttachmentFile(file);
    }
    catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown attachment decode/validation error';
        logAttachmentLifecycle('attachment_decode_fail', {
            sessionId: context.sessionId,
            attachmentIndex: context.attachmentIndex,
            attachmentKey: attachment.clientKey,
            filename: attachment.filename,
            error: message,
        });
        throw new Error(message);
    }
    logAttachmentLifecycle('attachment_upload_start', {
        sessionId: context.sessionId,
        attachmentIndex: context.attachmentIndex,
        attachmentKey: attachment.clientKey,
        filename: file.name,
        mimeType: file.type,
        sizeBytes: file.size,
    });
    try {
        const upload = await uploadFileFn(file);
        // TODO: Refactor to return updated attachment instead of mutating; callers currently rely on this in-place update.
        attachment.uploadId = upload.id;
        attachment.mimeType = upload.mimetype || file.type;
        attachment.filename = upload.name || file.name;
        attachment.sizeBytes = upload.size || file.size;
        logAttachmentLifecycle('attachment_upload_success', {
            sessionId: context.sessionId,
            attachmentIndex: context.attachmentIndex,
            attachmentKey: attachment.clientKey,
            filename: attachment.filename,
            uploadId: upload.id,
            reusedUploadId: false,
        });
        return upload;
    }
    catch (err) {
        const message = err instanceof Error ? err.message : 'Unknown upload error';
        logAttachmentLifecycle('attachment_upload_fail', {
            sessionId: context.sessionId,
            attachmentIndex: context.attachmentIndex,
            attachmentKey: attachment.clientKey,
            filename: file.name,
            error: message,
        });
        throw new Error(`Attachment "${attachment.filename}" upload failed: ${message}`);
    }
}
