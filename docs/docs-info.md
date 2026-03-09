# Baton Twilio Segment V2 - Connector Documentation

This document provides information needed to set up and use the connector.

## Connector Capabilities

### 1. What resources does the connector sync?

| Resource | Description |
|----------|-------------|
| **Workspace** | Top-level container representing the Segment account |
| **Users** | Workspace team members with email, name, and role assignments |
| **Groups** | User groups with shared permissions |
| **Roles** | IAM roles (Workspace Owner, Workspace Admin, Source Admin, etc.) |
| **Invites** | Pending workspace invitations |
| **Sources** | Data sources (JavaScript, iOS, Python, server-side, etc.) |
| **Warehouses** | Data warehouse destinations (BigQuery, Snowflake, Redshift) |
| **Functions** | Custom source and destination functions |
| **Spaces** | Personas environments for identity resolution |

### 2. Can the connector provision any resources? If so, which ones?

Yes, the connector supports provisioning:

| Resource | Grant | Revoke | Create | Delete |
|----------|-------|--------|--------|--------|
| **Group Membership** | ✅ Add users to groups | ✅ Remove users from groups | - | - |
| **Role Assignment** | ✅ Assign roles to users/groups | ✅ Remove role assignments | - | - |
| **Invites** | - | - | ✅ Create new invitations | ✅ Cancel pending invitations |
| **Users** | - | - | - | ✅ Remove users from workspace |

## Connector Credentials

### 1. What credentials or information are needed to set up the connector?

| Credential | Required | Description |
|------------|----------|-------------|
| **Access Token** | Yes | Personal Access Token (PAT) for Segment API authentication |
| **Base URL** | No | API base URL (default: `https://api.segmentapis.com`) |

### 2. Credential Details

#### Access Token (Personal Access Token)

**How to create:**

1. Log in to your Segment workspace at [app.segment.com](https://app.segment.com)
2. Navigate to **Settings** > **Workspace Settings** > **Access Management**
3. Go to the **Tokens** tab
4. Click **Create Token**
5. Give the token a descriptive name (e.g., "ConductorOne Baton Connector")
6. Select the appropriate permissions/scopes
7. Click **Create**
8. Copy the generated token (it won't be shown again)

**Documentation:**
- [Segment Public API Overview](https://docs.segmentapis.com/)
- [Segment Access Management](https://segment.com/docs/segment-app/iam/)

**Required scopes/permissions:**

For **sync only** (read access):
- Workspace Read
- Users Read
- Groups Read
- Sources Read
- Warehouses Read
- Functions Read (if using Functions feature)
- Spaces Read (if using Personas feature)

For **sync + provisioning** (read-write access):
- All read permissions above, plus:
- Users Write (for deleting users)
- Groups Write (for managing group membership)
- Invites Write (for creating/deleting invitations)

**Access level required to create credentials:**

- User must be a **Workspace Owner** or **Workspace Admin** to create API tokens
- The token inherits the permissions of the user who creates it

#### Base URL

**Default value:** `https://api.segmentapis.com`

This typically doesn't need to be changed unless:
- Using a test/mock server for development
- Segment provides a different regional endpoint

## Additional Notes

### Segment Plan Requirements

| Feature | Required Plan |
|---------|---------------|
| Public API Access | Team or Business |
| User Groups | Business only |

### API Documentation Links

- [Segment Public API](https://docs.segmentapis.com/)
- [IAM Users](https://docs.segmentapis.com/tag/IAM-Users)
- [IAM Groups](https://docs.segmentapis.com/tag/IAM-Groups)
- [IAM Roles](https://docs.segmentapis.com/tag/IAM-Roles)
- [IAM Invites](https://docs.segmentapis.com/tag/IAM-Invites)
- [Sources](https://docs.segmentapis.com/tag/Sources)
- [Warehouses](https://docs.segmentapis.com/tag/Warehouses)
- [Functions](https://docs.segmentapis.com/tag/Functions)
- [Spaces](https://docs.segmentapis.com/tag/Spaces)
