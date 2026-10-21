import type { SessionArtifact } from '../types';
export const FALLBACK_ARTIFACT_NAME = 'Untitled artifact';
export function getArtifactName(artifact: SessionArtifact): string {
    const name = artifact.name?.trim();
    return name && name.length > 0 ? name : FALLBACK_ARTIFACT_NAME;
}
export function isImageArtifact(artifact: SessionArtifact): boolean {
    const mime = artifact.mimeType?.toLowerCase() || '';
    const name = getArtifactName(artifact).toLowerCase();
    return mime.startsWith('image/') || /\.(png|jpe?g|gif|webp|svg|bmp)$/.test(name);
}
export function isPDFArtifact(artifact: SessionArtifact): boolean {
    const mime = artifact.mimeType?.toLowerCase() || '';
    const name = getArtifactName(artifact).toLowerCase();
    return mime.includes('pdf') || name.endsWith('.pdf');
}
export function isOfficeDocumentArtifact(artifact: SessionArtifact): boolean {
    const mime = artifact.mimeType?.toLowerCase() || '';
    const name = getArtifactName(artifact).toLowerCase();
    return (mime.includes('wordprocessingml') ||
        mime.includes('msword') ||
        mime.includes('excel') ||
        mime.includes('spreadsheet') ||
        /\.(docx?|xlsx?|xlsm|xlsb)$/.test(name));
}
export function isTextArtifact(artifact: SessionArtifact): boolean {
    const mime = artifact.mimeType?.toLowerCase() || '';
    const name = getArtifactName(artifact).toLowerCase();
    return (mime.startsWith('text/') ||
        mime.includes('json') ||
        mime.includes('xml') ||
        mime.includes('yaml') ||
        mime.includes('csv') ||
        mime.includes('tab-separated') ||
        /\.(txt|md|json|xml|ya?ml|csv|tsv|log|sql)$/.test(name));
}
