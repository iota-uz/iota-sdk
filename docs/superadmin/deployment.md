---
layout: default
title: Deployment
parent: SuperAdmin
nav_order: 4
description: "SuperAdmin Module Deployment Guide"
---

# Deployment

## Overview

The SuperAdmin server is deployed as a separate service from the main application, sharing the same database and environment configuration but running with restricted access and minimal modules.

## Environment Variables

The SuperAdmin server uses the **same environment variables** as the main application.

### Required Variables

```bash
# Core Application
LOG_LEVEL=debug
SESSION_DURATION=720h
DOMAIN=superadmin.yourdomain.com
GO_APP_ENV=production

# Database (shared with main app)
DB_HOST=localhost
DB_PORT=5432
DB_NAME=iota_erp
DB_USER=postgres
DB_PASSWORD=postgres

# Server
PORT=3000

# Authentication
GOOGLE_CLIENT_ID=your-client-id
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URL=https://superadmin.yourdomain.com/auth/google/callback
```

### Important Notes

- Update `DOMAIN` to match your SuperAdmin deployment URL
- Update `GOOGLE_REDIRECT_URL` to match superadmin domain
- Both main app and superadmin can use same `DB_*` variables (same database)
- `PORT` can be same in containers (different host ports in docker-compose)

### Optional Variables

```bash
# OpenTelemetry (distributed tracing)
OTEL_ENABLED=true
OTEL_SERVICE_NAME=iota-superadmin
OTEL_TEMPO_URL=http://tempo:4318

# Logging
LOG_FORMAT=json
LOG_OUTPUT=stdout
```

## Local Development

### Prerequisites

- Go 1.23.2+
- PostgreSQL 13+
- Make
- Docker (optional)

### Setup Steps

1. **Clone Repository:**
```bash
git clone https://github.com/iota-uz/iota-sdk.git
cd iota-sdk
```

2. **Configure Environment:**
```bash
cp .env.example .env.superadmin
# Edit .env.superadmin with your settings
```

3. **Run Database Migrations:**
```bash
make db migrate up
```

4. **Create Super Admin User:**
```sql
-- Run this SQL in your database
INSERT INTO users (
    id, email, first_name, last_name, type, tenant_id, created_at, updated_at
) VALUES (
    gen_random_uuid(),
    'admin@yourdomain.com',
    'Super',
    'Admin',
    'superadmin',
    NULL,
    NOW(),
    NOW()
);
```

5. **Build and Run:**
```bash
# Build
go build -o run_superadmin cmd/superadmin/main.go

# Run with environment file
set -a
source .env.superadmin
set +a
./run_superadmin
```

6. **Access Server:**
```
http://localhost:3000
```

### Using DevHub

