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

---

## Detailed Page Specifications

This section provides exhaustive specifications for each page, including exact field names, data types, and template variable mappings.

### Common Template Data

All pages receive a base context with these fields:

```go
// BaseTemplateData is embedded in all page-specific template data
type BaseTemplateData struct {
    PageTitle    string // Browser tab title, e.g., "Zones - Dynamic DNS"
    CurrentPath  string // Current URL path for nav highlighting, e.g., "/zones"
    FlashSuccess string // Success message to display (optional)
    FlashError   string // Error message to display (optional)
    CSRFToken    string // CSRF token for forms
    Username     string // Logged-in admin username (empty if not authenticated)
    IsLoggedIn   bool   // Whether user is authenticated
}
```

**Template Usage:**
```html
<title>{{ .PageTitle }}</title>
<input type="hidden" name="_csrf" value="{{ .CSRFToken }}">
{{ if .FlashSuccess }}<div class="alert-success">{{ .FlashSuccess }}</div>{{ end }}
{{ if .FlashError }}<div class="alert-error">{{ .FlashError }}</div>{{ end }}
```

---

### Page: Login (`/login`)

**Template File:** `web/templates/auth/login.html`

**Route Handler:** `GET /login` renders form, `POST /login` processes login

**Authentication Required:** No (redirects to `/zones` if already logged in)

**Template Data Structure:**
```go
type LoginPageData struct {
    BaseTemplateData
    Username     string // Pre-filled username on error (from previous attempt)
    ErrorMessage string // Specific error: "Invalid credentials" or "Account locked"
    IsLocked     bool   // True if account is locked due to failed attempts
    LockoutEnds  string // Human-readable lockout end time, e.g., "in 12 minutes"
}
```

**Form Fields:**

| Field Name | HTML Name | Type | Validation | Error Message |
|------------|-----------|------|------------|---------------|
| Username | `username` | text | Required, 1-50 chars | "Username is required" |
| Password | `password` | password | Required, 1-100 chars | "Password is required" |
| CSRF Token | `_csrf` | hidden | Must match session | "Invalid request, please try again" |

**Template Example:**
```html
<form method="POST" action="/login">
    <input type="hidden" name="_csrf" value="{{ .CSRFToken }}">

    {{ if .ErrorMessage }}
    <div class="error">{{ .ErrorMessage }}</div>
    {{ end }}

    {{ if .IsLocked }}
    <div class="warning">Account locked. Try again {{ .LockoutEnds }}.</div>
    {{ end }}

    <label for="username">Username</label>
    <input type="text" id="username" name="username" value="{{ .Username }}"
           required maxlength="50" autocomplete="username">

    <label for="password">Password</label>
    <input type="password" id="password" name="password"
           required maxlength="100" autocomplete="current-password">

    <button type="submit" {{ if .IsLocked }}disabled{{ end }}>Sign In</button>
</form>
```

**Handler Logic:**
```go
// POST /login handler
func (h *AuthHandler) HandleLogin(c *fiber.Ctx) error {
    username := c.FormValue("username")
    password := c.FormValue("password")
    csrfToken := c.FormValue("_csrf")

    // 1. Validate CSRF token matches session
    // 2. Check if account is locked (query DynamoDB for failed attempts)
    // 3. Validate credentials against ADMIN_USERNAME and bcrypt-hashed ADMIN_PASSWORD
    // 4. On success: create session in DynamoDB, set cookie, redirect to /zones
    // 5. On failure: increment failed attempts, re-render with error
}
```

**Session Cookie:**
```go
// Cookie settings for session
cookie := &fiber.Cookie{
    Name:     "session_id",
    Value:    sessionToken, // 32-byte random hex string
    Expires:  time.Now().Add(24 * time.Hour),
    HTTPOnly: true,
    Secure:   true, // HTTPS only
    SameSite: "Strict",
    Path:     "/",
}
```

---

### Page: Zones List (`/zones`)

**Template File:** `web/templates/zones/list.html`

**Route Handler:** `GET /zones`

**Authentication Required:** Yes (redirects to `/login` if not authenticated)

**Template Data Structure:**
```go
type ZonesListPageData struct {
    BaseTemplateData
    Zones []ZoneListItem
}

type ZoneListItem struct {
    ID           string // Route 53 zone ID, e.g., "Z0123456789ABC"
    Name         string // Zone name with trailing dot, e.g., "example.com."
    DisplayName  string // Zone name without trailing dot, e.g., "example.com"
    RecordCount  int    // Number of records in zone
    IsPrivate    bool   // True if private hosted zone
    DDNSCount    int    // Number of DDNS records configured for this zone
}
```

**Template Example:**
```html
<h1>Hosted Zones</h1>

{{ if not .Zones }}
<p>No hosted zones found. Ensure the Lambda IAM role has Route 53 permissions.</p>
{{ else }}
<table>
    <thead>
        <tr>
            <th>Zone Name</th>
            <th>Type</th>
            <th>Records</th>
            <th>DDNS Configured</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
        {{ range .Zones }}
        <tr>
            <td>{{ .DisplayName }}</td>
            <td>{{ if .IsPrivate }}Private{{ else }}Public{{ end }}</td>
            <td>{{ .RecordCount }}</td>
            <td>{{ .DDNSCount }}</td>
            <td>
                <a href="/zones/{{ .ID }}">View Records</a>
            </td>
        </tr>
        {{ end }}
    </tbody>
</table>
{{ end }}
```

---

### Page: Zone Detail (`/zones/{zoneId}`)

**Template File:** `web/templates/zones/detail.html`

**Route Handler:** `GET /zones/:zoneId`

**Authentication Required:** Yes

**URL Parameters:**
| Parameter | Description | Validation |
|-----------|-------------|------------|
| `zoneId` | Route 53 zone ID | Must match pattern `^Z[A-Z0-9]{10,32}$` |

**Template Data Structure:**
```go
type ZoneDetailPageData struct {
    BaseTemplateData
    Zone    ZoneDetail
    Records []DNSRecord
}

type ZoneDetail struct {
    ID          string // Route 53 zone ID
    Name        string // Zone name with trailing dot
    DisplayName string // Zone name without trailing dot
    RecordCount int
    IsPrivate   bool
    Comment     string // Zone comment/description from Route 53
}

type DNSRecord struct {
    Name         string   // Record name, e.g., "www.example.com."
    DisplayName  string   // Without trailing dot
    Type         string   // A, AAAA, CNAME, MX, TXT, etc.
    TTL          int      // TTL in seconds
    Values       []string // Record values (multiple for round-robin)
    IsDDNS       bool     // True if this record is managed by DDNS
    DDNSHostname string   // DDNS hostname if IsDDNS is true
}
```

**Template Example:**
```html
<h1>{{ .Zone.DisplayName }}</h1>
<p>Zone ID: {{ .Zone.ID }} | {{ if .Zone.IsPrivate }}Private{{ else }}Public{{ end }} | {{ .Zone.RecordCount }} records</p>

<h2>DNS Records</h2>
<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>TTL</th>
            <th>Value(s)</th>
            <th>DDNS</th>
        </tr>
    </thead>
    <tbody>
        {{ range .Records }}
        <tr>
            <td>{{ .DisplayName }}</td>
            <td>{{ .Type }}</td>
            <td>{{ .TTL }}s</td>
            <td>
                {{ range .Values }}
                <div>{{ . }}</div>
                {{ end }}
            </td>
            <td>
                {{ if .IsDDNS }}
                <a href="/ddns/{{ .DDNSHostname }}">Managed</a>
                {{ else }}
                -
                {{ end }}
            </td>
        </tr>
        {{ end }}
    </tbody>
</table>

<a href="/ddns/new?zone={{ .Zone.ID }}">+ Add DDNS Record</a>
```

---

### Page: DDNS List (`/ddns`)

**Template File:** `web/templates/ddns/list.html`

**Route Handler:** `GET /ddns`

**Authentication Required:** Yes

**Template Data Structure:**
```go
type DDNSListPageData struct {
    BaseTemplateData
    Records []DDNSListItem
}

type DDNSListItem struct {
    Hostname      string    // FQDN, e.g., "home.example.com"
    ZoneName      string    // Parent zone, e.g., "example.com"
    ZoneID        string    // Route 53 zone ID
    CurrentIP     string    // Current IP address or "Not set"
    TTL           int       // TTL in seconds
    Enabled       bool      // Whether updates are enabled
    LastUpdated   time.Time // Last successful update
    LastUpdatedAt string    // Formatted: "2 hours ago" or "Never"
    UpdateCount   int       // Total update count (last 30 days)
}
```

**Template Example:**
```html
<h1>DDNS Records</h1>

<a href="/ddns/new" class="btn-primary">+ New DDNS Record</a>

{{ if not .Records }}
<p>No DDNS records configured. Create one to get started.</p>
{{ else }}
<table>
    <thead>
        <tr>
            <th>Hostname</th>
            <th>Zone</th>
            <th>Current IP</th>
            <th>TTL</th>
            <th>Status</th>
            <th>Last Updated</th>
            <th>Actions</th>
        </tr>
    </thead>
    <tbody>
        {{ range .Records }}
        <tr>
            <td><a href="/ddns/{{ .Hostname }}">{{ .Hostname }}</a></td>
            <td>{{ .ZoneName }}</td>
            <td>{{ if .CurrentIP }}{{ .CurrentIP }}{{ else }}<em>Not set</em>{{ end }}</td>
            <td>{{ .TTL }}s</td>
            <td>
                {{ if .Enabled }}
                <span class="badge-success">Enabled</span>
                {{ else }}
                <span class="badge-warning">Disabled</span>
                {{ end }}
            </td>
            <td>{{ .LastUpdatedAt }}</td>
            <td>
                <a href="/ddns/{{ .Hostname }}">View</a>
                <a href="/ddns/{{ .Hostname }}/history">History</a>
            </td>
        </tr>
        {{ end }}
    </tbody>
</table>
{{ end }}
```

---

### Page: DDNS Create (`/ddns/new`)

**Template File:** `web/templates/ddns/new.html`

**Route Handler:** `GET /ddns/new` renders form, `POST /ddns` processes creation

**Authentication Required:** Yes

**Query Parameters:**
| Parameter | Description | Required |
|-----------|-------------|----------|
| `zone` | Pre-select zone ID | No |

**Template Data Structure:**
```go
type DDNSNewPageData struct {
    BaseTemplateData
    Zones         []ZoneOption      // Available zones for dropdown
    SelectedZone  string            // Pre-selected zone ID from query param
    FormData      DDNSCreateForm    // Form values on validation error
    FieldErrors   map[string]string // Field-specific errors
}

type ZoneOption struct {
    ID          string // Zone ID for value
    DisplayName string // Zone name for display
}

type DDNSCreateForm struct {
    ZoneID   string
    Hostname string // Subdomain part only, e.g., "home"
    TTL      int    // Default: 60
}
```

**Form Fields:**

| Field Name | HTML Name | Type | Validation | Error Message |
|------------|-----------|------|------------|---------------|
| Zone | `zone_id` | select | Required, must exist | "Please select a zone" |
| Hostname | `hostname` | text | Required, 1-63 chars, RFC 1123 subdomain | "Invalid hostname format" |
| TTL | `ttl` | number | Required, 60-86400 | "TTL must be between 60 and 86400 seconds" |
| CSRF Token | `_csrf` | hidden | Must match session | "Invalid request" |

