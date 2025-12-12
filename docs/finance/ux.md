---
layout: default
title: User Experience
parent: Finance Module
nav_order: 4
description: "Financial workflow patterns, interface flows, reporting, and user interactions"
---

# User Experience: Finance Module

## Financial Workflows

### Payment Recording Workflow

```
┌─────────────────────────────────────────────────────────┐
│            PAYMENT RECORDING WORKFLOW                    │
└─────────────────────────────────────────────────────────┘

  User initiates payment recording
        │
        ▼
  ┌──────────────────────────────────┐
  │ Payment Recording Page           │
  │ /finance/overview?tab=payments   │
  │                                  │
  │ [+ New Payment Button]           │
  └──────────┬───────────────────────┘
             │
             ▼
  ┌────────────────────────────────────────┐
  │ Payment Form                           │
  │ /finance/payments/new                  │
  │                                        │
  │ Account *        [Select Account ▼]   │
  │ Counterparty *   [Select Customer ▼]  │
  │ Amount *         [_______________]    │
  │ Category         [Select Category ▼]  │
  │ Date *           [Calendar Picker]    │
  │ Description      [Multiline text]     │
  │ Attachments      [Drag & drop]        │
  │                                        │
  │ [Record Payment] [Cancel]             │
  └────────┬─────────────────────────────┘
           │
           ▼
  ┌──────────────────────────────────────┐
  │ Validate Input                       │
  │ - Account exists                     │
  │ - Counterparty exists                │
  │ - Amount positive                    │
  │ - Sufficient balance                 │
  │ - Date valid                         │
  └────────┬────────┬────────────────────┘
           │        │
        Valid?    Invalid?
           │        │
           ▼        ▼
  ┌──────────────┐  ┌──────────────────┐
  │ Create       │  │ Show Errors      │
  │ Transaction  │  │ Highlight Fields │
  │ Create       │  └──────────────────┘
  │ Payment      │
  │ Update       │
  │ Balance      │
  └───┬──────────┘
      │
      ▼
  ┌──────────────────────────────────┐
  │ Payment Created                  │
  │ Transaction: TXN-12345           │
  │ New Balance: $8,450.00           │
  │                                  │
  │ [View Details] [Record Another]  │
  └──────────────────────────────────┘
```

### Expense Submission Workflow

```
┌─────────────────────────────────────────────────────────┐
│            EXPENSE SUBMISSION WORKFLOW                   │
└─────────────────────────────────────────────────────────┘

  Employee initiates expense submission
        │
        ▼
  ┌──────────────────────────────────┐
  │ Expense Recording                │
  │ /finance/overview?tab=expenses   │
  │                                  │
  │ [+ Record Expense Button]        │
  └──────────┬───────────────────────┘
             │
             ▼
  ┌────────────────────────────────────────┐
  │ Expense Form                           │
  │ /finance/expenses/new                  │
  │                                        │
  │ Category *       [Select Category ▼]   │
  │ Amount *         [_______________]    │
  │ Date *           [Calendar Picker]    │
  │ Description      [Multiline text]     │
  │ Payee/Vendor     [________________]   │
  │ Receipt          [Drag & drop]        │
  │ Project (opt.)   [Select Project ▼]   │
  │                                        │
  │ [Submit] [Save Draft] [Cancel]        │
  └────────┬──────────────────────────────┘
           │
           ▼
  ┌──────────────────────────────────┐
  │ Validate Input                   │
  │ - Category exists                │
  │ - Amount positive                │
  │ - Receipt attached (if req'd)    │
  └────────┬────────┬────────────────┘
           │        │
        Valid?    Invalid?
           │        │
           ▼        ▼
  ┌──────────────┐  ┌──────────────────┐
  │ Create       │  │ Show Errors      │
  │ Expense      │  └──────────────────┘
  │ Create       │
  │ Transaction  │
  │ Publish      │
  │ Event        │
  └───┬──────────┘
      │
      ▼
  ┌──────────────────────────────────┐
  │ Expense Submitted Successfully   │
  │ Reference: EXP-5678              │
  │                                  │
  │ [View Details] [Submit Another]  │
  └──────────────────────────────────┘
```

