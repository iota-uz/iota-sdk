# Example Applets

**Status:** Draft

## Overview

This document provides reference implementations based on existing IOTA SDK modules that would be good candidates for applets. These examples demonstrate the full applet development workflow.

## Example 1: AI Website Chat (Based on Website Module)

The Website module's AI chat functionality is an ideal applet candidate. It provides:
- Embeddable chat widget for external websites
- AI-powered responses using OpenAI/Dify
- CRM integration for client tracking
- Message routing and thread management

### Manifest

```yaml
# manifest.yaml
manifestVersion: "1.0"
id: "ai-website-chat"
version: "1.0.0"
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
license: "MIT"
icon: "assets/icon.svg"
category: "communication"
minSdkVersion: "2.0.0"

runtime:
  engine: "bun"
  entrypoint: "dist/backend/server.js"
  resources:
    maxMemoryMB: 256
    maxCpuPercent: 50

permissions:
  database:
    read:
      - clients
      - chats
      - chat_messages
      - users
    write:
      - clients
      - chats
      - chat_messages
    createTables: true
  http:
    external:
      - "api.openai.com"
      - "*.dify.ai"
  events:
    subscribe:
      - "chat.message.created"
    publish:
      - "ai.response.generated"
  ui:
    navigation: true
    pages: true
    widgets: true
  secrets:
    - name: "OPENAI_API_KEY"
      description: "OpenAI API key for chat completions"
      required: true
    - name: "DIFY_API_KEY"
      description: "Dify API key for RAG (optional)"
      required: false

tables:
  - name: "configs"
    description: "AI chat configuration per tenant"
    columns:
      - name: id
        type: bigserial
        primary: true
      - name: tenant_id
        type: uuid
        required: true
        index: true
      - name: model_name
        type: varchar(100)
        default: "gpt-4"
      - name: system_prompt
        type: text
        nullable: true
      - name: temperature
        type: decimal(3,2)
        default: 0.7
      - name: max_tokens
        type: integer
        default: 2000
      - name: welcome_message
        type: text
        nullable: true
      - name: widget_color
        type: varchar(20)
        default: "#3b82f6"
    indexes:
      - columns: [tenant_id]
        unique: true

  - name: "threads"
    description: "Chat thread tracking for AI context"
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
      - name: openai_thread_id
        type: varchar(100)
        nullable: true
      - name: context_summary
        type: text
        nullable: true

backend:
  handlers:
    - type: http
      path: "/api/applets/ai-chat/config"
      methods: [GET, POST, PUT]
      handler: "handlers/config.ts"
      auth: required
      permissions:
        - "ai-chat.config.read"
        - "ai-chat.config.write"

    - type: http
      path: "/api/applets/ai-chat/widget"
      methods: [POST]
      handler: "handlers/widget.ts"
      auth: optional
      rateLimit:
        requests: 60
        window: 60

    - type: http
      path: "/api/applets/ai-chat/embed.js"
      methods: [GET]
      handler: "handlers/embed-script.ts"
      auth: none

    - type: event
      events:
        - "chat.message.created"
      handler: "handlers/on-message.ts"
      async: true

frontend:
  framework: react
  navigation:
    - label:
        en: "AI Chat"
        ru: "AI Чат"
      icon: "chat"
      path: "/website/ai-chat"
      permissions:
        - "ai-chat.config.read"
      parent: "website"

  pages:
    - path: "/website/ai-chat"
      title:
        en: "AI Chat Configuration"
        ru: "Настройка AI Чата"
      component: "pages/ConfigPage"
      permissions:
        - "ai-chat.config.read"

    - path: "/website/ai-chat/embed"
      title:
        en: "Embed Widget"
      component: "pages/EmbedPage"
      public: true

  widgets:
    - target: "crm.chats.detail"
      position: "sidebar-right"
      component: "widgets/AiAssistButton"
      permissions:
        - "ai-chat.assist"

  embeddables:
    - name: "chat-widget"
      component: "components/ChatWidget"
      config:
        - name: "theme"
          type: "string"
          options: ["light", "dark", "auto"]
          default: "auto"

appletPermissions:
  - key: "ai-chat.config.read"
    name:
      en: "View AI Chat Configuration"
  - key: "ai-chat.config.write"
    name:
      en: "Edit AI Chat Configuration"
  - key: "ai-chat.assist"
    name:
      en: "Use AI Assistant"

dependencies:
  modules:
    - "crm"
```