**Template Example:**
```html
<h1>Create DDNS Record</h1>

<form method="POST" action="/ddns">
    <input type="hidden" name="_csrf" value="{{ .CSRFToken }}">

    <div class="form-group">
        <label for="zone_id">Hosted Zone *</label>
        <select id="zone_id" name="zone_id" required>
            <option value="">Select a zone...</option>
            {{ range .Zones }}
            <option value="{{ .ID }}" {{ if eq .ID $.SelectedZone }}selected{{ end }}>
                {{ .DisplayName }}
            </option>
            {{ end }}
        </select>
        {{ if .FieldErrors.zone_id }}
        <span class="error">{{ .FieldErrors.zone_id }}</span>
        {{ end }}
    </div>

    <div class="form-group">
        <label for="hostname">Hostname (subdomain) *</label>
        <input type="text" id="hostname" name="hostname"
               value="{{ .FormData.Hostname }}"
               required maxlength="63"
               pattern="^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$"
               placeholder="e.g., home or vpn">
        <small>Will create: <span id="fqdn-preview">[hostname].[zone]</span></small>
        {{ if .FieldErrors.hostname }}
        <span class="error">{{ .FieldErrors.hostname }}</span>
        {{ end }}
    </div>

    <div class="form-group">
        <label for="ttl">TTL (seconds) *</label>
        <input type="number" id="ttl" name="ttl"
               value="{{ if .FormData.TTL }}{{ .FormData.TTL }}{{ else }}60{{ end }}"
               required min="60" max="86400">
        <small>Recommended: 60 for dynamic IPs</small>
        {{ if .FieldErrors.ttl }}
        <span class="error">{{ .FieldErrors.ttl }}</span>
        {{ end }}
    </div>

    <button type="submit">Create DDNS Record</button>
</form>

<script>
// HTMX: Update FQDN preview when zone or hostname changes
document.getElementById('hostname').addEventListener('input', updatePreview);
document.getElementById('zone_id').addEventListener('change', updatePreview);
function updatePreview() {
    const hostname = document.getElementById('hostname').value || '[hostname]';
    const zone = document.getElementById('zone_id');
    const zoneName = zone.selectedOptions[0]?.text || '[zone]';
    document.getElementById('fqdn-preview').textContent = hostname + '.' + zoneName;
}
</script>
```

**POST Handler Response:**
On success, returns the created record with the **plaintext token shown once**:

```go
type DDNSCreatedPageData struct {
    BaseTemplateData
    Hostname       string // Created FQDN
    PlaintextToken string // ONE-TIME display of token
    TokenWarning   string // "Save this token now. It cannot be shown again."
}
```

---

### Page: DDNS Detail (`/ddns/{hostname}`)

**Template File:** `web/templates/ddns/detail.html`

**Route Handler:** `GET /ddns/:hostname`, `PUT /ddns/:hostname`, `DELETE /ddns/:hostname`

**Authentication Required:** Yes

**URL Parameters:**
| Parameter | Description | Validation |
|-----------|-------------|------------|
| `hostname` | FQDN | Must exist in database |

**Template Data Structure:**
```go
type DDNSDetailPageData struct {
    BaseTemplateData
    Record           DDNSDetailRecord
    RecentHistory    []DDNSUpdateEvent // Last 10 updates
    NewToken         string            // Only set after regeneration
    ShowTokenWarning bool              // True when NewToken is set
}

type DDNSDetailRecord struct {
    Hostname    string
    ZoneID      string
    ZoneName    string
    TTL         int
    CurrentIP   string    // Or empty string if never updated
    Enabled     bool
    CreatedAt   time.Time
    CreatedAtFmt string   // "Jan 2, 2006 3:04 PM"
    LastUpdated time.Time
    LastUpdatedFmt string // "2 hours ago" or "Never"
    UpdateCount int       // Last 30 days
}

type DDNSUpdateEvent struct {
    Timestamp    time.Time
    TimestampFmt string // "Jan 2, 2006 3:04:05 PM"
    PreviousIP   string
    NewIP        string
    SourceIP     string
    Status       string // "success", "no_change", "error"
    StatusBadge  string // CSS class: "badge-success", "badge-info", "badge-error"
}
```

**Template Example:**
```html
<h1>{{ .Record.Hostname }}</h1>

{{ if .ShowTokenWarning }}
<div class="alert-warning">
    <strong>New Update Token Generated</strong><br>
    <code>{{ .NewToken }}</code><br>
    <small>Save this token now. It cannot be shown again.</small>
</div>
{{ end }}

<div class="card">
    <h2>Configuration</h2>
    <dl>
        <dt>Hostname</dt>
        <dd>{{ .Record.Hostname }}</dd>

        <dt>Zone</dt>
        <dd><a href="/zones/{{ .Record.ZoneID }}">{{ .Record.ZoneName }}</a></dd>

        <dt>Current IP</dt>
        <dd>{{ if .Record.CurrentIP }}{{ .Record.CurrentIP }}{{ else }}<em>Not set</em>{{ end }}</dd>

        <dt>TTL</dt>
        <dd>{{ .Record.TTL }} seconds</dd>

        <dt>Status</dt>
        <dd>
            {{ if .Record.Enabled }}
            <span class="badge-success">Enabled</span>
            {{ else }}
            <span class="badge-warning">Disabled</span>
            {{ end }}
        </dd>

        <dt>Created</dt>
        <dd>{{ .Record.CreatedAtFmt }}</dd>

        <dt>Last Updated</dt>
        <dd>{{ .Record.LastUpdatedFmt }}</dd>

        <dt>Updates (30 days)</dt>
        <dd>{{ .Record.UpdateCount }}</dd>
    </dl>
</div>

<div class="actions">
    <h2>Actions</h2>

    <!-- Toggle Enable/Disable -->
    <form method="POST" action="/ddns/{{ .Record.Hostname }}" hx-post="/ddns/{{ .Record.Hostname }}" hx-swap="outerHTML">
        <input type="hidden" name="_csrf" value="{{ .CSRFToken }}">
        <input type="hidden" name="_method" value="PUT">
        <input type="hidden" name="enabled" value="{{ if .Record.Enabled }}false{{ else }}true{{ end }}">
        <button type="submit">
            {{ if .Record.Enabled }}Disable{{ else }}Enable{{ end }} Updates
        </button>
    </form>

    <!-- Regenerate Token -->
    <form method="POST" action="/ddns/{{ .Record.Hostname }}/regenerate-token">
        <input type="hidden" name="_csrf" value="{{ .CSRFToken }}">
        <button type="submit" onclick="return confirm('This will invalidate the current token. Continue?')">
            Regenerate Token
        </button>
    </form>

    <!-- Delete -->
    <form method="POST" action="/ddns/{{ .Record.Hostname }}" hx-delete="/ddns/{{ .Record.Hostname }}" hx-confirm="Delete this DDNS record? This cannot be undone.">
        <input type="hidden" name="_csrf" value="{{ .CSRFToken }}">
        <input type="hidden" name="_method" value="DELETE">
        <button type="submit" class="btn-danger">Delete Record</button>
    </form>
</div>

<div class="card">
    <h2>Recent Updates</h2>
    {{ if not .RecentHistory }}
    <p>No updates recorded yet.</p>
    {{ else }}
    <table>
        <thead>
            <tr>
                <th>Time</th>
                <th>Previous IP</th>
                <th>New IP</th>
                <th>Source</th>
                <th>Status</th>
            </tr>
        </thead>
        <tbody>
            {{ range .RecentHistory }}
            <tr>
                <td>{{ .TimestampFmt }}</td>
                <td>{{ if .PreviousIP }}{{ .PreviousIP }}{{ else }}-{{ end }}</td>
                <td>{{ .NewIP }}</td>
                <td>{{ .SourceIP }}</td>
                <td><span class="{{ .StatusBadge }}">{{ .Status }}</span></td>
            </tr>
            {{ end }}
        </tbody>
    </table>
    {{ end }}
    <a href="/ddns/{{ .Record.Hostname }}/history">View Full History</a>
</div>

<div class="card">
    <h2>Ubiquiti UDM Pro Configuration</h2>
    <pre>
Service:  custom
Hostname: {{ .Record.Hostname }}
Username: ddns
Password: [your-update-token]
Server:   [your-domain]/nic/update
    </pre>
</div>
```

---

### Page: DDNS History (`/ddns/{hostname}/history`)

**Template File:** `web/templates/ddns/history.html`

**Route Handler:** `GET /ddns/:hostname/history`

**Authentication Required:** Yes

**Query Parameters:**
| Parameter | Description | Default |
|-----------|-------------|---------|
| `page` | Page number | 1 |
| `limit` | Items per page | 50 |

**Template Data Structure:**
```go
type DDNSHistoryPageData struct {
    BaseTemplateData
    Hostname   string
    History    []DDNSUpdateEvent
    Pagination PaginationData
}

type PaginationData struct {
    CurrentPage  int
    TotalPages   int
    TotalItems   int
    HasPrevious  bool
    HasNext      bool
    PreviousPage int
    NextPage     int
}
```

---

## Authentication & Session Management

### Session Storage (DynamoDB)

```go
// Session record in DynamoDB
// PK: SESSION
// SK: {session_token}

type SessionRecord struct {
    PK           string    `dynamodbav:"PK"`           // "SESSION"
    SK           string    `dynamodbav:"SK"`           // session token (32-byte hex)
    Username     string    `dynamodbav:"username"`     // admin username
    CreatedAt    time.Time `dynamodbav:"created_at"`
    ExpiresAt    time.Time `dynamodbav:"expires_at"`
    TTL          int64     `dynamodbav:"ttl"`          // DynamoDB TTL (Unix timestamp)
    IPAddress    string    `dynamodbav:"ip_address"`   // Client IP at login
    UserAgent    string    `dynamodbav:"user_agent"`
}
```

### Login Attempt Tracking

```go
// Failed login tracking in DynamoDB
// PK: LOGIN_ATTEMPT
// SK: {username}

type LoginAttemptRecord struct {
    PK             string    `dynamodbav:"PK"`              // "LOGIN_ATTEMPT"
    SK             string    `dynamodbav:"SK"`              // username
    FailedAttempts int       `dynamodbav:"failed_attempts"`
    LastAttempt    time.Time `dynamodbav:"last_attempt"`
    LockedUntil    time.Time `dynamodbav:"locked_until"`    // Zero if not locked
    TTL            int64     `dynamodbav:"ttl"`             // Auto-expire after 1 hour
}
```

### Auth Middleware

```go
// AuthMiddleware checks for valid session on protected routes
func AuthMiddleware(sessionStore *SessionStore) fiber.Handler {
    return func(c *fiber.Ctx) error {
        // 1. Get session_id from cookie
        sessionID := c.Cookies("session_id")
        if sessionID == "" {
            return c.Redirect("/login")
        }

        // 2. Look up session in DynamoDB
        session, err := sessionStore.Get(c.Context(), sessionID)
        if err != nil || session == nil {
            // Clear invalid cookie
            c.ClearCookie("session_id")
            return c.Redirect("/login")
        }

        // 3. Check expiration
        if time.Now().After(session.ExpiresAt) {
            sessionStore.Delete(c.Context(), sessionID)
            c.ClearCookie("session_id")
            return c.Redirect("/login")
        }

        // 4. Set user context for handlers
        c.Locals("username", session.Username)
        c.Locals("session_id", sessionID)

        return c.Next()
    }
}
```

### CSRF Protection

```go
// CSRF token generation and validation
func CSRFMiddleware(store *SessionStore) fiber.Handler {
    return func(c *fiber.Ctx) error {
        sessionID := c.Cookies("session_id")

        // Generate token for GET requests
        if c.Method() == "GET" {
            token := generateCSRFToken(sessionID)
            c.Locals("csrf_token", token)
            return c.Next()
        }

        // Validate token for POST/PUT/DELETE
        formToken := c.FormValue("_csrf")
        if formToken == "" {
            formToken = c.Get("X-CSRF-Token")
        }

        expectedToken := generateCSRFToken(sessionID)
        if !secureCompare(formToken, expectedToken) {
            return c.Status(403).SendString("Invalid CSRF token")
        }

        return c.Next()
    }
}

func generateCSRFToken(sessionID string) string {
    // HMAC-SHA256 of session ID with APP_SECRET
    h := hmac.New(sha256.New, []byte(os.Getenv("APP_SECRET")))
    h.Write([]byte(sessionID))
    return hex.EncodeToString(h.Sum(nil))
}
```

---

