![Baton Logo](./baton-logo.png)

# `baton-segment` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-segment.svg)](https://pkg.go.dev/github.com/conductorone/baton-segment) ![ci](https://github.com/conductorone/baton-segment/actions/workflows/ci.yaml/badge.svg) ![verify](https://github.com/conductorone/baton-segment/actions/workflows/verify.yaml/badge.svg)

`baton-segment` is a connector for Twilio Segment built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It syncs IAM users, groups, roles, invitations, and resource access from your Segment workspace for complete access visibility.

Check out [Baton](https://github.com/conductorone/baton) to learn more about the project in general.

# Prerequisites

- A Twilio Segment workspace with **Team** or **Business** tier (required for API access)
- A Personal Access Token (PAT) with appropriate permissions

## Feature Availability by Plan

| Feature | Free | Team | Business |
|---------|------|------|----------|
| Users, Roles, Invites | ❌ | ✅ | ✅ |
| Sources, Warehouses, Functions | ❌ | ✅ | ✅ |
| **User Groups** | ❌ | ❌ | ✅ |

> **Note**: If you attempt to use features not available in your plan, you'll receive an error like:
> ```json
> {
>     "errors": [
>         {
>             "type": "bad-request",
>             "message": "Your plan does not include User Groups. Contact sales to upgrade to business tier."
>         }
>     ]
> }
> ```

## Creating an Access Token

1. Log in to your Segment workspace
2. Navigate to **Settings** > **Workspace Settings** > **Access Management**
3. Go to the **Tokens** tab
4. Click **Create Token** and select the appropriate scopes
5. Copy the generated token

For more information, see [Segment's Public API documentation](https://docs.segmentapis.com/).

# Getting Started

## brew

```bash
brew install conductorone/baton/baton conductorone/baton/baton-segment

BATON_TOKEN=your-access-token baton-segment
baton resources
```

## docker

```bash
docker run --rm -v $(pwd):/out -e BATON_TOKEN=your-access-token ghcr.io/conductorone/baton-segment:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```bash
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-segment/cmd/baton-segment@main

BATON_TOKEN=your-access-token baton-segment

baton resources
```

# Data Model

`baton-segment` syncs the following resources from your Segment workspace:

## IAM Resources

| Resource Type | Description |
|---------------|-------------|
| **Workspace** | Top-level container for all resources |
| **Users** | Workspace team members with their role assignments |
| **Groups** | User groups with shared permissions |
| **Roles** | IAM roles (e.g., Workspace Owner, Source Admin, Warehouse Admin) |
| **Invites** | Pending workspace invitations |

## Data Resources

| Resource Type | Description |
|---------------|-------------|
| **Sources** | Data sources (JavaScript, iOS, Python, etc.) |
| **Warehouses** | Data warehouses (BigQuery, Snowflake, Redshift, etc.) |
| **Functions** | Custom source and destination functions |
| **Spaces** | Personas environments for identity resolution |

## Entitlements

Role-based entitlements are created on **scope resources** (workspace, source, warehouse, function, space) using the ternary relationship model: `Principal → Role → Scope Resource`.

| Resource Type | Entitlement | Example | Description |
|---------------|-------------|---------|-------------|
| **Workspace** | `member` | `workspace:<workspace-id>:member` | User is a member of the workspace |
| **Role** | `member` | `role:<role-id>:member` | Workspace-scoped role assignment |
| **Source** | `<role-slug>` | `source:<source-id>:source_admin` | Role assignment on a source |
| **Function** | `<role-slug>` | `function:<function-id>:function_admin` | Role assignment on a function |
| **Group** | `member` | `group:<group-id>:member` | User is a member of this group |

Scope entitlements carry a snake_case slug derived from the role's name. A scope resource
receives an entitlement only for roles whose name matches its type, so a source carries
`source_admin` and a function carries `function_admin` and `function_read_only`. Roles that
name no scope type, such as `Workspace Owner` or `PII Access`, appear as
`role:<role-id>:member` and produce no scope entitlements.

## Grants

- **Workspace → User**: All users are members of the workspace
- **Scope Resource → User/Group**: Role assignments (e.g., "Workspace Owner on C1-twilio-test")
- **Group → User**: Group membership grants

# Provisioning

`baton-segment` supports the following provisioning capabilities:

## Group Management

| Capability | Description |
|------------|-------------|
| **Grant** | Add users to groups |
| **Revoke** | Remove users from groups |

## Role Management

| Capability | Description |
|------------|-------------|
| **Grant** | Assign a role to a user |
| **Revoke** | Remove a role assignment from a user |

Groups can hold roles in Segment and the connector syncs those assignments, but granting and
revoking a role requires a user principal. A group principal is rejected.

## Account Management

| Capability | Resource Type | Description |
|------------|---------------|-------------|
| **Create Account** | Invite | Invite new users to the workspace |
| **Delete Account** | User | Remove users from the workspace |
| **Delete Account** | Invite | Cancel pending invitations |

Grant and revoke run on the `baton-segment` binary. The `baton` CLI reads a sync bundle and
has no `grant` or `revoke` command. Each operation resolves its entitlement and principal
from the bundle, so sync first.

## Example: Grant group membership

```bash
baton-segment --token $TOKEN --file sync.c1z

baton-segment --token $TOKEN --file sync.c1z \
  --grant-entitlement "group:<group-id>:member" \
  --grant-principal "<user-id>" \
  --grant-principal-type user
```

## Example: Assign a role to a user

```bash
baton-segment --token $TOKEN --file sync.c1z \
  --grant-entitlement "role:<role-id>:member" \
  --grant-principal "<user-id>" \
  --grant-principal-type user
```

## Example: Revoke a role from a user

A grant identifier is `<entitlement-id>:<principal-type>:<principal-id>`.

```bash
baton-segment --token $TOKEN --file sync.c1z \
  --revoke-grant "role:<role-id>:member:user:<user-id>"
```

# Configuration

| Flag | Environment Variable | Description | Required |
|------|---------------------|-------------|----------|
| `--token` | `BATON_TOKEN` | Personal Access Token for Segment API | Yes |
| `--base-url` | `BATON_BASE_URL` | Base URL for Segment API (default: `https://api.segmentapis.com`) | No |

# Development

## Running the Test Server

For development without real API credentials, you can use the mock test server:

```bash
# Build the test server
go build -o dist/test-server ./cmd/test-server

# Start the test server
./dist/test-server

# In another terminal, run the connector
./dist/darwin_arm64/baton-segment \
  --base-url http://localhost:8080 \
  --token test-segment-token-12345
```

The test server provides mock endpoints for all Segment API operations used by the connector.

## Building

```bash
make build
```

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually
building spreadsheets. We welcome contributions, and ideas, no matter how
small—our goal is to make identity and permissions sprawl less painful for
everyone. If you have questions, problems, or ideas: Please open a GitHub Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-segment` Command Line Usage

```text
baton-segment

Usage:
  baton-segment [flags]
  baton-segment [command]

Available Commands:
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --base-url string            Base URL for the Segment API ($BATON_BASE_URL) (default "https://api.segmentapis.com")
      --client-id string           The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string       The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
  -f, --file string                The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                       help for baton-segment
      --log-format string          The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string           The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
  -p, --provisioning               Enable provisioning support ($BATON_PROVISIONING)
      --ticketing                  Enable ticketing support ($BATON_TICKETING)
      --token string        Personal Access Token for Segment API ($BATON_TOKEN)
  -v, --version                    version for baton-segment

Use "baton-segment [command] --help" for more information about a command.
```
