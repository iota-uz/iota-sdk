# Distribution Specification: Packaging, Registry, and Installation

**Status:** Draft

## Overview

This document covers how applets are:
1. **Packaged** - Built and bundled for distribution
2. **Published** - Uploaded to a registry
3. **Discovered** - Found by administrators
4. **Installed** - Deployed to SDK instances
5. **Updated** - Upgraded to new versions
6. **Uninstalled** - Removed cleanly

## Package Format

### Applet Package Structure

```
my-applet-1.0.0.zip
├── manifest.yaml           # Package manifest (required)
├── dist/
│   ├── backend/
│   │   └── server.js       # Bundled backend code
│   └── frontend/
│       ├── pages/
│       │   └── config.js   # Page bundles
│       └── widgets/
│           └── chat.js     # Widget bundles
├── assets/
│   ├── icon.svg            # Applet icon
│   └── screenshots/        # Gallery images
├── locales/
│   ├── en.json
│   ├── ru.json
│   └── uz.json
├── migrations/             # Database migrations
│   ├── 001_initial.sql
│   └── 002_add_column.sql
├── checksums.json          # File integrity hashes
└── signature.sig           # Package signature (optional)
```

### Manifest Requirements

```yaml
# manifest.yaml - Required fields
manifestVersion: "1.0"
id: "ai-website-chat"           # Unique identifier
version: "1.0.0"                # Semantic version
name:
  en: "AI Website Chat"
runtime:
  engine: "bun"
  entrypoint: "dist/backend/server.js"
```

### Build Process

```bash
# Development build
iota-applet build --dev

# Production build
iota-applet build --prod

# Build output
dist/
├── backend/server.js     # Minified, tree-shaken
├── frontend/             # Code-split bundles
└── package.zip           # Ready for upload
```

**Build Pipeline:**

```typescript
// build.config.ts
import { defineConfig } from '@iota/applet-cli';

export default defineConfig({
  backend: {
    entrypoint: 'src/backend/server.ts',
    target: 'bun',
    minify: true,
  },
  frontend: {
    framework: 'react',
    entrypoints: {
      pages: 'src/frontend/pages/**/*.tsx',
      widgets: 'src/frontend/widgets/**/*.tsx',
    },
    splitting: true,
    minify: true,
  },
  locales: {
    source: 'src/locales',
    languages: ['en', 'ru', 'uz'],
  },
});
```

### Checksums & Integrity

```json
// checksums.json
{
  "algorithm": "sha256",
  "files": {
    "manifest.yaml": "a1b2c3d4...",
    "dist/backend/server.js": "e5f6g7h8...",
    "dist/frontend/pages/config.js": "i9j0k1l2...",
    "locales/en.json": "m3n4o5p6..."
  }
}
```

**Verification:**

```go
func verifyPackageIntegrity(pkg *Package) error {
    checksums, err := parseChecksums(pkg.GetFile("checksums.json"))
    if err != nil {
        return err
    }

    for file, expectedHash := range checksums.Files {
        content := pkg.GetFile(file)
        actualHash := sha256.Sum256(content)

        if hex.EncodeToString(actualHash[:]) != expectedHash {
            return ErrChecksumMismatch{File: file}
        }
    }

    return nil
}
```

## Registry Architecture

### Registry Types

```
┌─────────────────────────────────────────────────────────────────┐
│                     Registry Architecture                        │
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ Official Registry (registry.iota.uz)                      │  │
│  │ - Curated, verified applets                               │  │
│  │ - Security reviewed                                        │  │
│  │ - Signed packages                                          │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                              ▼                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ Private Registry (your-company.registry.io)               │  │
│  │ - Organization-specific applets                           │  │
│  │ - Internal tools                                          │  │
│  │ - Custom integrations                                     │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│                              ▼                                   │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │ Local Installation                                        │  │
│  │ - Direct .zip upload                                       │  │
│  │ - Development/testing                                     │  │
│  │ - Air-gapped environments                                 │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Registry API

```yaml
# OpenAPI specification for registry
openapi: 3.0.0
info:
  title: IOTA Applet Registry
  version: 1.0.0