## Update Endpoint Specification

### DynDNS2 Protocol Implementation

**Endpoint:** `GET /nic/update`

**Authentication:** HTTP Basic Auth
- Username: `ddns` (fixed value, ignored)
- Password: Update token for the hostname

**Query Parameters:**
| Parameter | Required | Description |
|-----------|----------|-------------|
| `hostname` | Yes | FQDN to update |
| `myip` | No | IP address (defaults to request source IP) |

**Response Format:** Plain text, one line

| Response | Meaning |
|----------|---------|
| `good {ip}` | Update successful, IP changed to {ip} |
| `nochg {ip}` | No change, IP already set to {ip} |
| `nohost` | Hostname not found or not configured |
| `badauth` | Invalid token |
| `abuse` | Rate limit exceeded |
| `911` | Server error |

**Handler Implementation:**
```go
func (h *UpdateHandler) HandleUpdate(c *fiber.Ctx) error {
    // 1. Parse Basic Auth
    auth := c.Get("Authorization")
    if !strings.HasPrefix(auth, "Basic ") {
        return c.Status(401).SendString("badauth")
    }

    decoded, err := base64.StdEncoding.DecodeString(auth[6:])
    if err != nil {
        return c.Status(401).SendString("badauth")
    }

    parts := strings.SplitN(string(decoded), ":", 2)
    if len(parts) != 2 {
        return c.Status(401).SendString("badauth")
    }
    token := parts[1] // Password is the token

    // 2. Get hostname
    hostname := c.Query("hostname")
    if hostname == "" {
        return c.SendString("nohost")
    }

    // 3. Look up DDNS record
    record, err := h.ddnsStore.GetByHostname(c.Context(), hostname)
    if err != nil || record == nil {
        return c.SendString("nohost")
    }

    // 4. Check rate limit
    if h.rateLimiter.IsLimited(hostname) {
        return c.SendString("abuse")
    }

    // 5. Verify token
    if err := bcrypt.CompareHashAndPassword(
        []byte(record.UpdateTokenHash),
        []byte(token),
    ); err != nil {
        h.rateLimiter.RecordFailure(hostname)
        return c.SendString("badauth")
    }

    // 6. Get IP address
    ip := c.Query("myip")
    if ip == "" {
        ip = c.IP()
    }

    // 7. Validate IP format
    if net.ParseIP(ip) == nil {
        return c.SendString("911")
    }

    // 8. Check if IP changed
    if record.CurrentIP == ip {
        return c.SendString("nochg " + ip)
    }

    // 9. Update Route 53
    recordType := "A"
    if strings.Contains(ip, ":") {
        recordType = "AAAA"
    }

    err = h.route53Client.UpsertRecord(
        c.Context(),
        record.ZoneID,
        hostname,
        recordType,
        ip,
        record.TTL,
    )
    if err != nil {
        h.logger.Error("Route 53 update failed", "error", err)
        return c.SendString("911")
    }

    // 10. Update database
    record.CurrentIP = ip
    record.LastUpdated = time.Now()
    h.ddnsStore.Update(c.Context(), record)

    // 11. Log update
    h.ddnsStore.LogUpdate(c.Context(), DDNSUpdateLog{
        Hostname:   hostname,
        PreviousIP: record.CurrentIP,
        NewIP:      ip,
        SourceIP:   c.IP(),
        UserAgent:  c.Get("User-Agent"),
        Status:     "success",
    })

    return c.SendString("good " + ip)
}
```

---

## Complete Test Cases

### Unit Tests

#### Auth Handler Tests (`internal/api/handlers/auth_test.go`)

```go
func TestLoginHandler(t *testing.T) {
    tests := []struct {
        name           string
        username       string
        password       string
        csrfToken      string
        setupMock      func(*mocks.MockSessionStore)
        expectedStatus int
        expectedBody   string
        expectCookie   bool
    }{
        {
            name:           "successful login",
            username:       "admin",
            password:       "correct-password",
            csrfToken:      "valid-csrf",
            expectedStatus: 302, // Redirect
            expectCookie:   true,
        },
        {
            name:           "invalid password",
            username:       "admin",
            password:       "wrong-password",
            csrfToken:      "valid-csrf",
            expectedStatus: 200, // Re-render form
            expectedBody:   "Invalid credentials",
            expectCookie:   false,
        },
        {
            name:           "missing username",
            username:       "",
            password:       "password",
            csrfToken:      "valid-csrf",
            expectedStatus: 200,
            expectedBody:   "Username is required",
            expectCookie:   false,
        },
        {
            name:           "missing password",
            username:       "admin",
            password:       "",
            csrfToken:      "valid-csrf",
            expectedStatus: 200,
            expectedBody:   "Password is required",
            expectCookie:   false,
        },
        {
            name:           "invalid CSRF token",
            username:       "admin",
            password:       "correct-password",
            csrfToken:      "invalid-csrf",
            expectedStatus: 403,
            expectedBody:   "Invalid CSRF token",
            expectCookie:   false,
        },
        {
            name:           "account locked",
            username:       "admin",
            password:       "correct-password",
            csrfToken:      "valid-csrf",
            setupMock: func(m *mocks.MockSessionStore) {
                m.SetLockedUntil(time.Now().Add(10 * time.Minute))
            },
            expectedStatus: 200,
            expectedBody:   "Account locked",
            expectCookie:   false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup and execute test
        })
    }
}
```

#### DDNS Handler Tests (`internal/api/handlers/ddns_test.go`)

```go
func TestCreateDDNSRecord(t *testing.T) {
    tests := []struct {
        name           string
        zoneID         string
        hostname       string
        ttl            string
        expectedStatus int
        expectedError  string
        expectToken    bool
    }{
        {
            name:           "successful creation",
            zoneID:         "Z123456789",
            hostname:       "home",
            ttl:            "60",
            expectedStatus: 200,
            expectToken:    true,
        },
        {
            name:           "missing zone",
            zoneID:         "",
            hostname:       "home",
            ttl:            "60",
            expectedStatus: 200,
            expectedError:  "Please select a zone",
        },
        {
            name:           "invalid hostname - too long",
            zoneID:         "Z123456789",
            hostname:       strings.Repeat("a", 64),
            ttl:            "60",
            expectedStatus: 200,
            expectedError:  "Hostname must be 1-63 characters",
        },
        {
            name:           "invalid hostname - bad characters",
            zoneID:         "Z123456789",
            hostname:       "home_server",
            ttl:            "60",
            expectedStatus: 200,
            expectedError:  "Invalid hostname format",
        },
        {
            name:           "invalid TTL - too low",
            zoneID:         "Z123456789",
            hostname:       "home",
            ttl:            "30",
            expectedStatus: 200,
            expectedError:  "TTL must be between 60 and 86400",
        },
        {
            name:           "duplicate hostname",
            zoneID:         "Z123456789",
            hostname:       "existing",
            ttl:            "60",
            expectedStatus: 200,
            expectedError:  "Hostname already exists",
        },
    }
}

func TestUpdateDDNSRecord(t *testing.T) {
    tests := []struct {
        name           string
        hostname       string
        enabled        string
        expectedStatus int
    }{
        {
            name:           "enable record",
            hostname:       "home.example.com",
            enabled:        "true",
            expectedStatus: 200,
        },
        {
            name:           "disable record",
            hostname:       "home.example.com",
            enabled:        "false",
            expectedStatus: 200,
        },
        {
            name:           "non-existent record",
            hostname:       "notfound.example.com",
            enabled:        "true",
            expectedStatus: 404,
        },
    }
}

func TestDeleteDDNSRecord(t *testing.T) {
    tests := []struct {
        name           string
        hostname       string
        expectedStatus int
        expectDNSDelete bool
    }{
        {
            name:            "successful deletion",
            hostname:        "home.example.com",
            expectedStatus:  302, // Redirect to list
            expectDNSDelete: true,
        },
        {
            name:            "non-existent record",
            hostname:        "notfound.example.com",
            expectedStatus:  404,
            expectDNSDelete: false,
        },
    }
}

func TestRegenerateToken(t *testing.T) {
    tests := []struct {
        name           string
        hostname       string
        expectedStatus int
        expectNewToken bool
    }{
        {
            name:           "successful regeneration",
            hostname:       "home.example.com",
            expectedStatus: 200,
            expectNewToken: true,
        },
        {
            name:           "non-existent record",
            hostname:       "notfound.example.com",
            expectedStatus: 404,
            expectNewToken: false,
        },
    }
}
```

#### Update Endpoint Tests (`internal/api/handlers/update_test.go`)

```go
func TestNicUpdate(t *testing.T) {
    tests := []struct {
        name           string
        hostname       string
        myip           string
        authHeader     string
        setupMock      func(*mocks.MockDDNSStore, *mocks.MockRoute53Client)
        expectedStatus int
        expectedBody   string
    }{
        {
            name:           "successful update - IP changed",
            hostname:       "home.example.com",
            myip:           "1.2.3.4",
            authHeader:     basicAuth("ddns", "valid-token"),
            expectedStatus: 200,
            expectedBody:   "good 1.2.3.4",
        },
        {
            name:           "no change - same IP",
            hostname:       "home.example.com",
            myip:           "1.2.3.4",
            authHeader:     basicAuth("ddns", "valid-token"),
            setupMock: func(ddns *mocks.MockDDNSStore, r53 *mocks.MockRoute53Client) {
                ddns.SetCurrentIP("home.example.com", "1.2.3.4")
            },
            expectedStatus: 200,
            expectedBody:   "nochg 1.2.3.4",
        },
        {
            name:           "missing hostname",
            hostname:       "",
            myip:           "1.2.3.4",
            authHeader:     basicAuth("ddns", "valid-token"),
            expectedStatus: 200,
            expectedBody:   "nohost",
        },
        {
            name:           "invalid hostname",
            hostname:       "notfound.example.com",
            myip:           "1.2.3.4",
            authHeader:     basicAuth("ddns", "valid-token"),
            expectedStatus: 200,
            expectedBody:   "nohost",
        },
        {
            name:           "missing auth header",
            hostname:       "home.example.com",
            myip:           "1.2.3.4",
            authHeader:     "",
            expectedStatus: 401,
            expectedBody:   "badauth",
        },
        {
            name:           "invalid token",
            hostname:       "home.example.com",
            myip:           "1.2.3.4",
            authHeader:     basicAuth("ddns", "wrong-token"),
            expectedStatus: 200,
            expectedBody:   "badauth",
        },
        {
            name:           "rate limited",
            hostname:       "home.example.com",
            myip:           "1.2.3.4",
            authHeader:     basicAuth("ddns", "valid-token"),
            setupMock: func(ddns *mocks.MockDDNSStore, r53 *mocks.MockRoute53Client) {
                // Simulate 60+ requests in past hour
            },
            expectedStatus: 200,
            expectedBody:   "abuse",
        },
        {
            name:           "IPv6 address",
            hostname:       "home.example.com",
            myip:           "2001:db8::1",
            authHeader:     basicAuth("ddns", "valid-token"),
            expectedStatus: 200,
            expectedBody:   "good 2001:db8::1",
        },
        {
            name:           "IP from request when myip empty",
            hostname:       "home.example.com",
            myip:           "",
            authHeader:     basicAuth("ddns", "valid-token"),
            expectedStatus: 200,
            expectedBody:   "good 10.0.0.1", // Request source IP
        },
        {
            name:           "Route 53 error",
            hostname:       "home.example.com",
            myip:           "1.2.3.4",
            authHeader:     basicAuth("ddns", "valid-token"),
            setupMock: func(ddns *mocks.MockDDNSStore, r53 *mocks.MockRoute53Client) {
                r53.SetError(errors.New("API error"))
            },
            expectedStatus: 200,
            expectedBody:   "911",
        },
        {
            name:           "disabled record",
            hostname:       "disabled.example.com",
            myip:           "1.2.3.4",
            authHeader:     basicAuth("ddns", "valid-token"),
            setupMock: func(ddns *mocks.MockDDNSStore, r53 *mocks.MockRoute53Client) {
                ddns.SetEnabled("disabled.example.com", false)
            },
            expectedStatus: 200,
            expectedBody:   "nohost",
        },
    }
}
```

