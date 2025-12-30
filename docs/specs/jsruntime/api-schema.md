# API Schema: JavaScript Runtime

**Status:** Draft

## HTMX Endpoints

| Method | Path | Purpose | Response |
|--------|------|---------|----------|
| GET | /scripts | List all scripts | HTML page with scripts table |
| GET | /scripts/new | Show create form | HTML page with Monaco editor |
| POST | /scripts | Create new script | Redirect to /scripts/:id |
| GET | /scripts/:id | View script details | HTML page with execution history |
| GET | /scripts/:id/edit | Show edit form | HTML page with Monaco editor |
| PUT | /scripts/:id | Update script | Redirect to /scripts/:id |
| DELETE | /scripts/:id | Delete script | HTML fragment (remove table row) |
| POST | /scripts/:id/execute | Execute script manually | HTML fragment (execution row) |
| GET | /scripts/:id/executions | List executions (HTMX poll) | HTML fragment (execution rows) |

### Endpoint: GET /scripts

**Purpose:** Display paginated list of scripts with filtering and search

**Authorization:** `scripts.read` permission

**Request:**
- Query params:
  - `page` (int, default: 1) - Page number
  - `limit` (int, default: 20) - Items per page
  - `type` (string, optional) - Filter by script type (scheduled/http/event/oneoff/embedded)
  - `status` (string, optional) - Filter by status (draft/active/paused/disabled/archived)
  - `search` (string, optional) - Full-text search in name/description

**Response:** Full HTML page with:
- Scripts table (name, type, status, last execution, actions)
- Pagination controls
- Filter dropdowns
- Search input
- "New Script" button (if user has `scripts.create` permission)

### Endpoint: GET /scripts/new

**Purpose:** Display form to create new script

**Authorization:** `scripts.create` permission

**Request:** None

**Response:** Full HTML page with:
- Form fields: Name, Description, Type, Trigger config (cron/http path/event types), Resource limits
- Monaco editor for JavaScript code
- Save/Cancel buttons

### Endpoint: POST /scripts

**Purpose:** Create new script from form submission

**Authorization:** `scripts.create` permission

**Request:** Form fields (CamelCase):
- `Name` (string, required, min: 3, max: 100)
- `Description` (string, optional, max: 500)
- `Source` (string, required) - JavaScript code
- `Type` (string, required, enum: scheduled/http/event/oneoff/embedded)
- `Status` (string, default: draft)
- `CronExpression` (string, required if Type=scheduled)
- `HTTPPath` (string, required if Type=http)
- `HTTPMethods` ([]string, required if Type=http)
- `EventTypes` ([]string, required if Type=event)
- `MaxExecutionTimeMs` (int, optional, default: 30000)
- `MaxMemoryBytes` (int, optional, default: 67108864)
- `MaxConcurrentRuns` (int, optional, default: 5)
- `MaxAPICallsPerMinute` (int, optional, default: 60)
- `MaxOutputSizeBytes` (int, optional, default: 1048576)

**Response:**
- Success: Redirect to `/scripts/:id`
- Validation error: Re-render form with error messages

### Endpoint: GET /scripts/:id

**Purpose:** Display script details and execution history

**Authorization:** `scripts.read` permission

**Request:** Path param `id` (UUID)

**Response:** Full HTML page with:
- Script metadata (name, type, status, created/updated dates)
- JavaScript code preview (read-only Monaco editor)
- Resource limits display
- Trigger configuration (cron/http/events)
- Execution history table (recent 20 executions)
- Action buttons: Edit, Execute, Delete (permission-based)

### Endpoint: GET /scripts/:id/edit

**Purpose:** Display form to edit existing script

**Authorization:** `scripts.update` permission

**Request:** Path param `id` (UUID)

**Response:** Full HTML page with:
- Pre-filled form fields
- Monaco editor with current source code
- Save/Cancel buttons

### Endpoint: PUT /scripts/:id

