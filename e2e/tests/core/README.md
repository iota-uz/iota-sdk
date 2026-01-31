# Two-Factor Authentication (2FA) E2E Tests

This directory contains comprehensive end-to-end tests for the IOTA SDK two-factor authentication feature using Playwright.

## Overview

The 2FA test suite covers all authentication methods, user workflows, and edge cases:

- **TOTP** (Time-based One-Time Password) - Google Authenticator, Authy, etc.
- **SMS** - One-Time Password via SMS
- **Email** - One-Time Password via Email
- **Recovery Codes** - Backup authentication method

## Test Files

### 1. `2fa-totp-setup.spec.ts` - TOTP Setup Flow

Tests the initial setup of TOTP authentication for new users.

**Coverage:**
- Method selection from setup page
- QR code display and validation
- TOTP code verification
- Recovery codes generation
- Invalid code error handling
- Input validation (6-digit numeric)
- Help text and instructions

**Key Scenarios:**
- ✅ Successfully complete TOTP setup with valid code
- ✅ Display error for invalid TOTP code
- ✅ Allow retry after failed verification
- ✅ Preserve nextURL parameter throughout flow
- ✅ Generate and display recovery codes

---

### 2. `2fa-totp-login.spec.ts` - TOTP Login Flow

Tests login verification for users with TOTP already enabled.

**Coverage:**
- Redirect to verification after credentials
- TOTP code verification
- Multiple retry attempts
- Recovery code access
- Session state transitions
- Protected route access enforcement

**Key Scenarios:**
- ✅ Redirect to verification page after login
- ✅ Successfully verify with valid TOTP code
- ✅ Display error for invalid code
- ✅ Allow multiple retry attempts
- ✅ Access recovery code page from verification
- ✅ Preserve and redirect to nextURL after verification

---

### 3. `2fa-otp-setup.spec.ts` - Email/SMS OTP Setup Flow

Tests the setup of OTP-based authentication (Email and SMS methods).

**Coverage:**
- Email method selection and OTP sending
- SMS method selection and OTP sending
- OTP code verification
- Resend functionality
- Invalid code handling
- Method-specific validation (phone number requirement)

**Key Scenarios:**
- ✅ Automatically send OTP after method selection
- ✅ Successfully verify Email OTP
- ✅ Successfully verify SMS OTP
- ✅ Display error for invalid OTP
- ✅ Allow OTP resend
- ✅ No recovery codes for OTP methods (TOTP only)

---

### 4. `2fa-otp-login.spec.ts` - Email/SMS OTP Login Flow

Tests login verification for users with OTP authentication enabled.

**Coverage:**
- Automatic OTP sending on verification page
- Email OTP verification
- SMS OTP verification
- Resend functionality
- Invalid code handling
- Method-specific UI elements

**Key Scenarios:**
- ✅ Automatically send OTP on verification page load
- ✅ Successfully verify Email OTP
- ✅ Successfully verify SMS OTP
- ✅ Display resend button (not available for TOTP)
- ✅ Allow multiple retry attempts
- ✅ Preserve nextURL parameter

---

### 5. `2fa-recovery-codes.spec.ts` - Recovery Code Flow

Tests recovery code generation, display, and usage for account recovery.

**Coverage:**
- Recovery code display after TOTP setup
- Recovery code verification for login bypass
- One-time use enforcement
- Invalid code handling
- Recovery code depletion tracking
- Navigation between verification and recovery pages

**Key Scenarios:**
- ✅ Display recovery codes after TOTP setup
- ✅ Navigate to recovery page from verification
- ✅ Successfully login with valid recovery code
- ✅ Display error for invalid recovery code
- ✅ Mark recovery code as used after login
- ✅ Prevent reuse of same recovery code
- ✅ Display one-time use warning

---

### 6. `2fa-enforcement.spec.ts` - Enforcement & Edge Cases

Tests enforcement policies, security measures, and edge case handling.

**Coverage:**
- Users without 2FA login normally
- Protected route access control
- Redirect URL validation (open redirect prevention)
- Session state validation
- Duplicate 2FA setup prevention
- Input validation and sanitization
- CSRF protection
- Browser navigation (back button handling)

**Key Scenarios:**
- ✅ Users without 2FA bypass verification flow
- ✅ Block protected routes until 2FA completion
- ✅ Validate and sanitize nextURL parameter
- ✅ Reject external/malicious redirect URLs
- ✅ Require valid session state for 2FA pages
- ✅ Prevent setup when 2FA already enabled
- ✅ Sanitize code inputs (XSS prevention)
- ✅ Handle browser back button gracefully

---

## Supporting Infrastructure

### Page Objects

Located in `/e2e/pages/core/`:

**`twofactor-setup-page.ts`**
- Reusable methods for 2FA setup flow
- Method selection
- QR code extraction
- Code entry and verification
- Recovery code retrieval
- Error/success message verification

**`twofactor-verify-page.ts`**
- Reusable methods for 2FA verification flow
- Code entry for TOTP/OTP
- Recovery code entry
- Resend OTP functionality
- Navigation between verification types
- Error/success message verification

### Helpers

Located in `/e2e/helpers/`:

**`totp.ts`**
- TOTP code generation from secret
- Secret extraction from otpauth:// URL
- TOTP code validation
- Invalid code generation (for error testing)

**`otp.ts`**
- OTP code retrieval from database
- OTP code retrieval via test API
- Wait for OTP with timeout
- Invalid OTP generation (for error testing)

### Dependencies

**New E2E Dependencies:**
- `otplib@^12.0.1` - TOTP code generation library

**Existing Dependencies:**
- `@playwright/test` - E2E testing framework
- `pg` - PostgreSQL client for database access
- `dotenv` - Environment variable management

---

## Running the Tests

### Prerequisites

1. **E2E database must be set up:**
   ```bash
   make e2e reset seed
   ```

2. **Application server must be running:**
   ```bash
   air  # or make dev
   ```

3. **Install E2E dependencies:**
   ```bash
   cd e2e && npm install
   ```

### Run All 2FA Tests

```bash
# From project root
make e2e test

# Or from e2e directory
cd e2e && npx playwright test tests/core/
```

### Run Individual Test Suites

```bash
cd e2e

# TOTP setup flow
npx playwright test tests/core/2fa-totp-setup.spec.ts

# TOTP login flow
npx playwright test tests/core/2fa-totp-login.spec.ts

# OTP setup flow (Email/SMS)
npx playwright test tests/core/2fa-otp-setup.spec.ts

# OTP login flow (Email/SMS)
npx playwright test tests/core/2fa-otp-login.spec.ts

# Recovery codes
npx playwright test tests/core/2fa-recovery-codes.spec.ts

# Enforcement and edge cases
npx playwright test tests/core/2fa-enforcement.spec.ts
```

### Interactive Debugging

```bash
cd e2e

# UI mode (recommended for development)
npx playwright test --ui

# Debug mode with inspector
npx playwright test tests/core/2fa-totp-setup.spec.ts --debug

# Headed mode (watch browser)
npx playwright test tests/core/2fa-totp-setup.spec.ts --headed

# Generate traces
npx playwright test --trace on

# View traces
npx playwright show-trace trace.zip
```

### Run Specific Tests

```bash
cd e2e

# Run tests matching pattern
npx playwright test -g "should display QR code"

# Run single test file with specific test
npx playwright test tests/core/2fa-totp-setup.spec.ts -g "successfully complete"
```

---

## Test Data Management

### Database Reset

Tests use `resetTestDatabase()` and `seedScenario()` fixtures:

```typescript
test.beforeAll(async ({ request }) => {
  // Reset database and seed with comprehensive data
  await resetTestDatabase(request, { reseedMinimal: false });
  await seedScenario(request, 'comprehensive');
});
```

### Test Users

Tests create specific users with different 2FA configurations:

- **TOTP-enabled users** - For testing TOTP verification flow
- **Email OTP users** - For testing Email OTP flow
- **SMS OTP users** - For testing SMS OTP flow
- **Users without 2FA** - For testing normal login flow

### Database Seeding

The `populateTestData()` fixture creates custom test data:

```typescript
await populateTestData(request, {
  version: '1.0',
  data: {
    users: [
      {
        email: 'test@example.com',
        password: 'TestPass123!',
        twoFactorMethod: 'totp',
        totpSecretEncrypted: 'JBSWY3DPEHPK3PXP',
        twoFactorEnabledAt: new Date().toISOString(),
      },
    ],
  },
});
```

---

## Implementation Notes

### TOTP Code Generation

TOTP codes are generated using the `otplib` library:

```typescript
import { generateTOTPCode } from '../../helpers/totp';

const secret = 'JBSWY3DPEHPK3PXP'; // Base32 encoded secret
const code = generateTOTPCode(secret);
// Returns: "123456" (6-digit code)
```

### OTP Code Retrieval

OTP codes are retrieved from the database since E2E tests can't access SMS/Email:

```typescript
import { getOTPCodeFromDB, waitForOTP } from '../../helpers/otp';

// Wait for OTP to be sent and retrieve
const otpCode = await waitForOTP('test@example.com');

// Or retrieve directly from database
const otpCode = await getOTPCodeFromDB('+998901234567');
```

### Secret Extraction Limitation

**Important:** Extracting TOTP secrets from QR codes requires QR decoding, which is complex. Tests use alternative approaches:

1. **Page attribute extraction** - Look for `otpauth://` URL in HTML
2. **Test API endpoint** - Provide dedicated endpoint to retrieve setup challenge data
3. **Direct database access** - Query setup challenge cache (not recommended for E2E)

**Current Implementation:** Tests attempt to extract `otpauth://` URL from page content. If this fails, you may need to add a test-only data attribute to the QR code container with the secret.

### Recovery Code Database Schema

Recovery codes are stored in the `recovery_codes` table:

```sql
CREATE TABLE recovery_codes (
  id SERIAL PRIMARY KEY,
  user_id INTEGER NOT NULL,
  code VARCHAR(255) NOT NULL,
  used_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);
```

Tests query this table to:
- Retrieve valid recovery codes
- Verify code consumption (used_at field)
- Track remaining codes

---

## Common Issues & Solutions