paths:
  /api/v1/applets:
    get:
      summary: List applets
      parameters:
        - name: q
          in: query
          description: Search query
        - name: category
          in: query
        - name: page
          in: query
        - name: limit
          in: query
      responses:
        200:
          content:
            application/json:
              schema:
                type: object
                properties:
                  items:
                    type: array
                    items:
                      $ref: '#/components/schemas/AppletSummary'
                  total:
                    type: integer

  /api/v1/applets/{id}:
    get:
      summary: Get applet details
      responses:
        200:
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/AppletDetail'

  /api/v1/applets/{id}/versions:
    get:
      summary: List versions
    post:
      summary: Publish new version
      requestBody:
        content:
          multipart/form-data:
            schema:
              type: object
              properties:
                package:
                  type: string
                  format: binary

  /api/v1/applets/{id}/versions/{version}/download:
    get:
      summary: Download package

components:
  schemas:
    AppletSummary:
      type: object
      properties:
        id:
          type: string
        name:
          type: object
        description:
          type: object
        version:
          type: string
        author:
          $ref: '#/components/schemas/Author'
        downloads:
          type: integer
        rating:
          type: number
        icon:
          type: string

    AppletDetail:
      allOf:
        - $ref: '#/components/schemas/AppletSummary'
        - type: object
          properties:
            permissions:
              type: object
            screenshots:
              type: array
            changelog:
              type: string
            documentation:
              type: string
```

### Publishing Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                     Publishing Flow                              │
│                                                                  │
│  Developer                                                       │
│      │                                                           │
│      ▼                                                           │
│  1. iota-applet build --prod                                    │
│      │                                                           │
│      ▼                                                           │
│  2. iota-applet publish                                         │
│      │                                                           │
│      ├── Authenticate with registry                             │
│      ├── Upload package.zip                                     │
│      └── Wait for processing                                    │
│                                                                  │
│  Registry                                                        │
│      │                                                           │
│      ▼                                                           │
│  3. Package Validation                                          │
│      │                                                           │
│      ├── Verify checksums                                       │
│      ├── Validate manifest schema                               │
│      ├── Check version conflicts                                │
│      ├── Scan for vulnerabilities                               │
│      └── Verify signature (if signed)                           │
│                                                                  │
│      ▼                                                           │
│  4. Automated Review                                            │
│      │                                                           │
│      ├── Static analysis                                        │
│      ├── Permission audit                                       │
│      └── License check                                          │
│                                                                  │
│      ▼                                                           │
│  5. Manual Review (for official registry)                       │
│      │                                                           │
│      ├── Security review                                        │
│      ├── Code quality check                                     │
│      └── Functionality test                                     │
│                                                                  │
│      ▼                                                           │
│  6. Published                                                    │
│      │                                                           │
│      └── Available for installation                             │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**CLI Publishing:**

```bash
# Login to registry
iota-applet login

# Publish to official registry
iota-applet publish

# Publish to private registry
iota-applet publish --registry https://private.registry.io

# Publish with signing
iota-applet publish --sign --key ~/.iota/signing-key.pem
```

## Installation Flow

### Discovery UI

```
┌─────────────────────────────────────────────────────────────────┐
│  SDK Admin Panel > Applets > Browse                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  [Search applets...]                    [Category ▼] [Sort ▼]   │
│                                                                  │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐  │
│  │ ┌───┐           │  │ ┌───┐           │  │ ┌───┐           │  │
│  │ │ 🤖│ AI Chat   │  │ │ 📊│ Analytics │  │ │ 📦│ Inventory │  │
│  │ └───┘           │  │ └───┘           │  │ └───┘           │  │
│  │                 │  │                 │  │                 │  │
│  │ Website chatbot │  │ Business        │  │ Extended        │  │
│  │ with AI         │  │ intelligence    │  │ warehouse       │  │
│  │                 │  │                 │  │                 │  │
│  │ ★★★★☆ (4.5)    │  │ ★★★★★ (5.0)    │  │ ★★★☆☆ (3.2)    │  │
│  │ 1.2K installs   │  │ 5.6K installs   │  │ 890 installs    │  │
│  │                 │  │                 │  │                 │  │
│  │ [Install]       │  │ [Install]       │  │ [Install]       │  │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘  │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Installation Steps

