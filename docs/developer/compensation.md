# Compensation and reconciliation

Most processes in CFM are asynchronous by design. This means that when an operation is performed, it may not be
completed immediately, and there may be a delay before the results are visible to client applications.

While this design choice allows for greater scalability and resilience, it also introduces complexities in ensuring
consistent state across various CFM components.

To address this, CFM employs various compensation and reconciliation mechanisms.

## Definition of terms

- **Reconciliation**: The process of comparing the _desired_ system state against the _actual_ system state and making
  necessary adjustments to bring them into alignment. If a configuration change is made but not all components have
  applied it yet, reconciliation would involve detecting this discrepancy and applying the change to the remaining
  components. Here, the reconciliation would be a _retry_.

  > For example, requesting the issuance of a verifiable credential and waiting for it to be issued would involve
  periodically checking the status and retry the issuance process if a transient error occurs.

- **Compensation**: The process of undoing or mitigating the effects of an operation that has already been performed.
  This is typically used in scenarios where an operation fails after partially completing, and the system needs to
  revert to a previous consistent state. For example, a deployment process involves multiple agents (VPAs), but one of
  them fails with an unrecoverable error. In this case, compensation would involve rolling back the changes made by the
  other agents to ensure that the overall system remains consistent. Here, the compensation would be a _rollback_.

  > For example, attempting to provision a Web-DID may fail if the same Web-DID is already in use. Any prior steps taken
  to provision the Web-DID would need to be undone.

## Requirements for compensation and reconciliation

1. **Idempotency**: Operations should be designed to be idempotent, meaning that performing the same operation multiple
   times should yield the same result. This allows for safe retries without unintended side effects. For details on
   this, see the [Orchestrator Design document](./architecture/provisioning/orchestrator.design.md#).

2. **Consistent states**: Due to the modular and asynchronous nature of CFM, the states of several objects need to be
   consistent to derive a unique system state:

- `Orchestration`: represents the overall execution plan (`OrchestrationSteps`), for example, onboarding a new
  participant into a dataspace. It should reflect the summary results of all its `OrchestrationSteps`.
- `Activity`: activities contain the specific workflows, for example, requesting the issuance of a verifiable
  credential, or creating an account with the billing system.
- `VirtualParticipantAgent`: the state property of the VPA must also be consistent with the Activity result

  > Example: If the billing system account creation succeeded, but the verifiable credential issuance failed, the
  `Orchestration` should reflect a failed result.

1. **Reversability**: All activities must be reversible, possibly with side effects, meaning that they must be "undoable". This
   is necessary to allow compensation mechanisms to revert the system to a consistent state in case of failures.

   > For example, if a verifiable credential issuance process fails after the billing system account has been created,
   the compensation mechanism should be able to delete the billing system account to maintain consistency.

2. **Error handling**: Activity processors must return
   accurate [error states](./architecture/provisioning/orchestrator.design.md#activity-executors), and the orchestrator
   must handle these errors and update the `Orchestration` accordingly. This includes states for rescheduled activities,
   waiting activities and transient errors.

## State mapping for reconciliation and compensation

The orchestration state must always reflect the _summary result_ of all activities. The orchestration (=summary) state
is dictated by the result of the _least advanced_ activity or by an error result. Some examples:

- `[ActivityResultSchedule, ActivityResultComplete]`: summary state is `OrchestrationStateRunning`
- `[ActivityStateComplete, ActivityStateRetryError]`: summary state is `OrchestrationStateRunning`
- `[ActivityStateWait, ActivityStateFatalError]`: summary state is `OrchestrationStateErrored`

The following table illustrates the mapping from activity states onto orchestration states.

| ActivityResult                                                                       | Orchestration state             | Description                                                                                                                                                               |
|--------------------------------------------------------------------------------------|---------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| N/A                                                                                  | `OrchestrationStateInitialized` | Initial state, no activity has been started. After the orchestrator has started the <br/>initial activities, the orchestration transitions to `OrchestrationStateRunning` |
| `ActivityResultSchedule`, <br/>`ActivityResultWait`, <br/>`ActivityResultRetryError` | `OrchestrationStateRunning`     | One or more activities are still in progress and have not finished yet, <br/>or have not started yet, i.e. have one of the specified states.                              |
| `ActivityResultComplete`                                                             | `OrchestrationStateCompleted`   | All activities have successfully completed.                                                                                                                               |
| `ActivityResultFatalError`                                                           | `OrchestartionStateErrored`     | One or more activities have failed with an unrecoverable error. <br/>Exhausting a timeout or a numerical retry limit is regarded as unrecoverable error.                  |

Similarly, an Orchestration should accurately reflect the summary states of all VPAs. The following table illustrates
the correlation between an Orchestration state and VPA state:

| Orchestration state             | VPA state                                            | Description                                                                                                                  |
|---------------------------------|------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------------|
| `OrchestrationStateInitialized` | `DeploymentStateInitial`                             | Activity has not started yet                                                                                                 |
| `OrchestrationStateRunning`     | `DeploymentStatePending`, `DeploymentStateDisposing` | Activity has started and is in progress                                                                                      |
| `OrchestrationStateCompleted`   | `DeploymentStateActive`, `DeploymentStateDisposed`   | Activity has finished successfully                                                                                           |
| `OrchestrationStateErrored`     | `DeploymentStateError`                               | failed with an unrecoverable error. <br/>Exhausting a timeout or a numerical retry limit is regarded as unrecoverable error. |

Note that `DeploymentStateLocked` and `DeploymentStateOffline` are not considered here, as they have no effect on the
Orchestration or, in fact, compensation and reconciliation.

## Compensation processing

Compensation, in particular _rollbacks_, should always be allowed by the system. If the operation to be
compensated/rolled-back is still ongoing, then the compensation message should be enqueued and processed once the
ongoing operation has completed (either successfully or with a failure).

Additionally, as a last resort method, there should be a way to force a compensation process. This may be useful if a process gets stuck, an external system is unresponsive or a data inconsistency occurs. Forced compensations may require some manual intervention.

For example, if account creation in a billing system succeeds, but the verifiable credential issuance fails, the
compensation for the billing system account creation should be enqueued and executed after the verifiable credential
issuance has failed.

Compensation may be initiated by several triggers:

- automatically, after an unrecoverable error occurs during an orchestration, and the orchestation is marked as
  auto-compensate.

- manually, by an operator via the CFM API.

## Reconciliation processing

Reconciliation should be implemented in `ActivityProcessors`. Each activity processor must report `ActivityResultWait`
or `ActivityResultReschedule` when it detects that the desired state does not match the actual state. This will cause
the orchestrator to retry the activity after a delay.

In the case of `ActivityResultWait` the orchestrator must employ a timeout and eventually mark the orchestration as
failed if the wait condition is not resolved within the timeout period. This is to guard against zombie activities that
never complete.

Implementation note: if implementations of the `ActivityProcessor` do not implement a timeout or a retry count, the CFM core will employ a zombie killer message to clean up stuck processors.

## Limitations

Reconciliation may not be able to resolve all inconsistencies, for example, if an external system has permanently lost
data. In such cases, manual intervention may be required to restore consistency.
