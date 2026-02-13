# Changes to the Datamodel for Compensation

## Decision

This record describes necessary changes to the data model to accommodate the _compensation_ feature. Refer to
the [documentation](../../compensation.md) for details.

## Rationale

One goal is to keep the API as simple as possible while simultaneously good user feedback in case of an error. Another
goal is to maintain a high degree of flexibility. For example, it should be possible to define [0...N] activities that
perform cleanup actions.

At the same time, the internal data model must be kept as flexible as possible, and the following aspects must be kept
in mind:

- querying in the NATS KV store is very limited
- orchestration instances should be "pure," i.e., only contain activities of one discriminator. Multiple orchestrations
  must be generated if an `OrchestrationDefinition` contains activities of different discriminators.
- "deploy" and "dispose" orchestrations must reference each other to simplify lookup

## Approach

### Changes to the API data model

On the `OrchestrationDefinition` API, the following changes must be made:

- there is only one `OrchestrationDefinition` for both deploy and dispose orchestration definition
- the `OrchestrationDefinition` loses the `type` field
- each activity's `discriminator` field contains a unique and well-defined discriminator value
- necessary checks:
    - grouping by activity type, there should be exactly one `...deploy` and 1...N `...dispose` activities. A warning is
      issued if violated. This check can be disabled with a configuration flag.
    - If an orchestration definition does not contain at least one `...deploy` activity, a warning is issued.

The following snippet illustrates the changes to the `OrchestrationDefinition` API data model:

```json
{
  "activities": {
    "cfm.orchestration.vpa.deploy": [
      {
        "id": "kc-client-create",
        "type": "keycloak-activity",
        "dependsOn": []
      },
      {
        "id": "connector-create",
        "type": "edcv-activity",
        "dependsOn": [
          "kc-client-create"
        ]
      }
    ],
    "cfm.orchestration.vpa.dispose": [
      {
        "id": "kc-client-rollback",
        "type": "keycloak-activity",
        "dependsOn": []
      },
      {
        "id": "connector-cleanup",
        "type": "edcv-activity",
        "dependsOn": [
          "kc-client-rollback"
        ]
      }
    ],
    "description": "Rolls back the deployment of a new dataspace member",
    "schema": {},
    "id": "random-orchestration-id"
  }
```

### Changes to the internal data model

When converting the `OrchestrationDefinition` internal data model must reflect the changes described above. When
generating an `Orchestration` from an `OrchestrationDefinition`, the following adaptations must be made:

- group all activities by discriminator
- create a new orchestration definition instance for each group
- when instantiating an `Orchestration` from an `OrchestrationDefinition`, the `Orchestration` entity gains the
  `compensationId` field to identify   `deploy` and `dispose` orchestrations. This is necessary to look up the
  rollback-orchestration if a deploy-orchestration fails (fatally).

These changes are illustrated in the following snippets:

```go
type Orchestration struct {
ID                string                  `json:"id"`
CorrelationID     string                  `json:"correlationId"`
//...
OrchestrationType model.OrchestrationType `json:"orchestrationType"`
CompensationId    string                  `json:"compensationId" // <-- new field
}
```

The following pseudocode shows the conversion procedure:
```go
deployActivities := filterActivitiesByDiscriminator(definition.Activities, "cfm.orchestration.vpa.deploy")
disposeActivities := filterActivitiesByDiscriminator(definition.Activities, "cfm.orchestration.vpa.dispose")

id := uuid.New().String()

deployOchestration := api.Orchestration{
  //...
  CompensationId: id,
  Activities: deployActivities,
  Type:        "cfm.orchestration.vpa.deploy"
}

disposeOchestration := api.Orchestration{
  //...
  CompensationId: id,
  Activities: disposeActivities,
  Type:        "cfm.orchestration.vpa.dispose"
}
```