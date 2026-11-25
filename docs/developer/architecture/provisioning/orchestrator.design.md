# The Orchestrator

|                |                                             |
| -------------- | ------------------------------------------- |
| **Status**     | Implemented                                 |
| **Subsystem**  | Provisioning                                |
| **Committers** | [@jimmarino](https://github.com/jimmarino/) |

## Overview

The `Orchestrator` is responsible for executing operations and managing resource allocations. A _**resource**_ can be
anything from a compute cluster to tenant configuration at the application layer. An _**orchestration**_ is composed of
_**activities**_ that perform a unit of work. Since resources often depend on other resources, activities can be
ordered. Otherwise, if a set of activities are not dependent on each other, they will be executed in parallel.

The `Orchestrator` is generic: it does not know what a resource is, and the activities are implemented outside of it.
Consider the deployment of a workspace for a customer, which needs storage and a route under a domain supplied by the
customer. The deployment involves the following activities:

- Input the domain name, the customer metadata, and the target region
- Create the storage of the workspace in the target region
- Apply ingress routing configuration for the domain, using the storage that was created

The dataspace variant of the CFM, [cfm-dataspace](https://github.com/eclipse-cfm/cfm-dataspace), uses the same
mechanism to deploy the Virtual Participant Agents of a dataspace participant.

The orchestrator is responsible for executing these activities in the correct order, maintaining state, and ensuring
reliable processing. Because activities may have high latency and processing must scale-out, the orchestrator is
designed as a stateful message-based system.

## Stateful Messaging

Activities are executed on worker nodes that dequeue messages. During execution, activities have access to a shared
persistent context managed by the orchestration framework. Activities must be idempotent, that is, they must complete
with the same result if executed multiple times without side effects. For example, if an activity is invoked twice due
to a failure, it must ensure duplicate resources are not created and the same shared stated is applied to the context.

Activities may be implemented using a variety of programming languages and technologies, for example, a custom Go
service or Terraform script. The `Orchestrator` delegates to a _**provider**_ for an activity type that is an
extensibility point for the system.

### Messaging Implementation

While the messaging implementation is pluggable, the standard distribution ships with an implementation based on [NATS Jetstream](https://docs.nats.io/nats-concepts/jetstream). The
`Orchestrator` and the stores are defined as interfaces (`pmanager/api`), with the goal of allowing the use of other
technologies such as [Kafka](https://kafka.apache.org/).

The selection of a messaging design provides both spatial and temporal isolation between subsystems. For example, the
orchestrator is decoupled from activities executed on worker nodes. Furthermore, activities are decoupled from one
another. Security contexts can be isolated and worker nodes can be temporally offline (e.g., for maintenance) without
affecting the ability to receive orchestration requests. The orchestration system provides persistence so
data can be passed between activities: the state of an orchestration, including the data its activities share, is kept
in a distributed key/value store (a NATS JetStream key-value bucket). Sensitive data should not be put into the shared
data; an agent should keep it in a secure vault and share a reference.

### Kubernetes Integration

The Orchestrator is deployable as a standalone application or to a Kubernetes cluster. While it is
possible to implement the Orchestration Resource Model described below
as [Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/),
doing so would add additional complexity (the need to implement Kubernetes operators) and tie the solution to Kubernetes.

## Resource Model: Orchestrations and Activities

The `Orchestrator` is built on a resource model consisting of two types: an `OrchestrationDefinition` and an
`ActivityDefinition`. An `ActivityDefinition` declares a type of work, and an `OrchestrationDefinition` contains a
collection of activities that define the workflow for an operation. Both belong to a [namespace](../../namespaces.md)
and are identified by their type within it.

### Activity Definitions

An `ActivityDefinition` defines a work item reliably executed by a worker. It is created with
`POST /api/v1alpha1/activity-definitions`. For example:

```json
{
  "type": "storage.example.com",
  "description": "Creates or deletes the storage of a workspace",
  "inputSchema": {
    "type": "object",
    "required": ["workspace"],
    "properties": {
      "workspace": { "type": "string" },
      "region": { "type": "string" }
    }
  },
  "outputSchema": {
    "type": "object",
    "properties": {
      "bucketName": { "type": "string" }
    }
  }
}
```

The `ActivityDefinition` [JSON Schema](./activity.definition.schema.json) specifies the following properties:

- `type`: The activity type used as a reference. It may contain letters, digits, `.`, `_` and `-`. The agent that
  executes the activities of this type is started with the same type.
- `description`: A description of the activity.
- `inputSchema`: The schema of the values the activity reads. Its `required` fields must be provided by every
  orchestration that uses the activity.
- `outputSchema`: The schema of the values the activity adds to the data of the orchestration for the activities that
  depend on it.
- `namespace`: Optional. The namespace of the request is used when it is omitted, and it must be the namespace of the
  request when it is given.

### Orchestration Templates and Definitions

Orchestration definitions are created from an _**orchestration template**_ with
`POST /api/v1alpha1/orchestration-definitions`. A template groups the orchestration types that belong together, for
example the deployment and the disposal of the same resource. The following is an example of a template:

```json
{
  "id": "workspace.example.com",
  "description": "Deploys and disposes a workspace",
  "schema": {
    "type": "object",
    "required": ["workspace", "domain"],
    "properties": {
      "workspace": { "type": "string" },
      "region": { "type": "string" },
      "domain": { "type": "string" }
    }
  },
  "activities": {
    "workspace.deploy": [
      {
        "id": "create-storage",
        "type": "storage.example.com"
      },
      {
        "id": "create-route",
        "type": "routing.example.com",
        "dependsOn": ["create-storage"]
      }
    ],
    "workspace.dispose": [
      {
        "id": "delete-route",
        "type": "routing.example.com"
      },
      {
        "id": "delete-storage",
        "type": "storage.example.com",
        "dependsOn": ["delete-route"]
      }
    ]
  },
  "compensation": {
    "workspace.deploy": "workspace.dispose"
  }
}
```

The template [JSON Schema](./orchestration.definition.schema.json) defines the following properties:

- `id`: The ID of the template. It is generated when omitted, and returned in the response. The definitions of a
  template are read and deleted together by this ID (`/orchestration-definitions/{id}`).
- `description`: A description of the orchestrations.
- `schema`: The schema for input data, that is, for the payload of the orchestration manifest.
- `activities`: Maps each orchestration type to the activities that are executed as part of an orchestration of that
  type.
- `compensation`: Maps an orchestration type to the orchestration type of the same template that is started when an
  orchestration of the first type fails, see [Compensation and reconciliation](../../compensation.md).
- `namespace`: Optional, as for an activity definition.

The Provision Manager creates one `OrchestrationDefinition` for each entry of `activities`. Its `type` is the key of
the entry, its `templateRef` is the ID of the template, and its `compensationType` is the value of the entry of its type
in `compensation`. An orchestration is started by sending an orchestration manifest with the `orchestrationType` of a
definition.

The `properties` of the `schema` declare the fields of the orchestration manifest payload, which is available to every
activity. When a definition is created, every required input of an activity (`inputSchema`) must be declared in the
`schema` or in the `outputSchema` of an activity it depends on, and the types of matching fields must agree. This
applies to every orchestration type. A field must not be declared both in the `schema` and in the `outputSchema` of a
dependency, or in the `outputSchema` of more than one dependency. The definitions are checked against each other when
they are created; the payload of a manifest is not validated against the `schema` by the engine, an agent validates the
values it reads.

Activities form a Directed Acyclic Graph (DAG) by declaring dependencies using the `dependsOn` property. At execution
time, the activities will be ordered using a topological sort and grouping activities into tiers of parallel execution
steps based on their dependencies.

An activity has the following properties:

- `id`: The activity identifier, unique within its orchestration type.
- `type`: The activity type, which must be the type of an activity definition of the same namespace.
- `dependsOn`: An array of activity ids the activity depends on.

Every activity also has a _**discriminator**_, which is the orchestration type it runs in (`workspace.deploy` or
`workspace.dispose` above). It is set by the Provision Manager and lets one agent serve several orchestration types,
see [Resource Lifecycles](#resource-lifecycles).

### Orchestration Manifests and Responses

An orchestration is started with an `OrchestrationManifest`, sent to the subject
`event.<namespace>.cfm-orchestration` or posted to `/api/v1alpha1/orchestrations`:

```json
{
  "id": "workspace-acme-deploy-1",
  "correlationId": "acme",
  "orchestrationType": "workspace.deploy",
  "payload": {
    "workspace": "acme",
    "region": "eu-central",
    "domain": "acme.example.com"
  }
}
```

- `id`: Identifies the orchestration in its namespace. A manifest that is sent again with the same type and correlation
  ID does not start a second orchestration.
- `correlationId`: An ID of the initiator, returned in the response.
- `orchestrationType`: The type of the orchestration definition to run.
- `payload`: The input data, which becomes the initial data of the orchestration.
- `replyTo`: Optional. A subject token (letters, digits, `_` and `-`) that the response is published to instead of the
  default response subject.
- `namespace`: Optional. It must be the namespace of the subject or of the request when it is given.

When the orchestration completes or fails, an `OrchestrationResponse` is published to
`event.<namespace>.cfm-orchestration-response`, or to `event.<namespace>.<replyTo>`:

```json
{
  "id": "0b7e0a1c-6f0d-4d0c-9a53-0d6a4a3b2f11",
  "manifestId": "workspace-acme-deploy-1",
  "correlationId": "acme",
  "orchestrationType": "workspace.deploy",
  "success": true,
  "properties": {
    "bucketName": "bucket-acme"
  }
}
```

The `properties` are the output values the activities set. A failed orchestration has `success` set to `false`, an
`errorDetail` for the caller and an `errorCause` for operators. The response of a compensating orchestration names the
orchestration it compensated in `compensatedManifestId`.

## Activity Executors

When an orchestration is executed, the Orchestrator reliably enqueues activity messages which will be
dequeued and processed by an associated activity executor. The executor delegates to an `ActivityProcessor` to process
the message. The Orchestrator is responsible for handling system reliability, context persistence, and recovery.

An `ActivityProcessor` is an extensibility point for integrating technologies such as Terraform or custom operations
code into the orchestration process. For example, a Terraform processor would gather input data associated with the
orchestration and pass it to a Terraform script for execution. The `ActivityProcessor` interface is defined in
`pmanager/api` as follows:

```go
package api

type ActivityProcessor interface {
 Process(activityContext ActivityContext) ActivityResult
}

type ActivityResultType int

type ActivityResult struct {
 Result           ActivityResultType
 WaitOnReschedule time.Duration
 Error            error
}

const (
 ActivityResultWait       = 0
 ActivityResultComplete   = 1
 ActivityResultSchedule   = 2
 ActivityResultRetryError = -1
 ActivityResultFatalError = -2
)

```

The `ActivityResult` indicates the following actions to be taken:

- **ActivityResultWait** - The message is acknowledged and the activity must be marked for completion by an external
  process. This is useful for activity types that asynchronously execute a callback on completion.
- **ActivityResultComplete** - The activity is marked as completed and the message is acknowledged.
- **ActivityResultSchedule** - Schedules the message for redelivery as defined by `WaitOnReschedule`. This can be used
  to implement a completion polling mechanism. The data of the orchestration is persisted before, so the next
  invocation sees the values the activity set.
- **ActivityResultRetryError** - A recoverable error was raised and the message is negatively acknowledged so that it
  can be redelivered. After too many retries the orchestration is put into the error state.
- **ActivityResultFatalError** - A fatal error was raised, the orchestration is put into the error state, and the
  message is acknowledged so it will not be redelivered.

The `ActivityContext` gives the processor access to the orchestration:

| Method                                | Purpose                                                                                   |
| ------------------------------------- | ----------------------------------------------------------------------------------------- |
| `OID()`, `ID()`                       | The ID of the orchestration and of the current activity                                   |
| `Discriminator()`                     | The orchestration type the activity runs in                                               |
| `Value(key)`, `Values()`              | The data of the orchestration: the manifest payload and the values earlier activities set |
| `ReadValues(&input)`                  | Deserializes the data into a struct, and validates it by its `validate` tags              |
| `SetValue(key, value)`, `Delete(key)` | Changes the data for the activities that follow                                           |
| `SetOutputValue(key, value)`          | Sets a value that is returned to the manager in the `properties` of the response          |
| `Context()`                           | The `context.Context` of the invocation, which is canceled when the activity times out    |

## Activity Agents

An activity agent runs an activity executor in a dedicated process. A NATS-based agent framework is provided to
facilitate the creation of activity agents (`pmanager/natsagent`). The framework is built on the core modularity
system, allowing services such as the HTTP Client to be used. An agent is instantiated by passing a configuration to
the NATS agent launcher:

```go
package natsagent

type LauncherConfig struct {
 AgentName        string
 ConfigPrefix     string
 Namespace        string
 ActivityType     string
 AssemblyProvider func() []system.ServiceAssembly
 NewProcessor     func(ctx *AgentContext) api.ActivityProcessor
 ServiceName      string
}

func LaunchAgent(shutdown <-chan struct{}, config LauncherConfig)
```

- `AgentName` names the agent in logs, and `ServiceName` in telemetry.
- `ConfigPrefix` is the name of the configuration file and the prefix of the environment variables of the agent.
- `ActivityType` is the type of the activity definition the agent executes the activities of.
- `Namespace` is the [namespace](../../namespaces.md) of the agent. The configuration key `namespace` overrides it, and
  it is `default` when both are empty.
- The `AssemblyProvider` function is used to register services required by the agent.
- The `NewProcessor` function is used to instantiate the `ActivityProcessor` implementation.

The following is an example of an activity agent:

```go
package main

import (
 "net/http"

 "github.com/eclipse-cfm/cfm/assembly/httpclient"
 "github.com/eclipse-cfm/cfm/assembly/serviceapi"
 "github.com/eclipse-cfm/cfm/common/runtime"
 "github.com/eclipse-cfm/cfm/common/system"
 "github.com/eclipse-cfm/cfm/pmanager/api"
 "github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

func main() {
 config := natsagent.LauncherConfig{
  AgentName:    "Routing Agent",
  ConfigPrefix: "routingagent",
  ActivityType: "routing.example.com",
  AssemblyProvider: func() []system.ServiceAssembly {
   return []system.ServiceAssembly{
    &httpclient.HttpClientServiceAssembly{},
   }
  },
  NewProcessor: func(ctx *natsagent.AgentContext) api.ActivityProcessor {
   httpClient := ctx.Registry.Resolve(serviceapi.HttpClientKey).(http.Client)

   return &RoutingActivityProcessor{
    HTTPClient: &httpClient,
    Monitor:    ctx.Monitor,
   }
  },
 }
 natsagent.LaunchAgent(runtime.CreateSignalShutdownChan(), config)
}
```

The above example relies on the HTTP Client and Monitor services, passing them to the activity processor in the
`NewProcessor` function. A complete agent that can be run is in [`samples/agent`](../../../../samples/agent/main.go).

An agent reads the following configuration, here with the environment variables of the prefix `routingagent`:

| Key         | Environment variable     | Description                                                                  |
| ----------- | ------------------------ | ---------------------------------------------------------------------------- |
| `uri`       | `ROUTINGAGENT_URI`       | URI of the NATS server (required)                                            |
| `bucket`    | `ROUTINGAGENT_BUCKET`    | Key-value bucket of the orchestration state (required)                       |
| `stream`    | `ROUTINGAGENT_STREAM`    | Stream of the orchestration messages (required)                              |
| `namespace` | `ROUTINGAGENT_NAMESPACE` | Namespace of the agent, `default` if not set                                 |
| `httpPort`  | `ROUTINGAGENT_HTTPPORT`  | Serves the health endpoints `/healthz` and `/readyz` when set                |
| `nats.*`    | `ROUTINGAGENT_NATS_...`  | Authentication, TLS and JetStream settings, see [NATS](../../nats/README.md) |

The agents of the dataspace variant, for example for EDC-V and IdentityHub, are in
[`cfm-dataspace/agent/orchestration`](https://github.com/eclipse-cfm/cfm-dataspace/tree/main/agent/orchestration).

## Resource Lifecycles

Activities model resource lifecycles. For example, a resource may be deployed and undeployed. In many cases, it is not
desirable to require separate agents for each resource lifecycle operation. The engine calls `Process` for every
activity, and activity processors can use the `discriminator` property, which is the type of the orchestration the
activity runs in, to distinguish between different resource lifecycle operations:

```go
package example

import "github.com/eclipse-cfm/cfm/pmanager/api"

func (p ExampleProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
 switch ctx.Discriminator() {
 case "workspace.deploy":
  // deploy the resource
  return api.ActivityResult{Result: api.ActivityResultComplete}
 case "workspace.dispose":
  // dispose the resource
  return api.ActivityResult{Result: api.ActivityResultComplete}
 default:
  return api.ActivityResult{Result: api.ActivityResultFatalError, Error: fmt.Errorf("unsupported type %s", ctx.Discriminator())}
 }
}
```

The orchestration types are the keys of `activities` in the orchestration template, so the same activity type can be
used for the `deploy` and the `dispose` orchestration:

```json
{
  "id": "storage.example.com",
  "description": "Deploys and disposes storage",
  "activities": {
    "workspace.deploy": [
      {
        "id": "create-storage",
        "type": "storage.example.com"
      }
    ],
    "workspace.dispose": [
      {
        "id": "delete-storage",
        "type": "storage.example.com"
      }
    ]
  },
  "compensation": {
    "workspace.deploy": "workspace.dispose"
  }
}
```

A domain can wrap this pattern in an adapter. In the dataspace variant, agents that deploy and dispose VPAs implement
`vpa.DeployDisposeProcessor` from
[`agent/common/vpa`](https://github.com/eclipse-cfm/cfm-dataspace/tree/main/agent/common/vpa), which has a
`ProcessDeploy` and a `ProcessDispose` method, and register `vpa.AsActivityProcessor(processor)` as their
`ActivityProcessor`. It calls `ProcessDeploy` for the VPA deploy orchestration type, `ProcessDispose` for the VPA dispose
orchestration type, and fails the activity with a fatal error for any other type.

## Integration with External Systems

### Infrastructure as Code (IaC) Automation

The Orchestrator is designed to work with IaC Automation and GitOps systems such
as [Argo](https://argoproj.github.io/), [Atlantis](https://www.runatlantis.io/), [env0](https://www.env0.com/), [Scalr](https://scalr.com/),
and [Spacelift](https://spacelift.io/). These systems can be used to drive deployments using the Orchestrator
API.

### Infrastructure Provisioners

The Orchestrator can integrate with infrastructure provisioning technologies such
as [Eclipse Symphony](https://github.com/eclipse-symphony/symphony), [Fulcrum](https://github.com/fulcrumproject),
and [Liqo](https://liqo.io/) via `ActivityProcessor` implementations.
