# Claude Code Settings Reference

Complete reference for configuring Claude Code settings, permissions, hooks, and MCP servers.

**Official Documentation:** https://docs.claude.com/en/docs/claude-code/settings

## Settings File Locations & Precedence

Settings are loaded in priority order (highest to lowest):

1. **Enterprise Managed** - System-wide policies (highest priority)
2. **CLI Arguments** - Runtime overrides
3. **`.claude/settings.local.json`** - Personal project settings (not committed)
4. **`.claude/settings.json`** - Shared project settings (committed)
5. **`~/.claude/settings.json`** - User global settings (lowest priority)

**Best Practice:**

- Use `.local.json` for personal settings (gitignored)
- Use `.json` for team settings (committed)
- Use `~/.claude/` for global user preferences

## Core Configuration Structure

```json
{
  "permissions": {
    "allow": [
      "Bash(make test:*)",
      "Bash(go vet:*)"
    ],
    "deny": [
      "Read(.env)",
      "Read(**/*.key)"
    ],
    "ask": [
      "Bash(git push:*)"
    ]
  },
  "env": {
    "CUSTOM_VAR": "value",
    "GO_ENV": "development"
  },
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": {},
        "hooks": [
          {
            "type": "command",
            "command": "make check lint"
          }
        ]
      }
    ]
  },
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-github"
      ],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  },
  "enabledPlugins": [
    "plugin-name"
  ],
  "extraKnownMarketplaces": [
    "https://custom-marketplace.com"
  ]
}
```

## Configuration Elements

### Permissions

Control tool access with three levels:

- **`allow`**: Pre-approve safe operations (no confirmation needed)
- **`deny`**: Block dangerous operations (hard deny)
- **`ask`**: Confirm before execution (user approval required)

**Permission Patterns:**

```json
{
  "permissions": {
    "allow": [
      "Bash(go test:*)",
      "Bash(go vet:*)",
      "Bash(make:*)",
      "Bash(git status:*)",
      "Bash(git diff:*)",
      "Read(/path/to/safe/dir/**)"
    ],
    "deny": [
      "Read(.env)",
      "Read(**/*.key)",
      "Read(**/*.pem)",
      "Read(**/credentials.json)",
      "Bash(rm:*)",
      "Bash(sudo:*)"
    ],
    "ask": [
      "Bash(git push:*)",
      "Bash(git commit:*)",
      "Bash(npm publish:*)"
    ]
  }
}
```

### Environment Variables

Persistent variables for all Claude Code sessions:

```json
{
  "env": {
    "GO_ENV": "development",
    "API_BASE_URL": "https://api.example.com",
    "CUSTOM_PATH": "/usr/local/bin"
  }
}
```

**Use Cases:**

- API keys and tokens (with env var expansion for secrets)
- Development vs production settings
- Custom tool paths
- Feature flags

### Hooks

Auto-run commands triggered by events:

**Valid Events:**

- `PreToolUse` - Before tool execution
- `PostToolUse` - After tool execution
- `Notification` - On system notifications
- `UserPromptSubmit` - After user submits prompt
- `SessionStart` - When session begins
- `SessionEnd` - When session ends
- `Stop` - When user stops generation
- `SubagentStop` - When subagent stops
- `PreCompact` - Before conversation compaction

**Hook Format:**

```json
{
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": {},
        "hooks": [
          {
            "type": "command",
            "command": "make fix fmt && make check lint"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": {
          "tools": [
            "Bash",
            "Edit"
          ]
        },
        "hooks": [
          {
            "type": "command",
            "command": "echo 'Completed at $(date)'"
          }
        ]
      }
    ]
  }
}
```

**Matcher Patterns:**

- `{}` - Match all events
- `{"tools": ["Bash"]}` - Match specific tools
- `{"tools": ["Bash", "Edit"]}` - Match multiple tools

### Hook Environment Variables

Hooks execute with special environment variables for portable, context-aware commands:

**`$CLAUDE_PROJECT_DIR`** - Absolute path to project root where Claude Code started

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "cd \"$CLAUDE_PROJECT_DIR\"/back && templ generate"
          }
        ]
      }
    ]
  }
}
```

**Why needed:** Hooks may execute when Claude's working directory differs from project root. Using
`"$CLAUDE_PROJECT_DIR"` ensures scripts run correctly regardless of current directory.

**`$CLAUDE_CODE_REMOTE`** - Indicates execution environment

- `"true"` - Running in web/remote environment
- Not set or empty - Running in local CLI

```bash
if [ "$CLAUDE_CODE_REMOTE" = "true" ]; then
  echo "Web environment - skip local-only tasks"
else
  make format lint
fi
```

**`$CLAUDE_ENV_FILE`** - SessionStart hook only, persist environment variables

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "echo 'export NODE_ENV=development' >> \"$CLAUDE_ENV_FILE\""
          }
        ]
      }
    ]
  }
}
```

Variables written to `$CLAUDE_ENV_FILE` are available in all subsequent bash commands during the session.

**Best Practices:**

- Always quote paths: `"$CLAUDE_PROJECT_DIR"` (handles spaces)
- Use `$CLAUDE_PROJECT_DIR` for all project-relative scripts
- Test hooks with `bash -c "command"` before enabling
- Keep hooks fast (< 1 second) to avoid blocking workflow

**Standard variables also available:** `$PWD`, `$HOME`, `$USER`, `$PATH`

### MCP Servers

External integrations via Model Context Protocol:

```json
{
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-github"
      ],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    },
    "database": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-postgres"
      ],
      "env": {
        "DATABASE_URL": "${DATABASE_URL}"
      }
    }
  }
}
```

**MCP Features:**

- **Tool Naming:** Tools prefixed with `mcp__<server>__<tool>` (e.g., `mcp__github__create_issue`)
- **Scopes:** Local (`.local.json`), Project (`.json`), User (`~/.claude/`)
- **Security:** Requires explicit approval for project-scoped servers
- **Auth:** Use env var expansion: `${ENV_VAR}` for credentials
- **Server Types:** HTTP servers recommended (stdio deprecated)

**Security Notes:**

- Never hardcode credentials in settings files
- Always use `${ENV_VAR}` expansion for secrets
- Vet MCP servers carefully (prompt injection risks)
- Test in local scope before sharing in project scope

## Configuration Environment Variables

Configure Claude Code behavior via environment variables in `settings.json` → `env` section or system environment:

### Performance & Limits

- `CLAUDE_CODE_MAX_OUTPUT_TOKENS` - Max output tokens per request (default: `32000`)
- `BASH_DEFAULT_TIMEOUT_MS` - Default bash command timeout (default: `60000` - 60 sec)
- `BASH_MAX_TIMEOUT_MS` - Maximum bash timeout (default: `600000` - 10 min)
- `MAX_MCP_OUTPUT_TOKENS` - Max tokens in MCP tool responses (default: `25000`)
- `MCP_TIMEOUT` - MCP server startup timeout in ms (default: `30000`)
- `MCP_TOOL_TIMEOUT` - MCP tool execution timeout in ms (default: `60000`)

### Feature Toggles

Set to `"1"` or `"true"` to enable:

- `DISABLE_TELEMETRY` - Opt out of Statsig telemetry
- `DISABLE_ERROR_REPORTING` - Opt out of Sentry error reporting
- `DISABLE_PROMPT_CACHING` - Disable prompt caching globally
- `DISABLE_COST_WARNINGS` - Disable cost warning messages
- `DISABLE_AUTOUPDATER` - Disable automatic updates
- `CLAUDE_CODE_DISABLE_TERMINAL_TITLE` - Disable terminal title updates

### Network Configuration

- `HTTP_PROXY` - HTTP proxy server (example: `http://proxy.example.com:8080`)
- `HTTPS_PROXY` - HTTPS proxy server (example: `https://proxy.example.com:8443`)
- `NO_PROXY` - Bypass proxy for domains/IPs (example: `localhost,127.0.0.1`)

### Example Configuration