### Debt Settlement Workflow

```
┌─────────────────────────────────────────────────────────┐
│           DEBT SETTLEMENT WORKFLOW                       │
└─────────────────────────────────────────────────────────┘

  Finance team opens Debts management
        │
        ▼
  ┌──────────────────────────────────┐
  │ Debts List                       │
  │ /finance/debts                   │
  │                                  │
  │ Overdue  Outstanding  Settled    │
  │ $45K     $120K        $1.2M      │
  │                                  │
  │ Outstanding Debts:               │
  │ [Debt record] [Settle] [Details] │
  │ [Debt record] [Settle] [Details] │
  └──────────┬───────────────────────┘
             │
             ▼
  ┌────────────────────────────────────────┐
  │ Settle Debt Dialog                     │
  │                                        │
  │ Debt ID: DEBT-2024-001                 │
  │ Counterparty: ABC Supplier             │
  │ Outstanding: $5,000 USD                │
  │                                        │
  │ Settlement Method:                     │
  │ ⊙ Full Payment   ◯ Partial Payment    │
  │                                        │
  │ Amount to Pay *  [5000.00]            │
  │ From Account *   [Select Account ▼]   │
  │ Settlement Date  [Calendar Picker]    │
  │                                        │
  │ [Settle] [Cancel]                     │
  └────────┬─────────────────────────────┘
           │
           ▼
  ┌──────────────────────────────────┐
  │ Process Settlement                │
  │ - Create settlement transaction   │
  │ - Update account balance          │
  │ - Update debt status              │
  │ - Publish settlement event        │
  └────────┬─────────────────────────┘
           │
           ▼
  ┌──────────────────────────────────┐
  │ Settlement Complete              │
  │ Debt Status: SETTLED             │
  │ Settlement Ref: TXN-99999        │
  │ Updated Balance: $3,000.00       │
  │                                  │
  │ [View Debt] [Return to List]    │
  └──────────────────────────────────┘
```

## Entry Points & Navigation

### Finance Module Navigation

```
Dashboard
├── Financial Overview
│   ├── Payments (Tab)
│   ├── Expenses (Tab)
│   ├── Account Balance Summary
│   └── Quick Actions
├── Accounts
│   ├── Account List
│   ├── Account Details
│   ├── Create Account
│   └── Account Reconciliation
├── Payments
│   ├── Payment List
│   ├── Payment Details
│   ├── Create Payment
│   └── Payment Categories
├── Expenses
│   ├── Expense List
│   ├── Expense Details
│   ├── Record Expense
│   └── Expense Categories
├── Debts
│   ├── Receivables (Customer Debts)
│   ├── Payables (Vendor Debts)
│   ├── Debt Details
│   ├── Create Debt
│   └── Settlement Management
├── Counterparties
│   ├── Customer List
│   ├── Supplier List
│   ├── Contact Management
│   └── Counterparty Details
├── Inventory
│   ├── Product List
│   ├── Product Details
│   ├── Add Product
│   └── Stock Levels
└── Reports
    ├── Income Statement (P&L)
    ├── Cash Flow Statement
    ├── Account Statements
    ├── Aged Receivables
    ├── Aged Payables
    └── Custom Reports
```

## Page Structures

### Financial Overview Dashboard (`/finance/overview`)