```
┌─────────────────────────────────────────────────────────────────┐
│                     Installation Flow                            │
│                                                                  │
│  Admin clicks [Install]                                          │
│      │                                                           │
│      ▼                                                           │
│  1. Download Package                                            │
│      │                                                           │
│      ├── Fetch from registry                                    │
│      ├── Verify checksums                                       │
│      └── Extract to temp directory                              │
│                                                                  │
│      ▼                                                           │
│  2. Permission Review                                           │
│      │                                                           │
│      ┌─────────────────────────────────────────────────────┐    │
│      │ AI Website Chat requests:                            │    │
│      │                                                      │    │
│      │ ⚠️ DATABASE                                          │    │
│      │   Read: clients, chats, chat_messages               │    │
│      │   Write: clients, chats                             │    │
│      │   Create Tables: YES                                │    │
│      │                                                      │    │
│      │ 🌐 EXTERNAL HTTP                                     │    │
│      │   api.openai.com, *.dify.ai                         │    │
│      │                                                      │    │
│      │ 🔐 SECRETS REQUIRED                                  │    │
│      │   OPENAI_API_KEY                                    │    │
│      │                                                      │    │
│      │ [Review Tables] [Approve] [Cancel]                  │    │
│      └─────────────────────────────────────────────────────┘    │
│                                                                  │
│      ▼                                                           │
│  3. Configuration                                               │
│      │                                                           │
│      ├── Enter required secrets                                 │
│      ├── Configure tenant settings                              │
│      └── Set initial permissions                                │
│                                                                  │
│      ▼                                                           │
│  4. Database Migration                                          │
│      │                                                           │
│      ├── Create applet tables                                   │
│      ├── Run initial migrations                                 │
│      └── Seed default data (if any)                             │
│                                                                  │
│      ▼                                                           │
│  5. Runtime Initialization                                      │
│      │                                                           │
│      ├── Start Bun process (if needed)                          │
│      ├── Register HTTP handlers                                 │
│      ├── Subscribe to events                                    │
│      └── Register scheduled tasks                               │
│                                                                  │
│      ▼                                                           │
│  6. UI Registration                                             │
│      │                                                           │
│      ├── Add navigation items                                   │
│      ├── Register page routes                                   │
│      └── Initialize widgets                                     │
│                                                                  │
│      ▼                                                           │
│  7. Lifecycle Hook: onInstall                                   │
│      │                                                           │
│      └── Run applet's installation hook                         │
│                                                                  │
│      ▼                                                           │
│  8. Complete                                                    │
│      │                                                           │
│      └── Applet is now active                                   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

### Installation API

```go
type InstallationManager struct {
    registry      RegistryClient
    storage       PackageStorage
    migrator      MigrationRunner
    runtimeMgr    RuntimeManager
    permissionMgr PermissionManager
}

func (m *InstallationManager) Install(ctx context.Context, req InstallRequest) error {
    // 1. Download package
    pkg, err := m.registry.Download(req.AppletID, req.Version)
    if err != nil {
        return fmt.Errorf("download failed: %w", err)
    }

    // 2. Verify integrity
    if err := verifyPackageIntegrity(pkg); err != nil {
        return fmt.Errorf("integrity check failed: %w", err)
    }

    // 3. Parse manifest
    manifest, err := parseManifest(pkg.GetFile("manifest.yaml"))
    if err != nil {
        return fmt.Errorf("invalid manifest: %w", err)
    }

    // 4. Check permissions are approved
    if !req.PermissionsApproved {
        return ErrPermissionsNotApproved
    }

    // 5. Store package
    if err := m.storage.Store(manifest.ID, manifest.Version, pkg); err != nil {
        return fmt.Errorf("storage failed: %w", err)
    }

    // 6. Run migrations
    if err := m.migrator.InstallApplet(manifest); err != nil {
        m.storage.Remove(manifest.ID, manifest.Version)
        return fmt.Errorf("migration failed: %w", err)
    }

    // 7. Initialize runtime
    if err := m.runtimeMgr.InitializeApplet(manifest); err != nil {
        m.migrator.RollbackApplet(manifest)
        m.storage.Remove(manifest.ID, manifest.Version)
        return fmt.Errorf("runtime init failed: %w", err)
    }

    // 8. Register permissions
    if err := m.permissionMgr.RegisterAppletPermissions(manifest); err != nil {
        return fmt.Errorf("permission registration failed: %w", err)
    }

    // 9. Run onInstall hook
    if manifest.Lifecycle.OnInstall != "" {
        if err := m.runtimeMgr.Execute(ctx, manifest.ID, manifest.Lifecycle.OnInstall); err != nil {
            // Log warning but don't fail installation
            log.Warn("onInstall hook failed", "error", err)
        }
    }

    return nil
}
```

## Update Flow

### Update Detection

```go
type UpdateChecker struct {
    registry RegistryClient
    storage  PackageStorage
}

