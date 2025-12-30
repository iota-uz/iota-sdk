---
layout: default
title: User Experience
parent: Core Module
nav_order: 4
description: "User workflows, interface flows, page structures, and interaction patterns"
---

# User Experience: Core Module

## User Journeys

### Authentication Flow

```
┌─────────────────────────────────────────────────────────┐
│               LOGIN AUTHENTICATION FLOW                  │
└─────────────────────────────────────────────────────────┘

  Anonymous User
        │
        ▼
  ┌─────────────────┐
  │  Login Page     │
  │  /login         │
  └────────┬────────┘
           │
           │ Email + Password
           ▼
  ┌─────────────────────────────────────┐
  │ Verify Credentials in Database      │
  │ Hash comparison with bcrypt         │
  └────────┬────────────────────────────┘
           │
      ┌────┴─────┐
      │           │
   Valid?       Invalid?
      │           │
      ▼           ▼
  ┌──────┐    ┌──────────────┐
  │ Yes  │    │ Error Message│
  └──┬───┘    │ Retry Login  │
     │        └──────────────┘
     │
     ▼
┌─────────────────────────────────┐
│ Create Session Token            │
│ - Store user_id, tenant_id      │
│ - Set expiration (24h)          │
│ - Record IP, user agent         │
└────────┬────────────────────────┘
         │
         ▼
┌─────────────────────┐
│ Redirect to         │
│ Dashboard           │
│ With Session Cookie │
└─────────────────────┘

Authenticated User
        │
        ▼
  Accessing /dashboard
        │
        ▼
┌──────────────────────────┐
│ Validate Session         │
│ - Check token in db      │
│ - Verify expiration      │
│ - Load user permissions  │
└────────┬─────────────────┘
         │
      ┌──┴──┐
   Valid? Expired?
      │    │
   Yes  No
      │    │
      ▼    ▼
  Allow  Logout
```

### User Management Flow

```
┌─────────────────────────────────────────────────────────┐
│            USER CREATION & MANAGEMENT FLOW              │
└─────────────────────────────────────────────────────────┘

  Admin User
        │
        ▼
  ┌─────────────────────────┐
  │ Users List              │
  │ /users                  │
  │                         │
  │ [+ New User Button]     │
  └────────┬────────────────┘
           │
           ▼
  ┌──────────────────────────┐
  │ Create User Form         │
  │ /users/new               │
  │                          │
  │ First Name *             │
  │ Last Name *              │
  │ Email *                  │
  │ Phone                    │
  │ Password *               │
  │ Language Preference *    │
  │ [Create] [Cancel]        │
  └───────┬──────────────────┘
          │
          ▼
  ┌────────────────────────────┐
  │ Validate Input             │
  │ - Email unique per tenant  │
  │ - Phone unique per tenant  │
  │ - Password strength        │
  │ - Required fields present  │
  └───────┬──────────┬──────────┘
          │          │
        Valid?    Invalid?
          │          │
          ▼          ▼
    ┌────────┐  ┌──────────────┐
    │ Create │  │ Show Errors  │
    │ User   │  │ Highlight    │
    └───┬────┘  │ Invalid      │
        │       │ Fields       │
        │       └──────────────┘
        ▼
┌────────────────────────────────┐
│ Hash Password with bcrypt      │
│ Create User Record in DB       │
│ Publish UserCreatedEvent       │
└──────────┬─────────────────────┘
           │
           ▼
┌────────────────────────────────┐
│ Redirect to User Details       │
│ /users/:id                     │
│                                │
│ Show Success Message           │
│ User Created Successfully      │
└────────────────────────────────┘
```

### Role Assignment Flow

```
┌─────────────────────────────────────────────────────────┐
│           ROLE ASSIGNMENT TO USER FLOW                  │
└─────────────────────────────────────────────────────────┘

  Admin on User Details Page
        │
        ▼
  ┌─────────────────────────┐
  │ Current Roles Section   │
  │                         │
  │ Admin ✓ [×]            │
  │ Editor ✓ [×]           │
  │                         │
  │ [+ Assign Role Button]  │
  └────────┬────────────────┘
           │
           ▼
  ┌──────────────────────────┐
  │ Modal: Select Role       │
  │                          │
  │ Available Roles:         │
  │ □ Accountant             │
  │ □ Manager                │
  │ ✓ Admin                  │
  │ □ Viewer                 │
  │                          │
  │ [Assign] [Cancel]        │
  └───────┬──────────────────┘
          │
          ▼
┌───────────────────────────────┐
│ Create user_roles Record      │
│ Load Role Permissions         │
│ Update Permission Cache       │
│ Publish RoleAssignedEvent     │
└────────┬──────────────────────┘
         │
         ▼
┌─────────────────────────────┐
│ Show Success Toast          │
│ "Role Assigned Successfully"│
│ Update Role List in UI      │
└─────────────────────────────┘
```

