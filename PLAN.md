# Dynamic DNS Management System with AWS Route 53

## Project Overview

A web-based Dynamic DNS (DDNS) management system that integrates with AWS Route 53 for DNS management. The system provides a user-friendly interface for managing dynamic DNS records and exposes an API endpoint compatible with Ubiquiti Dream Machine Pro's built-in DDNS client.

---

## Technical Stack

| Component | Technology |
|-----------|------------|
| Backend | Go 1.21+ with Fiber v2 |
| Frontend | HTMX + Tailwind CSS (server-rendered) |
| Database | DynamoDB |
| DNS Provider | AWS Route 53 |
| Deployment | AWS Lambda + API Gateway |
| Infrastructure | AWS SAM / CloudFormation |

---

## Core Features

### 1. Admin Authentication

- **Single-Admin Mode**: Dashboard access via `ADMIN_USER` / `ADMIN_PASSWORD` (no user registration)
- **IAM Role Permissions**: Lambda execution role has Route 53 access (no user-provided credentials)
- **Session Management**: DynamoDB-backed sessions with secure cookies

### 2. DNS Zone Management

- **List Hosted Zones**: Display all Route 53 hosted zones accessible via Lambda IAM role
- **Zone Details**: Show zone ID, name, record count, and type (public/private)
- **Record Browser**: View existing DNS records within each zone

### 3. Dynamic DNS Configuration

- **DDNS Record Creation**:
  - Select target hosted zone
  - Specify hostname (subdomain)
  - Set TTL (default: 60 seconds for dynamic records)
  - Generate unique update token per record

- **DDNS Record Management**:
  - View all configured DDNS records
  - Enable/disable individual records
  - Regenerate update tokens
  - View last update timestamp and IP history
  - **Token Display**: Show plaintext token only once on creation (like API key UX), then only allow regeneration

### 4. Ubiquiti Dream Machine Pro Compatible Update Endpoint

The system exposes a DynDNS2-compatible API endpoint that works with UDM Pro's custom DDNS provider:

```
GET /nic/update?hostname={hostname}&myip={ip}
Authorization: Basic {base64(username:token)}
```

**Response Codes:**
- `good {ip}` - Update successful
- `nochg {ip}` - IP unchanged
- `nohost` - Hostname not found
- `badauth` - Invalid credentials
- `abuse` - Too many requests

---

## Data Model (Single DynamoDB Table)

All entities stored in one table using composite keys for efficient access patterns.

### DDNS Record
```
PK: DDNS
SK: {hostname}
- hostname: string (FQDN)
- zone_id: string
- zone_name: string
- ttl: number
- update_token_hash: string (bcrypt)
- current_ip: string
- enabled: boolean
- last_updated: timestamp
- created_at: timestamp
```

### Update Log
```
PK: LOG#{hostname}
SK: {timestamp}
- previous_ip: string
- new_ip: string
- source_ip: string
- user_agent: string
- status: string
- TTL: 30 days (auto-expire)
```

---

## API Endpoints

### Authentication
| Method | Path | Description |
|--------|------|-------------|
| GET | `/login` | Login page |
| POST | `/login` | Process admin login |
| POST | `/logout` | End session |

### Utility
| Method | Path | Description |
|--------|------|-------------|
| GET | `/ip` | Return caller's IP address (useful for testing) |

### DNS Zones
| Method | Path | Description |
|--------|------|-------------|
| GET | `/zones` | List all hosted zones |
| GET | `/zones/{zoneId}` | View zone details |
| GET | `/zones/{zoneId}/records` | List zone records |

### DDNS Management
| Method | Path | Description |
|--------|------|-------------|
| GET | `/ddns` | List DDNS configurations |
| GET | `/ddns/new` | New DDNS form |
| POST | `/ddns` | Create DDNS record |
| GET | `/ddns/{hostname}` | View DDNS details |
| PUT | `/ddns/{hostname}` | Update DDNS config |
| DELETE | `/ddns/{hostname}` | Delete DDNS record |
| POST | `/ddns/{hostname}/regenerate-token` | New update token |
| GET | `/ddns/{hostname}/history` | View update history |

### DDNS Update Endpoint (Ubiquiti Compatible)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/nic/update` | DynDNS2-compatible update |

---

## Ubiquiti Dream Machine Pro Configuration

To configure the UDM Pro to use this service:

1. **Navigate to**: Settings > Internet > WAN > Dynamic DNS

2. **Custom DDNS Configuration** (fields are separate in UDM UI):
   ```
   Service:  custom
   Hostname: your-subdomain.yourdomain.com
   Username: admin
   Password: {generated-update-token-for-this-hostname}
   Server:   your-ddns-service.com
   ```

3. **How it works**: UDM Pro constructs the request as:
   ```
   GET https://{Server}/nic/update?hostname={Hostname}&myip={DetectedIP}
   Authorization: Basic {base64(Username:Password)}
   ```

