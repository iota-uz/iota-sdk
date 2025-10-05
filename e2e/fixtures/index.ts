/**
 * Centralized exports for all test fixtures
 *
 * This allows simpler imports in test files:
 * import { login, resetDB, uploadFileAndWaitForAttachment } from '../fixtures';
 */

// Database operations
export { resetDB, seedDB, getEnvInfo } from './database';

// Authentication
export { login, logout, waitForAlpine } from './auth';

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
