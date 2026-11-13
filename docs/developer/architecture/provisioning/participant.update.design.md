# Participant Profile Updates

|                |                                                             |
| -------------- | ----------------------------------------------------------- |
| **Status**     | Proposed                                                    |
| **Subsystem**  | Provisioning, Tenant Manager                                |
| **Committers** | [@paullatzelsperger](https://github.com/paullatzelsperger/) |

## Overview

A participant profile is deployed once and can afterwards only be disposed. Everything that is decided at deployment
is fixed for the lifetime of the profile: the participant roles, which determine the credentials a participant is
entitled to, and the VPA properties, from which the agents configure the holder in the IssuerService, the connector,
and the data plane. A participant for which any of this changes has no path other than disposing the profile and
deploying a new one, which means a new participant context, a new wallet, and the loss of everything bound to the old
ones.

This document proposes updates as a third lifecycle operation of a participant profile, next to deploy and dispose:

- A Tenant Manager operation changes the desired state of a deployed participant profile.
- A third orchestration type, `cfm.orchestration.vpa.update`, brings the downstream systems in line with it.
- The existing activity agents process it through a new update handler. Each agent reconciles the part of the
  participant it already owns.

The participant's identity, wallet, and participant context are never touched by an update, and a failed update never
disposes the participant.

The mechanism is general. What can be updated grows with the agents that implement an update handler, see
[Update Handlers](#update-handlers). The first handler, specified in detail here, is the change of participant roles.

## Goals and Non-Goals

Goals:

- One mechanism for all changes to a deployed participant profile, with one API operation, one orchestration type, and
  one way to report progress and failure.
- An update that only runs the agents concerned by a change.
- A failed update that leaves a working participant in place, never triggers a dispose, and can be repeated.
- As the first use: change `participantRoles`, issue the credentials the participant becomes entitled to, and revoke
  the credentials it is no longer entitled to.

Non-goals:

- Changing the identity of a participant: its identifier (DID), participant context ID, cell, or tenant.
- Orchestration definitions per dataspace profile, and support for more than one issuer. The update orchestration
  inherits both limitations from the deploy orchestration.
- Updates that originate in agent configuration instead of the participant profile, for example changed service
  endpoint templates of the IdentityHub agent.

## Current Behavior

The design builds on the following properties of the code base.

**A profile has two kinds of mutable input.** `participantRoles` (dataspace profile ID to role names) are interpreted
only by the Tenant Manager: `generateCredentialSpecs` in `tmanager/core/service_participant.go` selects a
`CredentialSpec` of a dataspace profile if its `role` is empty or contained in the participant's roles. The result is
sent as `cfm.vpa.credentials`; the roles themselves do not exist as a first-class concept. Rather, they define which 
credentials are available to a participant. `vpaProperties` are stored per VPA type and sent
as `cfm.vpa.data`. Each agent reads the properties of one VPA type:

| VPA type        | Property                                | Read by            |
| --------------- | --------------------------------------- | ------------------ |
| `cfm.issuer`    | all properties, as holder properties    | registration agent |
| `cfm.issuer`    | `dataspaceIdentifier`                   | certo agent        |
| `cfm.connector` | `dataspaceProfiles`                     | edcv agent         |
| `cfm.dataplane` | `transferTypeMappings`, `authorization` | siglet agent       |

The jwtlet and IdentityHub agents read no profile input.

**The orchestration engine is keyed by orchestration type.** An orchestration template maps orchestration types to
activity lists. One `OrchestrationDefinition` is stored per type, and the type becomes the `discriminator` of each of
its activities. Activities are routed to agents by activity type only. The executor calls `ProcessDeploy` or
`ProcessDispose` for the two known discriminators and `Process` for any other. All agents embed
`BaseActivityProcessor`, whose `Process` fails with `the '...' discriminator is not supported`.

**A failed orchestration is compensated with a dispose.** The orchestration index watcher
(`pmanager/natsorchestration/watcher.go`) starts a dispose orchestration for every errored orchestration whose type is
not dispose, provided a dispose definition exists in the same template. The Tenant Manager accepts the response of
such a compensation only if the orchestration it is waiting for is a deploy.

**Deployment state is nested in the profile.** The output values of the deploy orchestration, among them the
`participantContextId` generated by the jwtlet agent, are stored in the profile property `cfm.vpa.state`. The
participant context ID is derived from the ID of the deploy orchestration and cannot be recomputed in another
orchestration.

**There is one pending-orchestration slot and no lock.** The profile property `cfm.orchestration.pending` records the
orchestration the Tenant Manager last started and is used to match responses. It is not checked before a new
orchestration is started. The participant profile row is not locked by `DisposeProfile` or by the response handlers,
and `Update` does not compare versions.

**Running deploy a second time is not an option.** A second deploy orchestration would derive a new participant
context ID, the Certo and holder registration activities accept only a `201 Created` and fail for a resource that
exists, the onboarding activity would request every credential again, and nothing would revoke a credential. The
Tenant Manager's deploy response handler replaces `cfm.vpa.state` and sets all VPAs to `error` on failure.

## Update Model

An update follows three rules.

1. **Desired state.** A request states what the profile should look like, not what to do. The update manifest carries
   the complete desired state of the participant, in the same shape as a deploy manifest.
2. **Changes select the work.** The Tenant Manager compares the request with the profile and lists what changed. An
   update handler acts only if a change concerns it, and completes immediately otherwise.
3. **Handlers reconcile.** A handler compares the desired state with the downstream system and applies the
   difference. Running it twice has the same result as running it once. This is what makes a failed update repeatable
   and removes the need for a rollback.

A change is identified by the payload key or VPA type it affects:

| Change                | Caused by                                                             |
| --------------------- | --------------------------------------------------------------------- |
| `cfm.vpa.credentials` | `participantRoles` changed so that the credential entitlement differs |
| `cfm.issuer`          | the properties of the `cfm.issuer` VPA changed                        |
| `cfm.connector`       | the properties of the `cfm.connector` VPA changed                     |
| `cfm.dataplane`       | the properties of the `cfm.dataplane` VPA changed                     |

## Design

### Orchestration Type and Discriminator

A new orchestration type and the matching discriminator are introduced:

```go
package model

const VPAUpdateType OrchestrationType = "cfm.orchestration.vpa.update"
```

```go
package api

const UpdateDiscriminator = Discriminator(model.VPAUpdateType)
```

The definition store, the DAG instantiation, the NATS subjects, and the NATS permissions need no change, because none
of them depends on the orchestration type.

### Tenant Manager API

A participant profile is updated with a new operation:

```
PATCH /api/v1alpha1/tenants/{tenantID}/participant-profiles/{participantID}
```

```json
{
  "participantRoles": {
    "<dataspace profile ID>": ["manufacturer", "supplier"]
  },
  "vpaProperties": {
    "cfm.issuer": {
      "legalName": "Example Corp"
    }
  }
}
```

Both members are optional, and at least one must be present. A member that is absent leaves that part of the profile
unchanged.

- `participantRoles` is the complete desired role map and replaces the current one. A dataspace profile that is not
  listed has no roles afterwards.
- `vpaProperties` contains the complete desired properties of each VPA type listed and replaces the properties of
  that VPA. VPA types that are not listed are unchanged.

The operation requires the scope `tenant-manager-api:write` and applies the same tenant ownership rules as the other
participant profile operations.

Validation, rejected with `400`:

- The struct validation is the same as for a deployment.
- Every key of `participantRoles` must be one of the profile's dataspace profile IDs. A deployment does not check
  this today and silently ignores such keys.
- Every change the request causes must be updatable. The Tenant Manager keeps the list of updatable changes, which
  initially contains `cfm.vpa.credentials` only and is extended with each update handler that is added. Without this
  check the profile could be changed without any agent applying the change.

Preconditions, rejected with `409 Conflict`:

- All VPAs of the profile are `active`.
- No update of the profile is in progress.

Result:

- `200 OK` with the participant profile, if the request causes no change. If `participantRoles` differ without
  changing the credential entitlement, they are stored. No orchestration is started.
- `202 Accepted` with the participant profile otherwise. The requested state is applied to the profile when the
  update orchestration has completed.

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
  "participantRoles": { "<dataspace profile ID>": ["manufacturer"] },
  "update": {
    "manifestId": "5f0c...",
    "state": "pending",
    "changes": ["cfm.vpa.credentials"],
    "participantRoles": { "<dataspace profile ID>": ["manufacturer", "supplier"] },
    "errorDetail": "",
    "errorCause": ""
  }
}
```

- `state` is `pending` while the orchestration runs and `failed` after it has failed.
- `changes` lists what the update changes, see [Update Model](#update-model).
- `participantRoles` and `vpaProperties` inside `update` are the requested values. The members of the same name on
  the profile and its VPAs always show what has been applied.
- `errorDetail` and `errorCause` have the same meaning and visibility as the fields of the same name on the profile.
- The field is removed when an update succeeds. A failed update stays visible until the next update is requested.

The field is stored in a new JSONB column of the `participant_profiles` table. It is not stored in `properties`,
which are returned unfiltered and share their namespace with caller-supplied values. `cfm.orchestration.pending` is
still set when the update is started, so that the existing response matching applies.

### Update Manifest

The Tenant Manager sends an `OrchestrationManifest` with the orchestration type `cfm.orchestration.vpa.update` and the
profile ID as correlation ID. The payload contains:

| Key                            | Content                                                                       |
| ------------------------------ | ----------------------------------------------------------------------------- |
| `cfm.participant.id`           | The participant identifier, as in deploy and dispose.                         |
| `cfm.vpa.data`                 | The VPA manifests with the requested properties.                              |
| `cfm.vpa.credentials`          | The credential specs the participant is entitled to with the requested roles. |
| `cfm.vpa.credentials.revoked`  | The credential specs the participant was entitled to and no longer is.        |
| `cfm.vpa.changes`              | The list of changes, for example `["cfm.vpa.credentials", "cfm.issuer"]`.     |
| `cfm.vpa.state`                | The stored deployment state, as in dispose.                                   |
| the entries of `cfm.vpa.state` | Copied to the top level of the payload, in particular `participantContextId`. |

`cfm.vpa.data` and `cfm.vpa.credentials` have the same meaning as in a deployment: they describe the complete desired
state, not a difference. A handler therefore reads its input exactly as its deploy handler does.

`cfm.vpa.credentials.revoked` is the one place where the manifest carries a difference, because the entitlement that
is no longer wanted cannot be derived from the desired state. It contains the specs selected by the applied roles
whose credential `type` is not selected by the requested roles. The comparison is by type, because revocation in the
IssuerService is by type and would otherwise revoke a credential that another spec still grants.

The state entries are copied to the top level because that is where every agent reads `participantContextId`, and
because the processing data of an orchestration is a copy of the top-level payload keys only. The Tenant Manager is
the only component that knows the value.

### Activity Processing

`ActivityProcessor` gets a method for the new discriminator, and the executor dispatches to it:

```go
type ActivityProcessor interface {
	Process(activityContext ActivityContext) ActivityResult
	ProcessDeploy(activityContext ActivityContext) ActivityResult
	ProcessDispose(activityContext ActivityContext) ActivityResult
	ProcessUpdate(activityContext ActivityContext) ActivityResult
}
```

`BaseActivityProcessor` provides a default `ProcessUpdate` that fails with the same error as `Process`. An activity
that is listed in an update orchestration without supporting updates therefore fails the orchestration, which is
preferable to an update that reports success without having done anything. It is also what an agent of an older
version does when it receives an update activity. As a consequence, an update orchestration lists only activities
that implement the handler.

An update handler has the following contract:

- It reads `cfm.vpa.changes` and returns `ActivityResultComplete` without calling a downstream system if no change
  concerns it. `ActivityContext` gets a helper for this check.
- It reads its desired state from the same keys as its deploy handler.
- It reconciles: it must be safe to run again after a partial failure, and in a later update orchestration.
- It keeps values it needs across redeliveries under keys of its own. The copied deployment state occupies the keys
  the deploy handlers use.
- It reports a failure that may be transient as `ActivityResultRetryError`. The existing handlers report every
  downstream error as fatal.
- It never creates or deletes the participant's resources. That remains with deploy and dispose.

All agents in this repository embed `BaseActivityProcessor` and compile unchanged. Processors that do not embed it,
which are the test processors in `pmanager/natsorchestration`, `pmanager/natsagent`, and `e2e/testagent`, need the
additional method.

### Orchestration Definition

The update orchestration is defined like the other two, as an entry of an orchestration template. There is one update
definition per platform. It lists every activity that implements an update handler, and `dependsOn` orders them where
one change builds on another:

```json
{
  "activities": {
    "cfm.orchestration.vpa.update": [
      {
        "id": "holder-update",
        "type": "registration-activity",
        "dependsOn": []
      },
      {
        "id": "credential-update",
        "type": "onboarding-activity",
        "dependsOn": ["holder-update"]
      }
    ]
  }
}
```

Every activity of the definition is enqueued for every update. Selecting the work is left to the handlers, which
costs one message per unconcerned activity and avoids a definition per kind of change, which the definition store
does not support: definitions are unique per orchestration type.

A platform that does not run an agent has no entry for it. If no update definition exists, an update fails with the
Provision Manager's `orchestration type '...' not found` response and is reported as a failed update.

There is no operation to change a stored definition. An installation that already has a template is extended by
posting a second template that contains only the update entry. This works with both definition stores. Posting the
complete template again overwrites the existing definitions in the SQL store and is rejected by the in-memory store.

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

Today this does not hold. The orchestration index watcher compensates every errored orchestration that is not a
dispose. A failed update would start a dispose if the update definition shares a template with the dispose
definition: the dispose orchestration would run with the processing data of the update, and the Tenant Manager would
ignore its response, so the profile would keep reporting `active` VPAs for a participant that has been torn down.
Whether it happens would depend only on the template a definition was posted with.

The condition in the watcher therefore changes from "not a dispose" to "a deploy":

```go
if orchestration.State == api.OrchestrationStateErrored && orchestration.OrchestrationType == model.VPADeployType {
```

This also covers orchestration types added in the future: no type other than deploy is compensated unless it is
explicitly opted in.

Within the update orchestration, the guarantee rests on the handler contract: an update handler never deletes the
participant's resources, and the Tenant Manager's update response handler never changes VPA states. A failed update
leaves the participant as it was before the update, or partially updated, and in both cases operational. Repairing a
partial update is the job of the next update, see [Failure Handling](#failure-handling).

### Concurrency

The Tenant Manager must not run two orchestrations for one profile at a time, and the Provision Manager does not
prevent it.

- `UpdateProfile` reads the profile with an exclusive row lock and rejects the request unless all VPAs are `active`
  and no update is `pending`.
- `DisposeProfile` takes the same lock and rejects a profile with a `pending` update. Otherwise a dispose and an
  update could run concurrently, and the update could, for example, request a credential after the dispose has
  revoked the participant's credentials.
- The update response handler takes the same lock.

## Failure Handling

A failed update leaves the profile with its previous roles and VPA properties, `active` VPAs, and an `update` field in
state `failed`. The downstream systems may be in an intermediate state: some handlers may have applied their change
and others not.

The client resolves this by requesting an update again, either with the same values or with the previous ones. Both
converge, because handlers reconcile against the downstream system. Three rules make this hold:

- While an `update` field in state `failed` exists, its `changes` are added to the changes of the next update. A
  request for the previous values would otherwise cause no change, although a handler has already applied the failed
  one.
- For the same reason, the entitlement used to compute `cfm.vpa.credentials.revoked` is then the union of the specs
  selected by the applied roles and by the roles of the failed update.
- `DisposeProfile` builds `cfm.vpa.credentials` from the same union. Dispose recomputes the credentials to revoke from
  the roles of the profile, and would otherwise miss a credential that a failed update has issued.

An update whose response never arrives stays `pending`. This is not specific to updates: the Tenant Manager has no
timeout for any orchestration, and a manifest that cannot be started after ten deliveries is discarded without a
response. See [Open Questions](#open-questions).

## Update Handlers

The table lists what each agent would reconcile and what stands in the way. Only the first row is specified in this
document. The others are candidates, each needing its own short design.

| Change                | Agent        | Handler                                                                 | Constraints                                                                                                                                                                                                                                                                                                            |
| --------------------- | ------------ | ----------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `cfm.vpa.credentials` | onboarding   | Revoke and request credentials, see below.                              | One issuer per request.                                                                                                                                                                                                                                                                                                |
| `cfm.issuer`          | registration | Set the holder's properties in the IssuerService.                       | The IssuerService has no partial update: the holder is replaced with DID, name, and properties, so the handler reads, merges, and writes without atomicity. A rule is needed for properties written by others, for example a registration desk. Changed properties affect credentials issued later, not existing ones. |
| `cfm.issuer`          | certo        | Update `dataspaceIdentifier` of the participant context.                | The agent only creates and deletes. An update call of Certo is needed.                                                                                                                                                                                                                                                 |
| `cfm.connector`       | edcv         | Associate dataspace profiles, write participant context config entries. | Removing the last profile has no code path. Config entries are not a profile input today and need a VPA property. The edcv and key management agents already write the same config without coordination, and an update adds a writer. It is safe only if the control plane merges entries.                             |
| `cfm.dataplane`       | siglet       | Replace transfer type mappings and authorization.                       | The deploy logic is an upsert and reusable. Removal is missing: an empty mapping set means "nothing to do". A stored mapping replaces the participant's whole set, which conflicts with other writers.                                                                                                                 |

Not covered by profile updates:

- The IdentityHub agent takes the service endpoints of the DID document from its own configuration, and the client
  has no call to change a participant context.
- The jwtlet agent writes the token exchange mappings of a participant once, with the scopes the agents needed at
  that time. An update handler in another agent that needs a new scope on the participant context would not get it
  for participants that are already deployed. Re-applying the mappings is idempotent and could become an update
  handler that acts on every update.

### Onboarding Agent: Credentials

The onboarding agent acts on the change `cfm.vpa.credentials` in three steps, reusing the building blocks of its
deploy and dispose handlers.

1. **Revoke.** For each spec in `cfm.vpa.credentials.revoked`, the agent queries the holder's credentials of that type
   in the IssuerService and revokes each of them. This is the revocation logic of `ProcessDispose`.
2. **Determine missing credentials.** For each spec in `cfm.vpa.credentials`, the agent queries the IssuerService for
   a credential of that type that the holder has and that is not revoked. The specs without such a credential are
   missing. If none is missing, the activity completes.
3. **Request and wait.** The agent sends one credential request for the missing specs through the IdentityHub of the
   participant, stores the request ID in the processing data, and polls the request until it is issued, as
   `ProcessDeploy` does.

Details:

- The request ID is stored under a key of its own, for example `cfm.update.holderPid`. The key `holderPid` must not be
  reused: the copied deployment state contains the `holderPid` of the original, already issued request.
- Step 2 requires `QueryCredentialsByType` to filter by credential state, which the client does not do today. The
  IssuerService can filter its credential query by `state`. This has been checked against the current IdentityHub
  sources and has to be confirmed for the IssuerService version a platform ships.
- Revoking an already revoked credential succeeds in the IssuerService, so step 1 can be repeated.
- The agent needs no additional scopes. The IssuerService is called with the scopes used for revocation on dispose,
  and the IdentityHub with the credential scopes every participant context is already granted at deployment.
- A credential request addresses one issuer. Specs of more than one issuer fail the update as they fail a deployment.

Revoked credentials are not removed from the participant's wallet. This matches the behavior on dispose.

Once the registration agent handles `cfm.issuer`, the definition orders it before the onboarding agent, because an
issuer may evaluate holder attributes when it issues a credential. Until then, holder attributes an issuer depends on
have to be current before roles are changed.

## Phases

1. **Framework and credentials.** Orchestration type, API, update status, manifest, `ProcessUpdate`, response
   handling, the compensation change, and the onboarding handler. `participantRoles` can be updated.
2. **Holder properties.** The registration handler, which needs read and replace calls in the IssuerService client.
   These are the calls the tolerant holder registration needs as well. `vpaProperties` of `cfm.issuer` can be updated.
3. **Data plane and connector.** The siglet and edcv handlers, after their constraints are resolved.

Each phase extends the Tenant Manager's list of updatable changes and the seeded update definition.

## Compatibility and Rollout

- The API change is additive. Profiles without an `update` field are unaffected.
- The Provision Manager change to compensation and the new Tenant Manager handler have to be deployed before an update
  definition is created. All CFM components are released together, so an update definition must only be seeded for a
  release that contains this design.
- Tenant Manager replicas share one consumer for orchestration responses. During a rolling upgrade, a replica without
  the update handler drops an update response. Updates should not be requested before all replicas are upgraded.
- The `ActivityProcessor` interface change breaks processors outside this repository that do not embed
  `BaseActivityProcessor` at compile time.
- An orchestration can also be started through the Provision Manager API by a caller with the scope
  `provision-manager-api:write`. Such a caller can already start deploy and dispose orchestrations with an arbitrary
  payload, and gains the update orchestration.

## Alternatives Considered

**Make the deploy orchestration aware of existing participants.** Every activity of the deploy orchestration would
have to become a create-or-update operation, whatever the change. The participant context ID would have to be passed
in instead of derived, a request could not express a revocation, and a failure would run the dispose compensation
against a working participant. Making create operations tolerate existing resources is still worthwhile, because
activity messages are redelivered, but as a robustness measure and independent of updates.

**One orchestration type per kind of update.** Separate types for roles, holder properties, and so on would let each
definition list exactly the activities concerned. It multiplies definitions, API operations, and response handlers,
and a request that changes two things would need two orchestrations and a rule for their order and partial failure.

**Separate activity types for updates.** Definitions are free to reference any activity type, so updates could be
handled by new agents. They would duplicate the clients, configuration, and scopes of the existing agents, while the
discriminator exists to let one agent handle several lifecycle operations of a resource.

**Handle the update in `Process` without changing the interface.** An agent can override `Process` and switch on the
discriminator. This avoids the interface change, but treats update differently from deploy and dispose and leaves the
dispatch to each agent.

**Compensate a failed update with an inverse update.** The Provision Manager could start an update back to the
previous state when an update fails. The inverse can fail as well, and it hides the failure from the client, which
then sees neither the requested nor a reliably restored state. Reporting the failure and letting the client repeat
or revert the update is simpler and converges for the same reason.

**Let clients change the downstream systems directly.** The profile would no longer describe the participant. For
credentials in particular, a later dispose, which derives the credentials to revoke from the roles, would not revoke
them.

## Related Issues

The following were found while writing this design. They are independent of it, but share code with it.

- A dispose requested through the Tenant Manager passes the deployment state only nested under `cfm.vpa.state`, while
  every agent reads `participantContextId` from the top level of the processing data. Only the dispose started as
  compensation receives it there. By reading the code, a requested dispose fails in the agents with a validation
  error. This has not been run. The end-to-end test does not show it, because its test agent reads nothing on dispose.
  Copying the state entries to the top level, as proposed for the update manifest, would apply to dispose as well.
- The dispose response handler sets all VPAs to `disposed` also when the dispose has failed.
- `error`, `errorDetail`, and `errorCause` of a profile are never cleared.

## Open Questions

- Should a `pending` update block a dispose without limit, or should an administrator be able to reset or supersede
  it? The second needs a rule for the orchestration that is still running.
- Should the update status be a field of its own, as proposed, or should a failed update set the `error` flag of the
  profile while the VPAs stay `active`?
- Should `vpaProperties` of a VPA type be replaced as a whole, as proposed, or patched with a set of changed and a
  list of removed keys, as the tenant properties are? Replacement fits desired state. A patch protects properties the
  caller does not know.
- Should a changed holder property lead to the reissuance of existing credentials?
- Should role names be validated against the roles used by the credential specs of the dataspace profile? Unknown
  roles are accepted on deployment today and have no effect.
- Should the profile expose `dataspaceProfileIds`? A client needs them to build a valid role map and cannot read them
  today.
- Should the dataspace profiles of a participant become updatable? Joining another dataspace changes the credential
  entitlement and the connector's profile associations together, and today the two are separate inputs.

## Testing

- Tenant Manager: service tests for the update operation (validation, preconditions, change detection, entitlement
  computation, send after commit, revert on send failure), response handler tests, ownership and scope tests, and a
  store round trip for the new column.
- Provision Manager: dispatch of the update discriminator, and watcher tests showing that a failed update, including
  one whose definition shares a template with a dispose definition, and a failed orchestration of an unknown type do
  not start a dispose.
- Agents: per handler, the skip when no change concerns it, the reconciliation, and repetition after a partial
  failure. For the onboarding agent: revocation, detection of missing credentials, and the request and polling cycle.
- End to end: an update entry in the test template and a flow that deploys a participant, adds a role, removes it,
  lets an update fail, checks that the participant is still active, and disposes the participant.
