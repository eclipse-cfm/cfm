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

Many things can change during the life of a participant. Two examples are used throughout this document:

- its **roles** in a dataspace, which determine the credentials it is entitled to,
- its **holder properties** in the IssuerService, from which the claims of its credentials are taken.

Update orchestrations are intended for the type of lifecycle changes that require a concerted effort and span across
multiple downstream components. The two listed above are typical of what updates are for: a change of roles usually
needs changed holder properties before the new credentials can be offered, so one change spans several components,
namely the IssuerService (to change the holder), and the IdentityHub (to request and receive the new credential)
see [Scope](#scope).

This document proposes updates as a third lifecycle operation of a participant profile, next to deploy and dispose:

- A Tenant Manager operation changes the desired state of a deployed participant profile with a `PATCH`.
- A third orchestration type, `cfm.orchestration.vpa.update`, brings the downstream systems in line with it. It is a
  third activity list in the orchestration template, next to deploy and dispose, and is ordered with `dependsOn` like
  them.
- The Tenant Manager sends the previous and the requested state of the profile and does not interpret either. Each
  agent determines its own changes.
- The existing activity agents handle updates in a new `ProcessUpdate` method, next to `ProcessDeploy` and
  `ProcessDispose`. Each agent updates the part of the participant it already owns.

An update never touches the participant's identity, wallet, or participant context. Updates are idempotent and are
never compensated: a failed update is repeated, and it never disposes the participant.

One decision is still open and is described in [Open Questions](#open-questions): whether update handlers belong into
the existing agents or into standalone update agents.

## Design Principles

1. **A missing update handler fails the update.** An activity in the update list whose agent does not handle updates
   fails the update instead of completing silently, as an unknown discriminator fails an activity today.
2. **`participantContextId` is mandatory in the processing data** of an update orchestration. It is how every handler
   addresses the participant.
3. **The Tenant Manager does not know the payload.** It applies the request to the stored profile data and sends the
   previous and the requested state in the format of a deployment. It knows nothing about what an agent reads from
   it, so a new kind of update needs no change to the Tenant Manager.
4. **Updates are ordered like deployments.** The update orchestration is an activity list of the orchestration
   template, and its activities are ordered with `dependsOn`, as for deploy and dispose.
5. **Updates are idempotent and never compensated.** A handler compares the requested state with the downstream
   system and applies the difference, so running it twice has the same result as running it once. A failed update is
   repeated, not rolled back.
6. **A failed update never disposes the participant.**

## Goals and Non-Goals

Goals:

- One mechanism for all changes to a deployed participant profile, with one API operation, one orchestration type, and
  one way to report progress and failure.
- New kinds of updates are added to the agent that owns the downstream system and to the orchestration template,
  without changes to the Tenant Manager or the orchestration engine.
- Updates whose activities depend on one another, for example a holder change before a credential request.
- A failed update that leaves a working participant in place and can be repeated.

Non-goals:

- Changing the identity of a participant: its identifier (DID), participant context ID, cell, or tenant.
- Adding or removing a VPA of a participant.
- Changes that concern one component only, for example the participant config of the control plane, see
  [Scope](#scope).
- Changing the credential specs of a dataspace profile. A different set of credentials is a new dataspace profile.
- Orchestration definitions per dataspace profile, and support for more than one issuer. Updates inherit both
  limitations from deployment.
- Updates that originate in agent configuration instead of the participant profile, for example changed service
  endpoint templates of the IdentityHub agent.

## Scope

Update orchestrations are intended for changes that span several components and have to be applied together and in
order. Updating the roles of a participant is the prime example: the holder properties in the IssuerService may have
to change first, because the issuer's attestation pipeline reads them, and then the credentials the participant is
offered and holds change in the IssuerService and the IdentityHub. No single component can apply such a change, and
the parts depend on each other.

Updates are not intended for changes that concern one component only, such as the participant config of the
connector's control plane. Users make such changes with the administrative API of that component, for example the
control plane management API. Routing them through an orchestration would add a Tenant Manager operation, an
orchestration, and an agent handler to what is one call to one API, and the profile would have to mirror
configuration that the component already owns.

A kind of change belongs into an update orchestration if it needs more than one component, or if it changes an input
that only the participant profile holds, such as the roles.

## Current Behavior

The design builds on the following properties of the code base.

**Activities are dispatched by discriminator.** An orchestration template maps orchestration types to activity lists,
and the type becomes the `discriminator` of each of its activities. The executor
(`pmanager/natsorchestration/activity.go`) calls `ProcessDeploy` or `ProcessDispose` for the two known
discriminators and `Process` for any other. All agents embed `BaseActivityProcessor`, whose `Process` fails with
`the '...' discriminator is not supported`. Activities are routed to agents by activity type only.

**Every orchestration needs a definition, and definitions are keyed by type.** `provisionManager.Start`
(`pmanager/core/provision.go`) looks up the definition of the manifest's orchestration type and fails with
`orchestration type '...' not found` if there is none. The ID of a definition is its orchestration type
(`OrchestrationDefinition.GetID`), so a platform has one definition per type, whatever the number of templates.
`InstantiateOrchestration` turns the activities of a definition into steps by their dependencies; activities without
dependencies form one parallel step.

**A profile has two kinds of mutable input.** `participantRoles` (dataspace profile ID to role names) are interpreted
only by the Tenant Manager: `generateCredentialSpecs` in `tmanager/core/service_participant.go` selects a
`CredentialSpec` of a dataspace profile if its `role` is empty or contained in the participant's roles, and the
result is sent to the agents as `cfm.vpa.credentials`. The roles themselves are never sent. `vpaProperties` are stored
per VPA type, sent as `cfm.vpa.data`, and read by the agents at deployment:

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

**Tenant properties are patched with a diff.** `PATCH` on a tenant takes a `TenantPropertiesDiff`: the keys to set
and a list of keys to delete.

## Design

### Tenant Manager API

A participant profile is updated with a new operation:

```
PATCH /api/v1alpha1/tenants/{tenantID}/participant-profiles/{participantID}
```

```json
{
  "participantRoles": {
    "properties": {
      "<dataspace profile ID>": [
        "manufacturer",
        "supplier"
      ]
    },
    "removed": [
      "<other dataspace profile ID>"
    ]
  },
  "vpaProperties": {
    "cfm.issuer": {
      "properties": {
        "legalName": "Example Corp"
      },
      "removed": [
        "formerKey"
      ]
    }
  }
}
```

Every map is changed with the diff of the tenant `PATCH` (`TenantPropertiesDiff`): `properties` holds the keys to
set, and `removed` lists the keys to delete. The Tenant Manager sets first and removes second. A key that is not
mentioned is unchanged, so a client cannot remove data
by leaving it out, and it does not have to read the profile before it changes it.

- `participantRoles` is one diff. Its keys are dataspace profile IDs, and each value is the complete role list of that
  dataspace profile.
- `vpaProperties` holds one diff per VPA type. The VPA type selects the VPA and is not itself set or removed.
- A diff changes one level. A value is replaced as a whole, for example the role list of a dataspace profile above.
- Both members are optional, and at least one must be present.

The operation requires the scope `tenant-manager-api:write` and applies the same tenant ownership rules as the other
participant profile operations.

Validation, rejected with `400`:

- The struct validation is the same as for a deployment, plus the limits of the tenant diff on the number and length
  of keys.
- Every key of `participantRoles` must be one of the profile's dataspace profile IDs. A deployment does not check
  this today and silently ignores such keys.
- Every VPA type in `vpaProperties` must be a VPA of the profile.

Preconditions, rejected with `409 Conflict`:

- All VPAs of the profile are `active`.
- No update of the profile is in progress.
- The stored deployment state contains a `participantContextId`.

Result:

`202 Accepted` with the participant profile. Clients read the `update` field to learn whether an orchestration was
started. If the requested state sent to the agents equals the previous state, for example because `participantRoles`
differ without changing the credential entitlement, the profile is stored directly, no orchestration is started and
`update` is not set. Otherwise, the requested state is applied to the profile when the update orchestration has
completed.

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
    "participantRoles": {
      "<dataspace profile ID>": [
        "manufacturer",
        "supplier"
      ]
    },
    "vpaProperties": {
      "cfm.issuer": {
        "legalName": "Example Corp"
      }
    },
    "errorDetail": "",
    "errorCause": ""
  }
}
```

- `state` is `pending` while the orchestration runs and `failed` after it has failed.
- `participantRoles` and `vpaProperties` inside `update` are the requested state: the stored values with the diff of
  the request applied, for the parts the request changes. The example is the update of the request above. The members
  of the same name on the profile and its VPAs always show what has been applied.
- `errorDetail` and `errorCause` have the same meaning and visibility as the fields of the same name on the profile.
- The field is removed when an update succeeds. A failed update stays visible until the next update is requested.

The field is stored in a new JSONB column of the `participant_profiles` table. It is not stored in `properties`,
which are returned unfiltered and share their namespace with caller-supplied values. `cfm.orchestration.pending` is
still set when the update is started, so that the existing response matching applies.

### Update Manifest

The Tenant Manager sends an `OrchestrationManifest` with the type `cfm.orchestration.vpa.update` and the profile ID as
correlation ID. The payload contains the requested state in the keys of a deployment, and the previous state in the
same format:

| Key                    | Content                                                                        |
|------------------------|--------------------------------------------------------------------------------|
| `participantContextId` | Mandatory. Copied from the stored deployment state.                            |
| `cfm.participant.id`   | The participant identifier, as in deploy and dispose.                          |
| `cfm.vpa.data`         | The VPAs with their requested properties, as in a deployment.                  |
| `cfm.vpa.credentials`  | The credential specs selected by the requested roles, as in a deployment.      |
| `cfm.vpa.state`        | The stored deployment state, as in dispose.                                    |
| `cfm.update.previous`  | A list of previous states, each with `cfm.vpa.data` and `cfm.vpa.credentials`. |

```json
{
  "participantContextId": "...",
  "cfm.participant.id": "did:web:...",
  "cfm.vpa.data": [
    "... requested ..."
  ],
  "cfm.vpa.credentials": [
    "... requested ..."
  ],
  "cfm.vpa.state": {
    "...": "..."
  },
  "cfm.update.previous": [
    {
      "cfm.vpa.data": [
        "... stored ..."
      ],
      "cfm.vpa.credentials": [
        "... stored ..."
      ]
    }
  ]
}
```

The Tenant Manager computes both states the same way: it applies the diff to the stored roles and VPA properties,
and it builds `cfm.vpa.data` and `cfm.vpa.credentials` with the code of a deployment. It computes nothing else and
does not know which agent reads which part.

`cfm.update.previous` normally holds one state, the stored one. After a failed update it also holds the requested
state of the failed update, see [Failure Handling](#failure-handling).

An agent reads the requested state with the same structs and helpers as at deployment, for example `VpaProperties`,
and the previous states with a struct of its own.

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

The executor dispatches `UpdateDiscriminator` to `ProcessUpdate`. `BaseActivityProcessor` implements `ProcessUpdate`
by failing the activity, as its `Process` does for an unknown discriminator today. `Process` does not change:

```go
package api

func (p BaseActivityProcessor) ProcessUpdate(ctx ActivityContext) ActivityResult {
	return ActivityResult{
		Result: ActivityResultFatalError,
		Error:  fmt.Errorf("the '%s' discriminator is not supported", ctx.Discriminator()),
	}
}
```

An agent without an update handler that is listed in the update list therefore fails the update, which points at the
misconfigured template instead of reporting an update that did nothing. Every agent in this repository compiles
unchanged. Processors that do not embed `BaseActivityProcessor` need the additional method. These are the test
processors in `pmanager/natsorchestration`, `pmanager/natsagent`, and `e2e/testagent`.

An agent that handles updates implements `ProcessUpdate`:

```go
package onboarding

type credentialUpdateData struct {
	ParticipantContextID string                   `json:"participantContextId" validate:"required,participantcontextid"`
	ParticipantID        string                   `json:"cfm.participant.id" validate:"required"`
	Credentials          []model.CredentialSpec   `json:"cfm.vpa.credentials"`
	Previous             []previousCredentialData `json:"cfm.update.previous" validate:"required,min=1"`
}

type previousCredentialData struct {
	Credentials []model.CredentialSpec `json:"cfm.vpa.credentials"`
}

func (p OnboardingActivityProcessor) ProcessUpdate(ctx api.ActivityContext) api.ActivityResult {
	var data credentialUpdateData
	if err := ctx.ReadValues(&data); err != nil {
		return api.ActivityResult{Result: api.ActivityResultFatalError, Error: err}
	}
	// reconcile ...
}
```

A handler:

- reads only the parts of the requested and previous states it uses, and may complete without action when these
  parts are equal in the requested state and in every previous state;
- otherwise reads the current state of its downstream system, writes what the requested state needs and the
  downstream system lacks, and removes what a previous state contains, the requested state does not, and the
  downstream system has;
- is idempotent: it must be safe to run again after a partial failure, and in a later update orchestration;
- fails with a fatal error if a value it needs is missing or invalid;
- keeps values it needs across redeliveries under keys of its own, and sets output values only under keys prefixed
  with its activity type;
- reports a failure that may be transient as `ActivityResultRetryError`, while the existing handlers report every
  downstream error as fatal;
- never calls the agent's dispose logic and never deletes the participant's resources.

Removing only what a previous state contains leaves alone what the profile never contained, for example holder
properties written by a registration desk.

### Update Orchestration in the Template

An orchestration template maps orchestration types to activity lists. The update type is a third key, next to deploy
and dispose. The format of the entries does not change, and neither do `provisionManager.Start` and
`ActivityDefinition`:

```json
{
  "id": "example-template",
  "description": "Participant deployment, disposal, and update",
  "activities": {
    "cfm.orchestration.vpa.deploy": [
      {
        "id": "register",
        "type": "registration-activity",
        "dependsOn": []
      },
      {
        "id": "connector",
        "type": "edcv-activity",
        "dependsOn": []
      },
      {
        "id": "onboard",
        "type": "onboarding-activity",
        "dependsOn": [
          "register"
        ]
      }
    ],
    "cfm.orchestration.vpa.dispose": [
      {
        "id": "offboard",
        "type": "onboarding-activity",
        "dependsOn": []
      },
      {
        "id": "connector-cleanup",
        "type": "edcv-activity",
        "dependsOn": []
      },
      {
        "id": "deregister",
        "type": "registration-activity",
        "dependsOn": [
          "offboard"
        ]
      }
    ],
    "cfm.orchestration.vpa.update": [
      {
        "id": "holder-update",
        "type": "registration-activity",
        "dependsOn": []
      },
      {
        "id": "credentials-update",
        "type": "onboarding-activity",
        "dependsOn": [
          "holder-update"
        ]
      }
    ]
  }
}
```

- The deploy and dispose lists are shortened for this example.
- The update list contains the agents that handle updates. Every activity of the list runs on every update, in the
  defined order, and completes immediately if its part of the profile does not change.
- Each activity of the list receives the discriminator `cfm.orchestration.vpa.update`, and the executor calls
  `ProcessUpdate` on its agent.
- A platform without an update definition does not support updates. `Start` fails with `orchestration type '...' not
  found`, which reaches the Tenant Manager as a failed update.

Definitions are keyed by orchestration type, so a platform has one update definition, like it has one deploy
definition. Update support is a property of the platform, not of a template.

### Ordering

Activities of the update list are ordered with `dependsOn`. The dependency is declared by whoever configures the
platform, because only the platform knows it. The runtime does not derive it from the kinds of change.

The example above orders the credential update after the holder update. The issuer's attestation pipeline reads the
holder properties, so a new entitlement usually requires changed holder properties before the new credential can be
issued. With the dependency, one update can change both, and the credential request sees the new holder properties.

Activities without dependencies run in parallel. Changes that depend on each other but are handled by activities
without a declared dependency may run into race conditions.

### Tenant Manager Response Handling

A handler for `cfm.orchestration.vpa.update` is registered next to the deploy and dispose handlers. A response for an
orchestration type without a handler is dropped, so the handler is also what makes a failed start visible.

- On success, the requested roles and VPA properties are applied to the profile and the `update` field is removed.
- On failure, the `update` field is set to `failed` with the error detail and cause of the response. The profile is
  not changed otherwise.
- In both cases `cfm.orchestration.pending` is removed, VPA states and the `error` flag are left alone, and the
  output values of the update orchestration are merged into `cfm.vpa.state`, see below.

The output values of an orchestration form one flat map, in which a later write to the same key wins, and
`cfm.vpa.state` holds the output values of the deployment, for example `participantContextId` and `holderPid`. Update
output values are therefore namespaced twice:

- The Tenant Manager merges them into a map of their own, `cfm.vpa.state["cfm.update"]`, so that an update cannot
  overwrite a value of the deployment, such as `participantContextId`.
- An update handler sets output values only under keys prefixed with its activity type, for example
  `onboarding-activity.credentialRequest`, so that two agents cannot overwrite each other's values.

A later update replaces the values of the same key, and other keys are kept. The values are also merged after a failed
update, so that a repeated update finds what the failed one recorded, as far as the response of the failed update
reports it. Because the payload carries `cfm.vpa.state`, a handler reads its earlier values there.

### No Disposal on Failure

A failed update, or a failed activity of an update, never leads to a dispose of the participant, and it is never
compensated in any other way. A dispose is the end of a participant and is only ever started by an explicit dispose
request or as the compensation of a failed deploy, where there is no working participant to lose.

Today the orchestration index watcher compensates every errored orchestration that is not a dispose, provided the
template of the failed orchestration has a dispose definition. The update definition is part of the same template as
the dispose definition, so a failed update would dispose a working participant, whose profile then keeps reporting
`active` VPAs, because the Tenant Manager ignores the response.

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
state `failed`. The downstream systems may be in an intermediate state: activities of one step run in parallel and the
others finish when one fails, later steps do not run, so some changes may be applied and others not.

A failed update is not compensated. The client resolves it by requesting an update again, either with the same values
or with the previous ones. Both converge, because handlers are idempotent and reconcile against the downstream system.
Two rules guarantee that:

- While an `update` field in state `failed` exists, the next update carries two previous states: the stored state and
  the requested state of the failed update. A handler then acts if its part differs from either, and removes what
  either contains and the new requested state does not. A request for the stored values would otherwise cause no
  change, although a handler has already applied part of the failed update.
- `DisposeProfile` builds `cfm.vpa.credentials` from the specs selected by the stored roles and by the roles of the
  failed update. Dispose recomputes the credentials to revoke from the roles of the profile, and would otherwise miss a
  credential that a failed update has issued.

An update whose response never arrives stays `pending`. This is not specific to updates: the Tenant Manager has no
timeout for any orchestration, and a manifest that cannot be started after ten deliveries is discarded without a
response.

## Examples

The two examples from the overview are sketched here to show that the model carries them and what each needs from
its downstream system. Each needs its own detailed design before it is built. They are independent of each other and
can be delivered in any order. None of them needs a change to the Tenant Manager.

### Roles

|                |                                                   |
|----------------|---------------------------------------------------|
| **Agent**      | onboarding (`onboarding-activity`)                |
| **Reads**      | `cfm.vpa.credentials`                             |
| **Depends on** | the holder update                                 |
| **Downstream** | IssuerService admin API, IdentityHub identity API |

A change of roles reaches the agent as a change of `cfm.vpa.credentials`, the complete entitlement selected by the
roles.

1. For each credential type of a previous state that no spec of the requested state grants, query the holder's
   credentials of that type in the IssuerService and revoke each of them. This is the revocation logic of
   `ProcessDispose`. Types are compared, because revocation in the IssuerService is by type and would otherwise revoke
   a credential that another spec still grants.
2. For each spec of the requested state, query the IssuerService for a credential of that type that the holder has
   and that is not revoked. The specs without such a credential are missing.
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

|                |                                        |
|----------------|----------------------------------------|
| **Agent**      | registration (`registration-activity`) |
| **Reads**      | the properties of the `cfm.issuer` VPA |
| **Downstream** | IssuerService admin API                |

1. Read the holder of the participant from the IssuerService.
2. Set the requested properties, and remove the keys that a previous state contains and the requested state does not.
3. Write the holder back.

Notes:

- The IssuerService has no partial update for holders. The holder is replaced with its DID, name, and properties, so
  the handler reads, merges, and writes without atomicity. The issuer client in `agent/common` needs read and replace
  calls, which the tolerant holder registration needs as well.
- Properties the profile never contained, for example ones written by a registration desk, are left alone.
- Changed properties affect credentials issued afterwards, not existing ones.
- The registration agent can serve updates on a platform that does not register holders during deployment: its
  activity only has to be part of the update list, not of the deploy list.

### Further Candidates

- **Dataspace profile associations** of the connector, in the edcv agent, together with the credential entitlement of
  the dataspace profile. Joining a dataspace changes both. The control plane call exists, but has no path for removing
  the last association.

Changes that concern one component, for example the participant config of the control plane, the transfer type
mappings and authorization of the data plane, or the dataspace identifier in Certo, are made through the
administrative API of that component, see [Scope](#scope).

## Phases

1. **Framework.** API, update status, manifest, `ProcessUpdate` and its default in `BaseActivityProcessor`, response
   handling, and the compensation change. An end-to-end test proves the flow with the test agent and a template with
   an update list.
2. **Handlers.** One at a time, each with its detailed design, its handler, and its entry in the update list of the
   template.

## Compatibility and Rollout

- The API change is additive. Profiles without an `update` field are unaffected.
- Agents built on `BaseActivityProcessor` outside this repository get a `ProcessUpdate` that fails. Listing such an
  agent in the update list fails every update until it implements a handler.
- The `ActivityProcessor` interface change breaks processors outside this repository that do not embed
  `BaseActivityProcessor` at compile time.
- A platform enables updates by deploying agent versions with update handlers and then adding the update list to its
  template. The definition store has no operation to change a stored definition, and its two implementations treat a
  repeated post differently (the SQL store overwrites, the memory store returns `409`), which has to be resolved for
  adding the list on a running platform.
- The Provision Manager change to compensation and the new Tenant Manager handler have to be deployed before a
  template has an update list.
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

**Update data objects built by the Tenant Manager.** The Tenant Manager could compute one data object per kind of
update, for example the credentials to request and to revoke, and send only the data objects of the parts that
change. Every new kind of update would then need a builder in the Tenant Manager, so an agent outside this repository
could not handle updates without a change to the Tenant Manager.

**An update orchestration built by the Provision Manager.** Activity definitions could declare the data objects they
handle, and `Start` could build an orchestration of the activities concerned by an update, without a definition.
This needs a new field on activity definitions and a second way to build orchestrations, and it cannot order
activities without a second ordering mechanism next to `dependsOn`.

**Compensate a failed update with an inverse update.** The Tenant Manager could start a second update with the
previous and the requested state exchanged. The inverse can fail as well, and it hides the failure from the client,
which then sees neither the requested nor a reliably restored state. It only sees the output values that the failed
orchestration reported with its first failure, and an activity of the same step that finishes later is not among
them. It also removes what the requested state contains and the downstream system has, which may include state that
existed before the update, for example a credential, which cannot be restored once revoked. Reporting the failure and
letting the client repeat or revert the update is simpler and converges for the same reason.

**`PUT` instead of `PATCH`.** A `PUT` replaces the complete roles and properties, so the client reads the profile,
modifies it, and sends all values back. The read-modify-write happens in the client, so two clients that update the
same profile can overwrite each other's changes unless the profile carries a version or ETag that the Tenant Manager
checks, and a client that leaves a value out removes it.

## Open Questions

### Integrated `ProcessUpdate` or Standalone Update Activities

This document integrates update handlers into the existing activity agents through a new handler `ProcessUpdate`. The
alternative is standalone update activities: a third agent category under `agent/update`, next to `agent/orchestration`
and `agent/lifecycle`, with one agent and one activity type per kind of update, for example `holder-update-activity`.
The rest of this design applies to both, since the update list of the template names the activity type either way.

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

### Further Questions

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