### Integration Tests

#### Full Login Flow (`tests/integration/auth_test.go`)

```go
func TestLoginFlow(t *testing.T) {
    app := setupTestApp(t)

    // 1. GET /login - should return login form
    resp := app.Test(httptest.NewRequest("GET", "/login", nil))
    assert.Equal(t, 200, resp.StatusCode)
    body := readBody(resp)
    assert.Contains(t, body, `name="username"`)
    assert.Contains(t, body, `name="password"`)
    assert.Contains(t, body, `name="_csrf"`)

    // Extract CSRF token from form
    csrfToken := extractCSRFToken(body)

    // 2. POST /login with valid credentials
    form := url.Values{}
    form.Set("username", "admin")
    form.Set("password", "test-password")
    form.Set("_csrf", csrfToken)

    req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Cookie", resp.Header.Get("Set-Cookie")) // Session cookie for CSRF

    resp = app.Test(req)
    assert.Equal(t, 302, resp.StatusCode)
    assert.Equal(t, "/zones", resp.Header.Get("Location"))

    // Verify session cookie is set
    cookies := resp.Header.Get("Set-Cookie")
    assert.Contains(t, cookies, "session_id=")
    assert.Contains(t, cookies, "HttpOnly")
    assert.Contains(t, cookies, "Secure")

    // 3. Access protected route with session
    req = httptest.NewRequest("GET", "/zones", nil)
    req.Header.Set("Cookie", cookies)
    resp = app.Test(req)
    assert.Equal(t, 200, resp.StatusCode)

    // 4. POST /logout
    logoutForm := url.Values{}
    logoutForm.Set("_csrf", extractCSRFToken(readBody(resp)))

    req = httptest.NewRequest("POST", "/logout", strings.NewReader(logoutForm.Encode()))
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    req.Header.Set("Cookie", cookies)

    resp = app.Test(req)
    assert.Equal(t, 302, resp.StatusCode)
    assert.Equal(t, "/login", resp.Header.Get("Location"))

    // 5. Verify session is invalidated
    req = httptest.NewRequest("GET", "/zones", nil)
    req.Header.Set("Cookie", cookies)
    resp = app.Test(req)
    assert.Equal(t, 302, resp.StatusCode) // Redirected to login
}
```

#### DDNS CRUD Flow (`tests/integration/ddns_test.go`)

```go
func TestDDNSCRUDFlow(t *testing.T) {
    app := setupTestApp(t)
    cookies := loginAsAdmin(t, app)

    // 1. List DDNS (empty)
    resp := getWithSession(app, "/ddns", cookies)
    assert.Equal(t, 200, resp.StatusCode)
    assert.Contains(t, readBody(resp), "No DDNS records configured")

    // 2. GET /ddns/new
    resp = getWithSession(app, "/ddns/new?zone=Z123456789", cookies)
    assert.Equal(t, 200, resp.StatusCode)
    body := readBody(resp)
    assert.Contains(t, body, `name="zone_id"`)
    assert.Contains(t, body, `selected`) // Zone should be pre-selected

    // 3. POST /ddns - create record
    csrf := extractCSRFToken(body)
    form := url.Values{}
    form.Set("zone_id", "Z123456789")
    form.Set("hostname", "test")
    form.Set("ttl", "60")
    form.Set("_csrf", csrf)

    resp = postWithSession(app, "/ddns", form, cookies)
    assert.Equal(t, 200, resp.StatusCode)
    body = readBody(resp)

    // Verify token is shown
    assert.Contains(t, body, "Save this token now")
    token := extractToken(body)
    assert.NotEmpty(t, token)

    // 4. GET /ddns/test.example.com - view detail
    resp = getWithSession(app, "/ddns/test.example.com", cookies)
    assert.Equal(t, 200, resp.StatusCode)
    body = readBody(resp)
    assert.Contains(t, body, "test.example.com")
    assert.Contains(t, body, "60 seconds")
    assert.Contains(t, body, "Enabled")

    // 5. PUT /ddns/test.example.com - disable
    csrf = extractCSRFToken(body)
    form = url.Values{}
    form.Set("enabled", "false")
    form.Set("_method", "PUT")
    form.Set("_csrf", csrf)

    resp = postWithSession(app, "/ddns/test.example.com", form, cookies)
    assert.Equal(t, 200, resp.StatusCode)
    assert.Contains(t, readBody(resp), "Disabled")

    // 6. POST /ddns/test.example.com/regenerate-token
    resp = getWithSession(app, "/ddns/test.example.com", cookies)
    csrf = extractCSRFToken(readBody(resp))
    form = url.Values{}
    form.Set("_csrf", csrf)

    resp = postWithSession(app, "/ddns/test.example.com/regenerate-token", form, cookies)
    assert.Equal(t, 200, resp.StatusCode)
    body = readBody(resp)
    newToken := extractToken(body)
    assert.NotEqual(t, token, newToken)

    // 7. DELETE /ddns/test.example.com
    csrf = extractCSRFToken(body)
    form = url.Values{}
    form.Set("_method", "DELETE")
    form.Set("_csrf", csrf)

    resp = postWithSession(app, "/ddns/test.example.com", form, cookies)
    assert.Equal(t, 302, resp.StatusCode)
    assert.Equal(t, "/ddns", resp.Header.Get("Location"))

    // 8. Verify deletion
    resp = getWithSession(app, "/ddns/test.example.com", cookies)
    assert.Equal(t, 404, resp.StatusCode)
}
```

#### Update Endpoint E2E (`tests/integration/update_test.go`)

```go
func TestUpdateEndpointE2E(t *testing.T) {
    app := setupTestApp(t)

    // Setup: Create a DDNS record
    cookies := loginAsAdmin(t, app)
    token := createDDNSRecord(t, app, cookies, "e2e-test", "Z123456789")

    // 1. First update - should succeed with "good"
    req := httptest.NewRequest("GET", "/nic/update?hostname=e2e-test.example.com&myip=1.2.3.4", nil)
    req.Header.Set("Authorization", basicAuth("ddns", token))

    resp := app.Test(req)
    assert.Equal(t, 200, resp.StatusCode)
    assert.Equal(t, "good 1.2.3.4", strings.TrimSpace(readBody(resp)))

    // 2. Same IP again - should return "nochg"
    resp = app.Test(req)
    assert.Equal(t, 200, resp.StatusCode)
    assert.Equal(t, "nochg 1.2.3.4", strings.TrimSpace(readBody(resp)))

    // 3. New IP - should return "good"
    req = httptest.NewRequest("GET", "/nic/update?hostname=e2e-test.example.com&myip=5.6.7.8", nil)
    req.Header.Set("Authorization", basicAuth("ddns", token))

    resp = app.Test(req)
    assert.Equal(t, 200, resp.StatusCode)
    assert.Equal(t, "good 5.6.7.8", strings.TrimSpace(readBody(resp)))

    // 4. Verify Route 53 was updated
    record := getRoute53Record(t, "Z123456789", "e2e-test.example.com", "A")
    assert.Equal(t, "5.6.7.8", record.Value)
    assert.Equal(t, 60, record.TTL)

    // 5. Verify history was logged
    resp = getWithSession(app, "/ddns/e2e-test.example.com/history", cookies)
    body := readBody(resp)
    assert.Contains(t, body, "1.2.3.4")
    assert.Contains(t, body, "5.6.7.8")
    assert.Contains(t, body, "success")
}
```

### E2E Test Checklist

| Test Case | Description | Steps | Expected Result |
|-----------|-------------|-------|-----------------|
| LOGIN-001 | Successful admin login | Enter valid credentials | Redirect to /zones, session cookie set |
| LOGIN-002 | Invalid password | Enter wrong password | Show "Invalid credentials", no cookie |
| LOGIN-003 | Account lockout | 5 failed attempts | Show lockout message, disable form |
| LOGIN-004 | Lockout expiry | Wait 15 minutes after lockout | Login succeeds |
| LOGIN-005 | Session expiry | Wait 24 hours | Redirect to login on next request |
| LOGIN-006 | CSRF validation | Submit without CSRF token | 403 error |
| ZONE-001 | List zones | Login, navigate to /zones | Show all Route 53 zones |
| ZONE-002 | View zone detail | Click zone name | Show zone info and records |
| ZONE-003 | Empty zones | No zones in account | Show "No hosted zones found" |
| DDNS-001 | Create record | Fill form, submit | Show token once, record appears in list |
| DDNS-002 | Duplicate hostname | Create same hostname twice | Show "Hostname already exists" |
| DDNS-003 | Invalid hostname | Enter "home_server" | Show "Invalid hostname format" |
| DDNS-004 | View record | Click hostname in list | Show all details, recent history |
| DDNS-005 | Disable record | Click disable button | Status changes to "Disabled" |
| DDNS-006 | Enable record | Click enable button | Status changes to "Enabled" |
| DDNS-007 | Regenerate token | Click regenerate | Show new token, old token invalid |
| DDNS-008 | Delete record | Click delete, confirm | Record removed, DNS record deleted |
| DDNS-009 | View history | Click history link | Show paginated update log |
| UPDATE-001 | Valid update | Send /nic/update with valid token | Return "good {ip}", update DNS |
| UPDATE-002 | No change | Send same IP twice | Return "nochg {ip}" |
| UPDATE-003 | Invalid token | Send wrong token | Return "badauth" |
| UPDATE-004 | Unknown hostname | Send non-existent hostname | Return "nohost" |
| UPDATE-005 | Disabled record | Update disabled record | Return "nohost" |
| UPDATE-006 | Rate limit | Send 61 requests in 1 hour | Return "abuse" |
| UPDATE-007 | IPv6 update | Send IPv6 address | Return "good {ip}", create AAAA record |
| UPDATE-008 | Auto-detect IP | Omit myip parameter | Use request source IP |
| SEC-001 | Protected routes | Access /zones without login | Redirect to /login |
| SEC-002 | XSS prevention | Enter script in hostname | Script is escaped in output |
| SEC-003 | SQL injection | Enter SQL in fields | No database error, input rejected |

---

## Complete SAM Template (`template.yaml`)

