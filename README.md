# `baton-segment` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-segment.svg)](https://pkg.go.dev/github.com/conductorone/baton-segment) ![main ci](https://github.com/conductorone/baton-segment/actions/workflows/main.yaml/badge.svg)

`baton-segment` is a connector for Segment built using the [Baton SDK](https://github.com/conductorone/baton-sdk). It communicates with the Segment API to sync data about users, groups, resources, roles and workspaces.
Check out [Baton](https://github.com/conductorone/baton) to learn more about the project in general.

# Getting Started

## Prerequisites

- Access to the Segment App
- Generate API token for your workspace. To generate a token go to `Settings -> Access Management -> Tokens -> Create Token`
- Choose `Workspace Owner` in `Assign Access` in order to be able to have full read and edit access to everything in the workspace. `Membership Access` can only view the workspace without access to any sub-resources.

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-segment

BATON_TOKEN=segmentApiToken baton-segment
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out -e BATON_TOKEN=segmentApiToken ghcr.io/conductorone/baton-segment:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-segment/cmd/baton-segment@main

BATON_TOKEN=segmentApiToken baton-segment
baton resources
```

# Data Model

`baton-segment` will pull down information about the following Segment resources:

- Users
- Groups
- Functions
- Sources
- Spaces
- Warehouses
- Roles
- Workspace

# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually building spreadsheets. We welcome contributions, and ideas, no matter how small -- our goal is to make identity and permissions sprawl less painful for everyone. If you have questions, problems, or ideas: Please open a Github Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-segment` Command Line Usage

```
baton-segment

Usage:
  baton-segment [flags]
  baton-segment [command]

Available Commands:
  completion         Generate the autocompletion script for the specified shell
  help               Help about any command

Flags:
      --client-id string       The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string   The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
  -f, --file string            The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                   help for baton-segment
      --log-format string      The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string       The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
  -p, --provisioning           This must be set in order for provisioning actions to be enabled. ($BATON_PROVISIONING)
      --token string           The Segment access token used to connect to the Segment API. ($BATON_TOKEN)
  -v, --version                version for baton-segment

Use "baton-segment [command] --help" for more information about a command.
```