4. **Update Interval**: UDM Pro checks every 5 minutes by default

5. **Testing**: Use the `/ip` endpoint to verify connectivity:
   ```
   curl https://your-ddns-service.com/ip
   ```

The system implements the standard DynDNS2 protocol that UDM Pro expects, ensuring drop-in compatibility.

---

## Security Requirements

### Authentication & Authorization
- bcrypt password hashing (cost 10)
- Session tokens stored in DynamoDB with 24-hour TTL
- Secure, HttpOnly, SameSite=Strict cookies
- CSRF protection on all forms
- **Brute force protection**: 5 failed login attempts triggers 15-minute lockout (stored in DynamoDB)

### Route 53 Access
- Lambda IAM role has least-privilege Route 53 permissions
- No user-provided AWS credentials stored or managed

### Update Endpoint Security
- Rate limiting: 60 requests per hour per hostname
- Update tokens are single-hostname scoped
- Tokens hashed with bcrypt (cost 10) before storage
- IP validation (IPv4/IPv6 format check)
- **IP spoofing protection**: `myip` parameter is optional; defaults to request source IP. If provided, must match source IP (prevents attackers from setting arbitrary IPs)

### Input Validation
- All user inputs sanitized and validated
- Hostname format validation (RFC 1123)

### Transport Security
- **HTTPS only**: API Gateway configured with HTTPS endpoints only
- Disable default execute-api endpoint when using custom domain (`DisableExecuteApiEndpoint: true`)
- HSTS header enabled for custom domain

---

## Logging Standards

All log entries include mandatory fields:

| Field | Format | Example |
|-------|--------|---------|
| `who` | `type:identifier` | `user:john@example.com` |
| `what` | Past tense action | `updated_ddns_record` |
| `why` | Business reason (max 100 chars) | `IP address changed for home network` |
| `where` | `service:component` | `ddns:update_handler` |

---

## Project Structure

```
dynamic-route-53-dns/
├── cmd/
│   └── lambda/
│       └── main.go
├── internal/
│   ├── api/
│   │   ├── handlers/
│   │   │   ├── auth.go
│   │   │   ├── zones.go
│   │   │   ├── ddns.go
│   │   │   └── update.go
│   │   ├── middleware/
│   │   │   ├── auth.go
│   │   │   ├── ratelimit.go
│   │   │   └── logging.go
│   │   └── routes.go
│   ├── auth/
│   │   └── session.go
│   ├── database/
│   │   ├── client.go
│   │   └── ddns.go
│   ├── route53/
│   │   ├── client.go
│   │   ├── zones.go
│   │   └── records.go
│   ├── service/
│   │   ├── auth.go
│   │   ├── zones.go
│   │   ├── ddns.go
│   │   └── update.go
├── web/
│   ├── templates/
│   │   ├── layouts/
│   │   │   └── base.html
│   │   ├── auth/
│   │   │   └── login.html
│   │   ├── zones/
│   │   │   ├── list.html
│   │   │   └── detail.html
│   │   └── ddns/
│   │       ├── list.html
│   │       ├── new.html
│   │       └── detail.html
│   └── static/
│       └── css/
│           └── tailwind.css
├── .github/
│   └── workflows/
│       └── deploy.yaml
├── go.mod
├── go.sum
├── Makefile
├── template.yaml
├── samconfig.toml
└── README.md
```

---

## GitHub Secrets & Variables

### GitHub Secrets (Repository Settings > Secrets)

| Secret | Description | Example |
|--------|-------------|---------|
| `AWS_ACCESS_KEY_ID` | AWS IAM access key for deployment | `AKIA...` |
| `AWS_SECRET_ACCESS_KEY` | AWS IAM secret key for deployment | `wJalr...` |
| `ADMIN_PASSWORD` | Initial admin user password | `SecureP@ss123!` |
| `APP_SECRET` | 32-byte secret for session signing | `<random-32-bytes>` |

### GitHub Variables (Repository Settings > Variables)

| Variable | Description | Example |
|----------|-------------|---------|
| `AWS_REGION` | AWS deployment region | `us-east-2` |
| `ADMIN_USER` | Initial admin username | `admin` |
| `DOMAIN_NAME` | Custom domain (or `DISABLED`) | `ddns.example.com` |
| `HOSTED_ZONE_ID` | Route 53 zone for custom domain (or `DISABLED`) | `Z0123456789ABC` |
| `CLOUDFRONT_CERT_ARN` | ACM certificate ARN in us-east-1 (or `DISABLED`) | `arn:aws:acm:us-east-1:...` |

---

## SAM Template Parameters