### Backend Implementation

```typescript
// src/backend/server.ts
import { serve } from "bun";
import { AppletServer } from "@iota/applet-sdk/server";
import { configHandler } from "./handlers/config";
import { widgetHandler } from "./handlers/widget";
import { onMessageHandler } from "./handlers/on-message";

const server = new AppletServer({
  handlers: {
    "handlers/config.ts": configHandler,
    "handlers/widget.ts": widgetHandler,
    "handlers/on-message.ts": onMessageHandler,
  },
});

serve({
  unix: process.env.SOCKET_PATH,
  fetch: server.handleRequest,
});
```

```typescript
// src/backend/handlers/config.ts
import { Handler, Context } from "@iota/applet-sdk";

interface AIConfig {
  modelName: string;
  systemPrompt: string;
  temperature: number;
  maxTokens: number;
  welcomeMessage: string;
  widgetColor: string;
}

export const configHandler: Handler = async (ctx: Context) => {
  const { method, body } = ctx.request;
  const { db, permissions } = ctx.sdk;

  if (method === "GET") {
    if (!permissions.check("ai-chat.config.read")) {
      return ctx.forbidden();
    }

    const config = await db.table("applet_ai_chat_configs")
      .where("tenant_id", ctx.tenantId)
      .first();

    return ctx.json(config || getDefaultConfig());
  }

  if (method === "POST" || method === "PUT") {
    if (!permissions.check("ai-chat.config.write")) {
      return ctx.forbidden();
    }

    const data = body as AIConfig;

    // Validate
    if (data.temperature < 0 || data.temperature > 2) {
      return ctx.badRequest("Temperature must be between 0 and 2");
    }

    // Upsert config
    const existing = await db.table("applet_ai_chat_configs")
      .where("tenant_id", ctx.tenantId)
      .first();

    if (existing) {
      await db.table("applet_ai_chat_configs")
        .where("id", existing.id)
        .update(data);
    } else {
      await db.table("applet_ai_chat_configs").insert({
        ...data,
        tenant_id: ctx.tenantId,
      });
    }

    return ctx.json({ success: true });
  }
};

function getDefaultConfig(): AIConfig {
  return {
    modelName: "gpt-4",
    systemPrompt: "You are a helpful assistant for our company.",
    temperature: 0.7,
    maxTokens: 2000,
    welcomeMessage: "Hello! How can I help you today?",
    widgetColor: "#3b82f6",
  };
}
```

```typescript
// src/backend/handlers/widget.ts
import { Handler, Context } from "@iota/applet-sdk";
import OpenAI from "openai";

export const widgetHandler: Handler = async (ctx: Context) => {
  const { message, threadId } = ctx.request.body;
  const { db, secrets, http } = ctx.sdk;

  // Get config
  const config = await db.table("applet_ai_chat_configs")
    .where("tenant_id", ctx.tenantId)
    .first();

  if (!config) {
    return ctx.badRequest("AI Chat not configured");
  }

  // Get or create thread
  let thread = threadId
    ? await db.table("applet_ai_chat_threads")
        .where("id", threadId)
        .first()
    : null;

  if (!thread) {
    // Create new chat in CRM
    const chat = await db.table("chats").insert({
      source: "website_widget",
      status: "open",
    });

    thread = await db.table("applet_ai_chat_threads").insert({
      chat_id: chat.id,
    });
  }

  // Store user message
  await db.table("chat_messages").insert({
    chat_id: thread.chat_id,
    content: message,
    sender_type: "client",
  });

  // Get conversation history
  const history = await db.table("chat_messages")
    .where("chat_id", thread.chat_id)
    .orderBy("created_at", "asc")
    .limit(20)
    .get();

  // Call OpenAI
  const openai = new OpenAI({
    apiKey: secrets.get("OPENAI_API_KEY"),
  });

  const completion = await openai.chat.completions.create({
    model: config.model_name,
    temperature: config.temperature,
    max_tokens: config.max_tokens,
    messages: [
      { role: "system", content: config.system_prompt },
      ...history.map((msg) => ({
        role: msg.sender_type === "client" ? "user" : "assistant",
        content: msg.content,
      })),
    ],
  });

  const aiResponse = completion.choices[0].message.content;

  // Store AI response
  await db.table("chat_messages").insert({
    chat_id: thread.chat_id,
    content: aiResponse,
    sender_type: "ai",
  });

  // Publish event
  await ctx.sdk.events.publish("ai.response.generated", {
    threadId: thread.id,
    chatId: thread.chat_id,
    response: aiResponse,
  });

  return ctx.json({
    threadId: thread.id,
    response: aiResponse,
  });
};
```