```
┌─────────────────────────────────────────────────────────┐
│                  FINANCIAL OVERVIEW                      │
├─────────────────────────────────────────────────────────┤
│                                                           │
│  ┌──────────────┬──────────────┬──────────────┐          │
│  │ Cash Balance │ Receivables  │ Payables     │          │
│  │              │              │              │          │
│  │ $45,230.50   │ $125,000     │ ($85,000)    │          │
│  │ USD          │ USD          │ USD          │          │
│  └──────────────┴──────────────┴──────────────┘          │
│                                                           │
│  Tabs: [Payments] [Expenses] [Transactions]              │
│                                                           │
│  ┌─────────────────────────────────────────────────┐    │
│  │ PAYMENTS (Payments Tab)                         │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ Search: [.........] Filters [v] [+ New Payment] │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ Counterparty | Amount  | Date      | Category   │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ ABC Corp     | $5,000  | Dec 12    | Office     │    │
│  │ XYZ Ltd      | $2,300  | Dec 11    | Travel     │    │
│  │ 123 Services | $1,500  | Dec 10    | Utilities  │    │
│  │                                                  │    │
│  │ [Load More] [1 2 3 ... 10]                      │    │
│  └─────────────────────────────────────────────────┘    │
│                                                           │
│  ┌─────────────────────────────────────────────────┐    │
│  │ EXPENSES (Expenses Tab)                         │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ Search: [.........] [+ Record Expense]          │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ Category   | Amount  | Date      | Submitted By │    │
│  ├─────────────────────────────────────────────────┤    │
│  │ Travel     | $450    | Dec 12    | John Doe     │    │
│  │ Office     | $200    | Dec 11    | Jane Smith   │    │
│  │ Meals      | $85     | Dec 10    | Bob Johnson  │    │
│  └─────────────────────────────────────────────────┘    │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

### Accounts Management (`/finance/accounts`)

```
┌─────────────────────────────────────────────────────────┐
│                    ACCOUNTS                              │
├─────────────────────────────────────────────────────────┤
│                                                           │
│ Search: [__________________] [+ New Account]            │
│                                                           │
├─────────────────────────────────────────────────────────┤
│ Account Name    | Number      | Balance  | Currency     │
├─────────────────────────────────────────────────────────┤
│ Main Bank       | 40702810... | $45,230  | USD          │
│ Petty Cash      | Cash-001    | $1,250   | USD          │
│ Savings         | 40702810... | $120,000 | USD          │
│ EUR Account     | EUR-001     | €35,500  | EUR          │
│                                                           │
│ [Reconcile] [View Statement] [Transactions]             │
│                                                           │
└─────────────────────────────────────────────────────────┘

Account Detail View:
┌──────────────────────────────────────────────────────────┐
│ Main Bank Account                              [Edit]   │
├──────────────────────────────────────────────────────────┤
│ Account Number: 40702810000001                          │
│ Currency: USD                                           │
│ Current Balance: $45,230.50                             │
│ Last Transaction: Dec 12, 2024 @ 2:30 PM               │
│                                                          │
│ Recent Transactions (Last 10):                          │
│ ─────────────────────────────────────────────────────   │
│ Dec 12 | Payment to ABC Corp      | -$5,000   | $45,230│
│ Dec 11 | Payment to XYZ Ltd       | -$2,300   | $50,230│
│ Dec 10 | Income from Sales        | +$8,000   | $52,530│
│ ...                                                      │
└──────────────────────────────────────────────────────────┘
```

### Debts Management (`/finance/debts`)

```
┌─────────────────────────────────────────────────────────┐
│                    DEBTS MANAGEMENT                      │
├─────────────────────────────────────────────────────────┤
│                                                           │
│ [Receivables] [Payables] [All Debts]                    │
│                                                           │
│ Status Filter: [All] [Pending] [Partial] [Settled]      │
│ [+ New Debt]                                             │
│                                                           │
├─────────────────────────────────────────────────────────┤
│ Counterparty | Outstanding | Due Date | Status | Actions│
├─────────────────────────────────────────────────────────┤
│ ABC Corp     | $5,000      | Dec 25   | PENDING| Settle │
│ XYZ Ltd      | $8,500      | Dec 20   | OVERDUE| Settle │
│ 123 Supplies | $2,000      | Jan 5    | PENDING| Settle │
│                                                           │
│ Summary:                                                 │
│ Total Receivables: $125,000                              │
│ Overdue (>30 days): $25,000                             │
│ Average Days to Pay: 45 days                             │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

### Financial Reports (`/finance/reports`)

