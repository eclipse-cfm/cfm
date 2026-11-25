<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/logo/cfm-logo-reversed.svg">
  <source media="(prefers-color-scheme: light)" srcset="./docs/logo/cfm-logo.svg">
  <img alt="CFM logo" src="./docs/logo/cfm-logo.svg" width="200">
</picture>

Component Fabric Manager (CFM) provides infrastructure for managing virtualized dataspace connectors. CFM consists of
the following components:

- **Provision Manager** - Resource provisioning and orchestration

## Dataspace components

The dataspace-specific components live in [eclipse-cfm/cfm-dataspace](https://github.com/eclipse-cfm/cfm-dataspace) and
build on this repository:

- the Tenant Manager: `github.com/eclipse-cfm/cfm-dataspace/tmanager`
- the orchestration and lifecycle agents: `github.com/eclipse-cfm/cfm-dataspace/agent`
- the dataspace model: `github.com/eclipse-cfm/cfm-dataspace/dataspace`

Their images are published as `ghcr.io/eclipse-cfm/cfm-dataspace/<name>`.

## Prerequisites

The following are required to build and run the system:

- Go
- A Docker-compatible CLI