```yaml
AWSTemplateFormatVersion: '2010-09-09'
Transform: AWS::Serverless-2016-10-31
Description: Dynamic DNS Management System with Route 53

Parameters:
  AdminUsername:
    Type: String
    Default: admin
    Description: Admin username for dashboard login
    MinLength: 1
    MaxLength: 50

  AdminPassword:
    Type: String
    NoEcho: true
    Description: Admin password for dashboard login (bcrypt hashed at runtime)
    MinLength: 8
    MaxLength: 100

  AppSecret:
    Type: String
    NoEcho: true
    Description: 32-byte secret for session signing (hex encoded)
    MinLength: 64
    MaxLength: 64

  DomainName:
    Type: String
    Default: DISABLED
    Description: Custom domain name (e.g., ddns.example.com) or DISABLED

  HostedZoneId:
    Type: String
    Default: DISABLED
    Description: Route 53 Hosted Zone ID for custom domain or DISABLED

  CloudFrontCertificateArn:
    Type: String
    Default: DISABLED
    Description: ACM certificate ARN in us-east-1 for CloudFront or DISABLED

Conditions:
  UseCustomDomain: !Not [!Equals [!Ref DomainName, DISABLED]]
  UseCloudFront: !And
    - !Not [!Equals [!Ref DomainName, DISABLED]]
    - !Not [!Equals [!Ref CloudFrontCertificateArn, DISABLED]]

Globals:
  Function:
    Timeout: 60
    MemorySize: 1024
    Runtime: provided.al2023
    Architectures:
      - arm64
    Environment:
      Variables:
        DYNAMODB_TABLE: !Ref DynamoDBTable
        ADMIN_USERNAME: !Ref AdminUsername
        ADMIN_PASSWORD: !Ref AdminPassword
        APP_SECRET: !Ref AppSecret

Resources:
  # DynamoDB Table (Single Table Design)
  DynamoDBTable:
    Type: AWS::DynamoDB::Table
    Properties:
      TableName: !Sub "${AWS::StackName}-data"
      BillingMode: PAY_PER_REQUEST
      AttributeDefinitions:
        - AttributeName: PK
          AttributeType: S
        - AttributeName: SK
          AttributeType: S
      KeySchema:
        - AttributeName: PK
          KeyType: HASH
        - AttributeName: SK
          KeyType: RANGE
      TimeToLiveSpecification:
        AttributeName: ttl
        Enabled: true
      Tags:
        - Key: Application
          Value: DynamicDNS

  # Lambda Function
  DDNSFunction:
    Type: AWS::Serverless::Function
    Metadata:
      BuildMethod: go1.x
    Properties:
      FunctionName: !Sub "${AWS::StackName}-handler"
      CodeUri: .
      Handler: bootstrap
      Events:
        Api:
          Type: HttpApi
          Properties:
            ApiId: !Ref HttpApi
            Path: /{proxy+}
            Method: ANY
        Root:
          Type: HttpApi
          Properties:
            ApiId: !Ref HttpApi
            Path: /
            Method: ANY
      Policies:
        - DynamoDBCrudPolicy:
            TableName: !Ref DynamoDBTable
        - Version: '2012-10-17'
          Statement:
            - Effect: Allow
              Action:
                - route53:ListHostedZones
                - route53:GetHostedZone
                - route53:ListResourceRecordSets
                - route53:ChangeResourceRecordSets
              Resource: "*"

  # HTTP API (API Gateway v2)
  HttpApi:
    Type: AWS::Serverless::HttpApi
    Properties:
      StageName: prod
      DisableExecuteApiEndpoint: !If [UseCustomDomain, true, false]
      CorsConfiguration:
        AllowOrigins:
          - !If [UseCustomDomain, !Sub "https://${DomainName}", !Sub "https://${HttpApi}.execute-api.${AWS::Region}.amazonaws.com"]
        AllowMethods:
          - GET
          - POST
          - PUT
          - DELETE
        AllowHeaders:
          - Content-Type
          - Authorization
          - X-CSRF-Token
        AllowCredentials: true

  # CloudFront Distribution (conditional)
  CloudFrontDistribution:
    Type: AWS::CloudFront::Distribution
    Condition: UseCloudFront
    Properties:
      DistributionConfig:
        Enabled: true
        Comment: !Sub "Dynamic DNS - ${DomainName}"
        Aliases:
          - !Ref DomainName
        DefaultCacheBehavior:
          TargetOriginId: ApiGateway
          ViewerProtocolPolicy: redirect-to-https
          AllowedMethods:
            - GET
            - HEAD
            - OPTIONS
            - PUT
            - POST
            - PATCH
            - DELETE
          CachedMethods:
            - GET
            - HEAD
          CachePolicyId: 4135ea2d-6df8-44a3-9df3-4b5a84be39ad  # CachingDisabled
          OriginRequestPolicyId: b689b0a8-53d0-40ab-baf2-68738e2966ac  # AllViewerExceptHostHeader
          ResponseHeadersPolicyId: !Ref SecurityHeadersPolicy
        Origins:
          - Id: ApiGateway
            DomainName: !Sub "${HttpApi}.execute-api.${AWS::Region}.amazonaws.com"
            CustomOriginConfig:
              HTTPSPort: 443
              OriginProtocolPolicy: https-only
              OriginSSLProtocols:
                - TLSv1.2
        ViewerCertificate:
          AcmCertificateArn: !Ref CloudFrontCertificateArn
          SslSupportMethod: sni-only
          MinimumProtocolVersion: TLSv1.2_2021
        HttpVersion: http2and3
        PriceClass: PriceClass_100

  # Security Headers Policy
  SecurityHeadersPolicy:
    Type: AWS::CloudFront::ResponseHeadersPolicy
    Condition: UseCloudFront
    Properties:
      ResponseHeadersPolicyConfig:
        Name: !Sub "${AWS::StackName}-security-headers"
        SecurityHeadersConfig:
          StrictTransportSecurity:
            AccessControlMaxAgeSec: 31536000
            IncludeSubdomains: true
            Override: true
            Preload: true
          ContentTypeOptions:
            Override: true
          FrameOptions:
            FrameOption: DENY
            Override: true
          XSSProtection:
            ModeBlock: true
            Override: true
            Protection: true
          ReferrerPolicy:
            ReferrerPolicy: strict-origin-when-cross-origin
            Override: true

  # Route 53 Record for Custom Domain
  DNSRecord:
    Type: AWS::Route53::RecordSet
    Condition: UseCloudFront
    Properties:
      HostedZoneId: !Ref HostedZoneId
      Name: !Ref DomainName
      Type: A
      AliasTarget:
        DNSName: !GetAtt CloudFrontDistribution.DomainName
        HostedZoneId: Z2FDTNDATAQYW2  # CloudFront hosted zone ID (constant)

Outputs:
  ApiUrl:
    Description: API Gateway URL
    Value: !If
      - UseCustomDomain
      - !Sub "https://${DomainName}"
      - !Sub "https://${HttpApi}.execute-api.${AWS::Region}.amazonaws.com"

  ApiGatewayUrl:
    Description: Direct API Gateway URL (for testing)
    Value: !Sub "https://${HttpApi}.execute-api.${AWS::Region}.amazonaws.com"

  CloudFrontDomain:
    Condition: UseCloudFront
    Description: CloudFront distribution domain
    Value: !GetAtt CloudFrontDistribution.DomainName

  DynamoDBTableName:
    Description: DynamoDB table name
    Value: !Ref DynamoDBTable

  FunctionArn:
    Description: Lambda function ARN
    Value: !GetAtt DDNSFunction.Arn
```

---

## Complete GitHub Actions Workflow (`.github/workflows/deploy.yaml`)

```yaml
name: Deploy to AWS

on:
  push:
    branches:
      - main
  workflow_dispatch:
    inputs:
      environment:
        description: 'Deployment environment'
        required: true
        default: 'production'
        type: choice
        options:
          - production
          - staging

env:
  GO_VERSION: '1.21'
  SAM_CLI_TELEMETRY: 0

jobs:
  test:
    name: Test
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Download dependencies
        run: go mod download

      - name: Run tests
        run: go test -v -race -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: coverage.out
          fail_ci_if_error: false

  build:
    name: Build
    runs-on: ubuntu-latest
    needs: test
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true

      - name: Build for Lambda (ARM64)
        env:
          GOOS: linux
          GOARCH: arm64
          CGO_ENABLED: 0
        run: |
          go build -ldflags="-s -w" -o bootstrap ./cmd/lambda

      - name: Upload artifact
        uses: actions/upload-artifact@v4
        with:
          name: bootstrap
          path: bootstrap
          retention-days: 1

  deploy:
    name: Deploy
    runs-on: ubuntu-latest
    needs: build
    environment: production
    permissions:
      id-token: write
      contents: read
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Download artifact
        uses: actions/download-artifact@v4
        with:
          name: bootstrap

      - name: Make bootstrap executable
        run: chmod +x bootstrap

      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v4
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: ${{ vars.AWS_REGION }}

      - name: Set up SAM CLI
        uses: aws-actions/setup-sam@v2
        with:
          use-installer: true

      - name: Deploy with SAM
        run: |
          sam deploy \
            --template-file template.yaml \
            --stack-name dynamic-dns-stack \
            --capabilities CAPABILITY_IAM \
            --no-confirm-changeset \
            --no-fail-on-empty-changeset \
            --resolve-s3 \
            --parameter-overrides \
              AdminUsername=${{ vars.ADMIN_USER }} \
              AdminPassword=${{ secrets.ADMIN_PASSWORD }} \
              AppSecret=${{ secrets.APP_SECRET }} \
              DomainName=${{ vars.DOMAIN_NAME }} \
              HostedZoneId=${{ vars.HOSTED_ZONE_ID }} \
              CloudFrontCertificateArn=${{ vars.CLOUDFRONT_CERT_ARN }}

      - name: Get stack outputs
        id: outputs
        run: |
          API_URL=$(aws cloudformation describe-stacks \
            --stack-name dynamic-dns-stack \
            --query 'Stacks[0].Outputs[?OutputKey==`ApiUrl`].OutputValue' \
            --output text)
          echo "api_url=$API_URL" >> $GITHUB_OUTPUT

      - name: Smoke test
        run: |
          # Wait for deployment to propagate
          sleep 10

          # Test IP endpoint
          response=$(curl -s -o /dev/null -w "%{http_code}" "${{ steps.outputs.outputs.api_url }}/ip")
          if [ "$response" != "200" ]; then
            echo "Smoke test failed: /ip returned $response"
            exit 1
          fi

          # Test login page
          response=$(curl -s -o /dev/null -w "%{http_code}" "${{ steps.outputs.outputs.api_url }}/login")
          if [ "$response" != "200" ]; then
            echo "Smoke test failed: /login returned $response"
            exit 1
          fi

          echo "Smoke tests passed!"

      - name: Print deployment URL
        run: |
          echo "::notice::Deployed to ${{ steps.outputs.outputs.api_url }}"
```

---

## Complete Makefile

```makefile
.PHONY: all build test clean run dev lint fmt deps tidy sam-build sam-deploy sam-local

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
GOLINT=golangci-lint

# Binary name
BINARY_NAME=bootstrap
MAIN_PATH=./cmd/lambda

# Build flags
LDFLAGS=-ldflags="-s -w"

# AWS SAM parameters
SAM_TEMPLATE=template.yaml
SAM_STACK_NAME=dynamic-dns-stack

all: deps lint test build

# Build the binary for Lambda (ARM64 Linux)
build:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)

# Build for local development (current OS/arch)
build-local:
	$(GOBUILD) -o $(BINARY_NAME)-local $(MAIN_PATH)

# Run tests with coverage
test:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

# Run tests with verbose output
test-verbose:
	$(GOTEST) -v -race -count=1 ./...

# Run integration tests
test-integration:
	$(GOTEST) -v -race -tags=integration ./tests/integration/...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME)-local coverage.out
	rm -rf .aws-sam

# Download dependencies
deps:
	$(GOGET) -v ./...

# Tidy go.mod
tidy:
	$(GOMOD) tidy

# Format code
fmt:
	$(GOFMT) -s -w .

# Run linter
lint:
	$(GOLINT) run ./...

# Run locally with SAM
sam-local:
	sam local start-api --template $(SAM_TEMPLATE) --warm-containers EAGER

# Build with SAM
sam-build: build
	sam build --template $(SAM_TEMPLATE) --use-container=false

# Deploy with SAM
sam-deploy: build
	sam deploy \
		--template-file $(SAM_TEMPLATE) \
		--stack-name $(SAM_STACK_NAME) \
		--capabilities CAPABILITY_IAM \
		--resolve-s3

# Validate SAM template
sam-validate:
	sam validate --template $(SAM_TEMPLATE)

# Generate CSS with Tailwind (requires npm/tailwindcss)
css:
	npx tailwindcss -i ./web/static/css/input.css -o ./web/static/css/tailwind.css --minify

# Watch CSS for development
css-watch:
	npx tailwindcss -i ./web/static/css/input.css -o ./web/static/css/tailwind.css --watch

# Generate bcrypt hash for admin password
hash-password:
	@read -p "Enter password: " pwd; \
	go run ./scripts/hash_password.go "$$pwd"

# Generate random APP_SECRET
generate-secret:
	@openssl rand -hex 32

# Show stack outputs
stack-outputs:
	aws cloudformation describe-stacks \
		--stack-name $(SAM_STACK_NAME) \
		--query 'Stacks[0].Outputs' \
		--output table

# View Lambda logs
logs:
	sam logs --stack-name $(SAM_STACK_NAME) --tail

# Delete stack
destroy:
	aws cloudformation delete-stack --stack-name $(SAM_STACK_NAME)
	aws cloudformation wait stack-delete-complete --stack-name $(SAM_STACK_NAME)

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build Lambda binary (ARM64 Linux)"
	@echo "  build-local    - Build for local OS"
	@echo "  test           - Run tests with coverage"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  lint           - Run golangci-lint"
	@echo "  fmt            - Format Go code"
	@echo "  clean          - Remove build artifacts"
	@echo "  deps           - Download dependencies"
	@echo "  tidy           - Tidy go.mod"
	@echo "  sam-local      - Run locally with SAM"
	@echo "  sam-build      - Build with SAM"
	@echo "  sam-deploy     - Deploy to AWS"
	@echo "  sam-validate   - Validate SAM template"
	@echo "  css            - Build Tailwind CSS"
	@echo "  css-watch      - Watch Tailwind CSS"
	@echo "  hash-password  - Generate bcrypt hash"
	@echo "  generate-secret- Generate APP_SECRET"
	@echo "  stack-outputs  - Show CloudFormation outputs"
	@echo "  logs           - Tail Lambda logs"
	@echo "  destroy        - Delete CloudFormation stack"
```

