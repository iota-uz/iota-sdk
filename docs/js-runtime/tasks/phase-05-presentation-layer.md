# Phase 5: Presentation Layer (UI/UX) (3 days)

## Overview
This phase is dedicated to building the user interface for managing and interacting with scripts. The UI will be built using the existing `templ` and HTMX stack to ensure consistency with the rest of the IOTA SDK. A key component will be the integration of the Monaco Editor to provide a rich code editing experience.

## Background
- The presentation layer follows a standard MVC-like pattern: Controller -> DTOs/ViewModels -> Mappers -> Templates.
- All UI components should be reusable and follow the existing design system.
- RBAC must be enforced at the controller level to restrict access to UI elements and actions.

## UI Layout Draft

A more detailed, high-resolution ASCII draft of the UI to guide development, matching the existing application style.

**Script List Page (`list.templ`)**
```
┌────────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────────┐
│      _____  ___  _______   │  Scripts                                                                                         │
│ IOTA|_   _||  _||__   __|  │──────────────────────────────────────────────────────────────────────────────────────────────────│
│     | | |  | |     | |     │  Search  [ Search by name..._________________________________ ]  Status [ All ▾ ]  [+ New Script]  │
│     | |_|  | |__   | |     │                                                                                                  │
│     |____/ |____|  |_|     │                                                                                                  │
│                            │                                                                                                  │
│  ┌──────────┐              │ ┌────────────────────────────────────────────────────────────────────────────────────────────────┐ │
│  │  Core ▣  │              │ │ Name                 | Type     | Status   | Last Execution      | Actions                     │ │
│  └──────────┘              │ ├─────────────────────────────────────��──────────────────────────────────────────────────────────┤ │
│  ┌──────────┐              │ │ Process Invoices     | Cron     | [Active] | 2025-07-18 10:00:05 | [Edit] [History] [Delete]   │ │
│  │  CRM     │              │ │ Sync External Users  | Endpoint | [Active] | 2025-07-17 14:30:10 | [Edit] [History] [Delete]   │ │
│  └──────────┘              │ │ Cleanup Temp Files   | Cron     | [Inactive] | N/A               | [Edit] [History] [Delete]   │ │
│                            │ │ Generate Report      | Manual   | [Active] | 2025-07-16 09:00:00 | [Edit] [History] [Delete]   │ │
│  ▼ Dashboard               │ │                      |          |          |                     |                             │ │
│  ▸ Administration          │ ├────────────────────────────────────────────────────────────────────────────────────────────────┤ │
│  ● BI‑Chat                 │ │                                                                    Page 1 of 5  [<] [1] [2] [3] [>] │ │
│  ● Employees               │ └────────────────────────────────────────────────────────────────────────────────────────────────┘ │
│  ▼ System                  │                                                                                                  │
│     • Settings             │                                                                                                  │
│     • Scripts              │                                                                                                  │
│  ▸ Warehouse               │                                                                                                  │
│  ● AI Chatbot              │                                                                                                  │
│                            │                                                                                                  │
│  ⏻  Sign out               │                                                                                                  │
└────────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Script Edit Page (`edit.templ`)**
```
┌────────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────────┐
│      _____  ___  _______   │  Create New Script                                                                               │
│ IOTA|_   _||  _||__   __|  │──────────────────────────────────────────────────────────────────────────────────────────────────│
│     | | |  | |     | |     │  [< Back to Scripts]                                                     [Save] [Run Now] [Cancel] │
│     | |_|  | |__   | |     │                                                                                                  │
│     |____/ |____|  |_|     │ ┌──────────────────────────────────────────────────────────────────────────────────────────────┐ │
│                            │ │ Metadata                                                                                     │ │
│  ┌──────────┐              │ │                                                                                              │ │
│  │  Core ▣  │              │ │   Name:         [ Process Invoices_________________________________________________________ ] │ │
│  └──────────┘              │ │   Description:  [ A short description of what this script does.____________________________ ] │ │
│  ┌──────────┐              │ │   Type:         [ Cron ▾ ]   Status:  (•) Active   ( ) Inactive                               │ │
│  │  CRM     │              │ │   Cron Trigger: [ 0 2 * * *________________________________________________________________ ] │ │
│  └──────────┘              │ │                                                                                              │ │
│                            │ ├──────────────────────────────────────────────────────────────────────────────────────────────┤ │
│  ▼ Dashboard               │ │ Code Editor                                                                                  │ │
│  ▸ Administration          │ │ ┌──────────────────────────────────────────────────────────────────────────────────────────┐ │ │
│  ● BI‑Chat                 │ │ │ 1 │ // Use IOTA SDK services to perform actions                                            │ │ │
│  ● Employees               │ │ │ 2 │ const invoices = await services.billing.findInvoices({ status: 'pending' });         │ │ │
│  ▼ System                  │ │ │ 3 │ console.log(`Found ${invoices.length} pending invoices.`);                             │ │ │
│     • Settings             │ │ │ 4 │ for (const invoice of invoices) {                                                      │ │ │
│     • Scripts              │ │ │ 5 │   await services.billing.processInvoice(invoice.id);                                   │ │ │
│  ▸ Warehouse               │ │ │ 6 │ }                                                                                      │ │ │
│  ● AI Chatbot              │ │ │ 7 │                                                                                        │ │ │
│                            │ │ │ 8 │                                                                                        │ │ │
│  ⏻  Sign out               │ │ │...│                                                                                        │ │ │
│                            │ │ └──────────────────────────────────────────────────────────────────────────────────────────┘ │ │
│                            │ │                                                                 Last saved: Just now         │ │
│                            │ └──────────────────────────────────────────────────────────────────────────────────────────────┘ │
└────────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