### Frontend Implementation

```tsx
// src/frontend/pages/ConfigPage.tsx
import { useState, useEffect } from "react";
import {
  Card,
  Input,
  Select,
  Textarea,
  Slider,
  Button,
  ColorPicker,
} from "@iota/components";
import { useAppletContext } from "@iota/applet-sdk/react";

interface AIConfig {
  modelName: string;
  systemPrompt: string;
  temperature: number;
  maxTokens: number;
  welcomeMessage: string;
  widgetColor: string;
}

const MODELS = [
  { value: "gpt-4", label: "GPT-4" },
  { value: "gpt-4-turbo", label: "GPT-4 Turbo" },
  { value: "gpt-3.5-turbo", label: "GPT-3.5 Turbo" },
  { value: "claude-3-opus", label: "Claude 3 Opus" },
  { value: "claude-3-sonnet", label: "Claude 3 Sonnet" },
];

export function ConfigPage() {
  const { t, api, toast, permissions } = useAppletContext();
  const [config, setConfig] = useState<AIConfig | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const canEdit = permissions.check("ai-chat.config.write");

  useEffect(() => {
    loadConfig();
  }, []);

  async function loadConfig() {
    try {
      const data = await api.get<AIConfig>("/config");
      setConfig(data);
    } catch (error) {
      toast.error(t("Config.LoadError"));
    } finally {
      setLoading(false);
    }
  }

  async function handleSave() {
    if (!config) return;

    setSaving(true);
    try {
      await api.put("/config", config);
      toast.success(t("Config.Saved"));
    } catch (error) {
      toast.error(t("Config.SaveError"));
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return <Card><Skeleton lines={6} /></Card>;
  }

  return (
    <div className="space-y-6">
      <Card title={t("Config.AISettings")}>
        <div className="space-y-4">
          <Select
            label={t("Config.Model")}
            value={config?.modelName}
            onChange={(v) => setConfig({ ...config!, modelName: v })}
            options={MODELS}
            disabled={!canEdit}
          />

          <Textarea
            label={t("Config.SystemPrompt")}
            value={config?.systemPrompt}
            onChange={(v) => setConfig({ ...config!, systemPrompt: v })}
            rows={6}
            disabled={!canEdit}
            help={t("Config.SystemPromptHelp")}
          />

          <Slider
            label={t("Config.Temperature")}
            value={config?.temperature}
            onChange={(v) => setConfig({ ...config!, temperature: v })}
            min={0}
            max={2}
            step={0.1}
            disabled={!canEdit}
          />

          <Input
            label={t("Config.MaxTokens")}
            type="number"
            value={config?.maxTokens}
            onChange={(v) => setConfig({ ...config!, maxTokens: parseInt(v) })}
            disabled={!canEdit}
          />
        </div>
      </Card>

      <Card title={t("Config.WidgetSettings")}>
        <div className="space-y-4">
          <Textarea
            label={t("Config.WelcomeMessage")}
            value={config?.welcomeMessage}
            onChange={(v) => setConfig({ ...config!, welcomeMessage: v })}
            rows={3}
            disabled={!canEdit}
          />

          <ColorPicker
            label={t("Config.WidgetColor")}
            value={config?.widgetColor}
            onChange={(v) => setConfig({ ...config!, widgetColor: v })}
            disabled={!canEdit}
          />
        </div>
      </Card>

      {canEdit && (
        <div className="flex justify-end">
          <Button
            variant="primary"
            onClick={handleSave}
            loading={saving}
          >
            {t("Common.Save")}
          </Button>
        </div>
      )}
    </div>
  );
}
```

