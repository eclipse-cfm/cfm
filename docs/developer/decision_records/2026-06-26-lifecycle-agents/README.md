# Lifecycle Agents

## Decision

CFM will support _**lifecycle agents**_: standalone services that react to domain lifecycle events delivered as
[CloudEvents](https://cloudevents.io/) over NATS JetStream subjects. A reusable framework
(`common/lifecycleagent`) will be provided for building these agents, and they will be organized separately from the
existing orchestration agents.

## Rationale

The orchestration agents (`agent/orchestration`) react to orchestration _activity_ messages produced by the
`Orchestrator` as part of executing an orchestration. They are tightly coupled to the orchestration resource model and
its message envelope. In addition, they are intended to provide feedback to the ochestrator about the outcome of
orchestration activities, which is not relevant for event-driven lifecycle agents.

A growing class of work, however, needs to react to domain _lifecycle_ events that are not part of an orchestration —
for example, reacting when a new signing key was created for a participant, or when a data transfer was finished. These
events are not part of the orchestration model, and they would overload the orchestration model with concerns that have
no notion of activities, ordering, or shared context.

A dedicated lifecycle agent abstraction was chosen because:

- It decouples event producers from consumers: any number of agents can fan out from the same event without the producer
  knowing about them.
- The producer could be any component that delivers events, such as an EDC control plane or the IdentityHub
- It keeps the orchestration model focused on orchestrations and activities.
- Events are described using CloudEvents, an open, vendor-neutral specification, rather than an ad-hoc envelope. This
  gives every event consistent metadata (`id`, `source`, `type`, `time`) and keeps the domain payload cleanly separated
  in the `data` field.

Alternatives considered:

- **Extend orchestration agents to handle lifecycle events.** Rejected because it couples unrelated concerns and forces
  lifecycle reactions through the activity/ordering machinery they do not need.
- **A bespoke event envelope.** Rejected in favor of CloudEvents to avoid reinventing event metadata and to remain
  interoperable with external tooling.

## Approach

The `common/lifecycleagent` package provides a generic framework that, given a set of NATS subjects and an event
processor, wires an agent into the service lifecycle: on start it connects to NATS, ensures the configured stream and a
durable consumer exist, and dispatches each decoded event to the processor.

- Events are delivered as CloudEvents v1.0 structured-mode envelopes. The framework is generic over the payload type
  `T`; an agent decodes `CloudEvent[T]`, where `T` is its domain payload carried in the `data` field.
- The event stream uses NATS JetStream `InterestPolicy` rather than `WorkQueuePolicy`, so a message is retained until
  all interested consumers have acknowledged it. This allows multiple lifecycle agents to fan out from the same event.
- An agent subscribes to one or more explicit subjects (for example `events.keypair.added`). At least one
  subject must be configured; the runtime fails fast if none are provided.
- Agents are organized under `agent/lifecycle`, parallel to the existing `agent/orchestration` agents, omitting the
  `"-agent"` suffix. For example, an agent to react to key pair events would be a `KeyPairAgent` located in the folder
  structure at (`agent/lifecycle/keypair`).
- Implementing catch-all agents is possible but should be avoided because it complicates the event payload type and
  violates the separation-of-concerns principle.