```
┌─────────────────────────────────────────────────────────┐
│                  FINANCIAL REPORTS                       │
├─────────────────────────────────────────────────────────┤
│                                                           │
│ Report Type:                                             │
│ [Income Statement] [Cash Flow] [Account Statement]      │
│ [Aged Receivables] [Aged Payables] [Custom]             │
│                                                           │
│ Period: [From: Dec 1] [To: Dec 31] [Generate]          │
│                                                           │
├─────────────────────────────────────────────────────────┤
│             INCOME STATEMENT - DECEMBER 2024             │
├─────────────────────────────────────────────────────────┤
│                                                           │
│ REVENUE                                                  │
│   Sales                          $125,000                │
│   Services                        $45,000                │
│   ─────────────────────────────────────────             │
│   Total Revenue                   $170,000               │
│                                                           │
│ EXPENSES                                                 │
│   Salaries                        ($80,000)              │
│   Office                          ($15,000)              │
│   Travel                          ($8,000)               │
│   Utilities                       ($3,500)               │
│   ─────────────────────────────────────────             │
│   Total Expenses                  ($106,500)             │
│                                                           │
│ ═════════════════════════════════════════════          │
│ NET INCOME                        $63,500                │
│ ═════════════════════════════════════════════          │
│                                                           │
│ [Export PDF] [Print] [Email]                            │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

## HTMX Interaction Patterns

### Dynamic Account Selection

```html
<!-- Account selector with balance preview -->
<select
  name="AccountID"
  hx-get="/finance/accounts/balance"
  hx-trigger="change"
  hx-target="#balance-preview"
  hx-include="[name='AccountID']"
  required
>
  <option value="">Select Account</option>
  <!-- Options populated via hx-load -->
</select>

<!-- Display balance dynamically -->
<div id="balance-preview">
  <!-- Balance shown via HTMX response -->
</div>
```

### Live Debt Settlement

```html
<!-- Settle debt with confirmation -->
<button
  hx-post="/finance/debts/{{ .DebtID }}/settle"
  hx-confirm="Settle debt for ${{ .Amount }}?"
  hx-target="closest tr"
  hx-swap="outerHTML"
>
  Settle
</button>
```

### Dynamic Report Generation

```html
<!-- Generate report with loading -->
<form
  hx-post="/finance/reports/generate"
  hx-target="#report-results"
  hx-indicator=".loading-spinner"
>
  <input type="date" name="StartDate" required />
  <input type="date" name="EndDate" required />
  <button type="submit">Generate Report</button>
</form>

<div id="report-results">
  <!-- Report rendered here -->
</div>
```

### Real-time Balance Update

```html
<!-- Display real-time account balance -->
<div
  hx-get="/finance/accounts/{{ .AccountID }}/balance"
  hx-trigger="load, updateBalance from:body"
  hx-swap="innerHTML"
>
  Loading balance...
</div>
```

## Alpine.js Patterns

### Amount Formatter

```html
<!-- Format currency input -->
<div x-data="{ amount: 0 }">
  <input
    type="number"
    x-model="amount"
    @blur="amount = (amount / 100).toFixed(2) * 100"
    placeholder="Amount"
  />
  <span x-text="'$' + (amount / 100).toFixed(2)"></span>
</div>
```

### Date Range Selector

```html
<!-- Date range for reports -->
<div x-data="{ startDate: '', endDate: '' }">
  <input
    type="date"
    x-model="startDate"
    @change="endDate = startDate"
  />
  <input type="date" x-model="endDate" />
  <button @click="generateReport()">Generate</button>
</div>
```

### Currency Selector

```html
<!-- Multi-currency support -->
<div x-data="{ currency: 'USD', amount: 0, convertedAmount: 0 }">
  <select x-model="currency">
    <option value="USD">USD</option>
    <option value="EUR">EUR</option>
    <option value="UZS">UZS</option>
  </select>
  <input x-model="amount" type="number" />
  <span x-text="currency + ' ' + amount"></span>
</div>
```

## Form Field Patterns

### Account Selection

```html
<div class="form-group">
  <label for="account">Account *</label>
  <select
    id="account"
    name="AccountID"
    required
    hx-get="/finance/accounts/balance"
    hx-trigger="change"
    hx-target="#balance-info"
  >
    <option value="">Select Account</option>
    <!-- Account options -->
  </select>
  <div id="balance-info">
    <!-- Balance info loaded via HTMX -->
  </div>
