# Open Questions & Unresolved Decisions

**Status:** Draft

## Overview

This document captures unresolved decisions, trade-offs requiring input, and open questions for the Applet System. These should be addressed before implementation begins.

---

## Runtime Architecture

### Q1: Single vs Multi-Process Runtime

**Question:** Should each applet run in its own Bun process, or share a process pool?

**Options:**

| Option | Pros | Cons |
|--------|------|------|
| **One process per applet** | Complete isolation, easy resource limits, crash isolation | Memory overhead, more processes to manage |
| **Shared process pool** | Lower memory usage, faster startup | Cross-applet interference risk, complex isolation |
| **Hybrid** | Best of both (separate for heavy, shared for light) | Complex management logic |

**Recommendation:** Start with one process per applet for simplicity and isolation. Optimize later if memory becomes an issue.

**Decision Required:** Which approach to implement?

---

### Q2: Goja vs Bun-Only Runtime

**Question:** Should we maintain Goja for simple scripts, or go Bun-only?

**Context:** We have existing jsruntime spec using Goja. Applets could use either.

**Options:**

| Option | Pros | Cons |
|--------|------|------|
| **Bun only** | Simpler, one runtime to maintain, full TypeScript | Heavier for simple scripts |
| **Goja for scripts, Bun for applets** | Lighter for simple cases, embedded | Two runtimes to maintain |
| **Goja first, Bun optional** | Backward compatible | Two code paths |

**Recommendation:** Bun-only for applets, keep Goja for existing jsruntime scripts (separate feature).

**Decision Required:** Final runtime strategy?

---

### Q3: Hot Reload Strategy

**Question:** How should applet code updates work during development?

**Options:**
1. Full process restart on changes
2. Bun's built-in hot reload
3. File watcher with graceful reload
4. Manual reload only

**Considerations:**
- Development experience vs complexity
- State preservation during reload
- Connection handling during restart

**Decision Required:** Development workflow approach?

---

## Frontend Framework

### Q4: React Component Library Scope

**Question:** How comprehensive should `@iota/components` be?

**Options:**

| Scope | Components | Effort |
|-------|------------|--------|
| **Minimal** | Button, Input, Select, Card, Table | 2-4 weeks |
| **Standard** | Above + Modal, Drawer, Toast, Tabs, etc. | 6-8 weeks |
| **Comprehensive** | Above + Charts, DatePicker, RichText, etc. | 12-16 weeks |

**Recommendation:** Start with Standard scope, add more as needed.

**Decision Required:** Initial component library scope?

---

### Q5: Web Components Priority

**Question:** Should we build Web Components alongside React components?

**Context:** Web Components enable framework-agnostic widgets, useful for embedding.

**Options:**
1. React only (simpler, faster to build)
2. Web Components for embeddables only (chat widgets, etc.)
3. Full parallel library (React + Web Components)

**Recommendation:** Option 2 - Web Components for embeddables only.

**Decision Required:** Web Components strategy?

---

### Q6: Styling Strategy

**Question:** How should applet styles be handled?

**Options:**

| Strategy | Approach | Isolation |
|----------|----------|-----------|
| **CSS-in-JS** | Styled-components, Emotion | High |
| **Tailwind** | Utility classes | Medium |
| **CSS Modules** | Scoped CSS files | High |
| **Shadow DOM** | Web Component encapsulation | Complete |

**Considerations:**
- SDK uses Tailwind currently
- Style conflicts between applets
- Bundle size impact
- Developer familiarity

**Recommendation:** Tailwind with CSS Modules for applet-specific styles.

**Decision Required:** Final styling approach?

---

## Database & Storage

### Q7: Custom Table Prefix Strategy

**Question:** How should applet tables be named?

**Options:**
1. `applet_{id}_{table}` - e.g., `applet_ai_chat_configs`
2. `app_{id}_{table}` - shorter
3. Separate schema per applet - `ai_chat.configs`
4. No prefix, validate uniqueness - `ai_chat_configs`

**Considerations:**
- PostgreSQL identifier length limits (63 chars)
- Query readability
- Schema management complexity

**Recommendation:** Option 1 with underscore-separated ID.

**Decision Required:** Final naming convention?

---

### Q8: Migration Versioning

**Question:** How should applet database migrations be versioned?

**Options:**
1. Sequential numbers (001, 002, 003)
2. Timestamps (20241201120000)
3. Semantic versions (1.0.0, 1.1.0)
4. Manifest version + sequential

**Considerations:**
- Applet version != migration version
- Rollback scenarios
- Team collaboration

**Recommendation:** Timestamps for uniqueness.

**Decision Required:** Migration versioning scheme?

---

### Q9: Data Retention on Uninstall

**Question:** Default behavior when applet is uninstalled?

**Options:**
1. Hard delete immediately
2. Soft delete (rename tables) with 30-day retention
3. Export to file then delete
4. Admin chooses per uninstallation