## Entry Points & Navigation

### Main Navigation Tree

```
Dashboard
├── Users
│   ├── List (searchable, paginated)
│   ├── Create
│   ├── Edit
│   └── Roles
├── Roles
│   ├── List
│   ├── Create
│   ├── Edit
│   └── Permissions
├── Groups
│   ├── List
│   ├── Create
│   ├── Edit
│   └── Members
├── Settings
│   ├── System Settings
│   ├── Localization
│   ├── Upload Settings
│   └── Session Management
└── Account
    ├── Profile
    ├── Change Password
    └── Preferences

Shortcuts (Spotlight/Quick Links)
├── Dashboard
├── Users List
├── New User
├── Groups
└── Roles
```

## Page Structures

### Users List Page (`/users`)

```
┌─────────────────────────────────────────────────────────┐
│                    USERS                                 │
├─────────────────────────────────────────────────────────┤
│                                                           │
│ Search: [__________________] Filters [v] [+ New User]   │
│                                                           │
├─────────────────────────────────────────────────────────┤
│ User                    | Email          | Roles | Actions│
├─────────────────────────────────────────────────────────┤
│ John Doe                | john@...       | 2     | Edit   │
│ Jane Smith              | jane@...       | 1     | Delete │
│ Bob Johnson             | bob@...        | 3     | View   │
│                                                           │
│ 1 2 3 ... 10  [Results 1-20 of 145]                     │
└─────────────────────────────────────────────────────────┘

Features:
- Sort by: Name, Email, Created Date, Last Login
- Filter by: Role, Group, Status
- Pagination: 20/50/100 per page
- Inline actions: Edit, Delete
- Bulk actions: Assign Role, Delete, Export
```

### User Create/Edit Form (`/users/new`, `/users/:id`)

```
┌─────────────────────────────────────────────────────────┐
│                    CREATE USER                           │
├─────────────────────────────────────────────────────────┤
│                                                           │
│ Personal Information                                      │
│ ┌──────────────────────────────────────────────────┐   │
│ │ First Name *     [___________________]           │   │
│ │ Last Name *      [___________________]           │   │
│ │ Middle Name      [___________________]           │   │
│ └──────────────────────────────────────────────────┘   │
│                                                           │
│ Contact Information                                       │
│ ┌──────────────────────────────────────────────────┐   │
│ │ Email *          [___________________]           │   │
│ │ Phone            [___________________]           │   │
│ │ UI Language *    [English        ↓]             │   │
│ └──────────────────────────────────────────────────┘   │
│                                                           │
│ Credentials                                               │
│ ┌──────────────────────────────────────────────────┐   │
│ │ Password *       [________] [show]              │   │
│ │ Confirm Password [________] [show]              │   │
│ │ ⓘ At least 8 characters, 1 uppercase, 1 number│   │
│ └──────────────────────────────────────────────────┘   │
│                                                           │
│ Roles & Permissions                                       │
│ ┌──────────────────────────────────────────────────┐   │
│ │ Assign Roles:                                    │   │
│ │ ☐ Admin          ☐ Editor                       │   │
│ │ ☐ Manager        ☐ Viewer                       │   │
│ └──────────────────────────────────────────────────┘   │
│                                                           │
│ Avatar (optional)                                         │
│ ┌──────────────────────────────────────────────────┐   │
│ │ [Select Image] or drag & drop                   │   │
│ │ Recommended: 200x200px, PNG/JPG                 │   │
│ └──────────────────────────────────────────────────┘   │
│                                                           │
│                      [Create] [Cancel]                   │
└─────────────────────────────────────────────────────────┘
```

### Roles List & Management (`/roles`)

```
┌─────────────────────────────────────────────────────────┐
│                    ROLES                                 │
├─────────────────────────────────────────────────────────┤
│                                                           │
│ Search: [__________________] [+ New Role]               │
│                                                           │
├─────────────────────────────────────────────────────────┤
│ Role Name           | Type    | Permissions | Actions    │
├─────────────────────────────────────────────────────────┤
│ Admin               | System  | 50+         | View       │
│ Accountant          | Custom  | 12          | Edit       │
│ Manager             | Custom  | 8           | Delete     │
│                                                           │
└─────────────────────────────────────────────────────────┘

Role Details Expand:
┌──────────────────────────────────────────────────────────┐
│ Admin (System Role)                                      │
│ Permissions Assigned:                                    │
│  ✓ users:create:all        ✓ roles:delete:all          │
│  ✓ users:read:all          ✓ groups:create:all         │
│  ✓ users:update:all        ✓ groups:read:all           │
│  ✓ users:delete:all        ✓ groups:update:all         │
│                                                          │
│ Users with this role: 2                                 │
│  • John Doe                                             │
│  • Admin User                                           │
└──────────────────────────────────────────────────────────┘
```

### Groups Management (`/groups`)

