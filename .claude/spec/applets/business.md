# Business Specification: Applet System

**Status:** Draft

## Problem Statement

IOTA SDK provides robust multi-tenant business management capabilities, but adding new functionality currently requires:

1. **Writing Go code** - High barrier for web developers
2. **SDK recompilation** - Deployment complexity, version management
3. **Core SDK changes** - Feature requests bottleneck on SDK team
4. **Forking the SDK** - Maintenance burden, merge conflicts

**Current Pain Points:**

- The **Website/Ali module** (AI chatbot) is embedded in SDK core, but it's a specialized feature that not all tenants need
- **Shyona** (AI analytics) required building a full Go module with 15+ services
- Partners want to add custom integrations (Slack, Zapier, custom CRMs) without SDK involvement
- Different tenants have vastly different needs that don't justify core SDK features

**Business Impact:**

- **Lost Revenue:** Potential customers need features SDK doesn't provide
- **Slow Time-to-Market:** Custom features take weeks/months to implement
- **High Development Cost:** Every integration requires Go expertise
- **Limited Ecosystem:** No third-party developer community

## Target Audience

### Primary: Internal Development Team

- **Goal:** Decouple optional features (AI, website widgets) from SDK core
- **Skill Level:** Full-stack developers (TypeScript, React, Go)
- **Frequency:** Creating new applets for client projects

### Secondary: Partner Developers

- **Goal:** Build custom integrations for their clients
- **Skill Level:** Web developers (TypeScript, React)
- **Frequency:** Building applets as part of implementation projects

### Tertiary: Third-Party Developers (Future)

- **Goal:** Publish applets to marketplace for distribution
- **Skill Level:** Varied (need good documentation, templates)
- **Frequency:** Building and maintaining applets as products

## Use Cases

### UC1: AI Website Chat (Reference: modules/website)

**Current State:** Compiled into SDK, requires Go expertise to modify

**Desired State:** Installable applet with:
- Configuration page for API keys, model selection
- Embeddable chat widget for customer websites
- CRM integration (creates clients, routes messages to chats)
- AI response generation via external APIs (OpenAI, custom)

**Applet Structure:**
```
ai-website-chat/
├── manifest.yaml
├── backend/
│   ├── handlers/
│   │   ├── config.ts      # CRUD for AI configuration
│   │   ├── threads.ts     # Thread management
│   │   └── messages.ts    # Message handling + AI response
│   └── services/
│       └── ai-service.ts  # OpenAI/LLM integration
├── frontend/
│   ├── pages/
│   │   └── config.tsx     # Configuration page (React)
│   └── components/
│       └── ChatWidget.tsx # Embeddable widget
└── locales/
    ├── en.json
    └── ru.json
```

### UC2: Shyona-Style AI Analytics (Reference: shy-trucks/modules/shyona)

**Current State:** Full Go module with complex agent framework

**Desired State:** Applet that provides:
- Natural language business queries
- SQL query generation and execution
- Multi-agent orchestration
- Knowledge base integration
- GraphQL API for frontend

**Challenges:**
- Complex service orchestration
- Heavy computation (may need Go or WASM)
- Deep SDK integration (GraphQL schema extension)

**Possible Approach:**
- Core agent framework remains in SDK (reusable)
- Applet defines specific agents, prompts, tools
- Applet provides UI for chat interface

### UC3: Custom CRM Integration

**Scenario:** Partner wants to sync clients with Salesforce

**Applet Provides:**
- Configuration page for Salesforce credentials
- Event handlers for `client.created`, `client.updated`
- Scheduled sync job (hourly full sync)
- UI widget showing sync status on client detail page

### UC4: Custom Reporting Dashboard

**Scenario:** Tenant needs specialized financial reports

**Applet Provides:**
- Custom SQL queries (read-only)
- Visualization components (charts, tables)
- PDF export functionality
- Scheduled email reports

### UC5: Third-Party Webhook Handler

**Scenario:** Receive webhooks from Stripe, process payments

**Applet Provides:**
- HTTP endpoint for webhook reception
- Event handlers to create payment records
- Configuration for webhook secret validation

## Requirements

### In Scope

**Runtime Capabilities:**
- [ ] HTTP endpoint handlers (GET, POST, PUT, DELETE)
- [ ] Event handlers (subscribe to SDK domain events)
- [ ] Scheduled tasks (cron-based)
- [ ] External HTTP requests (with security controls)
- [ ] Database read access (existing SDK tables)
- [ ] Database write access (with permissions)
- [ ] Custom database tables (with approval)
- [ ] Secret management (API keys, tokens)

**UI Capabilities:**
- [ ] Register navigation items
- [ ] Register full pages
- [ ] Inject widgets into existing pages
- [ ] Use SDK UI components for consistency
- [ ] Custom styling within design token constraints
- [ ] Localization/i18n support

**Developer Experience:**
- [ ] TypeScript support with full type definitions
- [ ] React/Next.js for frontend development
- [ ] Hot reload during development
- [ ] Local development server
- [ ] CLI for packaging and deployment
- [ ] Clear error messages and debugging

**Administration:**
- [ ] Install/uninstall applets per tenant
- [ ] Enable/disable applets
- [ ] Configure applet-specific settings
- [ ] View applet logs and metrics
- [ ] Permission review before installation

### Out of Scope (Initial Version)

- [ ] Real-time WebSocket support
- [ ] Background workers (long-running processes)
- [ ] File system access
- [ ] Native code execution
- [ ] Cross-tenant data access
- [ ] SDK core modification
- [ ] GraphQL schema extension (complex, security concerns)
- [ ] WASM modules (future consideration)

### Future Scope

- [ ] Applet marketplace/registry
- [ ] Applet versioning and updates
- [ ] Applet reviews and ratings
- [ ] Revenue sharing for paid applets
- [ ] WASM for compute-intensive applets
- [ ] GraphQL schema extension (controlled)
- [ ] Multi-applet communication

## Success Criteria

1. **Website/Ali can be extracted** from SDK core into an installable applet
2. **Partner can build** a custom integration in TypeScript without Go knowledge
3. **UI looks native** - applet pages indistinguishable from SDK pages
4. **Installation is simple** - admin can install from package in < 5 minutes
5. **Security is maintained** - no tenant isolation bypass, no SSRF, no data leaks

## Assumptions

- Developers have TypeScript/React experience
- Tenants have reliable internet for external API calls
- SDK team will maintain component library for UI consistency
- Initial distribution is file-based (no marketplace)

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| **Applet breaks tenant** | High | Sandboxing, resource limits, disable on error |
| **Security vulnerability** | Critical | Permission model, code review for marketplace |
| **Performance degradation** | Medium | Resource quotas, monitoring, circuit breakers |
| **UI inconsistency** | Medium | Mandatory component library, design tokens |
| **Maintenance burden** | Medium | Clear versioning, deprecation policy |
| **Complex debugging** | Medium | Good logging, error traces, dev tools |
