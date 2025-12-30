# Manifest Specification: Applet Package Definition

**Status:** Draft

## Overview

The manifest file (`manifest.yaml` or `manifest.json`) is the central configuration file for an applet. It declares metadata, permissions, entry points, and integration requirements.

## Schema Version

```yaml
manifestVersion: "1.0"
```

## Full Schema

```yaml
# =============================================================================
# APPLET MANIFEST SCHEMA v1.0
# =============================================================================

# -----------------------------------------------------------------------------
# METADATA
# -----------------------------------------------------------------------------
manifestVersion: "1.0"                    # Schema version
id: "ai-website-chat"                     # Unique identifier (kebab-case)
version: "1.0.0"                          # Semantic version
name:
  en: "AI Website Chat"
  ru: "AI Чат для сайта"
  uz: "Veb-sayt uchun AI Chat"
description:
  en: "Embeddable AI chatbot for your website with CRM integration"
  ru: "Встраиваемый AI чат-бот для вашего сайта с интеграцией CRM"
author:
  name: "IOTA Team"
  email: "team@iota.uz"
  url: "https://iota.uz"
license: "MIT"
repository: "https://github.com/iota-uz/applet-ai-chat"
homepage: "https://iota.uz/applets/ai-chat"
icon: "assets/icon.svg"                   # Applet icon
screenshots:                              # Gallery images
  - "assets/screenshot-1.png"
  - "assets/screenshot-2.png"
keywords:
  - ai
  - chatbot
  - website
  - crm
category: "communication"                 # Primary category
minSdkVersion: "2.0.0"                   # Minimum IOTA SDK version

# -----------------------------------------------------------------------------
# RUNTIME CONFIGURATION
# -----------------------------------------------------------------------------
runtime:
  engine: "bun"                           # Options: bun, goja, deno, node
  version: ">=1.0.0"                      # Engine version requirement
  entrypoint: "dist/server.js"            # Main entry file
  healthCheck:
    path: "/__health__"
    interval: 30                          # seconds
    timeout: 5                            # seconds
  resources:
    maxMemoryMB: 256                       # Memory limit
    maxCpuPercent: 50                      # CPU limit
    maxExecutionMs: 30000                  # Single request timeout
    maxConcurrentRequests: 100             # Concurrent request limit

# -----------------------------------------------------------------------------
# PERMISSIONS
# -----------------------------------------------------------------------------
permissions:
  # Database access
  database:
    # Read access to existing SDK tables
    read:
      - clients                           # CRM clients
      - chats                             # CRM chats
      - chat_messages                     # CRM messages
      - users                             # Core users
    # Write access to existing SDK tables
    write:
      - clients
      - chats
      - chat_messages
    # Permission to create custom tables
    createTables: true                    # Requires admin approval

  # External HTTP access
  http:
    external:
      - "api.openai.com"
      - "*.dify.ai"
      - "api.anthropic.com"
    # Blocked by default: private IPs, localhost, cloud metadata

  # Event bus access
  events:
    subscribe:
      - "chat.message.created"
      - "client.created"
    publish:
      - "ai.response.generated"
      - "ai.chat.started"

  # UI integration
  ui:
    navigation: true                      # Can add nav items
    pages: true                           # Can register pages
    widgets: true                         # Can inject widgets

  # Secret access
  secrets:
    - name: "OPENAI_API_KEY"
      description: "OpenAI API key for chat completions"
      required: true
    - name: "DIFY_API_KEY"
      description: "Dify API key for RAG"
      required: false

# -----------------------------------------------------------------------------
# DATABASE TABLES (if createTables: true)
# -----------------------------------------------------------------------------
tables:
  - name: "applet_ai_chat_configs"
    description: "AI chat configuration per tenant"
    columns:
      - name: id
        type: bigserial
        primary: true
      - name: tenant_id
        type: uuid
        required: true
        index: true
        foreignKey:
          table: tenants
          column: id
          onDelete: CASCADE
      - name: model_name
        type: varchar(100)
        default: "gpt-4"
      - name: base_url
        type: varchar(500)
        nullable: true
      - name: system_prompt
        type: text
        nullable: true
      - name: temperature
        type: decimal(3,2)
        default: 0.7
      - name: max_tokens
        type: integer
        default: 2000
      - name: created_at
        type: timestamptz
        default: now()
      - name: updated_at
        type: timestamptz
        default: now()
    indexes:
      - columns: [tenant_id]
        unique: true                      # One config per tenant

  - name: "applet_ai_chat_threads"
    description: "Chat thread tracking"
    columns:
      - name: id
        type: uuid
        primary: true
        default: gen_random_uuid()
      - name: tenant_id
        type: uuid
        required: true
      - name: chat_id
        type: bigint
        required: true
        foreignKey:
          table: chats
          column: id
          onDelete: CASCADE
      - name: created_at
        type: timestamptz
        default: now()
    indexes:
      - columns: [tenant_id, chat_id]

# -----------------------------------------------------------------------------
# BACKEND HANDLERS
# -----------------------------------------------------------------------------
backend:
  handlers:
    # HTTP handlers
    - type: http
      path: "/api/applets/ai-chat/config"
      methods: [GET, POST, PUT]
      handler: "handlers/config.ts"
      auth: required
      permissions:
        - "ai-chat.config.read"
        - "ai-chat.config.write"

    - type: http
      path: "/api/applets/ai-chat/models"
      methods: [GET]
      handler: "handlers/models.ts"
      auth: required
      permissions:
        - "ai-chat.config.read"

    - type: http
      path: "/api/applets/ai-chat/threads"
      methods: [POST]
      handler: "handlers/threads.ts"
      auth: optional                       # Public API for widget
      rateLimit:
        requests: 60
        window: 60                         # seconds

    - type: http
      path: "/api/applets/ai-chat/threads/:threadId/messages"
      methods: [GET, POST]
      handler: "handlers/messages.ts"
      auth: optional

    # Event handlers
    - type: event
      events:
        - "chat.message.created"
      handler: "handlers/on-message.ts"
      async: true                          # Don't block event bus
      retry:
        maxAttempts: 3
        backoffMs: 1000

    # Scheduled tasks
    - type: scheduled
      cron: "0 * * * *"                    # Every hour
      handler: "handlers/cleanup.ts"
      timezone: "UTC"

  # Service definitions (for dependency injection)
  services:
    - name: "aiConfigService"
      handler: "services/ai-config-service.ts"

    - name: "chatService"
      handler: "services/chat-service.ts"
      dependencies:
        - "aiConfigService"

# -----------------------------------------------------------------------------
# FRONTEND CONFIGURATION
# -----------------------------------------------------------------------------
frontend:
  framework: react                         # Options: react, vue, alpine
  build:
    bundler: bun                          # Options: bun, vite, esbuild
    entrypoint: "src/frontend/index.tsx"
    outdir: "dist/frontend"

  # Navigation items
  navigation:
    - label:
        en: "AI Chat"
        ru: "AI Чат"
      icon: "chat"                        # Icon name from SDK icon set
      path: "/website/ai-chat"
      permissions:
        - "ai-chat.config.read"
      parent: "website"                   # Nest under existing nav item
      order: 10

  # Page definitions
  pages:
    - path: "/website/ai-chat"
      title:
        en: "AI Chat Configuration"
        ru: "Настройка AI Чата"
      component: "pages/ConfigPage"
      layout: "standard"                  # SDK layout
      permissions:
        - "ai-chat.config.read"

    - path: "/website/ai-chat/embed"
      title:
        en: "Chat Widget"
        ru: "Виджет чата"
      component: "pages/EmbedPage"
      layout: "minimal"                   # No sidebar
      public: true                        # No auth required

  # Widget injections
  widgets:
    - target: "crm.chats.detail"          # Inject into CRM chat detail
      position: "sidebar-right"
      component: "widgets/AiAssistButton"
      permissions:
        - "ai-chat.assist"

    - target: "dashboard.overview"
      position: "card"
      component: "widgets/ChatStatsCard"
      permissions:
        - "ai-chat.stats.view"

  # Embeddable components (for external websites)
  embeddables:
    - name: "chat-widget"
      component: "components/ChatWidget"
      description: "Embeddable chat widget for external websites"
      config:
        - name: "theme"
          type: "string"
          options: ["light", "dark", "auto"]
          default: "auto"
        - name: "position"
          type: "string"
          options: ["bottom-right", "bottom-left"]
          default: "bottom-right"

# -----------------------------------------------------------------------------
# LOCALIZATION
# -----------------------------------------------------------------------------
locales:
  supported:
    - en
    - ru
    - uz
  default: en
  files:
    en: "locales/en.json"
    ru: "locales/ru.json"
    uz: "locales/uz.json"

# -----------------------------------------------------------------------------
# PERMISSIONS DEFINITIONS (RBAC)
# -----------------------------------------------------------------------------
appletPermissions:
  - key: "ai-chat.config.read"
    name:
      en: "View AI Chat Configuration"
      ru: "Просмотр настроек AI Чата"
    description:
      en: "Can view AI chat settings and models"
      ru: "Может просматривать настройки AI чата"

  - key: "ai-chat.config.write"
    name:
      en: "Edit AI Chat Configuration"
      ru: "Редактирование настроек AI Чата"
    description:
      en: "Can modify AI chat settings"
      ru: "Может изменять настройки AI чата"

  - key: "ai-chat.assist"
    name:
      en: "Use AI Assistant"
      ru: "Использование AI Ассистента"
    description:
      en: "Can use AI to generate responses in chats"
      ru: "Может использовать AI для генерации ответов"

  - key: "ai-chat.stats.view"
    name:
      en: "View AI Chat Statistics"
      ru: "Просмотр статистики AI Чата"

# -----------------------------------------------------------------------------
# DEPENDENCIES
# -----------------------------------------------------------------------------
dependencies:
  modules:
    - "crm"                               # Requires CRM module
  applets: []                             # Other applet dependencies

# -----------------------------------------------------------------------------
# LIFECYCLE HOOKS
# -----------------------------------------------------------------------------
lifecycle:
  onInstall: "hooks/on-install.ts"        # Run after installation
  onUninstall: "hooks/on-uninstall.ts"    # Run before uninstallation
  onUpdate: "hooks/on-update.ts"          # Run on version update
  onEnable: "hooks/on-enable.ts"          # Run when enabled for tenant
  onDisable: "hooks/on-disable.ts"        # Run when disabled for tenant
```

## Validation Rules

### ID Format
- Lowercase letters, numbers, hyphens
- Start with letter
- 3-50 characters
- Must be unique in registry

### Version Format
- Semantic versioning (MAJOR.MINOR.PATCH)
- Optional prerelease (-alpha.1, -beta.2)

### Permission Validation
- Cannot request more than declared capabilities
- External HTTP hosts validated against allowlist
- Database tables must exist or be declared

### File References
- All paths relative to manifest location
- Must exist at package time
- Max file size limits apply

## Example: Minimal Manifest

```yaml
manifestVersion: "1.0"
id: "hello-world"
version: "1.0.0"
name:
  en: "Hello World"

runtime:
  engine: "bun"
  entrypoint: "dist/server.js"

permissions:
  ui:
    pages: true

frontend:
  pages:
    - path: "/hello"
      title: { en: "Hello World" }
      component: "pages/HelloPage"
```