---

## Tailwind Configuration (`tailwind.config.js`)

```javascript
/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./web/templates/**/*.html",
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#eff6ff',
          100: '#dbeafe',
          200: '#bfdbfe',
          300: '#93c5fd',
          400: '#60a5fa',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8',
          800: '#1e40af',
          900: '#1e3a8a',
        },
      },
    },
  },
  plugins: [
    require('@tailwindcss/forms'),
  ],
}
```

---

## Input CSS (`web/static/css/input.css`)

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer components {
  /* Form styles */
  .form-group {
    @apply mb-4;
  }

  .form-group label {
    @apply block text-sm font-medium text-gray-700 mb-1;
  }

  .form-group input[type="text"],
  .form-group input[type="password"],
  .form-group input[type="email"],
  .form-group input[type="number"],
  .form-group select {
    @apply block w-full rounded-md border-gray-300 shadow-sm
           focus:border-primary-500 focus:ring-primary-500 sm:text-sm;
  }

  .form-group small {
    @apply text-xs text-gray-500 mt-1 block;
  }

  .form-group .error {
    @apply text-sm text-red-600 mt-1;
  }

  /* Button styles */
  .btn {
    @apply inline-flex items-center px-4 py-2 border rounded-md shadow-sm
           text-sm font-medium focus:outline-none focus:ring-2
           focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed;
  }

  .btn-primary {
    @apply btn border-transparent text-white bg-primary-600
           hover:bg-primary-700 focus:ring-primary-500;
  }

  .btn-secondary {
    @apply btn border-gray-300 text-gray-700 bg-white
           hover:bg-gray-50 focus:ring-primary-500;
  }

  .btn-danger {
    @apply btn border-transparent text-white bg-red-600
           hover:bg-red-700 focus:ring-red-500;
  }

  /* Alert styles */
  .alert {
    @apply p-4 rounded-md mb-4;
  }

  .alert-success {
    @apply alert bg-green-50 text-green-800 border border-green-200;
  }

  .alert-error {
    @apply alert bg-red-50 text-red-800 border border-red-200;
  }

  .alert-warning {
    @apply alert bg-yellow-50 text-yellow-800 border border-yellow-200;
  }

  .alert-info {
    @apply alert bg-blue-50 text-blue-800 border border-blue-200;
  }

  /* Badge styles */
  .badge {
    @apply inline-flex items-center px-2.5 py-0.5 rounded-full
           text-xs font-medium;
  }

  .badge-success {
    @apply badge bg-green-100 text-green-800;
  }

  .badge-warning {
    @apply badge bg-yellow-100 text-yellow-800;
  }

  .badge-error {
    @apply badge bg-red-100 text-red-800;
  }

  .badge-info {
    @apply badge bg-blue-100 text-blue-800;
  }

  /* Card styles */
  .card {
    @apply bg-white shadow rounded-lg p-6 mb-6;
  }

  .card h2 {
    @apply text-lg font-medium text-gray-900 mb-4;
  }

  /* Table styles */
  table {
    @apply min-w-full divide-y divide-gray-200;
  }

  thead {
    @apply bg-gray-50;
  }

  th {
    @apply px-6 py-3 text-left text-xs font-medium text-gray-500
           uppercase tracking-wider;
  }

  td {
    @apply px-6 py-4 whitespace-nowrap text-sm text-gray-900;
  }

  tbody tr:nth-child(even) {
    @apply bg-gray-50;
  }

  /* Navigation */
  .nav {
    @apply bg-white shadow;
  }

  .nav-link {
    @apply px-3 py-2 rounded-md text-sm font-medium text-gray-700
           hover:text-gray-900 hover:bg-gray-100;
  }

  .nav-link.active {
    @apply bg-primary-100 text-primary-700;
  }
}
```

---

## go.mod Specification

```go
module github.com/yourusername/dynamic-route-53-dns

go 1.21

require (
    github.com/aws/aws-lambda-go v1.47.0
    github.com/aws/aws-sdk-go-v2 v1.26.0
    github.com/aws/aws-sdk-go-v2/config v1.27.0
    github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue v1.13.0
    github.com/aws/aws-sdk-go-v2/service/dynamodb v1.31.0
    github.com/aws/aws-sdk-go-v2/service/route53 v1.40.0
    github.com/awslabs/aws-lambda-go-api-proxy v0.16.0
    github.com/gofiber/fiber/v2 v2.52.0
    github.com/gofiber/template/html/v2 v2.1.0
    golang.org/x/crypto v0.21.0
)
```

---

## Base Layout Template (`web/templates/layouts/base.html`)

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ .PageTitle }}</title>
    <link rel="stylesheet" href="/static/css/tailwind.css">
    <script src="https://unpkg.com/htmx.org@1.9.10"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    {{ if .IsLoggedIn }}
    <nav class="nav">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div class="flex justify-between h-16">
                <div class="flex">
                    <div class="flex-shrink-0 flex items-center">
                        <span class="text-xl font-bold text-primary-600">Dynamic DNS</span>
                    </div>
                    <div class="hidden sm:ml-6 sm:flex sm:space-x-4">
                        <a href="/zones" class="nav-link {{ if eq .CurrentPath "/zones" }}active{{ end }}">
                            Zones
                        </a>
                        <a href="/ddns" class="nav-link {{ if hasPrefix .CurrentPath "/ddns" }}active{{ end }}">
                            DDNS Records
                        </a>
                    </div>
                </div>
                <div class="flex items-center">
                    <span class="text-sm text-gray-500 mr-4">{{ .Username }}</span>
                    <form method="POST" action="/logout" class="inline">
                        <input type="hidden" name="_csrf" value="{{ .CSRFToken }}">
                        <button type="submit" class="btn-secondary text-sm">Logout</button>
                    </form>
                </div>
            </div>
        </div>
    </nav>
    {{ end }}

    <main class="max-w-7xl mx-auto py-6 px-4 sm:px-6 lg:px-8">
        {{ if .FlashSuccess }}
        <div class="alert-success">{{ .FlashSuccess }}</div>
        {{ end }}

        {{ if .FlashError }}
        <div class="alert-error">{{ .FlashError }}</div>
        {{ end }}

        {{ template "content" . }}
    </main>

    <footer class="mt-8 py-4 text-center text-sm text-gray-500">
        Dynamic DNS Management System
    </footer>
</body>
</html>
```

---

## Dual-Mode Execution (Local + Lambda)

The application must run in two modes:
1. **Local mode**: Standard HTTP server for development and testing
2. **Lambda mode**: AWS Lambda handler for production

### Project Structure Update

```
dynamic-route-53-dns/
├── cmd/
│   ├── lambda/main.go      # Lambda entry point
│   └── local/main.go       # Local server entry point
├── internal/
│   ├── app/
│   │   └── app.go          # Shared Fiber app configuration
│   └── ...
```

### Shared App Configuration (`internal/app/app.go`)

```go
package app

import (
    "embed"
    "io/fs"
    "os"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/logger"
    "github.com/gofiber/fiber/v2/middleware/recover"
    "github.com/gofiber/template/html/v2"

    "github.com/yourusername/dynamic-route-53-dns/internal/api/handlers"
    "github.com/yourusername/dynamic-route-53-dns/internal/api/middleware"
    "github.com/yourusername/dynamic-route-53-dns/internal/auth"
    "github.com/yourusername/dynamic-route-53-dns/internal/database"
    "github.com/yourusername/dynamic-route-53-dns/internal/route53"
)

//go:embed ../web/templates/*
var templatesFS embed.FS

//go:embed ../web/static/*
var staticFS embed.FS

// Config holds application configuration
type Config struct {
    // Database
    DynamoDBTable    string
    DynamoDBEndpoint string // Empty for AWS, "http://localhost:8000" for local

    // Auth
    AdminUsername string
    AdminPassword string
    AppSecret     string

    // AWS
    AWSRegion     string
    AWSEndpoint   string // Empty for AWS, custom for LocalStack

    // Server (local mode only)
    Port string

    // Mode
    IsLocal bool
}

// ConfigFromEnv creates Config from environment variables
func ConfigFromEnv() *Config {
    isLocal := os.Getenv("LOCAL_MODE") == "true"

    cfg := &Config{
        DynamoDBTable:    getEnv("DYNAMODB_TABLE", "dynamic-dns-local"),
        DynamoDBEndpoint: os.Getenv("DYNAMODB_ENDPOINT"), // Empty string uses AWS
        AdminUsername:    getEnv("ADMIN_USERNAME", "admin"),
        AdminPassword:    os.Getenv("ADMIN_PASSWORD"),
        AppSecret:        os.Getenv("APP_SECRET"),
        AWSRegion:        getEnv("AWS_REGION", "us-east-1"),
        AWSEndpoint:      os.Getenv("AWS_ENDPOINT"),
        Port:             getEnv("PORT", "3000"),
        IsLocal:          isLocal,
    }

    // Local mode defaults
    if isLocal {
        if cfg.DynamoDBEndpoint == "" {
            cfg.DynamoDBEndpoint = "http://localhost:8000"
        }
        if cfg.AdminPassword == "" {
            cfg.AdminPassword = "localdev123" // Default for local only
        }
        if cfg.AppSecret == "" {
            cfg.AppSecret = "0000000000000000000000000000000000000000000000000000000000000000"
        }
    }

    return cfg
}

func getEnv(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

// New creates a new Fiber application with all routes configured
func New(cfg *Config) (*fiber.App, error) {
    // Setup template engine
    templates, err := fs.Sub(templatesFS, "web/templates")
    if err != nil {
        return nil, err
    }
    engine := html.NewFileSystem(http.FS(templates), ".html")

    // Add template functions
    engine.AddFunc("hasPrefix", strings.HasPrefix)
    engine.AddFunc("timeAgo", formatTimeAgo)

    // Create Fiber app
    app := fiber.New(fiber.Config{
        Views:       engine,
        ViewsLayout: "layouts/base",
        ErrorHandler: customErrorHandler,
    })

    // Middleware
    app.Use(recover.New())
    if cfg.IsLocal {
        app.Use(logger.New(logger.Config{
            Format: "${time} ${method} ${path} ${status} ${latency}\n",
        }))
    }

    // Static files
    staticFiles, _ := fs.Sub(staticFS, "web/static")
    app.Use("/static", filesystem.New(filesystem.Config{
        Root: http.FS(staticFiles),
    }))

    // Initialize services
    db, err := database.New(cfg.DynamoDBTable, cfg.DynamoDBEndpoint, cfg.AWSRegion)
    if err != nil {
        return nil, err
    }

    var r53Client route53.Client
    if cfg.IsLocal {
        // Use mock Route 53 client for local development
        r53Client = route53.NewMockClient()
    } else {
        r53Client, err = route53.New(cfg.AWSRegion)
        if err != nil {
            return nil, err
        }
    }

    sessionStore := auth.NewSessionStore(db)
    authService := auth.NewService(cfg.AdminUsername, cfg.AdminPassword, cfg.AppSecret, sessionStore)

    // Initialize handlers
    authHandler := handlers.NewAuthHandler(authService)
    zoneHandler := handlers.NewZoneHandler(r53Client)
    ddnsHandler := handlers.NewDDNSHandler(db, r53Client)
    updateHandler := handlers.NewUpdateHandler(db, r53Client)

    // Auth middleware
    authMiddleware := middleware.NewAuthMiddleware(sessionStore)
    csrfMiddleware := middleware.NewCSRFMiddleware(cfg.AppSecret)

    // Public routes
    app.Get("/", func(c *fiber.Ctx) error {
        return c.Redirect("/login")
    })
    app.Get("/ip", handlers.HandleIP)
    app.Get("/nic/update", updateHandler.HandleUpdate)

    // Auth routes
    app.Get("/login", authHandler.ShowLogin)
    app.Post("/login", csrfMiddleware, authHandler.HandleLogin)
    app.Post("/logout", authMiddleware, csrfMiddleware, authHandler.HandleLogout)

    // Protected routes
    protected := app.Group("", authMiddleware, csrfMiddleware)

    // Zones
    protected.Get("/zones", zoneHandler.ListZones)
    protected.Get("/zones/:zoneId", zoneHandler.ShowZone)

    // DDNS
    protected.Get("/ddns", ddnsHandler.ListRecords)
    protected.Get("/ddns/new", ddnsHandler.ShowCreateForm)
    protected.Post("/ddns", ddnsHandler.CreateRecord)
    protected.Get("/ddns/:hostname", ddnsHandler.ShowRecord)
    protected.Put("/ddns/:hostname", ddnsHandler.UpdateRecord)
    protected.Post("/ddns/:hostname", ddnsHandler.HandleMethodOverride) // For HTML forms
    protected.Delete("/ddns/:hostname", ddnsHandler.DeleteRecord)
    protected.Post("/ddns/:hostname/regenerate-token", ddnsHandler.RegenerateToken)
    protected.Get("/ddns/:hostname/history", ddnsHandler.ShowHistory)

    return app, nil
}
```