**Purpose:** Update existing script

**Authorization:** `scripts.update` permission

**Request:**
- Path param `id` (UUID)
- Form fields (same as POST /scripts)

**Response:**
- Success: Redirect to `/scripts/:id`
- Validation error: Re-render form with error messages
- Creates new version in `script_versions` table

### Endpoint: DELETE /scripts/:id

**Purpose:** Delete script permanently

**Authorization:** `scripts.delete` permission

**Request:** Path param `id` (UUID)

**Response:**
- HTMX: HTML fragment (empty, triggers row removal via `hx-swap="outerHTML"`)
- Standard: Redirect to `/scripts`

### Endpoint: POST /scripts/:id/execute

**Purpose:** Execute script manually

**Authorization:** `scripts.execute` permission

**Request:**
- Path param `id` (UUID)
- Form field `Input` (JSONB, optional) - Input parameters for script

**Response:**
- HTMX: HTML fragment (new execution row prepended to history)
- Standard: Redirect to `/scripts/:id`

### Endpoint: GET /scripts/:id/executions

**Purpose:** Fetch latest executions (HTMX polling)

**Authorization:** `scripts.read` permission

**Request:**
- Path param `id` (UUID)
- Query param `limit` (int, default: 20)

**Response:** HTML fragment (table rows):
- Each row: execution ID, status, trigger type, started/completed times, duration, error (if failed)
- Status badge with color coding (pending/running/completed/failed/timeout/cancelled)

## JavaScript SDK API

**Purpose:** APIs injected into script execution context via Goja VM

### Context API

**Global:** `context`

**Properties:**
- `context.tenantId: string` (read-only) - Current tenant UUID
- `context.userId: number | null` (read-only) - Authenticated user ID
- `context.organizationId: string | null` (read-only) - Organization UUID
- `context.scriptId: string` (read-only) - Executing script UUID
- `context.executionId: string` (read-only) - Current execution UUID
- `context.input: object` - Input parameters passed to script
- `context.trigger: object` - Trigger information

**Trigger Object:**
- `type: string` - Trigger type (cron/http/event/manual/api)
- `eventType: string | null` - Event type (for event triggers)
- `eventPayload: object | null` - Event data (for event triggers)
- `httpRequest: object | null` - HTTP request details (for HTTP triggers)
  - `method: string` - HTTP method
  - `path: string` - Request path
  - `headers: object` - Request headers
  - `query: object` - Query parameters
  - `body: object` - Request body
- `cronExpression: string | null` - Cron schedule (for cron triggers)

### HTTP Client API

**Namespace:** `sdk.http`

**Methods:**
- `sdk.http.get(url: string, options?: HttpOptions): Promise<HttpResponse>`
- `sdk.http.post(url: string, body: any, options?: HttpOptions): Promise<HttpResponse>`
- `sdk.http.put(url: string, body: any, options?: HttpOptions): Promise<HttpResponse>`
- `sdk.http.delete(url: string, options?: HttpOptions): Promise<HttpResponse>`
- `sdk.http.patch(url: string, body: any, options?: HttpOptions): Promise<HttpResponse>`

**HttpOptions:**
- `headers?: object` - Custom HTTP headers
- `query?: object` - Query parameters
- `timeout?: number` - Request timeout in milliseconds (default: 10000)

**HttpResponse:**
- `status: number` - HTTP status code
- `headers: object` - Response headers
- `body: string | object` - Response body (auto-parsed JSON if Content-Type: application/json)

**Errors:**
- `Error: SSRF protection` - Blocked private IP or cloud metadata endpoint
- `Error: Rate limit exceeded` - Exceeded 60 requests/minute

### Database API

**Namespace:** `sdk.db`