### Issue: "No OTP found for destination"

**Cause:** OTP was not sent or database query failed.

**Solutions:**
1. Verify OTP sender is configured in test environment
2. Check database connection in E2E config
3. Ensure OTP table exists and is seeded correctly
4. Add debug logging to OTP helper

### Issue: "Could not extract TOTP secret from page"

**Cause:** QR code secret extraction failed.

**Solutions:**
1. Add data attribute with secret to TOTP setup page:
   ```html
   <div data-totp-secret="{{ .Secret }}">
   ```

2. Modify page object to read attribute:
   ```typescript
   const secret = await page.getAttribute('[data-totp-secret]', 'data-totp-secret');
   ```

3. Use test API endpoint to retrieve setup challenge

### Issue: "Test timeout waiting for OTP"

**Cause:** OTP sending is slow or failed.

**Solutions:**
1. Increase timeout in `waitForOTP()` call
2. Use mock OTP sender in test environment
3. Verify OTP expiry time is reasonable (5-10 minutes)
4. Check database connection pooling issues

### Issue: "Recovery codes not displayed"

**Cause:** Implementation differences between TOTP and OTP methods.

**Note:** Recovery codes are typically only generated for TOTP setups, not OTP methods. This is by design. OTP methods don't use recovery codes because the user can always request a new OTP.

---

## Test Coverage Summary

### User Workflows
- ✅ First-time 2FA setup (TOTP)
- ✅ First-time 2FA setup (Email OTP)
- ✅ First-time 2FA setup (SMS OTP)
- ✅ Login with TOTP verification
- ✅ Login with Email OTP verification
- ✅ Login with SMS OTP verification
- ✅ Login with recovery code
- ✅ Normal login (no 2FA)

### Error Scenarios
- ✅ Invalid TOTP code
- ✅ Invalid OTP code
- ✅ Invalid recovery code
- ✅ Expired OTP (implicit via timeout)
- ✅ Reused recovery code
- ✅ Missing phone number for SMS

### Security
- ✅ Open redirect prevention (nextURL validation)
- ✅ Session state validation
- ✅ Protected route access enforcement
- ✅ Input sanitization (XSS prevention)
- ✅ Duplicate setup prevention

### Edge Cases
- ✅ Browser navigation (back button)
- ✅ Multiple retry attempts
- ✅ OTP resend functionality
- ✅ Recovery code depletion
- ✅ Session expiration (implicit)

---

## Future Improvements

### Potential Enhancements

1. **Rate Limiting Tests**
   - Test max OTP/TOTP attempts
   - Test OTP resend rate limiting
   - Test account lockout after failed attempts

2. **Session Expiration Tests**
   - Test session timeout during 2FA flow
   - Test re-authentication requirements
   - Test remember device functionality (if implemented)

3. **Multi-Device Tests**
   - Test TOTP setup on one device, login on another
   - Test recovery code usage from different locations
   - Test concurrent session handling

4. **Accessibility Tests**
   - Keyboard navigation through 2FA forms
   - Screen reader compatibility
   - Focus management

5. **Performance Tests**
   - OTP delivery time
   - QR code generation speed
   - Recovery code generation performance

6. **Integration Tests**
   - OAuth login bypass (2FA not required for OAuth)
   - API authentication with 2FA
   - Mobile app integration

---

## Contributing

When adding new 2FA tests:

1. **Follow existing patterns** - Use page objects and helpers
2. **Test both happy and error paths** - Cover success and failure scenarios
3. **Use descriptive test names** - Clearly state what is being tested
4. **Reset database** - Ensure test isolation with proper cleanup
5. **Add documentation** - Update this README with new test coverage

### Test Naming Convention

```typescript
test('should [action] [expected outcome]', async ({ page }) => {
  // Test implementation
});
```

**Examples:**
- `should display QR code after selecting TOTP method`
- `should successfully complete TOTP setup with valid code`
- `should display error for invalid TOTP code`
- `should prevent reuse of recovery codes`

---

## Debugging Tips

### View Test Reports

```bash
cd e2e && npx playwright show-report
```

### Generate Screenshots on Failure

Screenshots are automatically captured on test failure and saved to:
```
e2e/test-results/<test-name>/screenshots/
```

### View Traces

```bash
# Run with trace
cd e2e && npx playwright test --trace on

# View trace
cd e2e && npx playwright show-trace test-results/<test-name>/trace.zip
```

### Increase Verbosity

```bash
cd e2e && npx playwright test --reporter=list
```

### Check Database State

```typescript
// Add to test for debugging
const pool = new Pool(getDBConfig());
const result = await pool.query('SELECT * FROM users WHERE email = $1', ['test@example.com']);
console.log('User state:', result.rows[0]);
await pool.end();
```

---

## References

- [Playwright Documentation](https://playwright.dev/)
- [IOTA SDK 2FA Controllers](/modules/core/presentation/controllers/twofactor_*.go)
- [IOTA SDK 2FA Service](/modules/core/services/twofactor/service.go)
- [E2E Test Fixtures](/e2e/fixtures/)
- [Page Objects Pattern](https://playwright.dev/docs/pom)
