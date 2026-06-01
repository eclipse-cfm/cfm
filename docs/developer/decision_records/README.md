# Decision Records

Decision Records document key architectural decisions made during the design process. A decision record should contain
the following sections:

* **Decision** - A concise description of the decision.
* **Rationale** - Details why the decision was made, including any alternatives considered.
* **Approach** - Describes how the decision will be implemented.

# Adopted Decision Records

| Date        | Title                                                                  | Description                                                                       |
|:------------|:-----------------------------------------------------------------------|:----------------------------------------------------------------------------------|
| 2025-05-07  | [Modularity](2025-05-07-modularity/README.md)                          | Details assemblies, the CFM modularity system                                     |
| 2025-05-08  | [Repository Layout](2025-05-08-01-repo-layout)                         | Code repository layout                                                            |
| 2025-05-08  | [Registries](2025-05-08-02-registries/README.md)                       | Describes how registries are used for extensibility                               |
| 2025-05-08  | [Configuration](2025-05-08-02-configuration/README.md)                 | Configuration management                                                          |
| 2025-05-08  | [Logging](2025-05-08-04-logging/README.md)                             | Logging library and conventions                                                   |
| 2025-05-08  | [HTTP Client](2025-05-08-05-http-client/README.md)                     | HTTP client library                                                               |
| 2025-05-08  | [HTTP Routing](2025-05-08-06-routing/README.md)                        | HTTP router library                                                               |
| 2025-05-08  | [Storage](2025-05-08-07-storage/README.md)                             | Storage extensibility                                                             |
| 2025-05-08  | [Core Subsystems](2025-05-09-core-subsystems/README.md)                | Core subsystems, including the Tenant Manager, Provision Manager and Provisioners |
| 2025-05-14  | [Testing and Test Frameworks](2025-05-14-testing-frameworks/README.md) | Testing and test frameworks guidelines                                            |
| 2025-05-14  | [NATS Orchestration](2025-06-19-nats-orchestration/README.md)          | Selection of NATS for orchestration                                               |
| 2025-07-13  | [Error Handling](2025-07-13-errors/README.md)                          | Raising and handling errors                                                       |
| 2025-10-20  | [Model, Type, and API Packages](2025-10-20-model-type-api/README.md)   | Model, Type, and API Packages                                                     |
| 2025-12-216 | [Single Go Module](2025-12-16-single-go-module/README.md)              | Adoption of a single Go module                                                    |
| 2025-12-216 | [Compensation Datamodel](2026-02-13-compensation-datamodel/README.md)  | Datamodel for compensation mechanisms                                             |
| 2025-12-216 | [Mandatory authn/authz](2026-06-01-mandatory-authn_authz/README.md)    | Mandatory API authn/authz using OAuth2                                            |