**Methods:**
- `sdk.db.query(sql: string, params: any[]): Promise<object[]>` - Execute SELECT query
- `sdk.db.execute(sql: string, params: any[]): Promise<number>` - Execute INSERT/UPDATE/DELETE (returns affected rows)
- `sdk.db.queryOne(sql: string, params: any[]): Promise<object | null>` - Get single row
- `sdk.db.transaction(fn: (tx: Transaction) => Promise<void>): Promise<void>` - Execute in transaction

**Security:**
- Automatic `tenant_id` injection in WHERE clause
- Parameterized queries only (no string concatenation)
- Read-only access (no DDL/DCL)
- Row limit: max 1000 rows
- Query timeout: max 5 seconds

**Errors:**
- `Error: Unauthorized operation` - Attempted DDL/DCL
- `Error: SQL injection detected` - String concatenation in query
- `Error: Row limit exceeded` - Query returned > 1000 rows
- `Error: Query timeout` - Query exceeded 5 seconds
- `Error: Rate limit exceeded` - Exceeded 60 queries/minute

### Cache API

**Namespace:** `sdk.cache`

**Methods:**
- `sdk.cache.get(key: string): Promise<any | null>` - Retrieve value
- `sdk.cache.set(key: string, value: any, ttl?: number): Promise<boolean>` - Store value (ttl in seconds)
- `sdk.cache.delete(key: string): Promise<boolean>` - Remove key
- `sdk.cache.exists(key: string): Promise<boolean>` - Check existence
- `sdk.cache.increment(key: string, delta?: number): Promise<number>` - Atomic increment (delta default: 1)
- `sdk.cache.expire(key: string, ttl: number): Promise<boolean>` - Update TTL

**Automatic Prefixing:**
- Keys automatically prefixed with `tenant:{tenantId}:script:{scriptId}:`
- Prevents cross-tenant and cross-script key collisions

**Errors:**
- `Error: Rate limit exceeded` - Exceeded 120 operations/minute

### Logging API

**Namespace:** `sdk.log`

**Methods:**
- `sdk.log.debug(message: string, metadata?: object): void` - Debug level
- `sdk.log.info(message: string, metadata?: object): void` - Info level
- `sdk.log.warn(message: string, metadata?: object): void` - Warning level
- `sdk.log.error(message: string, metadata?: object): void` - Error level

**Automatic Context:**
- `tenant_id`, `user_id`, `organization_id`
- `script_id`, `execution_id`
- `timestamp`, `level`
- Custom metadata merged with context

**Errors:**
- `Error: Rate limit exceeded` - Exceeded 100 logs/minute

### Events API

**Global:** `events`

**Methods:**
- `events.publish(eventType: string, payload: object): Promise<boolean>` - Publish domain event

**Event Types:** Follow convention `module.entity.action` (e.g., "user.created", "order.updated")

**Automatic Enrichment:**
- `tenant_id` injected automatically
- `timestamp` added
- `source_script_id` tracked
- `event_id` generated (UUID)

**Errors:**
- `Error: Invalid event type` - Event type doesn't match naming convention
- `Error: Rate limit exceeded` - Exceeded 30 events/minute

## Error Handling

| Error Code | Condition | Response |
|------------|-----------|----------|
| 400 | Validation failure (invalid form data) | Re-render form with inline error messages |
| 401 | Unauthenticated | Redirect to login page |
| 403 | Insufficient permissions | 403 Forbidden page with message |
| 404 | Script not found | 404 Not Found page |
| 409 | Duplicate name or HTTP path | Re-render form with conflict error |
| 500 | Execution error | Display error message with execution ID for support |
| 504 | Execution timeout | Display timeout message with duration limit |

**HTMX Error Responses:**
- Return HTML fragment with error styling
- Use `hx-swap="innerHTML"` to replace target with error message
- Include retry button for transient errors

**JavaScript SDK Errors:**
- Return JavaScript `Error` object with safe message
- No stack traces exposed to scripts
- Include error code for categorization (e.g., `SSRF_BLOCKED`, `RATE_LIMIT`, `QUERY_TIMEOUT`)

## Open Questions

None