### Lambda Entry Point (`cmd/lambda/main.go`)

```go
package main

import (
    "context"
    "log"

    "github.com/aws/aws-lambda-go/events"
    "github.com/aws/aws-lambda-go/lambda"
    fiberadapter "github.com/awslabs/aws-lambda-go-api-proxy/fiber"

    "github.com/yourusername/dynamic-route-53-dns/internal/app"
)

var fiberLambda *fiberadapter.FiberLambda

func init() {
    cfg := app.ConfigFromEnv()
    cfg.IsLocal = false // Force Lambda mode

    fiberApp, err := app.New(cfg)
    if err != nil {
        log.Fatalf("Failed to create app: %v", err)
    }

    fiberLambda = fiberadapter.New(fiberApp)
}

func Handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
    return fiberLambda.ProxyWithContextV2(ctx, req)
}

func main() {
    lambda.Start(Handler)
}
```

### Local Entry Point (`cmd/local/main.go`)

```go
package main

import (
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/yourusername/dynamic-route-53-dns/internal/app"
)

func main() {
    // Force local mode
    os.Setenv("LOCAL_MODE", "true")

    cfg := app.ConfigFromEnv()

    fmt.Println("===========================================")
    fmt.Println("  Dynamic DNS - Local Development Server")
    fmt.Println("===========================================")
    fmt.Printf("  URL:      http://localhost:%s\n", cfg.Port)
    fmt.Printf("  Username: %s\n", cfg.AdminUsername)
    fmt.Printf("  Password: %s\n", cfg.AdminPassword)
    fmt.Println("===========================================")

    fiberApp, err := app.New(cfg)
    if err != nil {
        log.Fatalf("Failed to create app: %v", err)
    }

    // Graceful shutdown
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-c
        fmt.Println("\nShutting down...")
        fiberApp.Shutdown()
    }()

    // Start server
    if err := fiberApp.Listen(":" + cfg.Port); err != nil {
        log.Fatalf("Server error: %v", err)
    }
}
```

---

## Local Development Setup

### Docker Compose for Local Services (`docker-compose.yml`)

```yaml
version: '3.8'

services:
  dynamodb-local:
    image: amazon/dynamodb-local:latest
    container_name: dynamodb-local
    ports:
      - "8000:8000"
    command: "-jar DynamoDBLocal.jar -sharedDb -dbPath /data"
    volumes:
      - dynamodb-data:/data

  dynamodb-admin:
    image: aaronshaf/dynamodb-admin:latest
    container_name: dynamodb-admin
    ports:
      - "8001:8001"
    environment:
      - DYNAMO_ENDPOINT=http://dynamodb-local:8000
      - AWS_REGION=us-east-1
      - AWS_ACCESS_KEY_ID=local
      - AWS_SECRET_ACCESS_KEY=local
    depends_on:
      - dynamodb-local

volumes:
  dynamodb-data:
```

### Local Environment File (`.env.local`)

```bash
# Server
LOCAL_MODE=true
PORT=3000

# Database
DYNAMODB_TABLE=dynamic-dns-local
DYNAMODB_ENDPOINT=http://localhost:8000

# Auth (safe defaults for local development)
ADMIN_USERNAME=admin
ADMIN_PASSWORD=localdev123
APP_SECRET=0000000000000000000000000000000000000000000000000000000000000000

# AWS (for DynamoDB Local)
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=local
AWS_SECRET_ACCESS_KEY=local
```

### DynamoDB Local Table Setup Script (`scripts/setup_local_db.go`)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
    ctx := context.Background()

    // Configure for local DynamoDB
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion("us-east-1"),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
    )
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
        o.BaseEndpoint = aws.String("http://localhost:8000")
    })

    tableName := "dynamic-dns-local"

    // Check if table exists
    _, err = client.DescribeTable(ctx, &dynamodb.DescribeTableInput{
        TableName: aws.String(tableName),
    })
    if err == nil {
        fmt.Printf("Table '%s' already exists\n", tableName)
        return
    }

    // Create table
    _, err = client.CreateTable(ctx, &dynamodb.CreateTableInput{
        TableName: aws.String(tableName),
        AttributeDefinitions: []types.AttributeDefinition{
            {
                AttributeName: aws.String("PK"),
                AttributeType: types.ScalarAttributeTypeS,
            },
            {
                AttributeName: aws.String("SK"),
                AttributeType: types.ScalarAttributeTypeS,
            },
        },
        KeySchema: []types.KeySchemaElement{
            {
                AttributeName: aws.String("PK"),
                KeyType:       types.KeyTypeHash,
            },
            {
                AttributeName: aws.String("SK"),
                KeyType:       types.KeyTypeRange,
            },
        },
        BillingMode: types.BillingModePayPerRequest,
    })
    if err != nil {
        log.Fatalf("Failed to create table: %v", err)
    }

    fmt.Printf("Table '%s' created successfully\n", tableName)
}
```

### Mock Route 53 Client (`internal/route53/mock.go`)

```go
package route53

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// MockClient implements the Route 53 client interface for local development
type MockClient struct {
    mu      sync.RWMutex
    zones   map[string]*MockZone
    records map[string]map[string]*MockRecord // zoneID -> recordName -> record
}

type MockZone struct {
    ID          string
    Name        string
    RecordCount int
    IsPrivate   bool
    Comment     string
}

type MockRecord struct {
    Name   string
    Type   string
    TTL    int
    Values []string
}

// NewMockClient creates a mock Route 53 client with sample data
func NewMockClient() *MockClient {
    client := &MockClient{
        zones:   make(map[string]*MockZone),
        records: make(map[string]map[string]*MockRecord),
    }

    // Add sample zones
    client.zones["Z0123456789ABC"] = &MockZone{
        ID:          "Z0123456789ABC",
        Name:        "example.com.",
        RecordCount: 5,
        IsPrivate:   false,
        Comment:     "Primary domain",
    }
    client.zones["Z9876543210XYZ"] = &MockZone{
        ID:          "Z9876543210XYZ",
        Name:        "test.local.",
        RecordCount: 2,
        IsPrivate:   true,
        Comment:     "Internal testing",
    }

    // Add sample records for example.com
    client.records["Z0123456789ABC"] = map[string]*MockRecord{
        "example.com.": {
            Name:   "example.com.",
            Type:   "A",
            TTL:    300,
            Values: []string{"93.184.216.34"},
        },
        "www.example.com.": {
            Name:   "www.example.com.",
            Type:   "CNAME",
            TTL:    300,
            Values: []string{"example.com."},
        },
        "mail.example.com.": {
            Name:   "mail.example.com.",
            Type:   "MX",
            TTL:    3600,
            Values: []string{"10 mail.example.com."},
        },
    }

    // Add sample records for test.local
    client.records["Z9876543210XYZ"] = map[string]*MockRecord{
        "test.local.": {
            Name:   "test.local.",
            Type:   "A",
            TTL:    60,
            Values: []string{"192.168.1.1"},
        },
    }

    return client
}

func (c *MockClient) ListHostedZones(ctx context.Context) ([]Zone, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    zones := make([]Zone, 0, len(c.zones))
    for _, z := range c.zones {
        zones = append(zones, Zone{
            ID:          z.ID,
            Name:        z.Name,
            RecordCount: z.RecordCount,
            IsPrivate:   z.IsPrivate,
        })
    }
    return zones, nil
}

func (c *MockClient) GetHostedZone(ctx context.Context, zoneID string) (*ZoneDetail, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    z, ok := c.zones[zoneID]
    if !ok {
        return nil, fmt.Errorf("zone not found: %s", zoneID)
    }

    return &ZoneDetail{
        ID:          z.ID,
        Name:        z.Name,
        RecordCount: z.RecordCount,
        IsPrivate:   z.IsPrivate,
        Comment:     z.Comment,
    }, nil
}

func (c *MockClient) ListResourceRecordSets(ctx context.Context, zoneID string) ([]Record, error) {
    c.mu.RLock()
    defer c.mu.RUnlock()

    zoneRecords, ok := c.records[zoneID]
    if !ok {
        return []Record{}, nil
    }

    records := make([]Record, 0, len(zoneRecords))
    for _, r := range zoneRecords {
        records = append(records, Record{
            Name:   r.Name,
            Type:   r.Type,
            TTL:    r.TTL,
            Values: r.Values,
        })
    }
    return records, nil
}

func (c *MockClient) UpsertRecord(ctx context.Context, zoneID, name, recordType, value string, ttl int) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if _, ok := c.zones[zoneID]; !ok {
        return fmt.Errorf("zone not found: %s", zoneID)
    }

    if c.records[zoneID] == nil {
        c.records[zoneID] = make(map[string]*MockRecord)
    }

    // Ensure name ends with dot
    if name[len(name)-1] != '.' {
        name = name + "."
    }

    c.records[zoneID][name] = &MockRecord{
        Name:   name,
        Type:   recordType,
        TTL:    ttl,
        Values: []string{value},
    }

    fmt.Printf("[MockRoute53] Upserted record: %s %s %s (TTL: %d)\n", name, recordType, value, ttl)
    return nil
}

func (c *MockClient) DeleteRecord(ctx context.Context, zoneID, name, recordType string) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    if _, ok := c.records[zoneID]; !ok {
        return nil
    }

    // Ensure name ends with dot
    if name[len(name)-1] != '.' {
        name = name + "."
    }

    delete(c.records[zoneID], name)
    fmt.Printf("[MockRoute53] Deleted record: %s %s\n", name, recordType)
    return nil
}
```

### Updated Makefile Targets

Add these targets to the existing Makefile:

```makefile
# Local development
.PHONY: local dev setup-local docker-up docker-down

# Start local development server
local: build-local setup-local
	source .env.local && ./bootstrap-local

