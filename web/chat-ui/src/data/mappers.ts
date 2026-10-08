/**
 * Type mapping and sanitization functions for converting RPC payloads
 * to domain types used by the BiChat UI.
 *
 * All functions are pure and side-effect-free (except console.warn for
 * malformed payloads).
 *
 * @internal — Not part of the public API. Consumed by HttpDataSource modules.
 */
import type { Session as RPCSession } from "./rpc.generated";
import type { PendingQuestion as RPCPendingQuestion } from "./rpc.generated";
import type { Session, SessionUser, SessionAccess, ConversationTurn, Artifact as DownloadArtifact, SessionArtifact, PendingQuestion, Question, Attachment, AssistantTurn, RenderTableData, } from "../types";
import { MessageRole } from "../types";
import { parseChartDataFromSpec, parseChartDataFromJsonString, isRecord, } from "../utils/chartSpec";
import { parseRenderTableDataFromJsonString, parseRenderTableDataFromMetadata, } from "../utils/tableSpec";
// ---------------------------------------------------------------------------
// Internal helper types
// ---------------------------------------------------------------------------
export interface RPCArtifact {
    id: string;
    sessionId: string;
    messageId?: string;
    uploadId?: number | null;
    type: string;
    name: string;
    description?: string;
    mimeType?: string;
    url?: string;
    sizeBytes: number;
    metadata?: Record<string, unknown>;
    createdAt: string;
}
// ---------------------------------------------------------------------------
// Logging helpers
// ---------------------------------------------------------------------------
export function warnMalformedSessionPayload(message: string, details?: Record<string, unknown>): void {
    console.warn(`[BiChat] ${message}`, details || {});
}
// ---------------------------------------------------------------------------
// Primitive readers
// ---------------------------------------------------------------------------
function readString(value: unknown, fallback = ""): string {
    return typeof value === "string" ? value : fallback;
}
function readNonEmptyString(value: unknown): string | null {
    if (typeof value !== "string") {
        return null;
    }
    const trimmed = value.trim();
    return trimmed.length > 0 ? trimmed : null;
}
function readFiniteNumber(value: unknown, fallback = 0): number {
    return typeof value === "number" && Number.isFinite(value) ? value : fallback;
}
function readOptionalFiniteNumber(value: unknown): number | undefined {
    return typeof value === "number" && Number.isFinite(value)
        ? value
        : undefined;
}
function extractFilenameFromURL(value: unknown): string | null {
    const raw = readNonEmptyString(value);
    if (!raw) {
        return null;
    }
    try {
        const parsed = new URL(raw);
        const path = parsed.pathname;
        if (!path) {
            return null;
        }
        const segments = path.split("/").filter(Boolean);
        if (segments.length === 0) {
            return null;
        }
        const candidate = decodeURIComponent(segments[segments.length - 1]);
        return readNonEmptyString(candidate);
    }
    catch {
        // Relative path fallback: extract last segment without URL parsing
        try {
            const segments = raw.split("/").filter(Boolean);
            if (segments.length === 0) {
                return null;
            }
            const candidate = decodeURIComponent(segments[segments.length - 1]);
            return readNonEmptyString(candidate);
        }
        catch {
            return null;
        }
    }
}
function resolveArtifactName(artifact: RPCArtifact): string {
    const explicit = readNonEmptyString(artifact.name);
    if (explicit) {
        return explicit;
    }
    const fromURL = extractFilenameFromURL(artifact.url);
    if (fromURL) {
        return fromURL;
    }
    const type = readNonEmptyString(artifact.type) || "artifact";
    const label = type.replace(/_/g, " ");
    if (label === "artifact" || label.endsWith(" artifact")) {
        return label;
    }
    return `${label} artifact`;
}
// ---------------------------------------------------------------------------
// Session mappers
// ---------------------------------------------------------------------------
function mapSessionUser(rawUser: unknown): SessionUser | undefined {
    if (!isRecord(rawUser)) {
        return undefined;
    }
    const rawId = rawUser.id;
    const id = readNonEmptyString(rawId) ??
        (typeof rawId === "number" && Number.isFinite(rawId)
            ? String(rawId)
            : null);
    if (!id) {
        return undefined;
    }
    const firstName = readString(rawUser.firstName);
    const lastName = readString(rawUser.lastName);
    const initials = readNonEmptyString(rawUser.initials) ||
        `${firstName.charAt(0)}${lastName.charAt(0)}`.trim().toUpperCase() ||
        "U";
    return {
        id,
        firstName,
        lastName,
        initials,
    };
}
function mapSessionAccess(rawAccess: unknown): SessionAccess | undefined {
    if (!isRecord(rawAccess)) {
        return undefined;
    }
    const role = readString(rawAccess.role).toLowerCase();
    const source = readString(rawAccess.source).toLowerCase();
    const normalizedRole: SessionAccess["role"] = role === "owner" ||
        role === "editor" ||
        role === "viewer" ||
        role === "read_all"
        ? role
        : "none";
    const normalizedSource: SessionAccess["source"] = source === "owner" || source === "member" || source === "permission"
        ? source
        : "none";
    const canRead = rawAccess.canRead === true || rawAccess.canRead === "true";
    const canWrite = rawAccess.canWrite === true || rawAccess.canWrite === "true";
    const canManageMembers = rawAccess.canManageMembers === true ||
        rawAccess.canManageMembers === "true";
    if (normalizedRole === "none" &&
        normalizedSource === "none" &&
        !canRead &&
        !canWrite &&
        !canManageMembers) {
        return undefined;
    }
    return {
        role: normalizedRole,
        source: normalizedSource,
        canRead,
        canWrite,
        canManageMembers,
    };
}
export function toSession(session: RPCSession): Session {
    return {
        id: readString(session.id),
        title: readString(session.title),
        status: session.status === "archived" ? "archived" : "active",
        pinned: Boolean(session.pinned),
        createdAt: readString(session.createdAt),
        updatedAt: readString(session.updatedAt),
        owner: mapSessionUser(session.owner),
        isGroup: Boolean(session.isGroup),
        memberCount: typeof session.memberCount === "number" ? session.memberCount : undefined,
        access: mapSessionAccess(session.access),
    };
}
export function toSessionArtifact(artifact: RPCArtifact): SessionArtifact {
    const rawCreatedAt = readNonEmptyString(artifact.createdAt);
    if (!rawCreatedAt) {
        warnMalformedSessionPayload("Artifact missing createdAt; defaulting to epoch", { id: artifact.id });
    }
    const createdAt = rawCreatedAt ?? "1970-01-01T00:00:00.000Z";
    return {
        id: readString(artifact.id),
        sessionId: readString(artifact.sessionId),
        messageId: readNonEmptyString(artifact.messageId) || undefined,
        uploadId: readOptionalFiniteNumber(artifact.uploadId),
        type: readNonEmptyString(artifact.type) || "other",
        name: resolveArtifactName(artifact),
        description: readNonEmptyString(artifact.description) || undefined,
        mimeType: readNonEmptyString(artifact.mimeType) || undefined,
        url: readNonEmptyString(artifact.url) || undefined,
        sizeBytes: Math.max(0, readFiniteNumber(artifact.sizeBytes)),
        metadata: isRecord(artifact.metadata) ? artifact.metadata : undefined,
        createdAt,
    };
}
// ---------------------------------------------------------------------------
// Normalizers
// ---------------------------------------------------------------------------
function normalizeQuestionType(rawType: unknown): "SINGLE_CHOICE" | "MULTIPLE_CHOICE" {
    const normalized = readString(rawType)
        .trim()
        .toUpperCase()
        .replace(/[\s-]+/g, "_");
    return normalized === "MULTIPLE_CHOICE" ? "MULTIPLE_CHOICE" : "SINGLE_CHOICE";
}
function normalizePendingQuestionStatus(rawStatus: unknown): PendingQuestion["status"] {
    const normalized = readString(rawStatus).trim().toUpperCase();
    switch (normalized) {
        case "ANSWER_SUBMITTED":
        case "REJECT_SUBMITTED":
        case "ANSWER_RESUME_FAILED":
        case "REJECT_RESUME_FAILED":
        case "ANSWERED":
        case "REJECTED":
        case "CANCELLED":
            return normalized;
        default:
            return "PENDING";
    }
}
function normalizeMessageRole(rawRole: unknown): MessageRole {
    const normalized = readString(rawRole).trim().toLowerCase();
    if (normalized === MessageRole.User) {
        return MessageRole.User;
    }
    if (normalized === MessageRole.System) {
        return MessageRole.System;
    }
    if (normalized === MessageRole.Tool) {
        return MessageRole.Tool;
    }
    return MessageRole.Assistant;
}
// ---------------------------------------------------------------------------
// Attachment sanitizers
// ---------------------------------------------------------------------------
function sanitizeAttachment(rawAttachment: unknown, turnId: string, index: number): Attachment | null {
    if (!isRecord(rawAttachment)) {
        warnMalformedSessionPayload("Dropped malformed attachment entry", {
            turnId,
            index,
        });
        return null;
    }
    const filename = readString(rawAttachment.filename, "attachment");
    const mimeType = readString(rawAttachment.mimeType, "application/octet-stream");
    const id = readNonEmptyString(rawAttachment.id) || undefined;
    const clientKey = readNonEmptyString(rawAttachment.clientKey) ||
        id ||
        `${turnId}-attachment-${index}`;
    return {
        id,
        clientKey,
        filename,
        mimeType,
        sizeBytes: readFiniteNumber(rawAttachment.sizeBytes),
        uploadId: readOptionalFiniteNumber(rawAttachment.uploadId),
        base64Data: readNonEmptyString(rawAttachment.base64Data) || undefined,
        url: readNonEmptyString(rawAttachment.url) || undefined,
        preview: readNonEmptyString(rawAttachment.preview) || undefined,
    };
}
function sanitizeUserAttachments(rawAttachments: unknown, turnId: string): Attachment[] {
    if (!Array.isArray(rawAttachments)) {
        return [];
    }
    const result: Attachment[] = [];
    for (let i = 0; i < rawAttachments.length; i++) {
        const sanitized = sanitizeAttachment(rawAttachments[i], turnId, i);
        if (sanitized) {
            result.push(sanitized);
        }
    }
    return result;
}
function sanitizeAssistantArtifacts(rawArtifacts: unknown, turnId: string): DownloadArtifact[] {
    if (!Array.isArray(rawArtifacts)) {
        return [];
    }
    const artifacts: DownloadArtifact[] = [];
    for (let i = 0; i < rawArtifacts.length; i++) {
        const raw = rawArtifacts[i];
        if (!isRecord(raw)) {
            warnMalformedSessionPayload("Dropped malformed assistant artifact", {
                turnId,
                index: i,
            });
            continue;
        }
        const type = readString(raw.type).toLowerCase();
        if (type !== "excel" && type !== "pdf") {
            continue;
        }
        const url = readNonEmptyString(raw.url);
        if (!url) {
            warnMalformedSessionPayload("Dropped assistant artifact without url", {
                turnId,
                index: i,
            });
            continue;
        }
        artifacts.push({
            type,
            filename: readString(raw.filename, "download"),
            url,
            sizeReadable: readNonEmptyString(raw.sizeReadable) || undefined,
            rowCount: typeof raw.rowCount === "number" && Number.isFinite(raw.rowCount)
                ? raw.rowCount
                : undefined,
            description: readNonEmptyString(raw.description) || undefined,
        });
    }
    return artifacts;
}
// ---------------------------------------------------------------------------
// Assistant turn sanitizer
// ---------------------------------------------------------------------------
function sanitizeAssistantTurn(rawAssistantTurn: unknown, fallbackCreatedAt: string, turnId: string): AssistantTurn | undefined {
    if (rawAssistantTurn == null) {
        return undefined;
    }
    if (!isRecord(rawAssistantTurn)) {
        warnMalformedSessionPayload("Dropped malformed assistant turn payload", {
            turnId,
        });
        return undefined;
    }
    const assistantID = readNonEmptyString(rawAssistantTurn.id);
    if (!assistantID) {
        warnMalformedSessionPayload("Dropped assistant turn without id", {
            turnId,
        });
        return undefined;
    }
    const citations = Array.isArray(rawAssistantTurn.citations)
        ? rawAssistantTurn.citations
            .filter((item) => isRecord(item))
            .map((item, index) => ({
            id: readString(item.id, `${assistantID}-citation-${index}`),
            type: readString(item.type),
            title: readString(item.title),
            url: readString(item.url),
            startIndex: readFiniteNumber(item.startIndex),
            endIndex: readFiniteNumber(item.endIndex),
            excerpt: readNonEmptyString(item.excerpt) || undefined,
        }))
        : [];
    const toolCalls = Array.isArray(rawAssistantTurn.toolCalls)
        ? rawAssistantTurn.toolCalls
            .filter((item) => isRecord(item))
            .map((item, index) => ({
            id: readString(item.id, `${assistantID}-tool-${index}`),
            name: readString(item.name),
            arguments: readString(item.arguments),
            result: readNonEmptyString(item.result) || undefined,
            error: readNonEmptyString(item.error) || undefined,
            durationMs: readFiniteNumber(item.durationMs),
        }))
        : [];
    const codeOutputs = Array.isArray(rawAssistantTurn.codeOutputs)
        ? rawAssistantTurn.codeOutputs
            .filter((item) => isRecord(item))
            .map((item) => ({
            type: ((): "image" | "text" | "error" => {
                const normalizedType = readString(item.type, "text").toLowerCase();
                if (normalizedType === "image" || normalizedType === "error") {
                    return normalizedType;
                }
                return "text";
            })(),
            content: readString(item.content),
            filename: readNonEmptyString(item.filename) || undefined,
            mimeType: readNonEmptyString(item.mimeType) || undefined,
            sizeBytes: readOptionalFiniteNumber(item.sizeBytes),
        }))
        : [];
    const debugTrace = isRecord(rawAssistantTurn.debug)
        ? {
            schemaVersion: readNonEmptyString(rawAssistantTurn.debug.schemaVersion) || undefined,
            startedAt: readNonEmptyString(rawAssistantTurn.debug.startedAt) || undefined,
            completedAt: readNonEmptyString(rawAssistantTurn.debug.completedAt) || undefined,
            generationMs: readOptionalFiniteNumber(rawAssistantTurn.debug.generationMs),
            traceId: readNonEmptyString(rawAssistantTurn.debug.traceId) || undefined,
            traceUrl: readNonEmptyString(rawAssistantTurn.debug.traceUrl) || undefined,
            sessionId: readNonEmptyString(rawAssistantTurn.debug.sessionId) || undefined,
            thinking: readNonEmptyString(rawAssistantTurn.debug.thinking) || undefined,
            observationReason: readNonEmptyString(rawAssistantTurn.debug.observationReason) ||
                undefined,
            usage: isRecord(rawAssistantTurn.debug.usage)
                ? {
                    promptTokens: readFiniteNumber(rawAssistantTurn.debug.usage.promptTokens),
                    completionTokens: readFiniteNumber(rawAssistantTurn.debug.usage.completionTokens),
                    totalTokens: readFiniteNumber(rawAssistantTurn.debug.usage.totalTokens),
                    cachedTokens: readOptionalFiniteNumber(rawAssistantTurn.debug.usage.cachedTokens),
                    cost: readOptionalFiniteNumber(rawAssistantTurn.debug.usage.cost),
                }
                : undefined,
            tools: Array.isArray(rawAssistantTurn.debug.tools)
                ? rawAssistantTurn.debug.tools
                    .filter((tool) => isRecord(tool))
                    .map((tool) => ({
                    callId: readNonEmptyString(tool.callId) || undefined,
                    name: readString(tool.name),
                    arguments: readNonEmptyString(tool.arguments) || undefined,
                    result: readNonEmptyString(tool.result) || undefined,
                    error: readNonEmptyString(tool.error) || undefined,
                    durationMs: readOptionalFiniteNumber(tool.durationMs),
                }))
                : [],
            attempts: Array.isArray(rawAssistantTurn.debug.attempts)
                ? rawAssistantTurn.debug.attempts
                    .filter((attempt) => isRecord(attempt))
                    .map((attempt) => ({
                    id: readNonEmptyString(attempt.id) || undefined,
                    requestId: readNonEmptyString(attempt.requestId) || undefined,
                    model: readNonEmptyString(attempt.model) || undefined,
                    provider: readNonEmptyString(attempt.provider) || undefined,
                    finishReason: readNonEmptyString(attempt.finishReason) || undefined,
                    promptTokens: readOptionalFiniteNumber(attempt.promptTokens),
                    completionTokens: readOptionalFiniteNumber(attempt.completionTokens),
                    totalTokens: readOptionalFiniteNumber(attempt.totalTokens),
                    cachedTokens: readOptionalFiniteNumber(attempt.cachedTokens),
                    cost: readOptionalFiniteNumber(attempt.cost),
                    latencyMs: readOptionalFiniteNumber(attempt.latencyMs),
                    input: readNonEmptyString(attempt.input) || undefined,
                    output: readNonEmptyString(attempt.output) || undefined,
                    thinking: readNonEmptyString(attempt.thinking) || undefined,
                    observationReason: readNonEmptyString(attempt.observationReason) || undefined,
                    startedAt: readNonEmptyString(attempt.startedAt) || undefined,
                    completedAt: readNonEmptyString(attempt.completedAt) || undefined,
                    toolCalls: Array.isArray(attempt.toolCalls)
                        ? attempt.toolCalls
                            .filter((tool) => isRecord(tool))
                            .map((tool) => ({
                            callId: readNonEmptyString(tool.callId) || undefined,
                            name: readString(tool.name),
                            arguments: readNonEmptyString(tool.arguments) || undefined,
                            result: readNonEmptyString(tool.result) || undefined,
                            error: readNonEmptyString(tool.error) || undefined,
                            durationMs: readOptionalFiniteNumber(tool.durationMs),
                        }))
                        : [],
                }))
                : [],
            spans: Array.isArray(rawAssistantTurn.debug.spans)
                ? rawAssistantTurn.debug.spans
                    .filter((span) => isRecord(span))
                    .map((span) => ({
                    id: readNonEmptyString(span.id) || undefined,
                    parentId: readNonEmptyString(span.parentId) || undefined,
                    generationId: readNonEmptyString(span.generationId) || undefined,
                    name: readNonEmptyString(span.name) || undefined,
                    type: readNonEmptyString(span.type) || undefined,
                    status: readNonEmptyString(span.status) || undefined,
                    level: readNonEmptyString(span.level) || undefined,
                    callId: readNonEmptyString(span.callId) || undefined,
                    toolName: readNonEmptyString(span.toolName) || undefined,
                    input: readNonEmptyString(span.input) || undefined,
                    output: readNonEmptyString(span.output) || undefined,
                    error: readNonEmptyString(span.error) || undefined,
                    durationMs: readOptionalFiniteNumber(span.durationMs),
                    startedAt: readNonEmptyString(span.startedAt) || undefined,
                    completedAt: readNonEmptyString(span.completedAt) || undefined,
                    attributes: isRecord(span.attributes)
                        ? span.attributes
                        : undefined,
                }))
                : [],
            events: Array.isArray(rawAssistantTurn.debug.events)
                ? rawAssistantTurn.debug.events
                    .filter((item) => isRecord(item))
                    .map((item) => ({
                    id: readNonEmptyString(item.id) || undefined,
                    name: readNonEmptyString(item.name) || undefined,
                    type: readNonEmptyString(item.type) || undefined,
                    level: readNonEmptyString(item.level) || undefined,
                    message: readNonEmptyString(item.message) || undefined,
                    reason: readNonEmptyString(item.reason) || undefined,
                    spanId: readNonEmptyString(item.spanId) || undefined,
                    generationId: readNonEmptyString(item.generationId) || undefined,
                    timestamp: readNonEmptyString(item.timestamp) || undefined,
                    attributes: isRecord(item.attributes)
                        ? item.attributes
                        : undefined,
                }))
                : [],
        }
        : undefined;
    return {
        id: assistantID,
        role: normalizeMessageRole(rawAssistantTurn.role),
        content: readString(rawAssistantTurn.content),
        explanation: readNonEmptyString(rawAssistantTurn.explanation) || undefined,
        citations,
        toolCalls,
        charts: undefined,
        renderTables: undefined,
        artifacts: sanitizeAssistantArtifacts(rawAssistantTurn.artifacts, turnId),
        codeOutputs,
        lifecycle: "complete",
        debug: debugTrace,
        createdAt: readString(rawAssistantTurn.createdAt, fallbackCreatedAt),
    };
}
// ---------------------------------------------------------------------------
// Conversation turn sanitizer
// ---------------------------------------------------------------------------
function sanitizeConversationTurn(rawTurn: unknown, index: number, fallbackSessionID: string): ConversationTurn | null {
    if (!isRecord(rawTurn)) {
        warnMalformedSessionPayload("Dropped malformed turn payload (not an object)", { index });
        return null;
    }
    if (!isRecord(rawTurn.userTurn)) {
        warnMalformedSessionPayload("Dropped malformed turn payload (missing user turn)", { index });
        return null;
    }
    const userTurnID = readNonEmptyString(rawTurn.userTurn.id);
    if (!userTurnID) {
        warnMalformedSessionPayload("Dropped malformed turn payload (missing user turn id)", { index });
        return null;
    }
    const turnID = readString(rawTurn.id, userTurnID);
    const createdAt = readString(rawTurn.createdAt, readString(rawTurn.userTurn.createdAt, new Date().toISOString()));
    return {
        id: turnID,
        sessionId: readString(rawTurn.sessionId, fallbackSessionID),
        userTurn: {
            id: userTurnID,
            content: readString(rawTurn.userTurn.content),
            attachments: sanitizeUserAttachments(rawTurn.userTurn.attachments, turnID),
            author: mapSessionUser(rawTurn.userTurn.author),
            createdAt: readString(rawTurn.userTurn.createdAt, createdAt),
        },
        assistantTurn: sanitizeAssistantTurn(rawTurn.assistantTurn, createdAt, turnID),
        createdAt,
    };
}
export function sanitizeConversationTurns(rawTurns: unknown, sessionID: string): ConversationTurn[] {
    if (!Array.isArray(rawTurns)) {
        warnMalformedSessionPayload("Session payload contained non-array turns field", { sessionID });
        return [];
    }
    const turns: ConversationTurn[] = [];
    let dropped = 0;
    for (let i = 0; i < rawTurns.length; i++) {
        const sanitizedTurn = sanitizeConversationTurn(rawTurns[i], i, sessionID);
        if (sanitizedTurn) {
            turns.push(sanitizedTurn);
        }
        else {
            dropped++;
        }
    }
    if (dropped > 0) {
        warnMalformedSessionPayload("Dropped malformed turns from session payload", {
            sessionID,
            dropped,
            total: rawTurns.length,
        });
    }
    return turns;
}
// ---------------------------------------------------------------------------
// Pending question sanitizer
// ---------------------------------------------------------------------------
export function sanitizePendingQuestion(rawPendingQuestion: RPCPendingQuestion | null | undefined, sessionID: string): PendingQuestion | null {
    if (!rawPendingQuestion) {
        return null;
    }
    const checkpointID = readNonEmptyString(rawPendingQuestion.checkpointId);
    if (!checkpointID) {
        warnMalformedSessionPayload("Dropped malformed pendingQuestion without checkpointId", { sessionID });
        return null;
    }
    if (!Array.isArray(rawPendingQuestion.questions)) {
        warnMalformedSessionPayload("Pending question had non-array questions payload", {
            sessionID,
            checkpointID,
        });
    }
    const questions: Question[] = Array.isArray(rawPendingQuestion.questions)
        ? rawPendingQuestion.questions
            .filter((question) => {
            if (!question || !isRecord(question)) {
                warnMalformedSessionPayload("Dropped malformed question from pendingQuestion", {
                    sessionID,
                    checkpointID,
                });
                return false;
            }
            return true;
        })
            .map((question, index) => {
            const questionID = readString(question.id, `${checkpointID}-q-${index}`);
            const options = Array.isArray(question.options)
                ? question.options
                    .filter((option) => {
                    if (!option || !isRecord(option)) {
                        warnMalformedSessionPayload("Dropped malformed pendingQuestion option", {
                            sessionID,
                            checkpointID,
                            questionID,
                        });
                        return false;
                    }
                    return true;
                })
                    .map((option, optionIndex) => {
                    const label = readString(option.label);
                    const id = readString(option.id, `${questionID}-opt-${optionIndex}`);
                    return {
                        id,
                        label,
                        value: id,
                    };
                })
                : [];
            return {
                id: questionID,
                text: readString(question.text),
                type: normalizeQuestionType(question.type),
                options,
            };
        })
        : [];
    return {
        id: checkpointID,
        turnId: readString(rawPendingQuestion.turnId),
        agentName: readNonEmptyString(rawPendingQuestion.agentName) || undefined,
        questions,
        status: normalizePendingQuestionStatus(rawPendingQuestion.status),
    };
}
// ---------------------------------------------------------------------------
// Artifact helpers
// ---------------------------------------------------------------------------
function formatSizeReadable(bytes: number): string | undefined {
    if (!Number.isFinite(bytes) || bytes <= 0) {
        return undefined;
    }
    const units = ["B", "KB", "MB", "GB", "TB"];
    let value = bytes;
    let idx = 0;
    while (value >= 1024 && idx < units.length - 1) {
        value /= 1024;
        idx++;
    }
    const precision = idx === 0 ? 0 : value >= 10 ? 1 : 2;
    return `${value.toFixed(precision)} ${units[idx]}`;
}
function parseRowCount(metadata?: Record<string, unknown>): number | undefined {
    if (!metadata) {
        return undefined;
    }
    const raw = metadata.row_count ??
        metadata.rowCount ??
        metadata.total_rows ??
        metadata.totalRows;
    if (typeof raw === "number" && Number.isSafeInteger(raw) && raw >= 0) {
        return raw;
    }
    if (typeof raw === "string") {
        const trimmed = raw.trim();
        if (!/^\d+$/.test(trimmed)) {
            return undefined;
        }
        const parsed = Number(trimmed);
        if (Number.isSafeInteger(parsed)) {
            return parsed;
        }
    }
    return undefined;
}
function inferDownloadType(artifact: SessionArtifact): DownloadArtifact["type"] | null {
    const mime = artifact.mimeType?.toLowerCase() || "";
    const name = artifact.name?.toLowerCase() || "";
    const cleanURL = artifact.url?.split("?")[0].toLowerCase() || "";
    const isPDF = mime.includes("pdf") || name.endsWith(".pdf") || cleanURL.endsWith(".pdf");
    if (isPDF) {
        return "pdf";
    }
    const isExcel = mime.includes("spreadsheet") ||
        mime.includes("excel") ||
        name.endsWith(".xlsx") ||
        name.endsWith(".xls") ||
        cleanURL.endsWith(".xlsx") ||
        cleanURL.endsWith(".xls");
    if (isExcel) {
        return "excel";
    }
    return null;
}
function extractFilename(artifact: SessionArtifact): string {
    const name = artifact.name?.trim();
    if (name) {
        return name;
    }
    const urlPath = artifact.url?.split("?")[0] || "";
    const fromURL = urlPath.split("/").filter(Boolean).pop();
    if (fromURL) {
        return fromURL;
    }
    return "download";
}
function toDownloadArtifact(artifact: SessionArtifact): DownloadArtifact | null {
    if (!artifact.url) {
        return null;
    }
    const type = inferDownloadType(artifact);
    if (!type) {
        return null;
    }
    return {
        type,
        filename: extractFilename(artifact),
        url: artifact.url,
        sizeReadable: formatSizeReadable(artifact.sizeBytes),
        rowCount: parseRowCount(artifact.metadata),
        description: artifact.description,
    };
}
// ---------------------------------------------------------------------------
// Tool-call extraction helpers
// ---------------------------------------------------------------------------
function cloneChartData(chart: import("../types").ChartData): import("../types").ChartData {
    return {
        ...chart,
        series: chart.series?.map((s) => ({ ...s, data: [...(s.data ?? [])] })) ?? [],
        labels: chart.labels ? [...chart.labels] : undefined,
        colors: chart.colors ? [...chart.colors] : undefined,
        options: chart.options &&
            typeof chart.options === "object" &&
            !Array.isArray(chart.options)
            ? JSON.parse(JSON.stringify(chart.options))
            : chart.options,
    };
}
function extractChartsFromToolCalls(toolCalls?: Array<{
    name: string;
    result?: string;
}>): import("../types").ChartData[] {
    if (!toolCalls) {
        return [];
    }
    const charts: import("../types").ChartData[] = [];
    for (const tc of toolCalls) {
        if (tc.name === "draw_chart" && tc.result) {
            const parsed = parseChartDataFromJsonString(tc.result);
            if (parsed) {
                charts.push(cloneChartData(parsed));
            }
        }
    }
    return charts;
}
function cloneRenderTableData(table: RenderTableData): RenderTableData {
    return {
        ...table,
        columns: [...(table.columns ?? [])],
        headers: [...(table.headers ?? [])],
        rows: table.rows?.map((row) => [...row]) ?? [],
    };
}
function extractRenderTablesFromToolCalls(toolCalls?: Array<{
    id: string;
    name: string;
    result?: string;
}>): RenderTableData[] {
    if (!toolCalls) {
        return [];
    }
    const tables: RenderTableData[] = [];
    for (const tc of toolCalls) {
        if (tc.name !== "render_table" || !tc.result) {
            continue;
        }
        const parsed = parseRenderTableDataFromJsonString(tc.result, tc.id);
        if (parsed) {
            tables.push(cloneRenderTableData(parsed));
        }
    }
    return tables;
}
const EXPORT_TOOL_NAMES: Record<string, DownloadArtifact["type"]> = {
    export_query_to_excel: "excel",
    export_data_to_excel: "excel",
    export_to_pdf: "pdf",
};
function extractDownloadArtifactsFromToolCalls(toolCalls?: Array<{
    name: string;
    result?: string;
}>): DownloadArtifact[] {
    if (!toolCalls) {
        return [];
    }
    const artifacts: DownloadArtifact[] = [];
    for (const tc of toolCalls) {
        const type = EXPORT_TOOL_NAMES[tc.name];
        if (!type || !tc.result) {
            continue;
        }
        let parsed: unknown;
        try {
            parsed = JSON.parse(tc.result);
        }
        catch {
            continue;
        }
        if (!isRecord(parsed) || typeof parsed.url !== "string" || !parsed.url) {
            continue;
        }
        const filename = typeof parsed.filename === "string" && parsed.filename
            ? parsed.filename
            : parsed.url.split("/").pop() || "download";
        const sizeKB = typeof parsed.file_size_kb === "number" ? parsed.file_size_kb : undefined;
        const sizeBytes = typeof parsed.size === "number"
            ? parsed.size
            : sizeKB != null
                ? sizeKB * 1024
                : undefined;
        artifacts.push({
            type,
            filename,
            url: parsed.url,
            sizeReadable: sizeBytes != null ? formatSizeReadable(sizeBytes) : undefined,
            rowCount: parseRowCount(parsed as Record<string, unknown>),
            description: typeof parsed.description === "string" ? parsed.description : undefined,
        });
    }
    return artifacts;
}
// ---------------------------------------------------------------------------
// Turn normalization
// ---------------------------------------------------------------------------
function normalizeAssistantTurn(turn: Partial<AssistantTurn> & {
    id: string;
    content: string;
    createdAt: string;
}): AssistantTurn {
    const existingArtifacts = turn.artifacts || [];
    const fromToolCalls = extractDownloadArtifactsFromToolCalls(turn.toolCalls);
    const renderTables = turn.renderTables || extractRenderTablesFromToolCalls(turn.toolCalls);
    // Merge: add tool-call artifacts that aren't already present (by URL + filename)
    const merged = [...existingArtifacts];
    for (const a of fromToolCalls) {
        if (!merged.some((e) => e.url === a.url && e.filename === a.filename)) {
            merged.push(a);
        }
    }
    const chartsRaw = turn.charts?.length
        ? turn.charts
        : extractChartsFromToolCalls(turn.toolCalls);
    const charts = chartsRaw?.map(cloneChartData) ?? [];
    const renderTablesCloned = renderTables.map(cloneRenderTableData);
    return {
        ...turn,
        role: (turn.role as MessageRole) || MessageRole.Assistant,
        charts: charts.length ? charts : undefined,
        renderTables: renderTablesCloned,
        citations: turn.citations || [],
        artifacts: merged,
        codeOutputs: turn.codeOutputs || [],
        lifecycle: turn.lifecycle || "complete",
    };
}
export function normalizeTurns(raw: ConversationTurn[]): ConversationTurn[] {
    return raw.map((turn) => {
        if (!turn.assistantTurn) {
            return turn;
        }
        return {
            ...turn,
            assistantTurn: normalizeAssistantTurn(turn.assistantTurn),
        };
    });
}
// ---------------------------------------------------------------------------
// Attach artifacts to turns
// ---------------------------------------------------------------------------
function toMillis(value: string): number {
    const parsed = Date.parse(value);
    return Number.isFinite(parsed) ? parsed : Number.NaN;
}
export function attachArtifactsToTurns(turns: ConversationTurn[], artifacts: SessionArtifact[]): ConversationTurn[] {
    if (artifacts.length === 0) {
        return turns;
    }
    const downloadArtifacts = artifacts
        .map((raw) => ({ raw, mapped: toDownloadArtifact(raw) }))
        .filter((entry): entry is {
        raw: SessionArtifact;
        mapped: DownloadArtifact;
    } => entry.mapped !== null)
        .sort((a, b) => toMillis(a.raw.createdAt) - toMillis(b.raw.createdAt));
    const chartArtifacts = artifacts
        .filter((a) => a.type === "chart")
        .sort((a, b) => toMillis(a.createdAt) - toMillis(b.createdAt));
    const tableArtifacts = artifacts
        .filter((a) => a.type === "table")
        .sort((a, b) => toMillis(a.createdAt) - toMillis(b.createdAt));
    if (downloadArtifacts.length === 0 &&
        chartArtifacts.length === 0 &&
        tableArtifacts.length === 0) {
        return turns;
    }
    const nextTurns = turns.map((turn) => {
        if (!turn.assistantTurn) {
            return turn;
        }
        return {
            ...turn,
            assistantTurn: {
                ...turn.assistantTurn,
                artifacts: [...(turn.assistantTurn.artifacts || [])],
                charts: turn.assistantTurn.charts
                    ? [...turn.assistantTurn.charts]
                    : undefined,
                renderTables: turn.assistantTurn.renderTables
                    ? [...turn.assistantTurn.renderTables]
                    : undefined,
            },
        };
    });
    const turnIndexByMessageID = new Map<string, number>();
    nextTurns.forEach((turn, index) => {
        turnIndexByMessageID.set(turn.userTurn.id, index);
        const assistantTurn = turn.assistantTurn;
        if (!assistantTurn) {
            return;
        }
        turnIndexByMessageID.set(assistantTurn.id, index);
    });
    for (const entry of downloadArtifacts) {
        const messageID = entry.raw.messageId;
        // Only attach artifacts that are explicitly linked to a message.
        // Orphan artifacts (uploaded via the artifacts panel) have no messageId
        // and should not appear on any message.
        if (!messageID) {
            continue;
        }
        const targetIndex = turnIndexByMessageID.get(messageID);
        if (targetIndex === undefined) {
            continue;
        }
        const assistantTurn = nextTurns[targetIndex]?.assistantTurn;
        if (!assistantTurn) {
            continue;
        }
        const exists = assistantTurn.artifacts.some((existing) => existing.url === entry.mapped.url &&
            existing.filename === entry.mapped.filename);
        if (!exists) {
            assistantTurn.artifacts.push(entry.mapped);
        }
    }
    for (const raw of chartArtifacts) {
        const messageID = raw.messageId;
        if (!messageID) {
            continue;
        }
        const targetIndex = turnIndexByMessageID.get(messageID);
        if (targetIndex === undefined) {
            continue;
        }
        const assistantTurn = nextTurns[targetIndex]?.assistantTurn;
        if (!assistantTurn) {
            continue;
        }
        const metadata = raw.metadata;
        if (!metadata || typeof metadata !== "object" || metadata === null) {
            continue;
        }
        const spec = metadata.spec &&
            typeof metadata.spec === "object" &&
            metadata.spec !== null
            ? (metadata.spec as Record<string, unknown>)
            : (metadata as Record<string, unknown>);
        const chart = parseChartDataFromSpec(spec, raw.name);
        if (chart) {
            if (!assistantTurn.charts) {
                assistantTurn.charts = [];
            }
            assistantTurn.charts.push(cloneChartData(chart));
        }
    }
    for (const raw of tableArtifacts) {
        const messageID = raw.messageId;
        if (!messageID) {
            continue;
        }
        const targetIndex = turnIndexByMessageID.get(messageID);
        if (targetIndex === undefined) {
            continue;
        }
        const assistantTurn = nextTurns[targetIndex]?.assistantTurn;
        if (!assistantTurn) {
            continue;
        }
        if (assistantTurn.renderTables === undefined) {
            assistantTurn.renderTables = extractRenderTablesFromToolCalls(assistantTurn.toolCalls);
        }
        const existing = assistantTurn.renderTables;
        const metadata = raw.metadata;
        if (!metadata || typeof metadata !== "object" || metadata === null) {
            continue;
        }
        const tableData = parseRenderTableDataFromMetadata(metadata as Record<string, unknown>, raw.id);
        if (!tableData) {
            continue;
        }
        const dedupeKey = (t: RenderTableData) => `${t.query}|${t.columns.join(",")}`;
        const key = dedupeKey(tableData);
        if (existing.some((t) => dedupeKey(t) === key)) {
            continue;
        }
        assistantTurn.renderTables = [...existing, cloneRenderTableData(tableData)];
    }
    return nextTurns;
}