**Execution History Page (`executions.templ`)**
```
┌────────────────────────────┬──────────────────────────────────────────────────────────────────────────────────────────────────┐
│      _____  ___  _______   │  Execution History: Process Invoices                                                             │
│ IOTA|_   _||  _||__   __|  │──────────────────────────────────────────────────────────────────────────────────────────────────│
│     | | |  | |     | |     │  [< Back to Scripts]                                                                    [Refresh] │
│     | |_|  | |__   | |     │                                                                                                  │
│     |____/ |____|  |_|     │                                                                                                  │
│                            │                                                                                                  │
│  ┌──────────┐              │ ┌────────────────────────────────────────────────────────────────────────────────────────────────┐ │
│  │  Core ▣  │              │ │ Start Time (UTC)    | Duration | Status    | Trigger         | Actions                          │ │
│  └──────────┘              │ ├────────────────────────────────────────────────────────────────────────────────────────────────┤ │
│  ┌──────────┐              │ │ ▼ 2025-07-18 02:00:00 | 12.34s   | [Success] | Cron Schedule   | [View Details]                   │ │
│  │  CRM     │              │ │   Logs:                                                                                        │ │
│  └──────────┘              │ │     Found 42 pending invoices.                                                                 │ │
│                            │ │     Successfully processed invoice INV-001.                                                    │ │
│  ▼ Dashboard               │ │     Successfully processed invoice INV-002.                                                    │ │
│  ▸ Administration          │ │     ...                                                                                        │ │
│  ● BI‑Chat                 │ ├────────────────────────────────────────────────────────────────────────────────────────────────┤ │
│  ● Employees               │ │ ▸ 2025-07-17 15:10:05 | 2.10s    | [Failed]  | Manual          | [View Details]                   │ │
│  ▼ System                  │ ├────────────────────────────────────────────────────────────────────────────────────────────────┤ │
│     • Settings             │ │ ▸ 2025-07-17 02:00:00 | 11.98s   | [Success] | Cron Schedule   | [View Details]                   │ │
│     • Scripts              │ ├───────────────────────────────────────────────────────────────────────────────��────────────────┤ │
│  ▸ Warehouse               │ │                                                                    Page 1 of 2  [<] [1] [2] [>] │ │
│  ● AI Chatbot              │ └────────────────────────────────────────────────────────────────────────────────────────────────┘ │
│                            │                                                                                                  │
│  ⏻  Sign out               │                                                                                                  │
└────────────────────────────┴──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

## Task 5.1: Script Management UI (Day 1)

### Objectives
- Create a page to list all scripts with filtering and sorting.
- Implement forms for creating and editing script metadata (name, description, type, etc.).
- Develop the necessary controllers, DTOs, viewmodels, and mappers.
- Enforce RBAC permissions for all UI actions.

### Detailed Steps

#### 1. Create DTOs and ViewModels
- `modules/scripts/presentation/controllers/dtos/script_dto.go`: Define DTOs for creating and updating scripts (`CreateScriptDTO`, `UpdateScriptDTO`). These should include validation tags.
- `modules/scripts/presentation/viewmodels/script_viewmodel.go`: Define viewmodels for the list page (`ScriptListItemViewModel`) and the edit page (`ScriptViewModel`).

#### 2. Implement Presentation Mapper
- `modules/scripts/presentation/mappers/mappers.go`: Create mappers to convert between domain entities, DTOs, and viewmodels.
  - `MapScriptToViewModel(script.Script) ScriptViewModel`
  - `MapScriptsToListItemViewModels([]script.Script) []ScriptListItemViewModel`

#### 3. Develop Script Controller
- `modules/scripts/presentation/controllers/script_controller.go`: Create a new controller to handle the UI logic.
  - `RegisterRoutes(router *chi.Mux)`
  - `list(w http.ResponseWriter, r *http.Request)`: Fetches scripts from the `ScriptService`, maps them to viewmodels, and renders the `list.templ` template.
  - `showNewForm(w http.ResponseWriter, r *http.Request)`: Renders the `new.templ` template.
  - `create(w http.ResponseWriter, r *http.Request)`: Handles the form submission from the new page, calls the `ScriptService`, and redirects on success or re-renders the form with errors on failure.
  - `showEditForm(w http.ResponseWriter, r *http.Request)`: Fetches a script, maps it to a viewmodel, and renders the `edit.templ` template.
  - `update(w http.ResponseWriter, r *http.Request)`: Handles the form submission from the edit page.
  - `delete(w http.ResponseWriter, r *http.Request)`: Deletes a script via an HTMX request.
  - **RBAC**: Each handler must check for the appropriate permissions (e.g., `scripts.read`, `scripts.create`).

#### 4. Create `templ` Templates
- `modules/scripts/presentation/templates/pages/scripts/list.templ`:
  - A table displaying scripts with columns for Name, Type, Status (Active/Inactive), and Actions.
  - Use HTMX for pagination, filtering, and sorting.
  - Action buttons (Edit, Delete) that make HTMX requests to the controller.
- `modules/scripts/presentation/templates/pages/scripts/new.templ`:
  - A form for creating a new script. This form will focus on metadata; the code editor will be on the edit page.
  - Fields: Name, Description, Type (dropdown: Cron, Endpoint, etc.).
- `modules/scripts/presentation/templates/pages/scripts/edit.templ`:
  - A form to edit script metadata.
  - This page will contain the Monaco Editor component (to be built in Task 5.2).

### Testing Requirements
- Write controller tests using the `controller-test-suite.md` pattern.
- Test all controller actions, including success cases, validation failures, and permission errors.
- Use E2E tests (Cypress) to verify the script list, creation, and editing workflows from the user's perspective.

## Task 5.2: Monaco Editor Integration (Day 2)

### Objectives
- Integrate the Monaco Editor as a reusable `templ` component.
- Load the custom TypeScript definitions (`iota-sdk.d.ts`) to provide IntelliSense.
- Implement save functionality that sends the script content to the backend.

### Detailed Steps

#### 1. Create Monaco Editor Component
- `components/monaco_editor.templ`: Create a new reusable component.
  - It should accept parameters like `initialContent`, `language`, and `saveEndpoint`.
  - It will contain the necessary HTML (`<div id="editor"></div>`) and JavaScript to initialize the editor.
```go
// components/monaco_editor.templ
templ MonacoEditor(initialContent, language, saveEndpoint string) {
    <div id="editor-container" style="height: 500px;"></div>
    <script>
        // Use a JS bundler or load directly
        require.config({ paths: { 'vs': 'path/to/monaco/vs' }});
        require(['vs/editor/editor.main'], function() {
            // Fetch TypeScript definitions
            fetch('/static/iota-sdk.d.ts')
                .then(response => response.text())
                .then(tsDefs => {
                    monaco.languages.typescript.javascriptDefaults.addExtraLib(tsDefs, 'iota-sdk.d.ts');
                });

            const editor = monaco.editor.create(document.getElementById('editor-container'), {
                value: initialContent,
                language: language,
                theme: 'vs-dark',
            });

            // Save functionality
            document.getElementById('save-button').addEventListener('click', () => {
                const content = editor.getValue();
                htmx.ajax('POST', saveEndpoint, {
                    values: { content: content },
                    target: '#save-status'
                });
            });
        });
    </script>
}
```

#### 2. Integrate Component into Edit Page
- `modules/scripts/presentation/templates/pages/scripts/edit.templ`:
  - Include the `@components.MonacoEditor()` component.
  - Pass the script's content, language ("javascript"), and the appropriate save endpoint from the controller.

#### 3. Create Static Asset Handling
- Ensure the Monaco Editor library files are served as static assets.
- Create an endpoint or a file server to serve the `iota-sdk.d.ts` file generated in the previous phase.

### Testing Requirements
- E2E Test: Open the script edit page and verify that the Monaco Editor loads with the correct content.
- E2E Test: Verify that IntelliSense for the IOTA SDK API (e.g., `services.`) is working.
- E2E Test: Modify the script content, click "Save", and verify that the content is updated in the database.

## Task 5.3: Execution History UI (Day 3)

### Objectives
- Create a page to view the execution history for a specific script.
- Display execution details, including status, duration, output, and errors.

### Detailed Steps

#### 1. Create Controller and ViewModels
- `modules/scripts/presentation/controllers/execution_controller.go`: A new controller to handle viewing execution history.
- `modules/scripts/presentation/viewmodels/execution_viewmodel.go`: Viewmodels for the execution list (`ExecutionListItemViewModel`) and details.

#### 2. Create `templ` Template
- `modules/scripts/presentation/templates/pages/scripts/executions.templ`:
  - A table or list displaying execution history.
  - Columns: Start Time, Duration, Status (Success/Failed/Timeout), Trigger.
  - Each row should be expandable or clickable to show a modal with detailed output and error messages.
  - Use HTMX to periodically refresh the list to show real-time updates for running scripts.

#### 3. Link from Script List
- In `list.templ`, add a "History" button for each script that links to the execution history page.

### Testing Requirements
- Controller tests for the `ExecutionController`.
- E2E Test: Execute a script (manually or via a test trigger) and verify that a new entry appears on the execution history page.
- E2E Test: Verify that the status, output, and error details are displayed correctly for both successful and failed executions.

### Deliverables Checklist
- [ ] Script list page with CRUD actions.
- [ ] Script create/edit forms.
- [ ] `ScriptController` with full functionality and RBAC.
- [ ] Reusable Monaco Editor `templ` component.
- [ ] IntelliSense for the IOTA SDK API is working in the editor.
- [ ] Execution history page displaying logs and outputs.
- [ ] `ExecutionController` to serve history data.
- [ ] Comprehensive controller and E2E tests for the UI.