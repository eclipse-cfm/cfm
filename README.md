<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/logo/cfm-logo-reversed.svg">
  <source media="(prefers-color-scheme: light)" srcset="./docs/logo/cfm-logo.svg">
  <img alt="CFM logo" src="./docs/logo/cfm-logo.svg" width="200">
</picture>

# Component Fabric Manager

Component Fabric Manager (CFM) is a generic orchestration engine. It executes stateful, message-based workflows, called
orchestrations, that provision and manage resources of any kind: tenant configuration in an application, cloud
infrastructure, credentials, or anything else that an agent can automate.

This repository contains the engine and the building blocks for extending it:

- **Provision Manager** (`pmanager`) - stores orchestration and activity definitions, executes orchestrations and
  serves the REST API. This is sometimes called the "orchstration engine" or just "the engine" in the documentation.
- **Agent frameworks** - [`pmanager/natsagent`](pmanager/natsagent) for agents that execute the activities of an
  orchestration, and [`common/lifecycleagent`](common/lifecycleagent) for agents that react to lifecycle events
- **Shared libraries** (`common`, `assembly`) - the modularity system, NATS client, storage, query language,
  authentication, HTTP routing and health checks that CFM runtimes are assembled from

## A generic engine

CFM does not know what it provisions. The Provision Manager only deals with orchestrations, activities, their
definitions and the data that flows between them. Everything that is specific to a domain is added from the outside:

| Extension point                    | What you provide                                                                       |
|------------------------------------|----------------------------------------------------------------------------------------|
| **Managers**                       | A service that owns the domain model, starts orchestrations and consumes their results |
| **Activity agents**                | The code that does the work of one activity type, in a process of its own              |
| **Orchestration definitions**      | Which activities an orchestration type runs, in which order, and with which input      |
| **Namespaces**                     | A space of its own for each manager, so that several managers share one engine         |
| **Lifecycle agents**               | The code that reacts to events outside of an orchestration                             |

Several managers can use one Provision Manager instance. Each manager works in its own
[namespace](docs/developer/namespaces.md), which keeps its definitions, orchestrations, messages and API access apart
from those of the other managers.

The engine does not depend on any of its variants. A lint rule (`depguard` in [`.golangci.yml`](.golangci.yml))
rejects any import of the dataspace components.

## CFM-Dataspace: the dataspace variant

CFM started as the "Connector Fabric Manager", a platform for managing virtualized dataspace connectors and related services. That use case
is now one variant on the generic engine. It lives in
[eclipse-cfm/cfm-dataspace](https://github.com/eclipse-cfm/cfm-dataspace), which builds on this repository as the Go
module `github.com/eclipse-cfm/cfm` and adds everything that is specific to dataspaces:

| Component                           | Go package                                           | Role on the engine                           |
|-------------------------------------|------------------------------------------------------|----------------------------------------------|
| Tenant Manager                      | `github.com/eclipse-cfm/cfm-dataspace/tmanager`      | The manager of the namespace `default`       |
| Orchestration and lifecycle agents  | `github.com/eclipse-cfm/cfm-dataspace/agent`         | Agents for EDC-V, IdentityHub and others     |
| Dataspace model                     | `github.com/eclipse-cfm/cfm-dataspace/dataspace`     | Tenants, participant profiles, VPAs, cells   |

Use cfm-dataspace if you want to manage dataspace participants. Use this repository on its own if you want to build
another manager and other agents on the engine. Both can run against the same Provision Manager.

The dataspace components were part of this repository up to the release `v0.0.10`. Their images moved from
`ghcr.io/eclipse-cfm/cfm/<name>` to `ghcr.io/eclipse-cfm/cfm-dataspace/<name>`, and their Go packages as described in
the [migration table](https://github.com/eclipse-cfm/cfm-dataspace#migrating-from-githubcomeclipse-cfmcfm) of
cfm-dataspace. The Provision Manager image stays `ghcr.io/eclipse-cfm/cfm/pmanager`.

## Getting started

The following are required to build and run the system:

- Go
- A Docker-compatible CLI

```sh
make build         # builds the Provision Manager to pmanager/bin/pmanager
make test          # needs Docker for the Postgres and NATS test containers
make lint
make docker-build  # builds the image ghcr.io/eclipse-cfm/cfm/pmanager:latest
```

At a minimum, the Provision Manager needs NATS with JetStream. It keeps its data in Postgres if configured, and in
memory otherwise:

```sh
docker run --rm -p 4222:4222 nats:2 -js

PM_URI=nats://localhost:4222 PM_BUCKET=cfm-bucket PM_STREAM=cfm-stream PM_AUTH_ENABLED=false \
  go run ./pmanager/cmd/server -mode development
```

The [samples](samples/README.md) continue from here: they add a custom agent, register definitions and start
orchestrations, also in a namespace.

## Samples

| Sample                                                  | Shows                                                               |
|---------------------------------------------------------|---------------------------------------------------------------------|
| [Custom agent](samples/agent/main.go)                   | An activity agent for a made-up activity type                       |
| [Manager](samples/manager/main.go)                      | Starting an orchestration over NATS and receiving its response      |
| [Definitions](samples/definitions)                      | An activity definition, an orchestration template and a manifest    |
| [API calls](samples/README.md#register-the-definitions) | `curl` calls to the REST API, with and without a namespace          |
| [Authenticated NATS](samples/README.md#use-authenticated-nats) | A NATS user with least-privilege permissions per component   |

## Documentation

| Document                                                                         | Content                                                      |
|----------------------------------------------------------------------------------|--------------------------------------------------------------|
| [System architecture](docs/developer/architecture/system.architecture.md)        | The engine, its extension points and its subsystems          |
| [The Orchestrator](docs/developer/architecture/provisioning/orchestrator.design.md) | Orchestrations, activities, definitions and agents        |
| [Namespaces](docs/developer/namespaces.md)                                       | Sharing one Provision Manager between several managers       |
| [Compensation and reconciliation](docs/developer/compensation.md)                | Rolling back failed orchestrations and retrying activities   |
| [The CFM Query Language](docs/developer/architecture/query/cfm.query.language.md) | Predicates for queries of the API and the stores            |
| [Securing the NATS control plane](docs/developer/nats/README.md)                 | Authentication, TLS and least-privilege permissions on NATS  |
| [Running CFM on Postgres](docs/developer/postgres/README.md)                     | Connection pool and schema                                   |
| [Decision records](docs/developer/decision_records/README.md)                    | Architectural decisions                                      |
| [OpenAPI](pmanager/docs/openapi.json)                                            | The REST API of the Provision Manager                        |
