---
name: Railway CLI
description: Railway CLI operations for authentication, context management, deployment, monitoring, and troubleshooting. Use when working with Railway infrastructure, deploying services, or managing environments.
---

# Railway CLI for IOTA SDK

## Check Authentication

```bash
railway whoami
```

## Context Management

Check current context:

```bash
railway status
```

List all projects with IDs:

```bash
railway list --json | jq '.'
```

Link project manually (create `.railway/config.json`):

```bash
mkdir -p .railway
cat > .railway/config.json << 'EOF'
{
  "projectId": "<your-project-id>"
}
EOF
```

Switch environment:

```bash
railway environment <environment>
```

## Deployment

Deploy to specific environment and service:

```bash
railway up -s <service> -e <environment> --detach
```

Redeploy latest deployment:

```bash
railway redeploy -s <service> -e <environment> -y
```

Check deployment status:

```bash
railway status -s <service> -e <environment>
```

## Monitoring

View deployment logs (real-time stream):

```bash
railway logs -s <service> -e <environment> --deployment
```

View build logs:

```bash
railway logs -s <service> -e <environment> --build
```

Get logs in JSON format (for parsing):

```bash
railway logs -s <service> -e <environment> --deployment --json
```

Filter logs for errors/panics:

```bash
# Text output with grep
railway logs -s iota-sdk -e staging --deployment 2>&1 | grep -i -E "(panic|error|fatal)"

# JSON output with jq (more reliable)
railway logs -s iota-sdk -e staging --deployment --json 2>&1 | \
  jq -r 'select(.message | test("panic|error|fatal"; "i")) | "\(.timestamp) \(.message)"'
```

Get context around errors:

```bash
# Get 30 lines after each match
railway logs -s iota-sdk -e staging --deployment 2>&1 | grep -A 30 "panic"

# Limit total output
railway logs -s iota-sdk -e staging --deployment 2>&1 | tail -100
```

**Note**: Railway enforces a rate limit of 500 logs/sec per replica. High-volume logging may result in dropped messages.

## Variables

Get all variables for a service:

```bash
railway variables -s <service> -e <environment> --kv
```

Set a variable:

```bash
railway variables -s <service> -e <environment> --set "KEY=value"
```

## Running Commands

Execute local command with remote environment variables:

```bash
railway run -s <service> -e <environment> -- <command>
```

**Note**: `railway run` executes locally, not on the remote service. Use for running local commands with staging/production environment variables.

## Database Access

Connect to database shell:

```bash
railway connect db -e <environment>
```

Get database connection URL:

```bash
railway variables -s db -e <environment> --kv | grep DATABASE_PUBLIC_URL
```

SSH into service (requires active deployment):

```bash
railway ssh -s <service> -e <environment>
```

## Common Operations for IOTA SDK

### Analyzing Panics and Errors

Find recent panics with stack traces:

```bash
# Get panic with 30 lines of context (includes stack trace)
railway logs -s iota-sdk -e staging --deployment 2>&1 | grep -A 30 "panic"

# Find specific error patterns
railway logs -s iota-sdk -e staging --deployment --json 2>&1 | \
  jq -r 'select(.message | test("your-pattern"; "i")) | .message' | tail -50
```

Common error patterns for IOTA SDK:
- Translation errors: `"message .* not found in language"`
- Database errors: `"pq:|postgres:|connection|timeout"`
- HTTP panics: `"http: panic serving"`
- Multi-tenant errors: `"tenant_id|organization_id|forbidden"`
- Permission errors: `"permission denied|insufficient permissions"`

### Running Migrations on Staging

Get database credentials and run migrations locally:

```bash
# Get the public database URL
railway variables -s db -e staging --kv | grep DATABASE_PUBLIC_URL
# Example output: DATABASE_PUBLIC_URL=postgresql://postgres:PASSWORD@HOST:PORT/railway

# Extract connection details from the URL and export
export DB_HOST=<host-from-url>          # Example: shuttle.proxy.rlwy.net
export DB_PORT=<port-from-url>          # Example: 31150
export DB_NAME=<database-from-url>      # Example: railway
export DB_USER=<user-from-url>          # Example: postgres
export DB_PASSWORD=<password-from-url>

# Check migration status
make db migrate status

# Run pending migrations
make db migrate up

# Rollback if needed
make db migrate down
```

### Deploying New Version

```bash
# 1. Verify current status
railway status -s iota-sdk -e staging

# 2. Deploy new version
railway up -s iota-sdk -e staging --detach

# 3. Monitor deployment logs
railway logs -s iota-sdk -e staging --deployment

# 4. Verify deployment success
railway status -s iota-sdk -e staging

# 5. Check for errors
railway logs -s iota-sdk -e staging --deployment 2>&1 | grep -i error | tail -20
```