func (c *UpdateChecker) CheckUpdates(ctx context.Context) ([]UpdateAvailable, error) {
    installed := c.storage.ListInstalled()
    var updates []UpdateAvailable

    for _, applet := range installed {
        latest, err := c.registry.GetLatestVersion(applet.ID)
        if err != nil {
            continue
        }

        if semver.Compare(latest.Version, applet.Version) > 0 {
            updates = append(updates, UpdateAvailable{
                AppletID:       applet.ID,
                CurrentVersion: applet.Version,
                LatestVersion:  latest.Version,
                Changelog:      latest.Changelog,
                Breaking:       latest.Breaking,
            })
        }
    }

    return updates, nil
}
```

### Update Process

```
┌─────────────────────────────────────────────────────────────────┐
│                       Update Flow                                │
│                                                                  │
│  1. Download new version                                        │
│      │                                                           │
│      ▼                                                           │
│  2. Compare permissions                                         │
│      │                                                           │
│      ├── New permissions? → Require approval                    │
│      └── Removed permissions? → Automatic                       │
│                                                                  │
│      ▼                                                           │
│  3. Run onUpdate hook (from OLD version)                        │
│      │                                                           │
│      └── Prepare for update                                     │
│                                                                  │
│      ▼                                                           │
│  4. Stop running instance                                       │
│      │                                                           │
│      └── Graceful shutdown                                      │
│                                                                  │
│      ▼                                                           │
│  5. Run migrations                                              │
│      │                                                           │
│      └── Apply schema changes                                   │
│                                                                  │
│      ▼                                                           │
│  6. Replace package files                                       │
│      │                                                           │
│      └── Atomic swap                                            │
│                                                                  │
│      ▼                                                           │
│  7. Start new version                                           │
│      │                                                           │
│      └── Initialize runtime                                     │
│                                                                  │
│      ▼                                                           │
│  8. Run onUpdate hook (from NEW version)                        │
│      │                                                           │
│      └── Post-update setup                                      │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

**Rollback Support:**

```go
func (m *InstallationManager) Update(ctx context.Context, appletID string, newVersion string) error {
    // Get current version for rollback
    current, err := m.storage.GetInstalled(appletID)
    if err != nil {
        return err
    }

    // Create rollback point
    rollback := m.createRollbackPoint(current)

    // Attempt update
    err = m.performUpdate(ctx, appletID, newVersion)
    if err != nil {
        // Rollback on failure
        if rollbackErr := m.rollback(rollback); rollbackErr != nil {
            return fmt.Errorf("update failed: %w, rollback failed: %v", err, rollbackErr)
        }
        return fmt.Errorf("update failed, rolled back: %w", err)
    }

    // Clean up rollback point
    m.cleanRollbackPoint(rollback)

    return nil
}
```

## Uninstallation