**Recommendation:** Option 4 (admin choice) with soft delete as default.

**Decision Required:** Default uninstall behavior?

---

## Security & Permissions

### Q10: Permission Granularity

**Question:** How granular should database permissions be?

**Options:**

| Level | Example | Complexity |
|-------|---------|------------|
| **Table** | Read `clients` table | Low |
| **Column** | Read `clients.name`, not `clients.email` | Medium |
| **Row** | Read clients where `type = 'active'` | High |
| **Cell** | Combination of column + row | Very High |

**Recommendation:** Table-level initially, column-level as Phase 2.

**Decision Required:** Initial permission granularity?

---

### Q11: External HTTP Wildcards

**Question:** Should wildcard domains be allowed in external HTTP permissions?

**Example:** `*.openai.com` would allow any subdomain.

**Options:**
1. No wildcards - explicit domains only
2. Single-level wildcards - `*.openai.com` but not `*.*.com`
3. Full wildcards - any pattern

**Security Concern:** Wildcards could be exploited if domain ownership changes.

**Recommendation:** Option 2 - single-level wildcards only.

**Decision Required:** Wildcard policy?

---

### Q12: Secret Storage

**Question:** Where and how should applet secrets be stored?

**Options:**
1. Database (encrypted column)
2. Environment variables
3. External secret manager (Vault, AWS Secrets Manager)
4. SDK settings table with encryption

**Considerations:**
- Multi-tenant isolation
- Rotation support
- Development vs production
- Self-hosted deployments

**Recommendation:** Database with encryption for simplicity, external integration as option.

**Decision Required:** Secret storage approach?

---

## Distribution & Registry

### Q13: Registry Hosting

**Question:** Should IOTA host an official applet registry?

**Options:**
1. Yes - central registry at `registry.iota.uz`
2. No - only private/local installations
3. Optional - SDK works without registry, registry as add-on
4. Federated - multiple registries can be configured

**Considerations:**
- Infrastructure cost
- Moderation responsibility
- Community building
- Enterprise requirements

**Recommendation:** Option 4 - federated, with optional official registry.

**Decision Required:** Registry strategy?

---

### Q14: Package Signing

**Question:** Should applet packages be signed?

**Options:**
1. Required - all packages must be signed
2. Optional - signing available but not required
3. Registry-dependent - official registry requires, private doesn't
4. Verification levels - unsigned, signed, verified

**Recommendation:** Option 4 - multiple verification levels.

**Decision Required:** Signing requirements?

---

### Q15: Update Notifications

**Question:** How should admins be notified of applet updates?

**Options:**
1. In-app notification badge
2. Email digest
3. Dashboard widget
4. No notification (manual check)
5. All of the above (configurable)

**Recommendation:** Option 5 - configurable notifications.

**Decision Required:** Update notification approach?

---

## UI Integration

### Q16: Widget Slot System

**Question:** How should widget injection points be defined?

**Options:**
1. Fixed slots (predefined by SDK)
2. Dynamic slots (components declare injection points)
3. CSS-selector-based (applets target any element)
4. Named regions with priorities

**Example Slots:**
- `dashboard.overview.cards`
- `crm.clients.detail.sidebar`
- `finance.payments.actions`

**Recommendation:** Named regions with priorities (Option 4).

**Decision Required:** Widget slot architecture?

---

### Q17: Navigation Nesting

**Question:** How deep can applet navigation be nested?

**Options:**
1. Top-level only
2. One level under existing items
3. Multiple levels (applet manages sub-navigation)
4. Flat with grouping

**Considerations:**
- UX consistency
- Navigation complexity
- Mobile responsiveness

**Recommendation:** One level under existing items (Option 2).

**Decision Required:** Navigation depth limit?

---

### Q18: Theme Customization

**Question:** Can applets customize beyond design tokens?

**Options:**
1. Tokens only - colors, spacing, fonts
2. Component overrides - custom Button styles
3. Full CSS access - complete styling control
4. Sandboxed CSS - scoped to applet container

**Recommendation:** Option 4 - sandboxed CSS with token inheritance.

**Decision Required:** Customization boundaries?

---

## Developer Experience

### Q19: CLI Tooling

**Question:** What CLI tools should be provided?

**Options:**

| Tool | Purpose | Priority |
|------|---------|----------|
| `create-iota-applet` | Scaffold new applet | High |
| `iota-applet dev` | Development server | High |
| `iota-applet build` | Production build | High |
| `iota-applet test` | Run tests | Medium |
| `iota-applet publish` | Publish to registry | Medium |
| `iota-applet validate` | Validate manifest | Medium |
| `iota-applet doctor` | Debug issues | Low |

**Recommendation:** High priority tools first, others in Phase 2.

**Decision Required:** Initial CLI scope?

---

### Q20: TypeScript SDK Package

**Question:** What should be included in `@iota/applet-sdk`?

