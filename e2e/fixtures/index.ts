/**
 * Centralized exports for all test fixtures
 *
 * This allows simpler imports in test files:
 * import { login, resetTestDatabase, uploadFileAndWaitForAttachment } from '../fixtures';
 */

// Database operations
export { getEnvInfo, withDatabase } from './database';

// Worker-owned environment
export { test, expect } from './test';
export { currentWorkerEnvironment } from './environment';

// Authentication
export { login, logout, waitForAlpine } from './auth';

// Onboarding of accounts created by an administrator
export { submitNewUserForm, completeOnboarding, loginThroughOnboarding } from './onboarding';
export type { OnboardingProfile } from './onboarding';

// File uploads
export { uploadFileAndWaitForAttachment } from './file-upload';

// Test data management
export {
	resetTestDatabase,
	populateTestData,
	seedScenario,
	getAvailableScenarios,
	checkTestEndpointsHealth,
	TestDataBuilders,
} from './test-data';

// Error handling
export { setupErrorHandling, shouldIgnoreError } from './error-handling';