### Uninstall Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                    Uninstallation Flow                           │
│                                                                  │
│  Admin clicks [Uninstall]                                        │
│      │                                                           │
│      ▼                                                           │
│  1. Confirmation                                                │
│      │                                                           │
│      ┌─────────────────────────────────────────────────────┐    │
│      │ Uninstall AI Website Chat?                          │    │
│      │                                                      │    │
│      │ ⚠️ This will:                                        │    │
│      │   • Remove all applet data                          │    │
│      │   • Disable chat widget on your website             │    │
│      │   • Remove navigation items                         │    │
│      │                                                      │    │
│      │ Data handling:                                       │    │
│      │ ○ Keep data for 30 days (can reinstall)             │    │
│      │ ○ Export data and delete                            │    │
│      │ ○ Delete immediately                                │    │
│      │                                                      │    │
│      │ [Cancel] [Uninstall]                                │    │
│      └─────────────────────────────────────────────────────┘    │
│                                                                  │
│      ▼                                                           │
│  2. Run onDisable hook                                          │
│      │                                                           │
│      └── Prepare for disable                                    │
│                                                                  │
│      ▼                                                           │
│  3. Stop runtime                                                │
│      │                                                           │
│      ├── Cancel scheduled tasks                                 │
│      ├── Unsubscribe from events                                │
│      └── Stop Bun process                                       │
│                                                                  │
│      ▼                                                           │
│  4. Unregister UI                                               │
│      │                                                           │
│      ├── Remove navigation items                                │
│      ├── Unregister routes                                      │
│      └── Remove widgets                                         │
│                                                                  │
│      ▼                                                           │
│  5. Run onUninstall hook                                        │
│      │                                                           │
│      └── Final cleanup                                          │
│                                                                  │
│      ▼                                                           │
│  6. Handle data                                                 │
│      │                                                           │
│      ├── soft_delete: Rename tables, keep for 30 days          │
│      ├── export: Export to JSON, then drop                     │
│      └── hard_delete: DROP TABLE immediately                   │
│                                                                  │
│      ▼                                                           │
│  7. Remove package files                                        │
│      │                                                           │
│      └── Clean storage                                          │
│                                                                  │
│      ▼                                                           │
│  8. Complete                                                    │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Multi-Tenant Considerations

### Tenant-Specific Installation

```go
type TenantAppletInstallation struct {
    AppletID   string
    TenantID   uuid.UUID
    Version    string
    Enabled    bool
    Config     JSONB
    Secrets    map[string]string  // Encrypted
    InstalledAt time.Time
    InstalledBy uint
}
```

### Per-Tenant Configuration

```yaml
# Admin can configure per tenant
applet_config:
  ai-chat:
    tenant_a:
      model: "gpt-4"
      temperature: 0.7
    tenant_b:
      model: "claude-3"
      temperature: 0.5
```

### Enable/Disable Per Tenant

```go
func (m *InstallationManager) EnableForTenant(appletID string, tenantID uuid.UUID) error {
    // 1. Verify applet is installed globally
    // 2. Run onEnable hook with tenant context
    // 3. Apply tenant-specific migrations (if any)
    // 4. Mark as enabled for tenant
}

func (m *InstallationManager) DisableForTenant(appletID string, tenantID uuid.UUID) error {
    // 1. Run onDisable hook with tenant context
    // 2. Mark as disabled (keep data)
}
```

## Security Considerations

### Package Signing

```bash
# Sign package with developer key
iota-applet sign --key ~/.iota/developer-key.pem

# Verify signature
iota-applet verify my-applet-1.0.0.zip
```

**Signature Verification:**

```go
func verifySignature(pkg *Package, trustedKeys []PublicKey) error {
    sig := pkg.GetFile("signature.sig")
    if sig == nil {
        return ErrNotSigned
    }

    content := pkg.GetContentForSigning()
    hash := sha256.Sum256(content)

    for _, key := range trustedKeys {
        if verifyWithKey(hash[:], sig, key) {
            return nil
        }
    }

    return ErrInvalidSignature
}
```

### Vulnerability Scanning

```go
type SecurityScanner struct {
    vulnerabilityDB VulnerabilityDatabase
    staticAnalyzer  StaticAnalyzer
}

func (s *SecurityScanner) Scan(pkg *Package) (*ScanReport, error) {
    report := &ScanReport{}

    // Check for known vulnerabilities in dependencies
    deps := extractDependencies(pkg)
    for _, dep := range deps {
        vulns := s.vulnerabilityDB.Check(dep.Name, dep.Version)
        report.Vulnerabilities = append(report.Vulnerabilities, vulns...)
    }

    // Static analysis of code
    findings := s.staticAnalyzer.Analyze(pkg.GetFile("dist/backend/server.js"))
    report.StaticAnalysis = findings

    // Check permissions for suspicious patterns
    manifest := parseManifest(pkg.GetFile("manifest.yaml"))
    if hasSuspiciousPermissions(manifest) {
        report.Warnings = append(report.Warnings, "Suspicious permission combination")
    }

    return report, nil
}
```

### Installation Restrictions

```go
type InstallationPolicy struct {
    AllowUnsigned      bool
    RequiredSigners    []string  // Required signer IDs
    BlockedApplets     []string  // Blocked applet IDs
    AllowedRegistries  []string  // Allowed registry URLs
    RequireReview      bool      // Manual review required
}
```