### Environment Variable Management

```bash
# List all environment variables
railway variables -s iota-sdk -e staging --kv

# Update application config
railway variables -s iota-sdk -e staging --set "LOG_LEVEL=debug"

# Update database connection (example)
railway variables -s iota-sdk -e staging --set "DB_HOST=new-host"

# Trigger restart after variable changes
railway redeploy -s iota-sdk -e staging -y
```

### Database Backup

```bash
# Get database URL
DB_URL=$(railway variables -s db -e staging --kv | grep DATABASE_PUBLIC_URL | cut -d'=' -f2)

# Create backup
pg_dump "$DB_URL" > backup_$(date +%Y%m%d_%H%M%S).sql

# Or with compression
pg_dump "$DB_URL" | gzip > backup_$(date +%Y%m%d_%H%M%S).sql.gz
```

## IOTA SDK Project Structure

**Environment Names**:
- `staging` - Staging environment
- `production` - Production environment

**Service Names** (adjust based on actual setup):
- `iota-sdk` - Main Go application
- `db` - PostgreSQL database

**Example with actual service names**:

```bash
# Deploy to staging
railway up -s iota-sdk -e staging --detach

# View logs
railway logs -s iota-sdk -e staging --deployment

# Check database
railway connect db -e staging
```

## Troubleshooting

**"Project not found"**: Create `.railway/config.json` with `projectId`

**"No deployment found"**: Service has no active deployment in that environment

**SSH connection fails**: Ensure deployment is running, use `railway logs` to check status

**Logs not showing**: Check service name and environment are correct

**Variable not updating**: Redeploy service after setting variables

## Production Safety

**Always specify `-e staging` explicitly** when working with staging to avoid accidentally affecting production.

### Before Running Production Operations

1. **Double-check the environment flag** (`-e production`)
2. **Verify current context** with `railway status`
3. **Test on staging first**
4. **Have a rollback plan ready**
5. **Announce deployment** to team
6. **Monitor logs during deployment**

### Production Deployment Checklist

- [ ] Code changes reviewed and approved
- [ ] Tests passing in CI/CD
- [ ] Staging deployment successful
- [ ] Database migrations tested on staging
- [ ] Rollback plan prepared
- [ ] Team notified of deployment
- [ ] Monitoring dashboard ready
- [ ] Environment flag verified (`-e production`)

### Emergency Rollback

```bash
# 1. Check recent deployments
railway status -s iota-sdk -e production

# 2. Redeploy previous version (if using git tags/branches)
git checkout <previous-version>
railway up -s iota-sdk -e production --detach

# 3. Or use Railway dashboard to rollback
# Visit Railway dashboard → Service → Deployments → Rollback
```

## Quick Reference

```bash
# Authentication
railway whoami
railway status

# Deployment
railway up -s iota-sdk -e staging --detach
railway redeploy -s iota-sdk -e staging -y

# Logs
railway logs -s iota-sdk -e staging --deployment
railway logs -s iota-sdk -e staging --deployment | grep -i error

# Variables
railway variables -s iota-sdk -e staging --kv
railway variables -s iota-sdk -e staging --set "KEY=value"

# Database
railway connect db -e staging
railway variables -s db -e staging --kv | grep DATABASE_PUBLIC_URL

# SSH
railway ssh -s iota-sdk -e staging
```

## Common Workflows

### Debug Production Issue

```bash
# 1. View recent logs
railway logs -s iota-sdk -e production --deployment | tail -100

# 2. Search for errors
railway logs -s iota-sdk -e production --deployment | grep -i error -A 10

# 3. Check database connection
railway variables -s iota-sdk -e production --kv | grep DB_

# 4. SSH into service if needed
railway ssh -s iota-sdk -e production
```

### Update Environment Configuration

```bash
# 1. Get current variables
railway variables -s iota-sdk -e staging --kv > current_vars.txt

# 2. Update variables
railway variables -s iota-sdk -e staging --set "NEW_FEATURE_FLAG=true"
railway variables -s iota-sdk -e staging --set "LOG_LEVEL=info"

# 3. Redeploy to apply changes
railway redeploy -s iota-sdk -e staging -y

# 4. Verify changes
railway logs -s iota-sdk -e staging --deployment | head -50
```

### Monitor Deployment

```bash
# Terminal 1: Watch deployment status
watch -n 5 'railway status -s iota-sdk -e staging'

# Terminal 2: Stream logs
railway logs -s iota-sdk -e staging --deployment

# Terminal 3: Check for errors
watch -n 10 'railway logs -s iota-sdk -e staging --deployment | grep -i error | tail -20'
```
