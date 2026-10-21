/**
 * Session artifact CRUD: fetch, upload, rename, delete artifacts.
 *
 * @internal — Not part of the public API. Consumed by HttpDataSource.
 */
import type { BichatRPC } from './rpc.generated';
import type { SessionArtifact } from '../types';
import { validateAttachmentFile, validateFileCount } from '../utils/fileUtils';
import { toSessionArtifact, type RPCArtifact } from './mappers';
import type { CoreUploadResponse } from './AttachmentUploader';
// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
type RPCCaller = <TMethod extends keyof BichatRPC & string>(method: TMethod, params: BichatRPC[TMethod]['params']) => Promise<BichatRPC[TMethod]['result']>;
// ---------------------------------------------------------------------------
// Artifact operations
// ---------------------------------------------------------------------------
export async function fetchSessionArtifacts(callRPC: RPCCaller, sessionId: string, options?: {
    limit?: number;
    offset?: number;
}): Promise<{
    artifacts: SessionArtifact[];
    hasMore?: boolean;
    nextOffset?: number;
}> {
    const limit = options?.limit ?? 50;
    const offset = options?.offset ?? 0;
    const data = await callRPC('bichat.session.artifacts', {
        sessionId,
        limit,
        offset,
    });
    const artifacts = (data.artifacts || []).map((artifact) => toSessionArtifact(artifact));
    const hasMore = typeof data.hasMore === 'boolean'
        ? data.hasMore
        : artifacts.length >= limit;
    const nextOffset = typeof data.nextOffset === 'number'
        ? data.nextOffset
        : offset + artifacts.length;
    return {
        artifacts,
        hasMore,
        nextOffset,
    };
}
export async function uploadSessionArtifacts(callRPC: RPCCaller, sessionId: string, files: File[], uploadFileFn: (file: File) => Promise<CoreUploadResponse>): Promise<{
    artifacts: SessionArtifact[];
}> {
    if (!Array.isArray(files) || files.length === 0) {
        return { artifacts: [] };
    }
    validateFileCount(0, files.length, 10);
    files.forEach((file) => validateAttachmentFile(file));
    const uploads = await Promise.all(files.map((file) => uploadFileFn(file)));
    const data = await callRPC('bichat.session.uploadArtifacts', {
        sessionId,
        attachments: uploads.map((upload) => ({
            id: String(upload.id),
            filename: upload.name,
            uploadId: upload.id,
            mimeType: upload.mimetype || 'application/octet-stream',
            sizeBytes: upload.size,
            url: upload.url,
        })),
    });
    return {
        artifacts: (data.artifacts || []).map((artifact) => toSessionArtifact(artifact)),
    };
}
export async function renameSessionArtifact(callRPC: RPCCaller, artifactId: string, name: string, description: string = ''): Promise<SessionArtifact> {
    const data = await callRPC('bichat.artifact.update', {
        id: artifactId,
        name,
        description,
    });
    return toSessionArtifact(data.artifact as RPCArtifact);
}
export async function deleteSessionArtifact(callRPC: RPCCaller, artifactId: string): Promise<void> {
    await callRPC('bichat.artifact.delete', { id: artifactId });
}