**Proposed Contents:**
```
@iota/applet-sdk/
├── server       # Backend utilities (Context, Handler types)
├── react        # React hooks and providers
├── types        # TypeScript definitions
├── testing      # Test utilities and mocks
└── cli          # CLI tooling
```

**Decision Required:** Package structure and contents?

---

### Q21: Documentation Strategy

**Question:** Where should applet documentation live?

**Options:**
1. This spec directory (developer reference)
2. Separate docs site (user-facing)
3. In-code JSDoc (API reference)
4. Storybook (component documentation)
5. All of the above

**Recommendation:** All of the above, with clear purposes.

**Decision Required:** Documentation approach?

---

## Performance & Scaling

### Q22: Cold Start Optimization

**Question:** How to handle applet cold starts?

**Options:**
1. Pre-warm all applets on SDK startup
2. Lazy start on first request
3. Keep-alive with timeout (warm for N minutes after use)
4. Admin configures per-applet

**Considerations:**
- Memory usage vs latency
- Rarely used applets
- Server restart time

**Recommendation:** Lazy start with keep-alive (Option 3).

**Decision Required:** Cold start strategy?

---

### Q23: Request Timeout Handling

**Question:** What happens when an applet request times out?

**Options:**
1. Kill request, return 504
2. Kill request, restart applet process
3. Queue for retry
4. Fail open (return cached/default response)

**Recommendation:** Option 1 with circuit breaker pattern.

**Decision Required:** Timeout handling?

---

### Q24: Resource Limit Enforcement

**Question:** How strictly should resource limits be enforced?

**Options:**

| Level | CPU | Memory | Action |
|-------|-----|--------|--------|
| **Soft** | Warn | Warn | Log only |
| **Medium** | Throttle | Warn | Slow down |
| **Hard** | Kill | Kill | Terminate |

**Recommendation:** Medium limits with escalation to hard after threshold.

**Decision Required:** Enforcement level?

---

## Compatibility & Migration

### Q25: Minimum SDK Version

**Question:** What SDK version should support applets?

**Options:**
1. Current version only (clean start)
2. Backport to last major version
3. Feature flag in multiple versions

**Recommendation:** Current version only (clean start).

**Decision Required:** Minimum SDK version?

---

### Q26: Breaking Change Policy

**Question:** How should applet API breaking changes be handled?

**Options:**
1. Semantic versioning - major version for breaking
2. Deprecation period - warn for N months before breaking
3. API versioning - `/api/v1/`, `/api/v2/`
4. Feature flags - opt-in to new behavior

**Recommendation:** API versioning with deprecation warnings.

**Decision Required:** Breaking change policy?

---

## Implementation Priority

### Q27: Implementation Phases

**Proposed Phases:**

**Phase 1: Foundation (4-6 weeks)**
- Manifest schema validation
- Bun runtime integration
- Basic permission enforcement
- Database access with tenant isolation
- Simple HTTP handlers

**Phase 2: Frontend (3-4 weeks)**
- React component library (minimal scope)
- Page registration
- Navigation integration
- Design tokens export

**Phase 3: Distribution (3-4 weeks)**
- Package format
- Installation flow
- Update mechanism
- Local registry

**Phase 4: Advanced (4-6 weeks)**
- Event system integration
- Widget slots
- Scheduled tasks
- Embeddables

**Phase 5: Ecosystem (ongoing)**
- Official registry
- CLI tooling
- Documentation
- Example applets

**Decision Required:** Phase priorities and timeline?

---

## Summary: Critical Decisions Needed

| # | Question | Recommendation | Urgency |
|---|----------|----------------|---------|
| Q1 | Process isolation | One per applet | High |
| Q2 | Runtime choice | Bun only for applets | High |
| Q4 | Component scope | Standard | High |
| Q10 | Permission granularity | Table-level | High |
| Q13 | Registry strategy | Federated | Medium |
| Q16 | Widget slots | Named regions | Medium |
| Q22 | Cold start | Lazy + keep-alive | Medium |
| Q27 | Implementation phases | As proposed | High |

---

## Notes & Additional Considerations

### Multi-Tenancy Edge Cases
- Applet enabled for some tenants but not others
- Tenant-specific configuration vs global
- Cross-tenant data access (strictly forbidden)
- Tenant migration with applet data

### Error Handling Patterns
- Applet throws unhandled exception
- Applet makes invalid DB query
- Applet exceeds rate limits
- Applet calls blocked external URL

### Testing Strategy
- Unit tests for applet code
- Integration tests with SDK
- E2E tests for installed applets
- Performance/load testing

### Monitoring & Observability
- Applet-level metrics
- Error tracking and alerting
- Performance profiling
- Audit logging

### Compliance Considerations
- GDPR and data residency
- Applet access to PII
- Data processing agreements
- Security certifications

---

## Change Log

| Date | Change | Author |
|------|--------|--------|
| 2024-12-31 | Initial draft | Claude |