```tsx
// src/frontend/components/ChatWidget.tsx
import { useState, useRef, useEffect } from "react";

interface ChatWidgetProps {
  tenantId: string;
  theme?: "light" | "dark" | "auto";
  position?: "bottom-right" | "bottom-left";
}

interface Message {
  id: string;
  content: string;
  sender: "user" | "ai";
  timestamp: Date;
}

export function ChatWidget({ tenantId, theme = "auto", position = "bottom-right" }: ChatWidgetProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [threadId, setThreadId] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  async function sendMessage() {
    if (!input.trim() || loading) return;

    const userMessage: Message = {
      id: crypto.randomUUID(),
      content: input,
      sender: "user",
      timestamp: new Date(),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput("");
    setLoading(true);

    try {
      const response = await fetch(`/api/applets/ai-chat/widget`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Tenant-ID": tenantId,
        },
        body: JSON.stringify({
          message: input,
          threadId,
        }),
      });

      const data = await response.json();

      setThreadId(data.threadId);

      const aiMessage: Message = {
        id: crypto.randomUUID(),
        content: data.response,
        sender: "ai",
        timestamp: new Date(),
      };

      setMessages((prev) => [...prev, aiMessage]);
    } catch (error) {
      console.error("Failed to send message:", error);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className={`chat-widget chat-widget--${position} chat-widget--${theme}`}>
      {isOpen ? (
        <div className="chat-widget__container">
          <div className="chat-widget__header">
            <span>Chat with us</span>
            <button onClick={() => setIsOpen(false)}>×</button>
          </div>

          <div className="chat-widget__messages">
            {messages.map((msg) => (
              <div key={msg.id} className={`chat-message chat-message--${msg.sender}`}>
                {msg.content}
              </div>
            ))}
            {loading && (
              <div className="chat-message chat-message--ai">
                <span className="typing-indicator">...</span>
              </div>
            )}
            <div ref={messagesEndRef} />
          </div>

          <div className="chat-widget__input">
            <input
              type="text"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyPress={(e) => e.key === "Enter" && sendMessage()}
              placeholder="Type a message..."
              disabled={loading}
            />
            <button onClick={sendMessage} disabled={loading}>
              Send
            </button>
          </div>
        </div>
      ) : (
        <button className="chat-widget__toggle" onClick={() => setIsOpen(true)}>
          💬
        </button>
      )}
    </div>
  );
}
```

### Embeddable Script

```typescript
// src/backend/handlers/embed-script.ts
import { Handler, Context } from "@iota/applet-sdk";

export const embedScriptHandler: Handler = async (ctx: Context) => {
  const tenantId = ctx.request.query.get("tenant");

  if (!tenantId) {
    return ctx.badRequest("Missing tenant parameter");
  }

  // Get widget config
  const config = await ctx.sdk.db.table("applet_ai_chat_configs")
    .where("tenant_id", tenantId)
    .first();

  const script = `
(function() {
  const TENANT_ID = "${tenantId}";
  const WIDGET_COLOR = "${config?.widget_color || "#3b82f6"}";
  const API_BASE = "${ctx.request.origin}";

  // Create widget container
  const container = document.createElement("div");
  container.id = "iota-chat-widget";
  document.body.appendChild(container);

  // Load widget styles
  const styles = document.createElement("link");
  styles.rel = "stylesheet";
  styles.href = API_BASE + "/api/applets/ai-chat/widget.css";
  document.head.appendChild(styles);

  // Load React and widget bundle
  const script = document.createElement("script");
  script.src = API_BASE + "/api/applets/ai-chat/widget-bundle.js";
  script.onload = function() {
    window.IOTAChatWidget.init({
      container: "#iota-chat-widget",
      tenantId: TENANT_ID,
      color: WIDGET_COLOR,
      apiBase: API_BASE,
    });
  };
  document.body.appendChild(script);
})();
`;

  return new Response(script, {
    headers: {
      "Content-Type": "application/javascript",
      "Cache-Control": "public, max-age=3600",
    },
  });
};
```