If using [DevHub](https://github.com/iota-uz/devhub):

```bash
# Add superadmin service
devhub add superadmin go run cmd/superadmin/main.go

# Start all services including superadmin
devhub start

# View superadmin logs
devhub logs superadmin
```

## Docker Build

### Build Docker Image

```bash
# Build SuperAdmin image
docker build -f Dockerfile.superadmin -t iota-sdk-superadmin:latest .

# Tag for registry
docker tag iota-sdk-superadmin:latest your-registry/iota-sdk-superadmin:latest

# Push to registry
docker push your-registry/iota-sdk-superadmin:latest
```

### Dockerfile.superadmin

```dockerfile
# Build stage
FROM golang:1.23.2 AS builder

WORKDIR /app
COPY . .

# Generate templates
RUN go install github.com/a-h/templ/cmd/templ@latest
RUN templ generate

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o run_superadmin cmd/superadmin/main.go

# Runtime stage
FROM alpine:3.18

RUN apk add --no-cache ca-certificates

WORKDIR /home/iota-user
COPY --from=builder /app/run_superadmin .

EXPOSE 3000
CMD ["./run_superadmin"]
```

## Docker Compose Deployment

### docker-compose.superadmin.yml

```yaml
version: '3.8'

services:
  superadmin:
    build:
      context: .
      dockerfile: Dockerfile.superadmin
    container_name: iota-superadmin
    ports:
      - "3001:3000"  # Different host port from main app
    environment:
      - LOG_LEVEL=info
      - SESSION_DURATION=720h
      - DOMAIN=admin.localhost
      - GO_APP_ENV=development
      - DB_HOST=db
      - DB_PORT=5432
      - DB_NAME=iota_erp
      - DB_USER=postgres
      - DB_PASSWORD=postgres
      - PORT=3000
      - GOOGLE_CLIENT_ID=${GOOGLE_CLIENT_ID}
      - GOOGLE_CLIENT_SECRET=${GOOGLE_CLIENT_SECRET}
      - GOOGLE_REDIRECT_URL=http://localhost:3001/auth/google/callback
    depends_on:
      - db
    restart: unless-stopped
    networks:
      - iota-network
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:3000/health"]
      interval: 10s
      timeout: 5s
      retries: 3
      start_period: 40s

  # Shared PostgreSQL
  db:
    image: postgres:13-alpine
    container_name: iota-postgres
    environment:
      POSTGRES_DB: iota_erp
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    volumes:
      - postgres_data:/var/lib/postgresql/data
    networks:
      - iota-network
    restart: unless-stopped

networks:
  iota-network:
    driver: bridge

volumes:
  postgres_data:
```

### Run Docker Compose

```bash
# Start services
docker-compose -f docker-compose.superadmin.yml up -d

# View logs
docker-compose -f docker-compose.superadmin.yml logs -f superadmin

# Stop services
docker-compose -f docker-compose.superadmin.yml down
```

## Railway Deployment

### Railway Configuration

Railway requires two separate services (main app and superadmin) since each needs independent configuration.

**Service 1: Main Application**

```toml
# railway.toml
[build]
builder = "dockerfile"
dockerfilePath = "Dockerfile"

# Startup is handled by Dockerfile CMD via scripts/start.sh
# This ensures consistent behavior between local Docker and Railway
```

**Service 2: Super Admin**

```toml
# railway.superadmin.toml
[build]
builder = "dockerfile"
dockerfilePath = "Dockerfile.superadmin"

[deploy]
startCommand = "command migrate up && run_superadmin"
```

### Railway Environment Setup

1. **Create two services in Railway:**
   - `iota-main` - Main application
   - `iota-superadmin` - SuperAdmin service

2. **Link shared PostgreSQL:**
   - Both services use same database
   - Configure via environment variables

3. **Configure domains:**
   - Main app: `app.yourdomain.com`
   - SuperAdmin: `admin.yourdomain.com`

4. **Set environment variables:**
   - Both services share `DB_*` variables
   - Each service has unique `DOMAIN` and OAuth redirect URL
   - Copy other variables identically

### Deployment Command

```bash
# Deploy via Railway CLI
railway up --service iota-superadmin
```

## Kubernetes Deployment

### Deployment YAML

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: iota-superadmin
  namespace: iota-production
spec:
  replicas: 2
  selector:
    matchLabels:
      app: iota-superadmin
  template:
    metadata:
      labels:
        app: iota-superadmin
    spec:
      containers:
      - name: superadmin
        image: your-registry/iota-sdk-superadmin:latest
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 3000
          name: http
        env:
        - name: LOG_LEVEL
          value: "info"
        - name: PORT
          value: "3000"
        - name: GO_APP_ENV
          value: "production"
        - name: DOMAIN
          value: "admin.yourdomain.com"
        - name: SESSION_DURATION
          value: "720h"
        - name: DB_HOST
          valueFrom:
            secretKeyRef:
              name: iota-db-secret
              key: host
        - name: DB_PORT
          value: "5432"
        - name: DB_NAME
          valueFrom:
            secretKeyRef:
              name: iota-db-secret
              key: database
        - name: DB_USER
          valueFrom:
            secretKeyRef:
              name: iota-db-secret
              key: username
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: iota-db-secret
              key: password
        - name: GOOGLE_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: iota-oauth-secret
              key: google-client-id
        - name: GOOGLE_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: iota-oauth-secret
              key: google-client-secret
        - name: GOOGLE_REDIRECT_URL
          value: "https://admin.yourdomain.com/auth/google/callback"
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 3000
          initialDelaySeconds: 30
          periodSeconds: 10
          timeoutSeconds: 5
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /health
            port: 3000
          initialDelaySeconds: 5
          periodSeconds: 5
          timeoutSeconds: 2
          failureThreshold: 2
        securityContext:
          runAsNonRoot: true
          runAsUser: 1000
          readOnlyRootFilesystem: true
          allowPrivilegeEscalation: false

---
apiVersion: v1
kind: Service
metadata:
  name: iota-superadmin-service
  namespace: iota-production
spec:
  selector:
    app: iota-superadmin
  ports:
  - protocol: TCP
    port: 80
    targetPort: 3000
    name: http
  type: ClusterIP

---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: iota-superadmin-ingress
  namespace: iota-production
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - admin.yourdomain.com
    secretName: superadmin-tls
  rules:
  - host: admin.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: iota-superadmin-service
            port:
              number: 80
```

### Deploy to Kubernetes

```bash
# Apply deployment
kubectl apply -f k8s/superadmin-deployment.yaml

# Check rollout status
kubectl rollout status deployment/iota-superadmin -n iota-production

# View logs
kubectl logs -f deployment/iota-superadmin -n iota-production

# Port forward for testing
kubectl port-forward service/iota-superadmin-service 3000:80 -n iota-production
```

## Health Checks

### Health Endpoint

```bash
curl http://localhost:3000/health
```

**Response:**
```json
{
  "status": "healthy",
  "database": "connected",
  "modules": ["core", "superadmin"],
  "uptime_seconds": 3600,
  "timestamp": "2025-09-30T10:00:00Z"
}
```

## Monitoring & Logging

### Application Logs

Configure log level via environment:

```bash
LOG_LEVEL=debug  # debug, info, warn, error
```

**Structured Logging Example:**

```go
logger.WithFields(logrus.Fields{
    "super_admin_id": userId,
    "tenant_id": tenantId,
    "action": "tenant_created",
}).Info("Super admin created new tenant")
```

### OpenTelemetry Tracing

Enable distributed tracing:

```bash
OTEL_ENABLED=true
OTEL_SERVICE_NAME=iota-superadmin
OTEL_TEMPO_URL=http://tempo:4318
```

### Metrics Tracking

Recommended metrics:

- **Request Metrics**: Requests/sec, response times (p50, p95, p99), error rates
- **Database Metrics**: Connection pool usage, query times, active connections
- **Business Metrics**: Tenant operations, super admin logins, API usage

## Security Considerations

### SSL/TLS

- **Production**: HTTPS required (TLS termination at load balancer)
- **Development**: HTTP acceptable for local testing
- **Certificates**: Use Let's Encrypt (automated via cert-manager in K8s)

### Database Security

1. **Connection Security:**
   - Use SSL/TLS for database connections
   - Store credentials in secrets management
   - Rotate passwords regularly

2. **Query Safety:**
   - All queries parameterized (no concatenation)
   - Prepared statements for all queries
   - Input validation on all endpoints

### Network Security

1. **Deployment Isolation:**
   - Deploy on separate subdomain: `admin.yourdomain.com`
   - Consider VPN or IP whitelist for additional security
   - Network policies in K8s to restrict traffic

2. **Super Admin Account Security:**
   - Create via database only (never API)
   - Use strong, unique passwords
   - Enable MFA if available
   - Minimal number of super admin accounts

### Audit Logging

All operations logged:

```go
logger.WithFields(logrus.Fields{
    "super_admin_id": superAdminID,
    "action": "delete_tenant",
    "tenant_id": tenantID,
    "ip_address": r.RemoteAddr,
    "user_agent": r.UserAgent(),
    "timestamp": time.Now(),
}).Warn("Super admin deleted tenant")
```

## Troubleshooting

### 403 Forbidden on all routes

**Cause**: User is not a super admin.

**Solution:**
```sql
-- Check user type
SELECT id, email, type FROM users WHERE email = 'your-email@domain.com';

-- Update user to superadmin
UPDATE users SET type = 'superadmin' WHERE email = 'your-email@domain.com';
```

### Database connection errors

**Cause**: Database not accessible or incorrect credentials.

**Solution:**
```bash
# Test database connection
psql "host=$DB_HOST port=$DB_PORT dbname=$DB_NAME user=$DB_USER password=$DB_PASSWORD"

# Check environment variables
env | grep DB_
```

### Module registration errors

**Cause**: Module dependencies not satisfied.

**Solution:**
```bash
go mod download
go build -o run_superadmin cmd/superadmin/main.go
```

### Session validation failures

**Cause**: Session cookie not being set or expired.

**Solution:**
- Check `SESSION_DURATION` configuration
- Verify cookie domain matches deployment URL
- Clear browser cookies and re-login
- Check clock synchronization between services

### Enable Debug Mode

```bash
LOG_LEVEL=debug ./run_superadmin
```

## Backup & Recovery

### Database Backup

```bash
# Backup SuperAdmin data
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME > backup_$(date +%Y%m%d).sql

# Restore from backup
psql -h $DB_HOST -U $DB_USER -d $DB_NAME < backup_20250101.sql
```

### Service Recovery

- SuperAdmin service can be restarted without affecting main application
- Shared database ensures data consistency
- No shared state or caching (stateless design)
- Multiple replicas ensure high availability

## Performance Tuning

### Database Connection Pool

```go
// Recommended settings
MaxOpenConns: 25
MaxIdleConns: 5
MaxConnLifetime: 5 minutes
```

### Query Optimization

1. Create indexes on frequently queried columns
2. Use EXPLAIN ANALYZE for slow queries
3. Cache analytics results (5-minute TTL)
4. Batch operations where possible

### Caching Strategy

- Dashboard metrics: 5-minute cache
- Tenant list: 1-minute cache
- User searches: No cache (real-time)

## Further Reading

- [Main SUPERADMIN.md Documentation](../SUPERADMIN.md)
- [Technical Architecture](./technical.md)
- [Data Model](./data-model.md)
- [Business Requirements](./business.md)
