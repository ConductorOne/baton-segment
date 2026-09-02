# Twilio Segment IAM Overview

This document provides an overview of Twilio Segment's Identity and Access Management (IAM) model and how it relates to the Baton connector.

## Table of Contents

- [What is Twilio Segment?](#what-is-twilio-segment)
- [IAM Model Overview](#iam-model-overview)
- [Resources](#resources)
  - [Workspace](#workspace)
  - [Users](#users)
  - [User Groups](#user-groups)
  - [Roles](#roles)
  - [Invites](#invites)
  - [Sources](#sources)
  - [Warehouses](#warehouses)
  - [Functions](#functions)
  - [Spaces](#spaces)
- [Permission Model](#permission-model)
- [Resource Relationships Diagram](#resource-relationships-diagram)
- [API Documentation Links](#api-documentation-links)

---

## What is Twilio Segment?

[Twilio Segment](https://segment.com/) is a Customer Data Platform (CDP) that helps companies collect, clean, and activate their customer data. It acts as a central hub for customer data, allowing businesses to:

- **Collect** data from websites, mobile apps, and servers
- **Unify** customer profiles across multiple touchpoints
- **Route** data to hundreds of analytics tools, marketing platforms, and data warehouses
- **Govern** data quality and privacy compliance

The platform consists of several key components:
- **Sources**: Where data comes from (websites, apps, servers)
- **Destinations**: Where data goes (analytics tools, warehouses, marketing platforms)
- **Warehouses**: Data storage destinations (BigQuery, Snowflake, Redshift)
- **Functions**: Custom code for data transformation
- **Spaces**: Personas environments for identity resolution and audience building

---

## IAM Model Overview

Segment uses a **Role-Based Access Control (RBAC)** model within a **Workspace**. The key concepts are:

1. **Workspace**: The top-level container that represents your Segment account
2. **Users**: Individual people who can access the workspace
3. **User Groups**: Collections of users for easier permission management
4. **Roles**: Predefined sets of permissions (e.g., Workspace Owner, Source Admin)
5. **Permissions**: A combination of a Role + Resources it applies to

### Key Principle: Permissions = Role + Resources

In Segment, permissions are assigned by combining:
- A **Role** (what actions can be performed)
- One or more **Resources** (what the role applies to)

For example:
- "Source Admin" role on "All Sources" = Can manage all sources
- "Source Read-only" role on "JavaScript Source" = Can only view one specific source

---

## Resources

### Workspace

The **Workspace** is the top-level container in Segment. It represents your organization's Segment account and contains all other resources.

| Property | Description |
|----------|-------------|
| ID | Unique identifier for the workspace |
| Name | Display name of the workspace |
| Slug | URL-friendly identifier |

**Key Points:**
- One access token is associated with one workspace
- All users, groups, sources, warehouses, etc. belong to a workspace
- Workspace-level roles grant permissions across the entire workspace

**Documentation**: [Segment Workspaces](https://segment.com/docs/segment-app/iam/workspaces/)

---

### Users

**Users** are individual people who have access to the Segment workspace. Each user has:

| Property | Description |
|----------|-------------|
| ID | Unique identifier |
| Name | Display name |
| Email | Email address (used for authentication) |
| Permissions | List of role assignments |

**Key Points:**
- Users are identified by email address
- Users can have multiple roles assigned
- Users can belong to multiple groups
- Permissions can be assigned directly to users or inherited from groups

**Documentation**: [IAM Users API](https://docs.segmentapis.com/tag/IAM-Users)

---

### User Groups

> **⚠️ Business Tier Required**: User Groups are only available on Segment's **Business tier** plan. Free and Team tier plans will receive the following error when attempting to access groups:
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

**User Groups** are collections of users that simplify permission management. Instead of assigning roles to each user individually, you can assign roles to a group.

| Property | Description |
|----------|-------------|
| ID | Unique identifier |
| Name | Group name |
| Member Count | Number of users in the group |
| Members | List of user IDs |
| Permissions | List of role assignments for the group |

**Key Points:**
- **Requires Business tier plan** - not available on Free or Team plans
- Users inherit all permissions from their groups
- A user can belong to multiple groups
- Groups can have multiple roles assigned
- Adding a user to a group automatically grants them the group's permissions

**Common Group Examples:**
- "Engineering" - Source Admin, Destination Admin
- "Data Science" - Warehouse Admin
- "Marketing" - Source Read-only
- "Administrators" - Workspace Admin

**Documentation**: [IAM Groups API](https://docs.segmentapis.com/tag/IAM-Groups)

---

### Roles

**Roles** define what actions a user or group can perform. Segment provides predefined roles that cannot be modified.

| Property | Description |
|----------|-------------|
| ID | Unique identifier (e.g., `role_workspace_owner`) |
| Name | Display name |
| Description | What the role allows |

**Available Roles:**

| Role | Description | Scope |
|------|-------------|-------|
| **Workspace Owner** | Full control including billing and team management | Workspace |
| **Workspace Admin** | Administrative access to workspace settings | Workspace |
| **Source Admin** | Full control over sources | Sources |
| **Source Read-only** | Read-only access to sources | Sources |
| **Destination Admin** | Full control over destinations | Destinations |
| **Destination Read-only** | Read-only access to destinations | Destinations |
| **Warehouse Admin** | Full control over warehouses | Warehouses |
| **Function Admin** | Full control over functions | Functions |
| **Tracking Plan Admin** | Full control over tracking plans | Tracking Plans |
| **Personas Admin** | Full control over Personas (Spaces) | Spaces |
| **Personas Read-only** | Read-only access to Personas | Spaces |

**Key Points:**
- Roles are predefined by Segment (not customizable)
- Roles can be scoped to specific resources or all resources of a type
- The same role can be assigned multiple times with different resource scopes

**Documentation**: [IAM Roles API](https://docs.segmentapis.com/tag/IAM-Roles)

---

### Invites

**Invites** are pending invitations to join the workspace. When you invite someone, they receive an email and must accept before becoming a full user.

| Property | Description |
|----------|-------------|
| Email | Email address of the invitee |
| Permissions | Pre-assigned permissions (activated upon acceptance) |

**Key Points:**
- Invites are identified by email address
- Permissions can be pre-assigned to invites
- Once accepted, the invite becomes a user with the specified permissions
- Invites can be deleted before acceptance

**Documentation**: [IAM Invites API](https://docs.segmentapis.com/tag/IAM-Invites)

---

### Sources

**Sources** are where data comes from. They represent the origins of your customer data.

| Property | Description |
|----------|-------------|
| ID | Unique identifier |
| Slug | URL-friendly identifier |
| Name | Display name |
| Workspace ID | Parent workspace |
| Enabled | Whether the source is active |

**Types of Sources:**
- **Website** (JavaScript, Analytics.js)
- **Mobile** (iOS, Android, React Native)
- **Server** (Node.js, Python, Ruby, Go, Java)
- **Cloud Apps** (Salesforce, Stripe, Zendesk)

**Key Points:**
- Sources can have specific role assignments (e.g., "Source Admin" on a specific source)
- Sources send data to destinations through connections
- Each source has its own write key for data collection

**Documentation**: [Sources API](https://docs.segmentapis.com/tag/Sources)

---

### Warehouses

**Warehouses** are data storage destinations where Segment sends your collected data for analysis.

| Property | Description |
|----------|-------------|
| ID | Unique identifier |
| Workspace ID | Parent workspace |
| Enabled | Whether the warehouse is active |
| Metadata | Connection details (type, name, description) |

**Supported Warehouses:**
- Google BigQuery
- Snowflake
- Amazon Redshift
- PostgreSQL
- Azure Synapse

**Key Points:**
- Warehouses receive data from all enabled sources
- Warehouse Admin role grants full control over warehouse settings
- Data is synced on a schedule (configurable)

**Documentation**: [Warehouses API](https://docs.segmentapis.com/tag/Warehouses)

---

### Functions

**Functions** allow you to write custom JavaScript code to transform or route data.

| Property | Description |
|----------|-------------|
| ID | Unique identifier |
| Workspace ID | Parent workspace |
| Display Name | Function name |
| Description | What the function does |
| Resource Type | SOURCE or DESTINATION |

**Types of Functions:**
- **Source Functions**: Custom data sources (webhooks, APIs)
- **Destination Functions**: Custom data destinations
- **Insert Functions**: Transform data in-flight

**Key Points:**
- Functions require the Functions feature to be enabled (may require specific plan)
- Function Admin role grants full control over functions
- Functions can be used to integrate with services not natively supported

**Documentation**: [Functions API](https://docs.segmentapis.com/tag/Functions)

---

### Spaces

**Spaces** are Personas environments used for identity resolution and audience building (part of Segment's CDP offering).

| Property | Description |
|----------|-------------|
| ID | Unique identifier |
| Name | Space name |
| Slug | URL-friendly identifier |

**Key Points:**
- Spaces are part of the Personas product
- Each space has its own identity graph and audiences
- Personas Admin/Read-only roles control access to spaces
- Requires Personas feature to be enabled

**Documentation**: [Spaces API](https://docs.segmentapis.com/tag/Spaces)

---

## Permission Model

### How Permissions Work

Permissions in Segment follow this structure:

```json
Permission = {
    roleId: "role_source_admin",
    resources: [
        { id: "source_001", type: "SOURCE" },
        { id: "source_002", type: "SOURCE" }
    ]
}
```

### Resource Types for Permission Scoping

| Type | Description | Example |
|------|-------------|---------|
| `WORKSPACE` | Entire workspace | Workspace Owner on workspace |
| `SOURCE` | Specific source(s) | Source Admin on JavaScript Source |
| `WAREHOUSE` | Specific warehouse(s) | Warehouse Admin on BigQuery |
| `FUNCTION` | Specific function(s) | Function Admin on Transform Function |
| `SPACE` | Specific space(s) | Personas Admin on Production Space |

### Permission Inheritance

```text
User Permissions = Direct Permissions + Group Permissions

Example:
- User "Alice" has Source Admin directly assigned
- User "Alice" is member of "Engineering" group
- "Engineering" group has Destination Admin assigned
- Result: Alice has both Source Admin AND Destination Admin
```

### Granting and Revoking Permissions

To modify permissions, you must:
1. **GET** current permissions for the user/group
2. **ADD** or **REMOVE** the permission from the list
3. **PUT** the entire new permission list (replaces all permissions)

**Important**: The API uses a "replace all" model, not incremental updates.

---

## Resource Relationships Diagram

```text
┌─────────────────────────────────────────────────────────────────────┐
│                           WORKSPACE                                  │
│                    (Top-level container)                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐  │
│   │  USERS   │────▶│  GROUPS  │     │  ROLES   │     │ INVITES  │  │
│   └──────────┘     └──────────┘     └──────────┘     └──────────┘  │
│        │                │                │                │         │
│        │                │                │                │         │
│        └────────────────┴────────────────┘                │         │
│                         │                                 │         │
│                         ▼                                 │         │
│              ┌─────────────────────┐                      │         │
│              │    PERMISSIONS      │◀─────────────────────┘         │
│              │  (Role + Resources) │                                │
│              └─────────────────────┘                                │
│                         │                                           │
│          ┌──────────────┼──────────────┬──────────────┐             │
│          ▼              ▼              ▼              ▼             │
│   ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐        │
│   │ SOURCES  │   │WAREHOUSES│   │FUNCTIONS │   │  SPACES  │        │
│   └──────────┘   └──────────┘   └──────────┘   └──────────┘        │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘

Legend:
────▶  Membership/Contains
- - -▶ Permission Assignment
```

### Relationship Summary

| Relationship | Description |
|--------------|-------------|
| Workspace → Users | Workspace contains users |
| Workspace → Groups | Workspace contains groups |
| Workspace → Roles | Workspace has available roles |
| Workspace → Sources/Warehouses/Functions/Spaces | Workspace contains these resources |
| Users → Groups | Users can be members of groups |
| Users → Permissions | Users have direct permission assignments |
| Groups → Permissions | Groups have permission assignments |
| Permissions → Roles | Permissions reference roles |
| Permissions → Resources | Permissions scope roles to specific resources |
| Invites → Permissions | Invites have pre-assigned permissions |

---

## API Documentation Links

### Official Segment API Documentation

| Resource | Documentation URL |
|----------|-------------------|
| **API Overview** | https://docs.segmentapis.com/ |
| **Workspace** | https://docs.segmentapis.com/tag/Workspace |
| **IAM Users** | https://docs.segmentapis.com/tag/IAM-Users |
| **IAM Groups** | https://docs.segmentapis.com/tag/IAM-Groups |
| **IAM Roles** | https://docs.segmentapis.com/tag/IAM-Roles |
| **IAM Invites** | https://docs.segmentapis.com/tag/IAM-Invites |
| **Sources** | https://docs.segmentapis.com/tag/Sources |
| **Warehouses** | https://docs.segmentapis.com/tag/Warehouses |
| **Functions** | https://docs.segmentapis.com/tag/Functions |
| **Spaces** | https://docs.segmentapis.com/tag/Spaces |
| **Audit Trail** | https://docs.segmentapis.com/tag/Audit-Trail |

### Segment Product Documentation

| Topic | Documentation URL |
|-------|-------------------|
| **IAM Overview** | https://segment.com/docs/segment-app/iam/ |
| **Roles & Permissions** | https://segment.com/docs/segment-app/iam/roles/ |
| **User Groups** | https://segment.com/docs/segment-app/iam/groups/ |
| **Sources Overview** | https://segment.com/docs/connections/sources/ |
| **Warehouses Overview** | https://segment.com/docs/connections/storage/warehouses/ |
| **Functions Overview** | https://segment.com/docs/connections/functions/ |
| **Personas (Spaces)** | https://segment.com/docs/personas/ |

---

## Connector Capabilities

This Baton connector supports:

| Capability | Description |
|------------|-------------|
| **Sync** | Syncs all IAM resources (users, groups, roles, invites, sources, warehouses, functions, spaces) |
| **Provisioning** | Grant/revoke group membership |
| **Role Assignment** | Grant/revoke role assignments for users, groups, and pending invites |
| **Account Creation** | Create new user invitations |
| **Role on Invite** | Attach a role to an invitation at invite time (`POST /invites` with `permissions`), so people without a Segment account can be provisioned |
| **Account Deletion** | Delete users and invitations |
| **Usage Tracking** | Track last login via audit events (Business tier only) |

---

## Glossary

| Term | Definition |
|------|------------|
| **CDP** | Customer Data Platform |
| **IAM** | Identity and Access Management |
| **RBAC** | Role-Based Access Control |
| **Personas** | Segment's identity resolution and audience building product |
| **Write Key** | Unique identifier for a source used to send data |
| **Slug** | URL-friendly identifier (lowercase, hyphenated) |
| **Business Tier** | Segment plan level that includes audit trail access |