---

## Example 2: Shyona-Style Business Analytics (Conceptual)

Based on the Shyona module in shy-trucks, this represents a more complex applet with:
- AI-powered business analytics
- Agent framework for autonomous tasks
- Knowledge base management
- GraphQL API extensions

### Manifest (Condensed)

```yaml
manifestVersion: "1.0"
id: "business-analytics-ai"
version: "1.0.0"
name:
  en: "AI Business Analytics"
  ru: "AI Бизнес-Аналитика"

runtime:
  engine: "bun"
  entrypoint: "dist/backend/server.js"
  resources:
    maxMemoryMB: 512
    maxCpuPercent: 70

permissions:
  database:
    read:
      - "*"  # Read access to all tables for analytics
    write:
      - analytics_reports
      - analytics_cache
    createTables: true
  http:
    external:
      - "api.openai.com"
      - "api.anthropic.com"
  events:
    subscribe:
      - "*"  # Subscribe to all events for analytics
    publish:
      - "analytics.report.generated"
      - "analytics.insight.discovered"
  ui:
    navigation: true
    pages: true
    widgets: true
  secrets:
    - name: "OPENAI_API_KEY"
      required: true

tables:
  - name: "reports"
    columns:
      - name: id
        type: bigserial
        primary: true
      - name: tenant_id
        type: uuid
        required: true
      - name: report_type
        type: varchar(50)
      - name: parameters
        type: jsonb
      - name: result
        type: jsonb
      - name: generated_at
        type: timestamptz

  - name: "agents"
    columns:
      - name: id
        type: uuid
        primary: true
      - name: tenant_id
        type: uuid
        required: true
      - name: name
        type: varchar(100)
      - name: type
        type: varchar(50)
      - name: config
        type: jsonb
      - name: status
        type: varchar(20)

  - name: "knowledge_base"
    columns:
      - name: id
        type: uuid
        primary: true
      - name: tenant_id
        type: uuid
        required: true
      - name: title
        type: varchar(255)
      - name: content
        type: text
      - name: embedding
        type: vector(1536)  # pgvector
      - name: metadata
        type: jsonb

backend:
  handlers:
    - type: http
      path: "/api/applets/analytics/dashboard"
      methods: [GET]
      handler: "handlers/dashboard.ts"

    - type: http
      path: "/api/applets/analytics/reports"
      methods: [GET, POST]
      handler: "handlers/reports.ts"

    - type: http
      path: "/api/applets/analytics/ask"
      methods: [POST]
      handler: "handlers/ask.ts"

    - type: http
      path: "/api/applets/analytics/agents"
      methods: [GET, POST, PUT, DELETE]
      handler: "handlers/agents.ts"

    - type: scheduled
      cron: "0 6 * * *"  # Daily at 6 AM
      handler: "handlers/daily-report.ts"

    - type: event
      events:
        - "payment.created"
        - "expense.created"
        - "order.completed"
      handler: "handlers/event-tracker.ts"
      async: true

  services:
    - name: "analyticsEngine"
      handler: "services/analytics-engine.ts"

    - name: "agentFramework"
      handler: "services/agent-framework.ts"

    - name: "knowledgeBase"
      handler: "services/knowledge-base.ts"

frontend:
  navigation:
    - label:
        en: "Analytics AI"
      icon: "chart-bar"
      path: "/analytics"
      parent: "dashboard"

  pages:
    - path: "/analytics"
      component: "pages/Dashboard"

    - path: "/analytics/reports"
      component: "pages/Reports"

    - path: "/analytics/ask"
      component: "pages/AskAI"

    - path: "/analytics/agents"
      component: "pages/Agents"

  widgets:
    - target: "dashboard.overview"
      position: "card"
      component: "widgets/InsightsCard"

appletPermissions:
  - key: "analytics.view"
    name: { en: "View Analytics" }
  - key: "analytics.reports"
    name: { en: "Generate Reports" }
  - key: "analytics.ask"
    name: { en: "Ask AI Questions" }
  - key: "analytics.agents"
    name: { en: "Manage AI Agents" }
```