```
┌─────────────────────────────────────────────────────────┐
│                    GROUPS                                │
├─────────────────────────────────────────────────────────┤
│                                                           │
│ Search: [__________________] [+ New Group]              │
│                                                           │
├─────────────────────────────────────────────────────────┤
│ Group Name          | Members | Roles | Actions         │
├─────────────────────────────────────────────────────────┤
│ Finance Team        | 3       | 1     | Edit            │
│ Sales Department    | 5       | 2     | Delete          │
│ Support Team        | 2       | 1     | View            │
│                                                           │
└─────────────────────────────────────────────────────────┘
```

## HTMX Interaction Patterns

### Dynamic User Search

```html
<!-- Search input with HTMX -->
<input
  type="text"
  name="search"
  placeholder="Search users..."
  hx-get="/users/search"
  hx-trigger="input changed delay:500ms"
  hx-target="#users-list"
  hx-indicator=".spinner"
/>

<!-- Results update in place -->
<div id="users-list">
  <!-- Updated via HTMX response -->
</div>
```

### Inline Role Assignment

{% raw %}
```html
<!-- Button to open assignment modal -->
<button
  hx-get="/users/{{ .ID }}/assign-role-modal"
  hx-target="body"
  hx-trigger="click"
  hx-swap="beforeend"
>
  Assign Role
</button>
```
{% endraw %}

### Toggle User Status

{% raw %}
```html
<!-- Status toggle with HTMX -->
<button
  hx-post="/users/{{ .ID }}/toggle-status"
  hx-target="this"
  hx-swap="outerHTML"
  class="status-badge"
>
  {{ if .IsActive }}Active{{ else }}Inactive{{ end }}
</button>
```
{% endraw %}

### Delete Confirmation

{% raw %}
```html
<!-- Delete with confirmation -->
<button
  hx-delete="/users/{{ .ID }}"
  hx-confirm="Are you sure you want to delete this user?"
  hx-target="closest tr"
  hx-swap="outerHTML swap:1s"
>
  Delete
</button>
```
{% endraw %}

## Alpine.js Patterns

### Dropdown Menus

```html
<!-- Role selection dropdown -->
<div x-data="{ open: false }">
  <button
    @click="open = !open"
    class="dropdown-trigger"
  >
    Select Role
  </button>

  <div
    x-show="open"
    @click.outside="open = false"
    class="dropdown-menu"
  >
    <a href="#" @click="selectRole('admin')">Admin</a>
    <a href="#" @click="selectRole('editor')">Editor</a>
  </div>
</div>
```

### Form Validation

```html
<!-- Real-time validation -->
<div x-data="{ email: '', valid: false }">
  <input
    x-model="email"
    @input="valid = /^[^@]+@[^@]+$/.test(email)"
    type="email"
    placeholder="Email address"
  />
  <span x-show="!valid" class="error">Invalid email</span>
</div>
```

### Pagination Controls

```html
<!-- Pagination with state -->
<div x-data="{ page: 1, pageSize: 20 }">
  <select x-model="pageSize" @change="page = 1">
    <option value="20">20 per page</option>
    <option value="50">50 per page</option>
    <option value="100">100 per page</option>
  </select>

  <button @click="page--" :disabled="page === 1">Previous</button>
  <span x-text="'Page ' + page"></span>
  <button @click="page++">Next</button>
</div>
```

## Form Field Patterns

### Text Input with Validation

{% raw %}
```html
<div class="form-group">
  <label for="firstName">First Name *</label>
  <input
    id="firstName"
    type="text"
    name="FirstName"
    value="{{ .User.FirstName }}"
    required
    placeholder="Enter first name"
  />
  {{ if .Errors.FirstName }}
    <span class="error">{{ .Errors.FirstName }}</span>
  {{ end }}
</div>
```
{% endraw %}

### Select with Dynamic Loading

```html
<div class="form-group">
  <label for="role">Assign Role *</label>
  <select
    id="role"
    name="RoleID"
    hx-get="/roles/options"
    hx-trigger="focus"
    required
  >
    <option value="">Select a role...</option>
  </select>
</div>
```

### Multi-select with HTMX

```html
<div class="form-group">
  <label>Roles</label>
  <div
    hx-get="/roles/available"
    hx-trigger="load"
    hx-target="this"
  >
    Loading roles...
  </div>
</div>
```

## Response States

### Success Messages

```
✓ User created successfully
  Redirecting to user details...
```

### Error Messages

```
✗ Email already exists for this tenant
  Please use a different email address.
```

### Loading States

```
⟳ Loading users...
  Please wait while we fetch the data.
```

## Keyboard Navigation

| Shortcut | Action |
|----------|--------|
| `Cmd+K` / `Ctrl+K` | Open Spotlight search |
| `Escape` | Close modals/dropdowns |
| `Enter` | Submit forms |
| `Tab` | Navigate form fields |
| `Shift+Tab` | Navigate backwards |