```json
{
  "env": {
    "CLAUDE_CODE_MAX_OUTPUT_TOKENS": "32000",
    "BASH_DEFAULT_TIMEOUT_MS": "60000",
    "DISABLE_TELEMETRY": "1",
    "HTTP_PROXY": "http://proxy.corp.com:8080"
  }
}
```

**Note:** Complete list at https://docs.claude.com/en/docs/claude-code/settings (includes cloud provider configs,
authentication, mTLS, model selection)

## Best Practices

### DO

- Use `.local.json` for personal settings (gitignored)
- Use `.json` for team settings (committed to version control)
- Protect secrets with `deny` permissions
- Use `ask` for destructive operations (push, publish, delete)
- Configure hooks for automated quality checks
- Use env var expansion for MCP credentials: `${GITHUB_TOKEN}`
- Vet MCP servers carefully before enabling
- Test hooks to ensure they're fast and reliable

### DON'T

- Don't commit secrets to shared settings
- Don't grant blanket `Bash(*)` permission
- Don't use blanket `allow` without restrictions
- Don't hardcode MCP credentials in settings files
- Don't use untrusted MCP servers (prompt injection risk)
- Don't create slow/unreliable hooks (blocks workflow)

## Example: EAI Monorepo Settings

```json
{
  "permissions": {
    "allow": [
      "Bash(go test:*)",
      "Bash(go vet:*)",
      "Bash(go build:*)",
      "Bash(make:*)",
      "Bash(git status:*)",
      "Bash(git diff:*)",
      "Bash(git log:*)",
      "Bash(templ generate:*)",
      "Read(/Users/user/Projects/sdk/eai/**)",
      "Read(/Users/user/Projects/sdk/iota-sdk/**)"
    ],
    "deny": [
      "Read(.env)",
      "Read(.env.*)",
      "Read(**/*.key)",
      "Read(**/*.pem)",
      "Read(**/credentials*)",
      "Bash(rm:*)",
      "Bash(sudo:*)",
      "Bash(git push --force:*)"
    ],
    "ask": [
      "Bash(git push:*)",
      "Bash(git commit:*)",
      "Bash(railway deploy:*)"
    ]
  },
  "env": {
    "GO_ENV": "development"
  },
  "hooks": {
    "UserPromptSubmit": [
      {
        "matcher": {},
        "hooks": [
          {
            "type": "command",
            "command": "cd back && make check lint"
          }
        ]
      }
    ]
  },
  "mcpServers": {
    "github": {
      "command": "npx",
      "args": [
        "-y",
        "@modelcontextprotocol/server-github"
      ],
      "env": {
        "GITHUB_TOKEN": "${GITHUB_PERSONAL_ACCESS_TOKEN}"
      }
    }
  }
}
```

## Troubleshooting

### Hooks Blocking Workflow

If hooks are slowing down your workflow:

1. Check hook execution time: add timing to commands
2. Remove slow hooks or optimize commands
3. Use `PostToolUse` instead of `UserPromptSubmit` for less frequent execution
4. Test hooks in isolation before enabling

### MCP Authentication Failures

If MCP servers fail to authenticate:

1. Verify environment variables are set: `echo $GITHUB_TOKEN`
2. Check env var expansion syntax: `${ENV_VAR}` (not `$ENV_VAR`)
3. Confirm credentials have required permissions
4. Check MCP server logs for detailed errors

### Permission Denied Errors

If Claude Code can't access files/commands:

1. Check `deny` list doesn't block needed operations
2. Add patterns to `allow` list for safe operations
3. Use `ask` for operations needing confirmation
4. Verify file paths in permissions match actual locations

## See Also

- `.claude/guides/claude-code/architecture.md` - Core principles and patterns
- `.claude/guides/claude-code/commands.md` - Creating slash commands
- `.claude/guides/claude-code/agents.md` - Creating specialized agents
- https://docs.claude.com/en/docs/claude-code/settings - Official settings documentation
- https://docs.claude.com/en/docs/claude-code/mcp - Official MCP documentation