### Key Backend Services

```typescript
// src/backend/services/analytics-engine.ts
import OpenAI from "openai";
import { Context } from "@iota/applet-sdk";

export class AnalyticsEngine {
  private openai: OpenAI;
  private ctx: Context;

  constructor(ctx: Context) {
    this.ctx = ctx;
    this.openai = new OpenAI({
      apiKey: ctx.sdk.secrets.get("OPENAI_API_KEY"),
    });
  }

  async generateReport(type: string, params: Record<string, any>) {
    // Gather data based on report type
    const data = await this.gatherData(type, params);

    // Use AI to analyze and generate insights
    const analysis = await this.analyzeWithAI(type, data);

    // Store report
    const report = await this.ctx.sdk.db.table("applet_analytics_reports").insert({
      report_type: type,
      parameters: params,
      result: analysis,
      generated_at: new Date(),
    });

    return report;
  }

  private async gatherData(type: string, params: Record<string, any>) {
    const { db } = this.ctx.sdk;
    const { startDate, endDate } = params;

    switch (type) {
      case "financial_summary":
        return {
          payments: await db.table("payments")
            .whereBetween("created_at", [startDate, endDate])
            .get(),
          expenses: await db.table("expenses")
            .whereBetween("created_at", [startDate, endDate])
            .get(),
          revenue: await db.raw(`
            SELECT SUM(amount) as total
            FROM payments
            WHERE created_at BETWEEN $1 AND $2
          `, [startDate, endDate]),
        };

      case "sales_performance":
        return {
          orders: await db.table("orders")
            .whereBetween("created_at", [startDate, endDate])
            .get(),
          topProducts: await db.raw(`
            SELECT p.name, SUM(oi.quantity) as total_sold
            FROM order_items oi
            JOIN products p ON oi.product_id = p.id
            WHERE oi.created_at BETWEEN $1 AND $2
            GROUP BY p.id
            ORDER BY total_sold DESC
            LIMIT 10
          `, [startDate, endDate]),
        };

      default:
        throw new Error(`Unknown report type: ${type}`);
    }
  }

  private async analyzeWithAI(type: string, data: any) {
    const prompt = this.buildAnalysisPrompt(type, data);

    const response = await this.openai.chat.completions.create({
      model: "gpt-4",
      messages: [
        {
          role: "system",
          content: "You are a business analyst. Analyze the provided data and generate actionable insights.",
        },
        {
          role: "user",
          content: prompt,
        },
      ],
      response_format: { type: "json_object" },
    });

    return JSON.parse(response.choices[0].message.content!);
  }

  private buildAnalysisPrompt(type: string, data: any): string {
    return `
Analyze the following ${type} data and provide:
1. Key metrics summary
2. Trends and patterns
3. Anomalies or concerns
4. Actionable recommendations

Data:
${JSON.stringify(data, null, 2)}

Respond in JSON format with sections: summary, metrics, trends, anomalies, recommendations.
    `;
  }
}
```