</div>
```

### Counterparty Selection with Quick Add

```html
<div class="form-group">
  <label for="counterparty">Counterparty *</label>
  <div class="input-group">
    <select
      id="counterparty"
      name="CounterpartyID"
      required
    >
      <option value="">Select or Create...</option>
      <!-- Counterparty options -->
    </select>
    <button
      type="button"
      hx-get="/finance/counterparties/quick-add"
      hx-target="body"
      hx-swap="beforeend"
    >
      +
    </button>
  </div>
</div>
```

### Amount Input with Currency

```html
<div class="form-group">
  <label for="amount">Amount *</label>
  <div class="input-group">
    <input
      id="amount"
      type="number"
      name="Amount"
      step="0.01"
      min="0"
      required
      placeholder="0.00"
    />
    <select name="Currency" required>
      <option value="USD">USD</option>
      <option value="EUR">EUR</option>
    </select>
  </div>
</div>
```

## Response States

### Success Confirmation

```
✓ Payment recorded successfully
  Transaction: TXN-12345
  New balance: $45,230.50
  [View Details]
```

### Validation Errors

```
✗ Unable to record payment
  - Account balance insufficient ($1,200 < $5,000)
  - Please select a different account or reduce amount
```

### Processing State

```
⟳ Processing payment...
  Account: Main Bank
  Amount: $5,000
  Please wait...
```

## Report Layouts

### Income Statement Format

```
[Company Name]
INCOME STATEMENT
For the Period: January 1 - December 31, 2024

REVENUE
  Sales                          $1,250,000
  Services                         $450,000
  ────────────────────────────────────────
  Total Revenue                  $1,700,000

COST OF GOODS SOLD
  Materials                        ($400,000)
  Labor                            ($300,000)
  ────────────────────────────────────────
  Total COGS                       ($700,000)

GROSS PROFIT                     $1,000,000

OPERATING EXPENSES
  Salaries                         ($400,000)
  Rent                             ($60,000)
  Utilities                        ($24,000)
  Marketing                        ($50,000)
  ────────────────────────────────────────
  Total Operating Expenses         ($534,000)

OPERATING INCOME                   $466,000

OTHER INCOME/EXPENSES
  Interest Income                   $5,000
  Interest Expense                  ($2,000)
  ────────────────────────────────────────
  Total Other Income                $3,000

NET INCOME                         $469,000
═════════════════════════════════════════
```

### Cash Flow Statement Format

```
[Company Name]
STATEMENT OF CASH FLOWS
For the Period: January 1 - December 31, 2024

OPERATING ACTIVITIES
  Net Income                       $469,000
  Adjustments:
    Depreciation                    $40,000
    Accounts Receivable Change     ($50,000)
    Inventory Change               ($30,000)
  ────────────────────────────────────────
  Net Cash from Operations          $429,000

INVESTING ACTIVITIES
  Equipment Purchases             ($150,000)
  Sale of Fixed Assets              $25,000
  ────────────────────────────────────────
  Net Cash from Investing          ($125,000)

FINANCING ACTIVITIES
  Loan Proceeds                    $100,000
  Loan Repayments                  ($50,000)
  ────────────────────────────────────────
  Net Cash from Financing            $50,000

NET CHANGE IN CASH                 $354,000
Cash at Beginning of Period        $200,000
────────────────────────────────────────
Cash at End of Period              $554,000
═════════════════════════════════════════
```

## Keyboard Navigation

| Shortcut | Action |
|----------|--------|
| `Ctrl+N` / `Cmd+N` | New payment/expense |
| `Escape` | Close modals |
| `Enter` | Submit forms |
| `Tab` | Navigate fields |
| `Shift+Tab` | Navigate backwards |
| `Ctrl+S` / `Cmd+S` | Save/Submit |
| `Ctrl+P` / `Cmd+P` | Print report |
| `Ctrl+E` / `Cmd+E` | Export to PDF |
