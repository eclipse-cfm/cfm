# Participant Updates

|                |                                                             |
|----------------|-------------------------------------------------------------|
| **Status**     | Proposed                                                    |
| **Subsystem**  | Provisioning, Tenant Manager                                |
| **Committers** | [@paullatzelsperger](https://github.com/paullatzelsperger/) |

## Overview

A participant profile is deployed once and can afterwards only be disposed. Everything that is decided at deployment
is fixed for the lifetime of the profile. A participant for which something changes has no path other than disposing
the profile and deploying a new one, which means a new participant context, a new wallet, and the loss of everything
bound to the old ones.

Many things can change during the life of a participant. Three examples are used throughout this document:

- its **roles** in a dataspace, which determine the credentials it is entitled to,
- its **holder properties** in the IssuerService, from which the claims of its credentials are taken,
- its **participant config** in the control plane.

This document proposes updates as a third lifecycle operation of a participant profile, next to deploy and dispose:

- A Tenant Manager operation changes the desired state of a deployed participant profile.
- A third orchestration type, `cfm.orchestration.vpa.update`, brings the downstream systems in line with it. The
  Provision Manager builds it from the update itself, without an orchestration definition.
- The existing activity agents handle it in a new `ProcessUpdate` method, next to `ProcessDeploy` and
  `ProcessDispose`. Each agent updates the part of the participant it already owns.

An update never touches the participant's identity, wallet, or participant context, and a failed update never
disposes the participant.

Two decisions are still open and are described in [Open Questions](#open-questions): whether update handlers belong
into the existing agents or into standalone update agents, and whether updates need ordering.

## Design Principles

1. **Unknown discriminators are ignored.** An activity processor that does not handle a discriminator completes the
   activity without doing anything. `BaseActivityProcessor` changes accordingly. Today it fails the activity.
2. **`participantContextId` is mandatory in the processing data** of an update orchestration. It is how every handler
   addresses the participant.
3. **Each update handler has its own data object.** Like a deploy handler, an update handler expects a data object of
   its own in the processing data and reads it with `ReadValues`.
4. **Update activities are independent.** An update orchestration has no definition and no dependencies between its
   activities. They run in parallel.
5. **Desired state, reconciled.** A request states what the profile should look like. A handler compares this with
   the downstream system and applies the difference, so running it twice has the same result as running it once.
6. **A failed update never disposes the participant.**

## Goals and Non-Goals

Goals:

- One mechanism for all changes to a deployed participant profile, with one API operation, one orchestration type, and
  one way to report progress and failure.
- New kinds of updates are added to the agent that owns the downstream system, without changes to the orchestration
  engine.
- An update that only runs the activities concerned by a change.
- A failed update that leaves a working participant in place and can be repeated.

Non-goals:

- Changing the identity of a participant: its identifier (DID), participant context ID, cell, or tenant.
- Orchestration definitions per dataspace profile, and support for more than one issuer. Updates inherit both
  limitations from deployment.
- Updates that originate in agent configuration instead of the participant profile, for example changed service
  endpoint templates of the IdentityHub agent.

## Current Behavior

The design builds on the following properties of the code base.

**Activities are dispatched by discriminator.** An orchestration template maps orchestration types to activity lists,
and the type becomes the `discriminator` of each of its activities. The executor
(`pmanager/natsorchestration/activity.go`) calls `ProcessDeploy` or `ProcessDispose` for the two known
discriminators and `Process` for any other. All agents embed `BaseActivityProcessor`, whose `Process` fails with
`the '...' discriminator is not supported`. Activities are routed to agents by activity type only.

**Every orchestration needs a definition.** `provisionManager.Start` (`pmanager/core/provision.go`) looks up the
definition of the manifest's orchestration type and fails with `orchestration type '...' not found` if there is none.
Definitions are unique per orchestration type. `InstantiateOrchestration` turns the activities of a definition into
steps by their dependencies; activities without dependencies form one parallel step.

**A profile has two kinds of mutable input.** `participantRoles` (dataspace profile ID to role names) are interpreted
only by the Tenant Manager: `generateCredentialSpecs` in `tmanager/core/service_participant.go` selects a
`CredentialSpec` of a dataspace profile if its `role` is empty or contained in the participant's roles. The roles
themselves are never sent to the agents. `vpaProperties` are stored per VPA type and read by the agents at
deployment:

| VPA type        | Property                                | Read by            |
|-----------------|-----------------------------------------|--------------------|
| `cfm.issuer`    | all properties, as holder properties    | registration agent |
| `cfm.issuer`    | `dataspaceIdentifier`                   | certo agent        |
| `cfm.connector` | `dataspaceProfiles`                     | edcv agent         |
| `cfm.dataplane` | `transferTypeMappings`, `authorization` | siglet agent       |

**A failed orchestration is compensated with a dispose.** The orchestration index watcher
(`pmanager/natsorchestration/watcher.go`) starts a dispose orchestration for every errored orchestration whose type is
not dispose, provided a dispose definition exists in the same template as the failed orchestration's definition. The
Tenant Manager accepts the response of such a compensation only if the orchestration it is waiting for is a deploy.

**Deployment state is nested in the profile.** The output values of the deploy orchestration, among them the
`participantContextId` generated by the jwtlet agent, are stored in the profile property `cfm.vpa.state`. The
participant context ID is derived from the ID of the deploy orchestration and cannot be recomputed in another
orchestration. Agents read it from the top level of the processing data, which is a copy of the top-level keys of the
manifest payload.

**There is one pending-orchestration slot and no lock.** The profile property `cfm.orchestration.pending` records the
orchestration the Tenant Manager last started and is used to match responses. It is not checked before a new
orchestration is started. The participant profile row is not locked by `DisposeProfile` or by the response handlers,
and `Update` does not compare versions.

## Design

### Tenant Manager API

A participant profile is updated with a new operation:

```
PATCH /api/v1alpha1/tenants/{tenantID}/participant-profiles/{participantID}
```

```json
{
  "participantRoles": {
    "<dataspace profile ID>": [
      "manufacturer",
      "supplier"
    ]
  },
  "vpaProperties": {
    "cfm.connector": {
      "config": {
        "edc.example.setting": "value"
      }
    }
  }
}
```

Both members are optional, and at least one must be present. A member that is absent leaves that part of the profile
unchanged.

- `participantRoles` is the complete desired role map and replaces the current one. A dataspace profile that is not
  listed has no roles afterwards.
- `vpaProperties` contains the complete desired properties of each VPA type listed and replaces the entire map of
  properties of that VPA. Properties that are not listed (e.g. `edc.example.setting2`) are assumed to be `nil` or
  default, as applicable. VPA types that are not listed are unchanged.

The operation requires the scope `tenant-manager-api:write` and applies the same tenant ownership rules as the other
participant profile operations.

Validation, rejected with `400`:

- The struct validation is the same as for a deployment.
- Every key of `participantRoles` must be one of the profile's dataspace profile IDs. A deployment does not check
  this today and silently ignores such keys.
- Every change the request causes must map to an update data object the Tenant Manager knows, see
  [Update Data Objects](#update-data-objects).

Preconditions, rejected with `409 Conflict`:

- All VPAs of the profile are `active`.
- No update of the profile is in progress.
- The stored deployment state contains a `participantContextId`.

Result:

`202 Accepted` with the participant profile. Clients read the `update` field to learn whether an orchestration was
started. If the request causes no change, or `participantRoles` differ without changing the credential entitlement,
the profile is stored directly, no orchestration is started and `update` is not set. Otherwise, the requested state is
applied to the profile when the update orchestration has completed.

The service implementation follows `DisposeProfile`: the profile is changed in a transaction, the manifest is sent
after the commit, and the change is reverted if the manifest cannot be sent. Unlike `DisposeProfile`, the profile is
read with `FindByIDAndLock(..., store.LockExclusive)`, see [Concurrency](#concurrency).

### Update Status

The VPAs of a profile stay `active` while an update is in progress and after an update has failed, because the
participant remains operational in both cases. The `error` flag of the profile is not set by a failed update for the
same reason: clients treat it as the end of a deployment.

The update is tracked in a new optional field of the participant profile instead:

```json
{
  "update": {
    "manifestId": "5f0c...",
    "state": "pending",
    "changes": [
      "cfm.update.roles",
      "cfm.update.participantconfig"
    ],
    "participantRoles": {
      "<dataspace profile ID>": [
        "manufacturer",
        "supplier"
      ]
    },
    "vpaProperties": {
      "cfm.connector": {
        "config": {
          "edc.example.setting": "value"
        }
      }
    },
    "errorDetail": "",
    "errorCause": ""
  }
}
```

- `state` is `pending` while the orchestration runs and `failed` after it has failed.
- `changes` lists the update data objects the update carries. The example is the update of the request above: the
  added role changes the credential entitlement and the connector config changes, so both data objects are sent.
- `participantRoles` and `vpaProperties` inside `update` are the requested values. The members of the same name on
  the profile and its VPAs always show what has been applied.
- `errorDetail` and `errorCause` have the same meaning and visibility as the fields of the same name on the profile.
- The field is removed when an update succeeds. A failed update stays visible until the next update is requested.

The field is stored in a new JSONB column of the `participant_profiles` table. It is not stored in `properties`,
which are returned unfiltered and share their namespace with caller-supplied values. `cfm.orchestration.pending` is
still set when the update is started, so that the existing response matching applies.

### Update Manifest

The Tenant Manager sends an `OrchestrationManifest` with the type `cfm.orchestration.vpa.update` and the profile ID as
correlation ID. The payload contains:

| Key                    | Content                                                                   |
|------------------------|---------------------------------------------------------------------------|
| `participantContextId` | Mandatory. Copied from the stored deployment state.                       |
| `cfm.participant.id`   | The participant identifier, as in deploy and dispose.                     |
| `cfm.update.*`         | One update data object per change, see below. Absent for unchanged parts. |

The payload deliberately contains neither the complete `cfm.vpa.data` nor the other entries of `cfm.vpa.state`. A
handler finds everything it needs in its own data object. In particular, the onboarding agent cannot confuse the
`holderPid` of the original credential request in the deployment state with a request of the update.

### Update Data Objects

Each kind of update has a data object under a key of its own, built by the Tenant Manager from the request and the
profile, and read by exactly one handler:

| Key                            | Built from                                       | Handled by         |
|--------------------------------|--------------------------------------------------|--------------------|
| `cfm.update.roles`             | `participantRoles` and the dataspace profiles    | onboarding agent   |
| `cfm.update.holder`            | the properties of the `cfm.issuer` VPA           | registration agent |
| `cfm.update.participantconfig` | the `config` property of the `cfm.connector` VPA | edcv agent         |

A data object is only present if its part of the profile changes. Each data object carries the complete desired state
of its part, so that the handler can reconcile, plus whatever the handler cannot derive from the desired state. The
Tenant Manager has one builder per data object. Adding a kind of update means adding a builder in the Tenant Manager,
a handler in an agent, and the declaration of the data object in the agent's activity definition.

### Activity Processing

`ActivityProcessor` gets a method for the new discriminator:

```go
package api

type ActivityProcessor interface {
	Process(activityContext ActivityContext) ActivityResult
	ProcessDeploy(activityContext ActivityContext) ActivityResult
	ProcessDispose(activityContext ActivityContext) ActivityResult
	ProcessUpdate(activityContext ActivityContext) ActivityResult
}
```

```go
package model

const VPAUpdateType OrchestrationType = "cfm.orchestration.vpa.update"
```

```go
package api

const UpdateDiscriminator = Discriminator(model.VPAUpdateType)
```

The executor dispatches `UpdateDiscriminator` to `ProcessUpdate`. `BaseActivityProcessor` implements both `Process`
and `ProcessUpdate` by completing the activity:

```go
package api

func (p BaseActivityProcessor) Process(ctx ActivityContext) ActivityResult {
	return ActivityResult{Result: ActivityResultComplete}
}

func (p BaseActivityProcessor) ProcessUpdate(ctx ActivityContext) ActivityResult {
	return ActivityResult{Result: ActivityResultComplete}
}
```

An agent without an update handler is therefore a no-op for updates, and every agent in this repository compiles
unchanged. Processors that do not embed `BaseActivityProcessor` need the additional method. These are the test
processors in `pmanager/natsorchestration`, `pmanager/natsagent`, and `e2e/testagent`. Tests that assert the
`not supported` error of `BaseActivityProcessor` change.

An agent that handles updates implements `ProcessUpdate`. The data object is read with `ReadValues` into a struct of
the handler's own, as deploy handlers do:

```go
package onboarding

type roleUpdateData struct {
	ParticipantContextID string     `json:"participantContextId" validate:"required,participantcontextid"`
	ParticipantID        string     `json:"cfm.participant.id" validate:"required"`
	Update               roleUpdate `json:"cfm.update.roles" validate:"required"`
}

func (p OnboardingActivityProcessor) ProcessUpdate(ctx api.ActivityContext) api.ActivityResult {
	var data roleUpdateData
	if err := ctx.ReadValues(&data); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}
	// reconcile ...
}
```

A handler:

- fails with a fatal error if its data object is missing or invalid. It is only started for an update that carries
  its data object, see [Building the Update Orchestration](#building-the-update-orchestration);
- reconciles: it must be safe to run again after a partial failure, and in a later update orchestration;
- keeps values it needs across redeliveries under keys of its own;
- reports a failure that may be transient as `ActivityResultRetryError`, while the existing handlers report every
  downstream error as fatal;
- never calls the agent's dispose logic and never deletes the participant's resources.

### Building the Update Orchestration

The update orchestration has no orchestration definition. `provisionManager.Start` builds it from the manifest
instead of looking up a definition.

An activity definition declares the update data objects its agent handles, in a new optional field:

```json
{
  "type": "registration-activity",
  "description": "Registers the participant as a holder in the IssuerService",
  "updates": [
    "cfm.update.holder"
  ]
}
```

For a manifest of type `cfm.orchestration.vpa.update`, `Start`:

1. collects the `cfm.update.*` keys of the payload;
2. finds, for each key, the activity definition that declares it. A key that no activity definition declares, or that
   more than one declares, fails the start with a client error, which reaches the Tenant Manager as a failed update;
3. creates one activity per key, with the activity type of that definition, the discriminator
   `cfm.orchestration.vpa.update`, and no dependencies;
4. instantiates the orchestration with these activities and a fixed definition ID, `cfm.orchestration.vpa.update`, and
   executes it as usual.

All activities form one parallel step. De-duplication by manifest ID, redelivery, retries, and the response to the
Tenant Manager work as for every other orchestration.

Only activities concerned by the update are started. An agent that is not involved receives no message, and an
activity type whose agent is not deployed cannot keep an update pending, as long as no activity definition declares
a data object for it.

`ActivityDefinition` (`pmanager/api/types.go`), its API representation, and its SQL table get the `updates` field. The
field is not interpreted for deploy and dispose.

### Ordering

Update activities do not depend on one another and run in parallel. The runtime does not know what an update does
and does not order data objects. A request that carries changes depending on each other may therefore run into race
conditions. For example, an issuer may evaluate holder properties when it issues a credential, so a credential
requested in the same update as a holder properties change may be issued from the old properties, or fail its
attestation. A
client that needs a particular order sends one update per step and waits for each to complete.

Whether this is sufficient is an open question, see [Do Updates Need Ordering?](#do-updates-need-ordering).

### Tenant Manager Response Handling

A handler for `cfm.orchestration.vpa.update` is registered next to the deploy and dispose handlers. A response for an
orchestration type without a handler is dropped, so the handler is also what makes a failed start visible.

- On success, the requested roles and VPA properties are applied to the profile and the `update` field is removed.
- On failure, the `update` field is set to `failed` with the error detail and cause of the response. The profile is
  not changed otherwise.
- In both cases `cfm.orchestration.pending` is removed, VPA states and the `error` flag are left alone, and
  `cfm.vpa.state` is kept. Output values of the update orchestration are not merged into it.

### No Disposal on Failure

A failed update, or a failed activity of an update, never leads to a dispose of the participant. A dispose is the
end of a participant and is only ever started by an explicit dispose request or as the compensation of a failed
deploy, where there is no working participant to lose.

Today the orchestration index watcher compensates every errored orchestration that is not a dispose. For an update
orchestration this has two effects: with a dispose definition found, it disposes a working participant whose profile
then keeps reporting `active` VPAs, because the Tenant Manager ignores the response. Without a stored definition for
the update, which is the case in this design, the watcher's definition lookup fails, and the watcher retries the
failed orchestration without end.

The condition in the watcher therefore changes from "not a dispose" to "a deploy":

```go
if orchestration.State == api.OrchestrationStateErrored && orchestration.OrchestrationType == model.VPADeployType {
```

This also covers orchestration types added in the future: no type other than deploy is compensated unless it is
explicitly opted in.

Within the update orchestration, the guarantee rests on the handler contract, since update handlers live in the same
agents as dispose handlers: an update handler never calls dispose logic and never deletes resources, and the Tenant
Manager's update response handler never changes VPA states. A failed update leaves the participant as it was, or
partially updated, and in both cases operational. Repairing a partial update is the job of the next update, see
[Failure Handling](#failure-handling).

### Concurrency

The Tenant Manager must not run two orchestrations for one profile at a time, and the Provision Manager does not
prevent it.

- `UpdateProfile` reads the profile with an exclusive row lock and rejects the request unless the preconditions hold.
- `DisposeProfile` takes the same lock and rejects a profile with a `pending` update. Otherwise a dispose and an
  update could run concurrently, and the update could, for example, request a credential after the dispose has
  revoked the participant's credentials.
- The update response handler takes the same lock.

## Failure Handling

A failed update leaves the profile with its previous roles and VPA properties, `active` VPAs, and an `update` field in
state `failed`. The downstream systems may be in an intermediate state: activities run in parallel, and the others
finish when one fails, so some changes may be applied and others not.

The client resolves this by requesting an update again, either with the same values or with the previous ones. Both
converge, because handlers reconcile against the downstream system. Three rules guarantee that:

- While an `update` field in state `failed` exists, its data objects are included in the next update, built from the
  values of the new request. A request for the previous values would otherwise cause no change, although a handler
  has already applied the failed one.
- For the same reason, the entitlement used to compute the credentials to revoke is then the union of the specs
  selected by the applied roles and by the roles of the failed update.
- `DisposeProfile` builds `cfm.vpa.credentials` from the same union. Dispose recomputes the credentials to revoke from
  the roles of the profile, and would otherwise miss a credential that a failed update has issued.

An update whose response never arrives stays `pending`. This is not specific to updates: the Tenant Manager has no
timeout for any orchestration, and a manifest that cannot be started after ten deliveries is discarded without a
response.

## Examples

The three examples from the overview are sketched here to show that the model carries them and what each needs from
its downstream system. Each needs its own detailed design before it is built. They are independent of each other and
can be delivered in any order.

### Roles

|                 |                                                   |
|-----------------|---------------------------------------------------|
| **Agent**       | onboarding (`onboarding-activity`)                |
| **Data object** | `cfm.update.roles`                                |
| **Downstream**  | IssuerService admin API, IdentityHub identity API |

```json
{
  "cfm.update.roles": {
    "credentials": [
      {
        "id": "...",
        "type": "ManufacturerCredential",
        "issuer": "did:web:...",
        "format": "VC1_0_JWT",
        "role": "manufacturer"
      }
    ],
    "revoked": [
      {
        "id": "...",
        "type": "SupplierCredential",
        "issuer": "did:web:...",
        "format": "VC1_0_JWT",
        "role": "supplier"
      }
    ]
  }
}
```

`credentials` is the complete entitlement with the requested roles. `revoked` holds the specs that the participant
was entitled to and no longer is, compared by credential type, because revocation in the IssuerService is by type and
would otherwise revoke a credential that another spec still grants.

1. For each spec in `revoked`, query the holder's credentials of that type in the IssuerService and revoke each of
   them. This is the revocation logic of `ProcessDispose`.
2. For each spec in `credentials`, query the IssuerService for a credential of that type that the holder has and that
   is not revoked. The specs without such a credential are missing.
3. Send one credential request for the missing specs through the IdentityHub of the participant, keep the request ID
   under a key of its own, and poll the request until it is issued, as `ProcessDeploy` does.

Notes:

- Step 2 requires the credential query to filter by state, which the client does not do today. The IssuerService can
  filter its credential query by `state`. This has been checked against the current IdentityHub sources and has to be
  confirmed for the IssuerService version a platform ships.
- Revoking an already revoked credential succeeds in the IssuerService, so step 1 can be repeated.
- A credential request addresses one issuer. Specs of more than one issuer fail the update as they fail a deployment.
- Revoked credentials are not removed from the participant's wallet. This matches the behavior on dispose.

### Holder Properties

|                 |                                        |
|-----------------|----------------------------------------|
| **Agent**       | registration (`registration-activity`) |
| **Data object** | `cfm.update.holder`                    |
| **Downstream**  | IssuerService admin API                |

```json
{
  "cfm.update.holder": {
    "properties": {
      "legalName": "Example Corp"
    },
    "removed": [
      "formerKey"
    ]
  }
}
```

1. Read the holder of the participant from the IssuerService.
2. Set the desired properties and remove the removed keys.
3. Write the holder back.

Notes:

- The IssuerService has no partial update for holders. The holder is replaced with its DID, name, and properties, so
  the handler reads, merges, and writes without atomicity. The issuer client in `agent/common` needs read and replace
  calls, which the tolerant holder registration needs as well.
- `removed` is computed by the Tenant Manager from the previously applied properties. Properties the profile never
  contained, for example ones written by a registration desk, are left alone.
- Changed properties affect credentials issued afterwards, not existing ones.
- The registration agent can serve updates on a platform that does not register holders during deployment: its
  activity definition only has to exist, and its activity does not have to be part of the deploy definition.

### Participant Config

|                 |                                |
|-----------------|--------------------------------|
| **Agent**       | edcv (`edcv-activity`)         |
| **Data object** | `cfm.update.participantconfig` |
| **Downstream**  | Control plane management API   |

```json
{
  "cfm.update.participantconfig": {
    "entries": {
      "edc.example.setting": "value"
    }
  }
}
```

1. Patch the participant context config in the control plane with the entries.

Notes:

- Config entries are not a profile input today. The `config` property of the `cfm.connector` VPA is new.
- The client call exists (`PatchConfig`) and already distinguishes recoverable from permanent errors.
- The edcv agent at deployment and the key management agent write entries of the same config, without coordination.
  The Tenant Manager rejects requests that set the keys they own (`edc.participant.id`, `edc.iam.issuer.id`,
  `edc.vault.hashicorp.config`, and the `edc.iam.sts.*` entries). This is safe only if the control plane merges
  patched entries, which is not verified.
- A patch cannot remove an entry. Whether removal is needed, and how it is expressed, is open.

### Further Candidates

- **Dataspace profile associations** of the connector, in the edcv agent. The control plane call exists, but has no
  path for removing the last association.
- **Transfer type mappings and authorization** of the data plane, in the siglet agent. The deploy logic is an upsert
  and reusable. Removal is missing, and a stored mapping replaces the participant's whole set, which conflicts with
  other writers.
- **Dataspace identifier** in Certo, in the certo agent. Certo is only called to create and delete today.

## Phases

1. **Framework.** API, update status, manifest, `ProcessUpdate`, the change to `BaseActivityProcessor`, the
   `updates` field of activity definitions, building the update orchestration, response handling, and the
   compensation change. An end-to-end test proves the flow with the test agent.
2. **Handlers.** One at a time, each with its detailed design, its data object builder in the Tenant Manager, its
   handler, and the declaration in its activity definition.

## Compatibility and Rollout

- The API change is additive. Profiles without an `update` field are unaffected.
- The change to `BaseActivityProcessor` affects every agent built on it, also outside this repository: an activity
  with a discriminator the agent does not handle now completes instead of failing. Today this only happens with a
  misconfigured template.
- The `ActivityProcessor` interface change breaks processors outside this repository that do not embed
  `BaseActivityProcessor` at compile time.
- Activity definitions without `updates` behave as before. An agent receives update activities only once its activity
  definition declares a data object, so a platform enables an update by deploying the agent version with the handler
  and then extending its activity definition.
- The Provision Manager change to compensation and the new Tenant Manager handler have to be deployed before any
  activity definition declares a data object.
- Tenant Manager replicas share one consumer for orchestration responses. During a rolling upgrade, a replica without
  the update handler drops an update response. Updates should not be requested before all replicas are upgraded.
- An orchestration can also be started through the Provision Manager API by a caller with the scope
  `provision-manager-api:write`. Such a caller can already start deploy and dispose orchestrations with an arbitrary
  payload, and gains the update orchestration.

## Alternatives Considered

**Make the deploy orchestration aware of existing participants.** Every activity of the deploy orchestration would
have to become a create-or-update operation, whatever the change. The participant context ID would have to be passed
in instead of derived, a request could not express a revocation, and a failure would run the dispose compensation
against a working participant. Making create operations tolerate existing resources is still worthwhile, because
activity messages are redelivered, but as a robustness measure and independent of updates.

**Updates as events for lifecycle agents.** An update could be published as an event and handled by lifecycle agents.
While the payload could be transmitted in event data, lifecycle agents report no outcome, so it would be impossible to
track an update result.

**One orchestration type per kind of update.** It multiplies response handlers, and a request that changes two things
would need two orchestrations and a rule for their partial failure.

**Compensate a failed update with an inverse update.** The inverse can fail as well, and it hides the failure from the
client, which then sees neither the requested nor a reliably restored state. Reporting the failure and letting the
client repeat or revert the update is simpler and converges for the same reason.

**Enqueue every registered activity type for every update.** It needs no declaration in activity definitions, but an
activity type whose agent is not deployed would leave every update pending, because nothing consumes its message.

## Open Questions

### `PATCH` vs `PUT`?

`PATCH` performs a partial update, e.g. modifying a single property value, whereas `PUT` modifies the _entire object_.
In practice, this would have the following implications:

- `PATCH`: no _read_ beforehand, simply send the modified value. Deleting a value requires a dedicated contract, such
  as listing the fields under dedicated `added` or `removed` objects, as the tenant properties do today, or naively
  setting the value to `null` in JSON:
  ```json
  {
    "key1": "newValue",
    "key2": null
  }
  ```
  `key1` is updated to `newValue`, and `key2` is deleted, regardless of its type.

- `PUT`: requires a _read_ beforehand, to have the latest data, then modifying a value, then sending _all_ values back.
  ```json
  {
    "key1": "newValue",
    "key2": "oldValue",
    "key3": "anotherOldValue",
    "...": "..."
  }
  ```
  The read-modify-write happens in the client, so two clients that update the same profile can overwrite each other's
  changes. The row lock of `UpdateProfile` only protects the Tenant Manager's side. Preventing lost updates requires
  a version or ETag on the profile that the client sends back and the Tenant Manager checks.

### Integrated `ProcessUpdate` or Standalone Update Activities

This document integrates update handlers into the existing activity agents through a new handler `ProcessUpdate`. The
alternative is standalone update activities: a third agent category under `agent/update`, next to `agent/orchestration`
and
`agent/lifecycle`, with one agent and one activity type per kind of update, for example `holder-update-activity`. The
rest of this design applies to both, since the Provision Manager finds the activity through the `updates` declaration
of its activity definition either way.

Integrated `ProcessUpdate`:

- **Pro:** no new deployables, images, configuration, NATS users, or token exchange mappings. An update is enabled by
  updating an agent that is already deployed.
- **Pro:** the update handler reuses the clients, configuration, scopes, and error handling of the agent that already
  owns the downstream system, and code for one downstream system stays in one place.
- **Pro:** update is a lifecycle operation of the same resource as deploy and dispose, and is modelled the same way.
  For example, revoking credentials could be modelled as an update, without requiring a dedicated new agent.
- **Con:** update code runs in the same process as dispose code. The guarantee that an update never deletes the
  participant's resources rests on review of the handler, not on the absence of dispose code.
- **Con:** the `ActivityProcessor` interface changes, and every processor that does not embed `BaseActivityProcessor`
  has to implement `ProcessUpdate`.
- **Con:** an update that has no deploy counterpart has to be placed in an agent whose deploy logic is unrelated, or
  needs a new orchestration agent with a deploy handler that does nothing.
- **Con:** an agent is released, scaled, and operated as one unit for all three operations. A fix to an update handler
  means a new release of an agent that also deploys participants.

Standalone update activities:

- **Pro:** a clear separation: update agents cannot create or delete participants because they have no such code.
- **Pro:** the `ActivityProcessor` interface does not change. An update agent implements `Process` or a small
  `UpdateProcessor` interface through a launcher of its own.
- **Pro:** each kind of update is developed, released, and deployed on its own, and a platform runs exactly the update
  agents it needs.
- **Pro:** an update without a deploy counterpart has a natural home.
- **Con:** every kind of update is a new deployable with its own image, configuration, NATS user, and service account
  mappings.
- **Con:** the clients and configuration of the agent that owns the downstream system are duplicated, for example the
  issuer service URL and issuer ID in a holder update agent and the registration agent.
- **Con:** two agents write to the same downstream resource, for example the registration agent creates the holder and
  a holder update agent changes it, which spreads the knowledge about that resource.

### Do Updates Need Ordering?

This document assumes that they do not: update activities are independent, the update orchestration has no
definition, and a client that needs an order sends separate updates, see [Ordering](#ordering).

The case that challenges this assumption is a change of roles. The issuer's attestation pipeline reads the holder
properties, so a new entitlement usually requires changed holder properties before the new credential can be issued.
The holder change is not an exception but the precondition of the role change. Under this design, the client has to
split one logical change into two updates, holder first and roles second, wait for the first to complete in between,
and handle a failure of the second while the first has already been applied. Sent as one update, both handlers start
at the same time, and whether the credential request sees the new holder properties depends on timing. The holder
write is a single synchronous call and the credential request is processed asynchronously, so the request will often
see the new properties, which makes the failure rare and hard to reproduce.

This makes a case for expressing dependencies between updates:

- The dependency is a fact of the platform, not of a single request. Every client that changes roles has to know about
  it and implement the same two-step sequence, and a client that does not know about it works most of the time.
- A single update with an order is applied or failed as a whole and reported in one `update` status. Two updates
  produce two statuses and an intermediate state the client has to recover from.
- The runtime must not learn what an update does. A dependency therefore has to be declared by whoever configures the
  platform, not derived by the Tenant Manager or the Provision Manager from the kinds of change.
- The engine already orders activities with `dependsOn` in an orchestration definition. Expressing update dependencies
  there reuses that mechanism, while ordering declared on data objects would add a second one to maintain.

- **If updates do not need ordering**, the design stays as described. Each kind of update is started for its data
  object alone, the Provision Manager builds the orchestration, and a platform enables an update by declaring the data
  object in an activity definition.
- **If updates need ordering**, the update orchestration gets a dedicated orchestration definition, like deploy and
  dispose. It lists the update activities with `dependsOn`, for example the holder update before the credential
  update. The `updates` declaration then selects which activities of the definition are relevant to a manifest, and
  the others complete immediately, or the declaration is dropped and every activity of the definition runs and
  completes immediately when its data object is absent. The definition has to be seeded and extended like the deploy
  definition. The definition store has no operation to change a stored definition, and its two implementations treat
  a repeated post differently, which makes extending it on a running platform awkward. The watcher's compensation
  lookup works again, but the compensation change stays necessary, because a dispose definition in the same template
  would otherwise dispose the participant.

Further questions:

- How does the executor tell an ignored activity from a handled one, for logging and possibly for metrics?
- Should a `pending` update block a dispose without limit, or should an administrator be able to reset or supersede
  it?
- Should the update status be a field of its own, as proposed, or should a failed update set the `error` flag of the
  profile while the VPAs stay `active`?
- Should a changed holder property lead to the reissuance of existing credentials?
- Should the dataspace profiles of a participant become updatable? Joining another dataspace changes the credential
  entitlement and the connector's profile associations together, and today the two are separate inputs.

## Related Issues

The following were found while writing this design. They are independent of it, but share code with it.

- A dispose requested through the Tenant Manager passes the deployment state only nested under `cfm.vpa.state`, while
  every agent reads `participantContextId` from the top level of the processing data. Only the dispose started as
  compensation receives it there. By reading the code, a requested dispose fails in the agents with a validation
  error. This has not been run. The end-to-end test does not show it, because its test agent reads nothing on dispose.
  Making `participantContextId` mandatory at the top level, as this design does for updates, would apply to dispose as
  well.
- The dispose response handler sets all VPAs to `disposed` also when the dispose has failed.
- `error`, `errorDetail`, and `errorCause` of a profile are never cleared.