```typescript
// src/backend/services/agent-framework.ts
import { Context } from "@iota/applet-sdk";

interface Agent {
  id: string;
  name: string;
  type: "monitor" | "reporter" | "optimizer";
  config: AgentConfig;
  status: "active" | "paused" | "error";
}

interface AgentConfig {
  trigger: "schedule" | "event" | "threshold";
  triggerConfig: any;
  actions: AgentAction[];
}

interface AgentAction {
  type: "notify" | "report" | "execute";
  config: any;
}

export class AgentFramework {
  private ctx: Context;
  private runningAgents: Map<string, NodeJS.Timeout> = new Map();

  constructor(ctx: Context) {
    this.ctx = ctx;
  }

  async startAgent(agentId: string) {
    const agent = await this.loadAgent(agentId);

    if (agent.config.trigger === "schedule") {
      // Set up scheduled execution
      const interval = this.parseScheduleInterval(agent.config.triggerConfig.cron);
      const timer = setInterval(() => this.executeAgent(agent), interval);
      this.runningAgents.set(agentId, timer);
    }

    await this.updateAgentStatus(agentId, "active");
  }

  async stopAgent(agentId: string) {
    const timer = this.runningAgents.get(agentId);
    if (timer) {
      clearInterval(timer);
      this.runningAgents.delete(agentId);
    }
    await this.updateAgentStatus(agentId, "paused");
  }

  private async executeAgent(agent: Agent) {
    try {
      for (const action of agent.config.actions) {
        await this.executeAction(action);
      }
    } catch (error) {
      await this.updateAgentStatus(agent.id, "error");
      console.error(`Agent ${agent.id} failed:`, error);
    }
  }

  private async executeAction(action: AgentAction) {
    switch (action.type) {
      case "notify":
        // Send notification via SDK
        break;
      case "report":
        // Generate and send report
        break;
      case "execute":
        // Execute custom logic
        break;
    }
  }

  private async loadAgent(agentId: string): Promise<Agent> {
    return this.ctx.sdk.db.table("applet_analytics_agents")
      .where("id", agentId)
      .first();
  }

  private async updateAgentStatus(agentId: string, status: string) {
    await this.ctx.sdk.db.table("applet_analytics_agents")
      .where("id", agentId)
      .update({ status });
  }

  private parseScheduleInterval(cron: string): number {
    // Simplified - in production use a proper cron parser
    return 60 * 60 * 1000; // 1 hour default
  }
}
```

---

## Example 3: Simple Webhook Handler (Minimal Applet)

A minimal applet demonstrating the simplest possible implementation:

```yaml
# manifest.yaml
manifestVersion: "1.0"
id: "webhook-forwarder"
version: "1.0.0"
name:
  en: "Webhook Forwarder"

runtime:
  engine: "bun"
  entrypoint: "dist/server.js"

permissions:
  http:
    external:
      - "*"  # Forward to any URL
  secrets:
    - name: "FORWARD_URL"
      required: true

backend:
  handlers:
    - type: http
      path: "/api/applets/webhook/receive"
      methods: [POST]
      handler: "handlers/receive.ts"
      auth: none
```

```typescript
// src/handlers/receive.ts
import { Handler, Context } from "@iota/applet-sdk";

export const receiveHandler: Handler = async (ctx: Context) => {
  const forwardUrl = ctx.sdk.secrets.get("FORWARD_URL");
  const payload = ctx.request.body;

  // Forward the webhook
  const response = await fetch(forwardUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "X-Forwarded-From": "iota-sdk",
      "X-Tenant-ID": ctx.tenantId,
    },
    body: JSON.stringify(payload),
  });

  if (!response.ok) {
    return ctx.json({
      success: false,
      error: `Forward failed: ${response.status}`,
    }, 502);
  }

  return ctx.json({ success: true, forwarded: true });
};
```

---

## Development Workflow Summary

### 1. Create New Applet

```bash
npx create-iota-applet my-applet
cd my-applet
```

### 2. Development

```bash
# Start dev server with hot reload
bun dev

# Run tests
bun test

# Type check
bun tsc --noEmit
```

### 3. Build & Package

```bash
# Production build
bun run build

# Package for distribution
bun run package
# Creates: my-applet-1.0.0.zip
```

### 4. Publish

```bash
# Login to registry
iota-applet login

# Publish
iota-applet publish
```

### 5. Install in SDK

Via Admin UI:
1. Go to Settings > Applets > Browse
2. Search for applet
3. Click Install
4. Review permissions
5. Configure secrets
6. Enable for tenants
