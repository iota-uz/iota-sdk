/**
 * ApiError represents an error returned from the API
 * Unlike regular errors, it can store additional metadata like error codes
 */
export class ApiError extends Error {
  public code: string;
  public details?: Record<string, any>;

  constructor(message: string, code: string, details?: Record<string, any>) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.details = details;
    
    // This is necessary for instanceof to work with custom Error classes in TypeScript
    Object.setPrototypeOf(this, ApiError.prototype);
  }

  /**
   * Check if this error has a specific code
   */
  hasCode(code: string): boolean {
    return this.code === code;
  }

  /**
   * Check if this error has one of the specified codes
   */
  hasOneOfCodes(codes: string[]): boolean {
    return codes.includes(this.code);
  }
}