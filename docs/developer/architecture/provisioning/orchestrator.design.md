# The Orchestrator

|                |                                             |
|----------------|---------------------------------------------|
| **Status**     | Implemented                                 |
| **Subsystem**  | Provisioning                                |
| **Committers** | [@jimmarino](https://github.com/jimmarino/) |

## Overview

The `Orchestrator` is responsible for executing operations and managing resource allocations. A _**resource**_ can be
anything from a compute cluster to tenant configuration at the application layer. An _**orchestration**_ is composed of
_**activities**_ that perform a unit of work. Since resources often depend on other resources, activities can be
ordered. Otherwise, if a set of activities are not dependent on each other, they will be executed in parallel.

Consider a tenant deployment that involves the creation of a Web DID using a domain supplied by the tenant owner. The
deployment involves the following activities:

- Input domain name, tenant metadata, and target cell
- Stage and apply the tenant configuration to the application in the target cell
- Apply ingress routing configuration for the tenant domain

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

The messaging implementation will be pluggable. The initial system will be based
on [NATS Jetstream](https://docs.nats.io/nats-concepts/jetstream). A design goal is to allow the use of other
technologies such as [Kafka](https://kafka.apache.org/).

The selection of a messaging design provides both spatial and temporal isolation between subsystems. For example, the
orchestrator is decoupled from activities executed on worker nodes. Furthermore, activities are decoupled from one
another. Security contexts can be isolated and worker nodes can be temporally offline (e.g., for maintenance) without
affecting the ability to receive orchestration requests. The orchestration system provides persistence so
data can be passed between activities. For non-sensitive data, persistence is provided using a distributed key/value
store. For sensitive data, persistence is provided using a secure vault.

### Kubernetes Integration

The Orchestrator will be deployable as a standalone application or to a Kubernetes cluster. While it is
possible to implement the Orchestration Resource Model described above
as [Custom Resource Definitions](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/),
doing so will add additional complexity (the need to implement Kubernetes operators) and tie the solution to Kubernetes.

## Resource Model: Orchestrations and Activities

The `Orchestrator` is built on a resource model consisting of two types: an `OrchestrationDefinition` and an
`ActivityDefinition`. An `OrchestrationDefinition` contains a collection of `Activities` that define the workflow
for an operation. The following is an example of an `OrchestrationDefinition`:

```json
{
  "type": "tenant.example.com",
  "active": true,
  "schema": {},
  "activities": [
    {
      "id": "activity1",
      "type": "activity1.example.com",
      "inputs": [
        "cell",
        "baseUrl"
      ]
    },
    {
      "id": "activity2",
      "type": "activity2.example.com",
      "dependsOn": "activity1",
      "inputs": [
        {
          "source": "activity1.resource",
          "target": "resourceId"
        }
      ]
    }
  ]
}
```

The `OrchestrationDefinition` [JSON Schema](./orchestration.definition.schema.json) defines the following properties and
types:

- `type`: The definition type used when creating a corresponding resource.
- `active`: If the definition is active.
- `activities`: Defines the sequence of activities that are executed as part of the orchestration.
- `input`: The input data for the orchestration.
- `output`: The output data from the orchestration.
- `schema`: The schema for input data.

Activities form a Directed Acyclic Graph (DAG) by declaring dependencies using the `dependsOn` property. At execution
time, the activities will be ordered using a topological sort and grouping activities into tiers of parallel execution
steps based on their dependencies.

An activity has the following properties:

- `id`: The activity identifier.
- `type`: The activity type.
- `discriminator`: An optional string used by activity implementation to distinguish activity subtypes. For example, an
  activity type may support `deploy` and `dispose` operations that may be defined for different orchestrations.
- `dependsOn`: An array of activity ids the activity depends on.
- `inputs`: An array of input properties. The input properties may include references to properties contained in the
  input data or references to output data properties from a previous activity. References to activity output
  data are prefixed with the activity identifier followed by a '.'. Activity output data is defined in the activity
  definition described below. An input property may be specified using a string or an object containing `source` and
  `target` properties if a mapping is required.

An `ActivityDefinition` defines a work item reliably executed by a worker. For example:

```json
{
  "type": "activity1.example.com",
  "provider": "provisioner.example.com",
  "description": "Provisions a resource for a tenant",
  "inputSchema": {
    "openAPIV3Schema": {}
  },
  "outputSchema": {
    "openAPIV3Schema": {}
  }
}
```

The `ActivityDefinition` [JSON Schema](./activity-definition.schema.json) specifies the following properties:

- `type`: The activity type used as a reference.
- `provider`: The provisioner that executes the activity. A provisioner could be a service, Terraform script, or
  other technology.
- `description`: A description of the activity.
- `inputSchema`: The schema for input properties when creating a resource of the definition type. Currently,
  `openAPIV3Schema` is the only supported schema type.
- `outputSchema`: The schema for output properties when creating a resource of the definition type.
  Currently, `openAPIV3Schema` is the only supported schema type.

## Activity Executors

When an orchestration is executed, the Orchestrator reliably enqueues activity messages which will be
dequeued and processed by an associated activity executor. The executor delegates to an `ActivityProcessor` to process
the message. The Orchestrator is responsible for handling system reliability, context persistence, and recovery.

An `ActivityProcessor` is an extensibility point for integrating technologies such as Terraform or custom operations
code into the orchestration process. For example, a Terraform processor would gather input data associated with the
orchestration and pass it to a Terraform script for execution. The `ActivityProcess` interface is defined as follows:

```
package api

type ActivityProcessor interface {
	Process(activityContext ActivityContext) ActivityResult
}

type ActivityResultType int

type ActivityResult struct {
	Result     ActivityResultType
	WaitMillis time.Duration
	Error      error
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
- **ActivityResultSchedule** - Schedules the message for redelivery as defined by `WaitMillis`. This can be used to
  implement a completion polling mechanism.
- **ActivityResultRetryError** - A recoverable error was raised and the message is negatively acknowledged so that it
  can be redelivered.
- **ActivityResultRetryError** - A fatal error was raised, the orchestration is put into the error state, and the
  message is acknowledged so it will not be redelivered.

## Activity Agents

An activity agent runs an activity executor in a dedicated process. A NATS-based agent framework is provided to
facilitate the creation of activity agents. The framework is built on the core modularity system, allowing services such
as the HTTP Client and Router to be used. An agent is instantiated by passing a configuration to the NATS agent
launcher:

```
package launcher

import (
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/api"
)

type LauncherConfig struct {
	AgentName        string
	ConfigPrefix     string
	ActivityType     string
	AssemblyProvider func() []system.ServiceAssembly
	NewProcessor     func(ctx *AgentContext) api.ActivityProcessor
}
natsagent.LaunchAgent(shutdown, config)

```

The `AssemblyProvider` function is used to register services required by the agent. The `NewProcessor` function is used
to instantiate the `ActivityProcessor` implementation.

The following is an example of an activity agent configuration:

```
package launcher

import (
	"github.com/eclipse-cfm/cfm/agent/edcv/activity"
	"github.com/eclipse-cfm/cfm/assembly/httpclient"
	"github.com/eclipse-cfm/cfm/assembly/serviceapi"
	"github.com/eclipse-cfm/cfm/assembly/vault"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/pmanager/natsagent"
)

func LaunchAndWaitSignal(shutdown <-chan struct{}) {
	config := natsagent.LauncherConfig{
		AgentName:    "Test Agent",
		ConfigPrefix: "test",
		ActivityType: "test-activity",
		AssemblyProvider: func() []system.ServiceAssembly {
			return []system.ServiceAssembly{
				&httpclient.HttpClientServiceAssembly{},
				&vault.VaultServiceAssembly{},
			}
		},
		NewProcessor: func(ctx *natsagent.AgentContext) api.ActivityProcessor {
			httpClient := ctx.Registry.Resolve(serviceapi.HttpClientKey).(http.Client)
			vaultClient := ctx.Registry.Resolve(serviceapi.VaultKey).(serviceapi.VaultClient)

			return &activity.TestActivityProcessor{
				HTTPClient:  &httpClient,
				VaultClient: vaultClient,
				Monitor:     ctx.Monitor,
			}
		},
	}
	natsagent.LaunchAgent(shutdown, config)
}
```

The above example relies on the HTTP Client, Vault, and Monitor services, passing them to the activity processor in the
`NewProcessor` function.

## Resource Lifecycles

Activities model resource lifecycles. For example, a resource may be deployed and undeployed. In many cases, it is not
desirable to require separate agents for each resource lifecycle operation. Activity processors can use the
`discriminator` property to distinguish between different resource lifecycle states:

```
package example

import "github.com/eclipse-cfm/cfm/pmanager/api"

func (p ExampleProcessor) Process(ctx api.ActivityContext) api.ActivityResult {
	if ctx.Discriminator() == api.DeployDiscriminator {
		// deploy the resource
	} else {
		// dispose the resource
	}
}
```

The activity can be configured with `deploy` and `dispose` orchestrations:

```json
{
  "type": "test.example.com",
  "description": "Test Deploy Orchestration",
  "active": true,
  "activities": [
    {
      "id": "test-activity",
      "type": "test",
      "discriminator": "deploy"
    }
  ]
}
```

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