```yaml
Parameters:
  AdminUsername:
    Type: String
    Default: admin
    Description: Admin username for initial setup

  AdminPassword:
    Type: String
    NoEcho: true
    Description: Admin password for initial setup

  DomainName:
    Type: String
    Default: DISABLED
    Description: Custom domain name for the application

  HostedZoneId:
    Type: String
    Default: DISABLED
    Description: Route53 Hosted Zone ID for custom domain

  CloudFrontCertificateArn:
    Type: String
    Default: DISABLED
    Description: ARN of ACM certificate in us-east-1 for CloudFront
```

---

## Required IAM Permissions (Lambda Execution Role)

```yaml
# Route 53 permissions for DNS management
- Effect: Allow
  Action:
    - route53:ListHostedZones
    - route53:GetHostedZone
    - route53:ListResourceRecordSets
    - route53:ChangeResourceRecordSets
  Resource: "*"

# DynamoDB permissions for data storage
- Effect: Allow
  Action:
    - dynamodb:GetItem
    - dynamodb:PutItem
    - dynamodb:UpdateItem
    - dynamodb:DeleteItem
    - dynamodb:Query
    - dynamodb:Scan
  Resource: !GetAtt DynamoDBTable.Arn
```

---

## Lambda Environment Variables

| Variable | Description |
|----------|-------------|
| `DYNAMODB_TABLE` | Single DynamoDB table name |
| `ADMIN_USERNAME` | Admin username (from parameter) |
| `ADMIN_PASSWORD` | Admin password (from parameter) |
| `APP_SECRET` | Secret for session signing |

---

## Lambda Configuration

| Setting | Value | Rationale |
|---------|-------|-----------|
| **Memory** | 1024 MB | Fast CPU allocation for bcrypt operations. Reduces bcrypt to ~15-25ms. Comfortable headroom for concurrent requests. |
| **Timeout** | 60 seconds | Conservative timeout for Route 53 API edge cases and complex operations. Most requests complete in <500ms. |
| **Architecture** | arm64 | 20% cheaper than x86_64, comparable performance for Go. |
| **Ephemeral Storage** | 512 MB (default) | No file storage needed. |

### Cold Start Considerations
- Go has minimal cold start (~100-200ms init + ~1s Lambda overhead)
- DynamoDB client initialized in `init()` and reused across invocations
- Route 53 client initialized in `init()` and reused
- bcrypt cost 10 takes ~15-25ms per hash operation at 1GB memory

---

## SAM Configuration (samconfig.toml)

```toml
version = 0.1

[default.deploy.parameters]
stack_name = "dynamic-dns-stack"
s3_bucket = "sam-deploy-dynamic-dns-us-east-2"
capabilities = "CAPABILITY_IAM"
confirm_changeset = false
fail_on_empty_changeset = false
region = "us-east-2"
```

---

## Implementation Checklist

### Phase 1: Core Infrastructure
- [ ] Initialize Go module and dependencies
- [ ] Set up single DynamoDB table schema
- [ ] Implement structured logging
- [ ] Create base Fiber application with Lambda adapter
- [ ] Add `/ip` endpoint for IP detection

### Phase 2: Authentication
- [ ] Admin login with `ADMIN_USER` / `ADMIN_PASSWORD`
- [ ] Session management with DynamoDB
- [ ] Session middleware for protected routes

### Phase 3: Zone Management
- [ ] Route 53 client using Lambda IAM role
- [ ] List hosted zones handler
- [ ] Zone detail view with records
- [ ] Zone caching (5-minute TTL)

### Phase 4: DDNS Configuration
- [ ] DDNS record CRUD operations
- [ ] Update token generation and hashing
- [ ] Record enable/disable toggle
- [ ] Update history logging

### Phase 5: Update Endpoint
- [ ] DynDNS2-compatible endpoint implementation
- [ ] Basic auth parsing and validation
- [ ] Route 53 A/AAAA record update logic
- [ ] Rate limiting middleware
- [ ] Response codes matching DynDNS2 spec

### Phase 6: Frontend
- [ ] Base layout with Tailwind CSS
- [ ] HTMX integration for dynamic updates
- [ ] All page templates
- [ ] Form validation and error display

### Phase 7: Deployment
- [ ] SAM template with all resources
- [ ] IAM role with Route 53 permissions
- [ ] samconfig.toml configuration
- [ ] GitHub Actions CI/CD pipeline (deploy.yml)
- [ ] Custom domain configuration
- [ ] SSL certificate setup

---

## Testing Requirements

- **Unit Tests**: 80% coverage minimum
- **Integration Tests**: Route 53 operations with localstack
- **E2E Tests**: Full DDNS update flow
- **Security Tests**: Token handling, injection prevention

---

## Success Criteria

1. Admin can login and access the dashboard
2. Admin can view all Route 53 hosted zones via Lambda IAM role
3. Admin can create DDNS configurations for any hostname in their zones
4. Ubiquiti Dream Machine Pro successfully updates DNS records via the `/nic/update` endpoint
5. DNS records update within 60 seconds of IP change
6. Update tokens never logged or exposed after initial creation
7. System handles 1000+ update requests per minute
