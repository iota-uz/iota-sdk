# Business history: coverage matrix

Business history (`business_events`) records what happened to business objects, who did it and
through which channel. An event is written by the application service in the same PostgreSQL
transaction as the change itself, so a committed change always has its event and a rolled-back
one never does. HTTP middleware only marks the request as a UI operation; it records nothing on
its own, and a mutation is covered only when this matrix links it to an event and a test.

`modules/logging/coverage_test.go` keeps this document honest: every exported mutating method of
an application service in core, crm, finance, hrm, projects and warehouse must appear below as
`Service.Method` (in the events table or in the exceptions), and every registered event type must
be documented here. A new service mutation fails the build until it is classified.

## Installing

History is on when the application loads the logging component (`logging.NewComponent()`, part of
`modules.Components()`). Core provides defaults that the component replaces: an in-memory registry
and `history.Disabled()`, a writer that records nothing. An application that composes core,
finance and the other modules without the logging component keeps working exactly as before and
records no history; once it adds the component, every rule below applies, fail-closed included.

## Entry points and trusted context

Actor, channel, tenant and correlation IDs come from `history.Execution`, set by the entry point
and never from request bodies, imported files or model arguments. With history installed, a service
called without it fails with `history.ErrExecutionMissing` (reason `execution_missing`) and the
change rolls back.

| Entry point | Sets | Channel | Actor | Initiator | operation_id |
|---|---|---|---|---|---|
| Every HTTP request (`history.Middleware`, contributed by the logging component) | `logging` component | `ui` | authenticated user | — | new per request; `request_id` = `X-Request-Id` |
| `importpkg.ImportRunner.Start` | `pkg/import` | `import` | actor of the caller (the importing user) | caller's initiator | new per run; `run_id` = run ID |
| Warehouse position import (`PositionController` upload) | warehouse | `import` | importing user | — | new per upload; `run_id` = upload ID |
| `jobs.Runner` handlers | `pkg/jobs` | `job` | service `job:<kind>` | enqueuing user | job ID (stable across retries of the job) |
| `periodics` tasks | `pkg/periodics` | `job` | system `periodic:<task>` | — | run receipt ID |
| `dbctl` operations (migrate, seed, e2e) | `pkg/dbctl/execution` | `job` | system `dbctl:<operation>` | — | new per run |
| `e2e seed` command | `pkg/commands/e2e` | `job` | system `e2e:seed` | — | new per run |
| MCP / AI chat executor | host (see below) | `mcp` / `chat` | account of the user | — | stable per plan apply |

Hosts wrap their own entry points with one call:

- `history.SystemExecution(ctx, id, displayName)` — CLI commands, seeds, startup hooks, schedulers;
- `history.JobExecution(ctx, runID, kind, history.UserInitiator(ctx))` — background work started by a user;
- `history.ImportExecution(ctx, runID)` — one import batch;
- `history.WithExecution(ctx, history.Execution{...})` — anything else, e.g. MCP and chat.