# Start with hot reload (requires air: go install github.com/cosmtrek/air@latest)
dev: setup-local
	source .env.local && air -c .air.toml

# Setup local DynamoDB table
setup-local: docker-up
	@echo "Waiting for DynamoDB Local to start..."
	@sleep 2
	go run ./scripts/setup_local_db.go

# Start Docker services
docker-up:
	docker-compose up -d

# Stop Docker services
docker-down:
	docker-compose down

# View DynamoDB Admin UI
dynamo-admin:
	@echo "Opening DynamoDB Admin at http://localhost:8001"
	@open http://localhost:8001 2>/dev/null || start http://localhost:8001 2>/dev/null || xdg-open http://localhost:8001

# Build for local OS
build-local:
	$(GOBUILD) -o bootstrap-local ./cmd/local
```

### Air Configuration for Hot Reload (`.air.toml`)

```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ./cmd/local"
  bin = "./tmp/main"
  full_bin = "LOCAL_MODE=true ./tmp/main"
  include_ext = ["go", "html", "css"]
  exclude_dir = ["tmp", "vendor", ".git", "node_modules"]
  exclude_regex = ["_test.go"]
  delay = 1000

[log]
  time = false

[color]
  main = "magenta"
  watcher = "cyan"
  build = "yellow"
  runner = "green"

[misc]
  clean_on_exit = true
```

---

## Playwright MCP Testing Specification

### Test Configuration

The Playwright MCP server allows automated browser testing of the local development server. Tests verify UI functionality end-to-end.

### Playwright MCP Test Scenarios

#### Test Suite: Authentication (`tests/e2e/auth.spec.ts`)

```typescript
// These tests are executed via Playwright MCP browser automation

/**
 * Test: LOGIN-E2E-001 - Successful Login
 *
 * Steps:
 * 1. Navigate to http://localhost:3000/login
 * 2. Verify login form is displayed
 * 3. Enter username: "admin"
 * 4. Enter password: "localdev123"
 * 5. Click "Sign In" button
 * 6. Verify redirect to /zones
 * 7. Verify navigation shows "Zones" as active
 * 8. Verify username "admin" displayed in header
 */

/**
 * Test: LOGIN-E2E-002 - Invalid Credentials
 *
 * Steps:
 * 1. Navigate to http://localhost:3000/login
 * 2. Enter username: "admin"
 * 3. Enter password: "wrongpassword"
 * 4. Click "Sign In" button
 * 5. Verify still on /login
 * 6. Verify error message "Invalid credentials" displayed
 * 7. Verify username field retains "admin" value
 */

/**
 * Test: LOGIN-E2E-003 - Session Persistence
 *
 * Steps:
 * 1. Login successfully
 * 2. Navigate to /ddns
 * 3. Refresh page
 * 4. Verify still on /ddns (not redirected to login)
 * 5. Verify username still displayed in header
 */

/**
 * Test: LOGOUT-E2E-001 - Successful Logout
 *
 * Steps:
 * 1. Login successfully
 * 2. Click "Logout" button
 * 3. Verify redirect to /login
 * 4. Navigate to /zones
 * 5. Verify redirect to /login (session expired)
 */
```

#### Test Suite: Zones (`tests/e2e/zones.spec.ts`)

```typescript
/**
 * Test: ZONES-E2E-001 - List Hosted Zones
 *
 * Prerequisites: Logged in
 *
 * Steps:
 * 1. Navigate to /zones
 * 2. Verify page title is "Hosted Zones"
 * 3. Verify table headers: Zone Name, Type, Records, DDNS Configured, Actions
 * 4. Verify "example.com" zone is listed
 * 5. Verify "test.local" zone is listed with "Private" type
 * 6. Click "View Records" for example.com
 * 7. Verify navigation to /zones/Z0123456789ABC
 */

/**
 * Test: ZONES-E2E-002 - Zone Detail View
 *
 * Prerequisites: Logged in
 *
 * Steps:
 * 1. Navigate to /zones/Z0123456789ABC
 * 2. Verify zone name "example.com" displayed
 * 3. Verify zone ID displayed
 * 4. Verify records table shows:
 *    - example.com (A record)
 *    - www.example.com (CNAME)
 *    - mail.example.com (MX)
 * 5. Click "+ Add DDNS Record"
 * 6. Verify navigation to /ddns/new?zone=Z0123456789ABC
 */
```

#### Test Suite: DDNS Management (`tests/e2e/ddns.spec.ts`)

```typescript
/**
 * Test: DDNS-E2E-001 - Create DDNS Record
 *
 * Prerequisites: Logged in
 *
 * Steps:
 * 1. Navigate to /ddns
 * 2. Verify "No DDNS records configured" message (if empty)
 * 3. Click "+ New DDNS Record"
 * 4. Verify navigation to /ddns/new
 * 5. Select zone "example.com" from dropdown
 * 6. Enter hostname: "home"
 * 7. Verify FQDN preview shows "home.example.com"
 * 8. Verify TTL defaults to 60
 * 9. Click "Create DDNS Record"
 * 10. Verify success page with token displayed
 * 11. Verify warning "Save this token now" is visible
 * 12. Copy token value for later tests
 * 13. Navigate to /ddns
 * 14. Verify "home.example.com" appears in list
 */

/**
 * Test: DDNS-E2E-002 - View DDNS Record Detail
 *
 * Prerequisites: DDNS record "home.example.com" exists
 *
 * Steps:
 * 1. Navigate to /ddns
 * 2. Click "home.example.com" link
 * 3. Verify page shows:
 *    - Hostname: home.example.com
 *    - Zone: example.com
 *    - Current IP: "Not set" (initially)
 *    - TTL: 60 seconds
 *    - Status: Enabled
 * 4. Verify "Disable Updates" button is visible
 * 5. Verify "Regenerate Token" button is visible
 * 6. Verify "Delete Record" button is visible
 * 7. Verify Ubiquiti configuration section shows correct values
 */

/**
 * Test: DDNS-E2E-003 - Toggle Enable/Disable
 *
 * Prerequisites: DDNS record exists, enabled
 *
 * Steps:
 * 1. Navigate to /ddns/home.example.com
 * 2. Verify status shows "Enabled"
 * 3. Click "Disable Updates"
 * 4. Verify status changes to "Disabled"
 * 5. Click "Enable Updates"
 * 6. Verify status changes back to "Enabled"
 */

/**
 * Test: DDNS-E2E-004 - Regenerate Token
 *
 * Prerequisites: DDNS record exists
 *
 * Steps:
 * 1. Navigate to /ddns/home.example.com
 * 2. Click "Regenerate Token"
 * 3. Accept confirmation dialog
 * 4. Verify new token is displayed
 * 5. Verify warning message is shown
 * 6. Test old token returns "badauth" on /nic/update
 * 7. Test new token succeeds on /nic/update
 */

/**
 * Test: DDNS-E2E-005 - Delete DDNS Record
 *
 * Prerequisites: DDNS record exists
 *
 * Steps:
 * 1. Navigate to /ddns/home.example.com
 * 2. Click "Delete Record"
 * 3. Accept confirmation dialog
 * 4. Verify redirect to /ddns
 * 5. Verify record no longer in list
 * 6. Navigate to /ddns/home.example.com
 * 7. Verify 404 page
 */

/**
 * Test: DDNS-E2E-006 - Form Validation
 *
 * Steps:
 * 1. Navigate to /ddns/new
 * 2. Click "Create DDNS Record" without filling form
 * 3. Verify "Please select a zone" error
 * 4. Select zone
 * 5. Enter hostname: "invalid_hostname!"
 * 6. Click "Create DDNS Record"
 * 7. Verify "Invalid hostname format" error
 * 8. Enter hostname: "" (empty)
 * 9. Click "Create DDNS Record"
 * 10. Verify required field validation
 */
```

#### Test Suite: Update Endpoint (`tests/e2e/update.spec.ts`)

```typescript
/**
 * Test: UPDATE-E2E-001 - Successful IP Update
 *
 * Prerequisites: DDNS record with known token
 *
 * Steps (via JavaScript fetch in browser):
 * 1. Send GET /nic/update?hostname=home.example.com&myip=1.2.3.4
 *    with Authorization: Basic ddns:token
 * 2. Verify response is "good 1.2.3.4"
 * 3. Navigate to /ddns/home.example.com
 * 4. Verify Current IP shows "1.2.3.4"
 * 5. Verify history shows update entry
 */

/**
 * Test: UPDATE-E2E-002 - No Change Response
 *
 * Prerequisites: Record already has IP 1.2.3.4
 *
 * Steps:
 * 1. Send same update request with myip=1.2.3.4
 * 2. Verify response is "nochg 1.2.3.4"
 * 3. Verify history shows "no_change" status
 */

/**
 * Test: UPDATE-E2E-003 - Invalid Token
 *
 * Steps:
 * 1. Send GET /nic/update?hostname=home.example.com&myip=1.2.3.4
 *    with Authorization: Basic ddns:wrongtoken
 * 2. Verify response is "badauth"
 */
```

### Playwright MCP Test Execution Guide

To run these tests using Playwright MCP:

1. **Start local development server:**
   ```bash
   make docker-up
   make setup-local
   make local
   ```

2. **Use Playwright MCP to execute tests:**

   For each test scenario, use the browser automation tools:

   ```
   # Example: Execute LOGIN-E2E-001

   1. mcp__playwright__browser_navigate to http://localhost:3000/login
   2. mcp__playwright__browser_snapshot to verify form elements
   3. mcp__playwright__browser_type in username field: "admin"
   4. mcp__playwright__browser_type in password field: "localdev123"
   5. mcp__playwright__browser_click on "Sign In" button
   6. mcp__playwright__browser_snapshot to verify /zones page
   7. Verify navigation shows "Zones" active
   8. Verify "admin" displayed in header
   ```

### Test Data Reset Script (`scripts/reset_test_data.go`)

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
    ctx := context.Background()

    cfg, _ := config.LoadDefaultConfig(ctx,
        config.WithRegion("us-east-1"),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("local", "local", "")),
    )

    client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
        o.BaseEndpoint = aws.String("http://localhost:8000")
    })

    tableName := "dynamic-dns-local"

    // Scan and delete all items
    paginator := dynamodb.NewScanPaginator(client, &dynamodb.ScanInput{
        TableName: aws.String(tableName),
    })

    for paginator.HasMorePages() {
        page, err := paginator.NextPage(ctx)
        if err != nil {
            log.Fatalf("Scan failed: %v", err)
        }

        for _, item := range page.Items {
            _, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
                TableName: aws.String(tableName),
                Key: map[string]types.AttributeValue{
                    "PK": item["PK"],
                    "SK": item["SK"],
                },
            })
            if err != nil {
                log.Printf("Delete failed: %v", err)
            }
        }
    }

    fmt.Println("Test data reset complete")
}
```

### Playwright MCP Test Checklist

| Test ID | Description | Status |
|---------|-------------|--------|
| LOGIN-E2E-001 | Successful login | ⬜ |
| LOGIN-E2E-002 | Invalid credentials error | ⬜ |
| LOGIN-E2E-003 | Session persistence | ⬜ |
| LOGOUT-E2E-001 | Successful logout | ⬜ |
| ZONES-E2E-001 | List hosted zones | ⬜ |
| ZONES-E2E-002 | Zone detail view | ⬜ |
| DDNS-E2E-001 | Create DDNS record | ⬜ |
| DDNS-E2E-002 | View DDNS detail | ⬜ |
| DDNS-E2E-003 | Toggle enable/disable | ⬜ |
| DDNS-E2E-004 | Regenerate token | ⬜ |
| DDNS-E2E-005 | Delete DDNS record | ⬜ |
| DDNS-E2E-006 | Form validation | ⬜ |
| UPDATE-E2E-001 | Successful IP update | ⬜ |
| UPDATE-E2E-002 | No change response | ⬜ |
| UPDATE-E2E-003 | Invalid token | ⬜ |
