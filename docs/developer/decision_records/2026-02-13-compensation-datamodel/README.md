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

- introduction of an `OrchestrationTemplate`, that is used to generate `OrchestrationDefinition` objects. It contains a
  list of activities, grouped by an orchestration type/discriminator.
- one `OrchestrationDefinition` is created for each orchestration type, containing all relevant activities
- each activity is listed under its appropriate `orchestrationType`/`discriminator`, which is used as key in the map.
  While there are well-defined discriminator values (`cfm.orchestration.vpa.deploy` and
  `cfm.orchestration.vpa.dispose`), in the future these discriminators may be extended.
- necessary checks:
    - grouping by activity type, each activity `type` should be unique within each group. A warning is issued if
      violated. This check can be disabled with a configuration flag.
    - If an orchestration template does not contain at least one `...deploy` activity, a warning is issued.

The following snippet shows the `OrchestrationTemplate` object:

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
}
```

### Changes to the internal data model

When converting the `OrchestrationTemplate` DTO to the internal representation (`OrchestrationDefinition` entity),
the changes described above must be reflected there. When generating an `Orchestration` from an
`OrchestrationDefinition`, the following adaptations must be made:

- group all activities by discriminator
- create a new orchestration definition instance for each group
- when instantiating an `Orchestration` from an `OrchestrationDefinition`, the `Orchestration` entity gains the
  `compensationRefId` field to identify   `deploy` and `dispose` orchestrations. This is necessary to look up the
  rollback-orchestration if a deploy-orchestration fails (fatally).

These changes are illustrated in the following snippets:

```go
type Orchestration struct {
ID                string                     `json:"id"`
CorrelationID     string                     `json:"correlationId"`
//...
OrchestrationType model.OrchestrationType    `json:"orchestrationType"`
CompensationRefId    string                  `json:"compensationRefId" // <-- new field
}
```

The following pseudocode shows the conversion procedure:

```go
refId := uuid.New().String()
deployActivities := filterActivitiesByDiscriminator(definition.Activities, "cfm.orchestration.vpa.deploy")
deployOchestration := api.Orchestration{
//...
CompensationRefId: refId,
Activities: deployActivities,
Type:        "cfm.orchestration.vpa.deploy"
}

disposeActivities := filterActivitiesByDiscriminator(definition.Activities, "cfm.orchestration.vpa.dispose")
disposeOchestration := api.Orchestration{
//...
CompensationRefId: refId,
Activities: disposeActivities,
Type:        "cfm.orchestration.vpa.dispose"
}
```
