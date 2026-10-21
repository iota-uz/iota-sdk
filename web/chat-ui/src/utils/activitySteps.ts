/**
 * Utility functions for activity step processing.
 */
import type { ActivityStep } from '../types';
/**
 * Group activity steps by agent — top-level steps (no agentName) and
 * agent groups (keyed by agentName).
 */
export function groupSteps(steps: ActivityStep[]): {
    topLevel: ActivityStep[];
    agentGroups: [
        string,
        ActivityStep[]
    ][];
} {
    const topLevel: ActivityStep[] = [];
    const agentMap = new Map<string, ActivityStep[]>();
    for (const step of steps) {
        if (step.agentName) {
            const group = agentMap.get(step.agentName) || [];
            group.push(step);
            agentMap.set(step.agentName, group);
        }
        else {
            topLevel.push(step);
        }
    }
    const agentGroups = Array.from(agentMap.entries());
    return { topLevel, agentGroups };
}