The MCP / chat executor (#1084, `pkg/mcp`) must call `history.WithExecution` before it invokes
services, with `Channel: businessevent.ChannelMCP` (or `ChannelChat`), `OperationID` equal to the
plan or apply ID so a retried apply yields the same event IDs, `RequestID` of the HTTP call,
`ClientID` of the MCP client, and `Actor: nil` so the authenticated account stays the actor.
None of these values may come from tool arguments.

## Import batches

An import shares one `operation_id`; every object gets its own event with its own `event_id`.
`history.ImportItem` runs each item in its own transaction (a savepoint inside an outer one): a
committed item has its `succeeded` object events, a failed item is rolled back together with its
events and is recorded as a separate `import.item` event with outcome `failed` (or `denied` for a
permission error), written outside of the rolled-back transaction. The imported file is never part
of an event: only the item reference (`row:<n>`) is stored.

## Outbox for downstream consumers

`history.WithOutbox` (the `history.Writer` provided by the logging component) schedules every
recorded event for every registered `history.Consumer` in `business_event_deliveries`, in the same
transaction. `history.Dispatcher` (the `business-history-outbox` hook) delivers due rows with
`FOR UPDATE SKIP LOCKED`; a consumer's database writes commit together with the delivery mark, so
consumers must be idempotent by `event_id` only for external side effects. A failed delivery is
retried with exponential backoff (up to an hour) and never blocks other consumers or the stored
history. Diagnostics:

- `/system/info` capability `business_history_outbox` turns `degraded` with the number of failed
  deliveries and the oldest failed event, operation and consumer;
- `Dispatcher.ListFailed(ctx, limit)` lists the tenant's failed deliveries with event and operation
  IDs, attempts and the last error;
- `Dispatcher.Retry(ctx, eventID, consumer)` makes one due again. Redelivery never rewrites the event,
  its actor or its time.

Consumers are registered with `history.ContributeConsumers`. The SDK ships none; the in-memory
`eventbus` keeps publishing after commit as before and is not a delivery guarantee.

## Coverage boundary

Not recorded, by design:

- direct SQL and SQL migrations (`migrations/*.sql`), including data migrations;
- seeds (`modules/*/seed`, `dbctl seed`), which write through repositories, not services;
- repository calls that bypass application services;
- the superadmin console (tenant administration) and the request-level `action_logs` it replaces;
- technical tables: sessions, authentication logs, uploads, notifications, chat messages, caches,
  outbox and job state.

## Events

Actor is the execution actor (the user for UI and import, `job:<kind>` with the user as initiator
for jobs, `system` for scheduled and CLI runs). Channels are those of the entry points above.

| event_type | Business name | Resource | Fields | Service.Method | Test |
|---|---|---|---|---|---|
| `import.item` | Import item rejected | `import.item` | — (outcome `failed` / `denied`) | `history.ImportItem; warehouse/PositionRowHandler` | `TestPositionRowHandler_PartialImport`, `TestPositionRowHandler_RecordRejectedRows` |
| `core.user.created` | User created | `core.user` | first_name, last_name, middle_name, email\*, phone\*, ui_language, status | `core/UserService.Create`, `core/UserService.CreateWithTemporaryPassword` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user.updated` | User changed | `core.user` | same | `core/UserService.Update`, `core/UserService.UpdateSelf` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user.deleted` | User deleted | `core.user` | same | `core/UserService.Delete` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user.blocked` | User blocked | `core.user` | blocked, block_reason | `core/UserService.BlockUser` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user.unblocked` | User unblocked | `core.user` | blocked, block_reason | `core/UserService.UnblockUser` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user.password_changed` | Password changed | `core.user` | — (the fact only, never a value) | `core/UserService.ChangePassword` | `TestCoreHistory_PasswordChangeRecordsNoSecret` |
| `core.user.temporary_password_issued` | Temporary password issued | `core.user` | status, password_expires_at | `core/UserService.IssueTemporaryPassword` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user.onboarding_completed` | Account set up | `core.user` | first_name, last_name, middle_name, phone\*, ui_language, status | `core/UserService.CompleteOnboarding` | `TestCoreHistory_OnboardingWithoutUserInContext` |
| `core.user.role_assigned` | Role assigned to user | `core.user` + `core.role` | — (link) | `core/UserService.Create`, `core/UserService.Update` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user.role_unassigned` | Role taken from user | `core.user` + `core.role` | — (link) | `core/UserService.Update` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user.permission_granted` | Permission granted to user | `core.user` + `core.permission` | — (link) | `core/UserService.Create`, `core/UserService.Update` | `TestCoreHistory_RecordsServiceMutations (shared link code)` |
| `core.user.permission_revoked` | Permission revoked from user | `core.user` + `core.permission` | — (link) | `core/UserService.Update` | `TestCoreHistory_RecordsServiceMutations (shared link code)` |
| `core.role.created` | Role created | `core.role` | name, description | `core/RoleService.Create` | `TestCoreHistory_RecordsServiceMutations` |
| `core.role.updated` | Role changed | `core.role` | name, description | `core/RoleService.Update` | `TestCoreHistory_RecordsServiceMutations` |
| `core.role.deleted` | Role deleted | `core.role` | name, description | `core/RoleService.Delete` | `TestCoreHistory_RecordsServiceMutations` |
| `core.role.permission_granted` | Permission added to role | `core.role` + `core.permission` | — (link) | `core/RoleService.Create`, `core/RoleService.Update` | `TestCoreHistory_RecordsServiceMutations` |
| `core.role.permission_revoked` | Permission removed from role | `core.role` + `core.permission` | — (link) | `core/RoleService.Update` | `TestCoreHistory_RecordsServiceMutations` |
| `core.group.created` | Group created | `core.group` | name, description | `core/GroupService.Create` | `TestCoreHistory_RecordsServiceMutations` |
| `core.group.updated` | Group changed | `core.group` | name, description | `core/GroupService.Update` | `TestCoreHistory_UnchangedUpdateRecordsNothing` |
| `core.group.deleted` | Group deleted | `core.group` | name, description | `core/GroupService.Delete` | `TestCoreHistory_RecordsServiceMutations` |
| `core.group.member_added` | User added to group | `core.group` + `core.user` | — (link) | `core/GroupService.Create`, `core/GroupService.Update`, `core/GroupService.AddUser`, `core/UserService.Update` | `TestCoreHistory_RecordsServiceMutations` |
| `core.group.member_removed` | User removed from group | `core.group` + `core.user` | — (link) | `core/GroupService.Update`, `core/GroupService.RemoveUser`, `core/UserService.Update` | `TestCoreHistory_RecordsServiceMutations` |
| `core.group.role_assigned` | Role assigned to group | `core.group` + `core.role` | — (link) | `core/GroupService.Create`, `core/GroupService.Update`, `core/GroupService.AssignRole` | `TestCoreHistory_RecordsServiceMutations` |
| `core.group.role_unassigned` | Role taken from group | `core.group` + `core.role` | — (link) | `core/GroupService.Update`, `core/GroupService.RemoveRole` | `TestCoreHistory_RecordsServiceMutations` |
| `core.department.created` | Department created | `core.department` | code, name, parent_id, order, status | `core/DepartmentService.Create` | `TestCoreHistory_RecordsServiceMutations` |
| `core.department.updated` | Department changed | `core.department` | same | `core/DepartmentService.Update` | `TestCoreHistory_RecordsServiceMutations` |
| `core.department.deleted` | Department deleted | `core.department` | same | `core/DepartmentService.Delete` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user_position.created` | Staff position created | `core.user_position` | title, user_id, department_id, is_manager, is_primary, status | `core/UserPositionService.Create` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user_position.updated` | Staff position changed | `core.user_position` | same | `core/UserPositionService.Update` | `TestCoreHistory_RecordsServiceMutations` |
| `core.user_position.deleted` | Staff position deleted | `core.user_position` | same | `core/UserPositionService.Delete` | `TestCoreHistory_RecordsServiceMutations` |
| `crm.client.created` | Client created | `crm.client` | first_name, last_name, middle_name, phone\*, email\*, address\*, gender, date_of_birth\*, passport\*, pin\*, comments | `crm/ClientService.Create`, `crm/ChatService.CreateOrGetClientByPhone` | `TestCRMHistory_RecordsServiceMutations`, `TestCRMHistory_SensitiveFieldsKeepValues` |
| `crm.client.updated` | Client changed | `crm.client` | same | `crm/ClientService.Update` | `TestCRMHistory_RecordsServiceMutations` |
| `crm.client.deleted` | Client deleted | `crm.client` | same | `crm/ClientService.Delete` | `TestCRMHistory_RecordsServiceMutations` |
| `finance.payment.created` | Payment created | `finance.payment` | amount, account_id, category_id, counterparty_id, transaction_date, accounting_period, comment | `finance/PaymentService.Create` | `TestFinanceHistory_PaymentsAndExpenses, `TestBusinessHistory_SameActionAcrossChannels`` |
| `finance.payment.updated` | Payment changed | `finance.payment` | same | `finance/PaymentService.Update` | `TestFinanceHistory_PaymentsAndExpenses` |
| `finance.payment.deleted` | Payment deleted | `finance.payment` | same | `finance/PaymentService.Delete` | `TestFinanceHistory_PaymentsAndExpenses` |
| `finance.payment.file_attached` | File attached to payment | `finance.payment` + `core.upload` | — (link) | `finance/PaymentService.AttachFileToPayment` | `TestFinanceHistory_PaymentsAndExpenses` |
| `finance.payment.file_detached` | File removed from payment | `finance.payment` + `core.upload` | — (link) | `finance/PaymentService.DetachFileFromPayment` | `TestFinanceHistory_PaymentsAndExpenses` |
| `finance.expense.created` | Expense created | `finance.expense` | amount, account_id, category_id, date, accounting_period, comment | `finance/ExpenseService.Create` | `TestFinanceHistory_PaymentsAndExpenses` |
| `finance.expense.updated` | Expense changed | `finance.expense` | same | `finance/ExpenseService.Update` | `TestFinanceHistory_PaymentsAndExpenses` |
| `finance.expense.deleted` | Expense deleted | `finance.expense` | same | `finance/ExpenseService.Delete` | `TestFinanceHistory_PaymentsAndExpenses` |
| `finance.expense.file_attached` | File attached to expense | `finance.expense` + `core.upload` | — (link) | `finance/ExpenseService.AttachFileToExpense` | `TestFinanceHistory_PaymentsAndExpenses` |
| `finance.expense.file_detached` | File removed from expense | `finance.expense` + `core.upload` | — (link) | `finance/ExpenseService.DetachFileFromExpense` | `TestFinanceHistory_PaymentsAndExpenses` |
| `finance.transaction.created` | Transfer between accounts or currency exchange | `finance.transaction` | type, amount, origin_account_id, destination_account_id, exchange_rate, destination_amount, transaction_date, accounting_period, comment | `finance/TransactionService.Create` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.transaction.updated` | Transfer or exchange changed | `finance.transaction` | same | `finance/TransactionService.Update` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.transaction.deleted` | Transfer or exchange deleted | `finance.transaction` | same | `finance/TransactionService.Delete` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.money_account.created` | Money account created | `finance.money_account` | name, account_number, balance, description | `finance/MoneyAccountService.Create` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.money_account.updated` | Money account changed (including a manual balance correction) | `finance.money_account` | same | `finance/MoneyAccountService.Update` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.money_account.deleted` | Money account deleted | `finance.money_account` | same | `finance/MoneyAccountService.Delete` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.counterparty.created` | Counterparty created | `finance.counterparty` | name, tin, type, legal_type, legal_address | `finance/CounterpartyService.Create` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.counterparty.updated` | Counterparty changed | `finance.counterparty` | same | `finance/CounterpartyService.Update` | `TestFinanceHistory_RecordsServiceMutations, `TestFinanceHistory_UnchangedUpdateRecordsNothing`` |
| `finance.counterparty.deleted` | Counterparty deleted | `finance.counterparty` | same | `finance/CounterpartyService.Delete` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.debt.created` | Debt created | `finance.debt` | type, status, counterparty_id, original_amount, outstanding_amount, due_date, money_account_id, project_id, settlement_transaction_id, description | `finance/DebtService.Create` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.debt.updated` | Debt changed | `finance.debt` | same | `finance/DebtService.Update` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.debt.settled` | Debt settled, fully or in part | `finance.debt` | same | `finance/DebtService.Settle` | `TestFinanceHistory_RecordsServiceMutations, `TestFinanceHistory_RollbackLeavesNoEvents`` |
| `finance.debt.written_off` | Debt written off | `finance.debt` | same | `finance/DebtService.WriteOff` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.debt.cancelled` | Debt cancelled | `finance.debt` | same | `finance/DebtService.Cancel` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.debt.deleted` | Debt deleted | `finance.debt` | same | `finance/DebtService.Delete` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.inventory.created` | Inventory item created | `finance.inventory` | name, price, quantity, description | `finance/InventoryService.Create` | `TestFinanceHistory_RecordsServiceMutations, `TestFinanceHistory_MissingExecutionFailsClosed`` |
| `finance.inventory.updated` | Inventory item changed | `finance.inventory` | same | `finance/InventoryService.Update` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.inventory.deleted` | Inventory item deleted | `finance.inventory` | same | `finance/InventoryService.Delete` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.expense_category.created` | Expense category created | `finance.expense_category` | name, is_cogs, description | `finance/ExpenseCategoryService.Create` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.expense_category.updated` | Expense category changed | `finance.expense_category` | same | `finance/ExpenseCategoryService.Update` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.expense_category.deleted` | Expense category deleted | `finance.expense_category` | same | `finance/ExpenseCategoryService.Delete` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.payment_category.created` | Payment category created | `finance.payment_category` | name, description | `finance/PaymentCategoryService.Create` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.payment_category.updated` | Payment category changed | `finance.payment_category` | same | `finance/PaymentCategoryService.Update` | `TestFinanceHistory_RecordsServiceMutations` |
| `finance.payment_category.deleted` | Payment category deleted | `finance.payment_category` | same | `finance/PaymentCategoryService.Delete` | `TestFinanceHistory_RecordsServiceMutations` |
| `hrm.employee.created` | Employee hired | `hrm.employee` | first_name, last_name, middle_name, email, phone, hire_date, resignation_date, birth_date\*\*, salary\*\*, tin\*\*, pin\*\* | `hrm/EmployeeService.Create` | `TestHRMHistory_RecordsServiceMutations` |
| `hrm.employee.updated` | Employee changed | `hrm.employee` | same | `hrm/EmployeeService.Update` | `TestHRMHistory_RecordsServiceMutations`, `TestHRMHistory_SalaryChangeKeepsValues` |
| `hrm.employee.deleted` | Employee deleted | `hrm.employee` | same | `hrm/EmployeeService.Delete` | `TestHRMHistory_RecordsServiceMutations` |
| `hrm.position.created` | Job position created | `hrm.position` | name, description | `hrm/PositionService.Create` | `TestHRMHistory_RecordsServiceMutations`, `TestHRMHistory_RollbackLeavesNoEvents` |
| `hrm.position.updated` | Job position changed | `hrm.position` | name, description | `hrm/PositionService.Update` | `TestHRMHistory_RecordsServiceMutations`, `TestHRMHistory_UnchangedUpdateRecordsNothing` |
| `hrm.position.deleted` | Job position deleted | `hrm.position` | name, description | `hrm/PositionService.Delete` | `TestHRMHistory_RecordsServiceMutations` |
| `projects.project.created` | Project created | `projects.project` | name, counterparty_id, description, contract | `projects/ProjectService.Create`, `projects/ProjectService.Save (new ID)` | `TestProjectsHistory_RecordsServiceMutations`, `TestProjectsHistory_RollbackLeavesNoEvents` |
| `projects.project.updated` | Project changed | `projects.project` | same | `projects/ProjectService.Update`, `projects/ProjectService.Save (existing ID)` | `TestProjectsHistory_RecordsServiceMutations`, `TestProjectsHistory_UnchangedUpdateRecordsNothing` |
| `projects.project.deleted` | Project deleted | `projects.project` | same | `projects/ProjectService.Delete` | `TestProjectsHistory_RecordsServiceMutations` |
| `projects.project_stage.created` | Project stage created | `projects.project_stage` | project_id, stage_number, description, total_amount, start_date, planned_end_date, factual_end_date | `projects/ProjectStageService.Create`, `projects/ProjectStageService.Save (new ID)` | `TestProjectsHistory_RecordsServiceMutations` |
| `projects.project_stage.updated` | Project stage changed | `projects.project_stage` | same | `projects/ProjectStageService.Update`, `projects/ProjectStageService.Save (existing ID)` | `TestProjectsHistory_RecordsServiceMutations` |
| `projects.project_stage.deleted` | Project stage deleted | `projects.project_stage` | same | `projects/ProjectStageService.Delete` | `TestProjectsHistory_RecordsServiceMutations` |
| `projects.acceptance.created` | Acceptance document created | `projects.acceptance` | project_id, kind, number, date, amount, status, description | `projects/AcceptanceService.Create` | `TestProjectsHistory_RecordsServiceMutations` |
| `projects.acceptance.signed` | Acceptance document signed | `projects.acceptance` | same (status changes) | `projects/AcceptanceService.Sign` | `TestProjectsHistory_RecordsServiceMutations` |
| `projects.acceptance.cancelled` | Acceptance document cancelled | `projects.acceptance` | same (status changes) | `projects/AcceptanceService.Cancel` | `TestProjectsHistory_RecordsServiceMutations` |
| `projects.acceptance.deleted` | Acceptance document deleted | `projects.acceptance` | same | `projects/AcceptanceService.Delete` | `TestProjectsHistory_RecordsServiceMutations` |
| `warehouse.product.created` | Product created | `warehouse.product` | rfid, status, position_id | `warehouse/ProductService.Create`, `warehouse/ProductService.BulkCreate (one event per product)`, `warehouse/ProductService.CreateProductsFromTags` | `TestWarehouseHistory_RecordsServiceActions, `TestWarehouseHistory_BulkCreateRecordsEachProduct`, `TestPositionRowHandler_PartialImport`` |
| `warehouse.product.updated` | Product changed | `warehouse.product` | same | `warehouse/ProductService.Update`, `warehouse/ProductService.ValidateProducts` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.product.deleted` | Product deleted | `warehouse.product` | same | `warehouse/ProductService.Delete` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.position.created` | Stock item created | `warehouse.position` | title, barcode, unit_id | `warehouse/PositionService.Create`, `warehouse/PositionService.CreatePositionFromXlsRow (new barcode)` | `TestWarehouseHistory_RecordsServiceActions, `TestPositionRowHandler_PartialImport`` |
| `warehouse.position.updated` | Stock item changed | `warehouse.position` | same | `warehouse/PositionService.Update`, `warehouse/PositionService.CreatePositionFromXlsRow (known barcode)` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.position.deleted` | Stock item deleted | `warehouse.position` | same | `warehouse/PositionService.Delete` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.unit.created` | Unit of measure created | `warehouse.unit` | title, short_title | `warehouse/UnitService.Create (also implicitly by the import)` | `TestWarehouseHistory_RecordsServiceActions, `TestPositionRowHandler_PartialImport`` |
| `warehouse.unit.updated` | Unit of measure changed | `warehouse.unit` | same | `warehouse/UnitService.Update` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.unit.deleted` | Unit of measure deleted | `warehouse.unit` | same | `warehouse/UnitService.Delete` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.order.created` | Warehouse order created | `warehouse.order` | type, status | `warehouse/OrderService.Create` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.order.updated` | Warehouse order changed | `warehouse.order` | same | `warehouse/OrderService.Update` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.order.completed` | Warehouse order completed | `warehouse.order` | status | `warehouse/OrderService.Complete` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.order.deleted` | Warehouse order deleted | `warehouse.order` | same | `warehouse/OrderService.Delete` | `TestWarehouseHistory_RecordsServiceActions` |
| `warehouse.inventory_check.created` | Inventory check created | `warehouse.inventory_check` | name, status | `warehouse/InventoryService.Create` | — (creation fails in the SDK today, see below) |
| `warehouse.inventory_check.updated` | Inventory check changed | `warehouse.inventory_check` | same | `warehouse/InventoryService.Update` | `TestWarehouseHistory_RecordsInventoryCheckActions` |
| `warehouse.inventory_check.deleted` | Inventory check deleted | `warehouse.inventory_check` | same | `warehouse/InventoryService.Delete` | `TestWarehouseHistory_RecordsInventoryCheckActions` |

"same" repeats the fields of the created event. \* — values readable only with `Client.Read` (crm)
or `User.Read` (core); \*\* — only with `Employee.Read` (hrm). Passwords, password hashes, tokens and
TOTP secrets are never part of a snapshot. Links between objects (roles, groups, permissions,
attached files) are separate events with both objects as subjects.

Known gaps in the services themselves, outside history: `warehouse/InventoryService.Create` fails on
`inventory_checks.type` being null, so its event has no test; cascade deletes in the database
(stages and acceptance documents of a project, products of a stock item) produce only the event of
the deleted parent.

## Exceptions

| Service.Method | Reason |
|---|---|
| `core/AuthFlowService.SetTwoFactorPolicy` | Wires the authentication policy at startup; no data changes |
| `core/AuthLogService.Create`, `core/AuthLogService.Update`, `core/AuthLogService.Delete` | Authentication log, a technical table |
| `core/AuthService.CreateSession`, `core/AuthService.CreateAgentSession`, `core/AuthService.CreateOnboardingSession` | Sessions, technical |
| `core/BrowserSessionService.Activate`, `core/BrowserSessionService.Add`, `core/BrowserSessionService.AddFromRequest`, `core/BrowserSessionService.RemoveAll`, `core/BrowserSessionService.RemoveCurrent`, `core/BrowserSessionService.SetAuthorizationRequestValidator` | Browser sessions, technical |
| `core/SessionService.Create`, `core/SessionService.Update`, `core/SessionService.Delete`, `core/SessionService.DeleteByUserID` | Sessions, technical |
| `core/TwoFactorService.ConfirmSetup`, `core/TwoFactorService.Disable`, `core/RecoveryCodeService.DeleteAll` | Authentication factors and their secrets; covered by the authentication log, not business history |
| `core/CurrencyService.Create`, `core/CurrencyService.Update`, `core/CurrencyService.Delete` | Shared currency catalogue without a tenant |
| `core/PermissionService.Save`, `core/PermissionService.Delete` | Permission catalogue synchronised from code |
| `core/NotificationRoutingService.SaveRule`, `core/NotificationService.MarkRead`, `core/NotificationService.MarkAllRead` | Notification settings and inbox state, technical |
| `core/TenantService.Create`, `core/TenantService.Update`, `core/TenantService.Delete` | Tenant administration in the superadmin console |
| `core/UploadService.Create`, `core/UploadService.CreateMany`, `core/UploadService.CreatePrivate`, `core/UploadService.Delete` | File storage; attaching a file to a business object is recorded by its owner |
| `core/ProjectService.Create`, `core/ProjectService.Update`, `core/ProjectService.Delete`, `core/ProjectService.Save` | Legacy service that no component provides; projects are recorded by `projects/ProjectService` |
| `core/UserService.UpdateLastLogin`, `core/UserService.UpdateLastAction` | Activity timestamps |
| `core/UserService.ReserveTemporaryPasswordAttempt`, `core/UserService.ReleaseTemporaryPasswordAttempt`, `core/UserService.LockCredentials` | Sign-in attempt bookkeeping and row locks before authentication |
| `crm/ChatService.SendMessage`, `crm/ChatService.AddMessageToChat`, `crm/ChatService.Save`, `crm/ChatService.Delete` | Correspondence threads, not business objects |
| `crm/MessageTemplateService.Create`, `crm/MessageTemplateService.Update`, `crm/MessageTemplateService.Delete` | Message templates are configuration |
| `finance/MoneyAccountService.RecalculateBalance` | Recomputes the balance from transactions inside the caller's transaction; the caller records the business event |
| `projects/ProjectStageService.UpdatePaidAmounts` | No-op: paid amounts are computed on read |
| `warehouse/PositionService.LoadFromFilePath`, `warehouse/PositionService.UpdateWithFile` | No entry point calls them; nested position, unit and product events are still recorded in the caller's operation |
