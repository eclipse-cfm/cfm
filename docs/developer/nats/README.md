# Securing the NATS control plane

CFM components coordinate exclusively through NATS: managers start orchestrations, the Provision Manager (PM) and
orchestration agents execute them and keep their state in a JetStream key-value bucket, and lifecycle agents react to
CloudEvents. Every component trusts the messages it receives, so anyone who can publish to the bus can start
orchestrations, forge their outcome or trigger lifecycle agents. The broker is therefore the security boundary of the
control plane and must be locked down in every non-development deployment.

The examples in this document use the components of the dataspace variant,
[cfm-dataspace](https://github.com/eclipse-cfm/cfm-dataspace): the Tenant Manager (TM) as the manager of the namespace
`default`, and its key management lifecycle agent, which reacts to the key pair events of IdentityHub. With them,
anyone who can publish to the bus can deploy or dispose participants, mark deployments as active or swap signing keys.
The same rules apply to any other manager and agent.

## 1. Authenticate every connection

Each component reads its NATS authentication from configuration, relative to its configuration prefix (for example
`PM_`, the prefix of a manager such as `TM_`, or the agent's prefix in environment variables):

| Key                     | Description                                                                      |
|-------------------------|----------------------------------------------------------------------------------|
| `nats.auth.method`      | `none` (default), `userinfo`, `token`, `nkey` or `credentials`                   |
| `nats.auth.username`    | User name for `userinfo`                                                         |
| `nats.auth.password`    | Password for `userinfo`                                                          |
| `nats.auth.token`       | Token for `token`                                                                |
| `nats.auth.nkeySeedFile`| NKey seed file for `nkey`                                                        |
| `nats.auth.credsFile`   | JWT credentials file for `credentials` (decentralized/operator mode)             |
| `nats.auth.allowAnonymous` | Must be `true` to connect without authentication in production mode          |

Components run in production mode unless started with `-mode development` or `-mode debug`. In production mode a
component refuses to start when `nats.auth.method` is unset or `none`, unless `nats.auth.allowAnonymous` is explicitly
set to `true`. Use that flag only for local setups and tests against an unsecured broker.

Prefer `nkey` or `credentials`: the secret never leaves the component, and per-user permissions (section 3) can be
managed centrally with operator JWTs.

## 2. Encrypt the connection

| Key                | Description                                                                         |
|--------------------|-------------------------------------------------------------------------------------|
| `nats.tls.enabled` | Require TLS and verify the server against the system trust store                    |
| `nats.tls.ca`      | PEM CA bundle used to verify the server instead of the system trust store            |
| `nats.tls.cert`    | PEM client certificate for mutual TLS                                               |
| `nats.tls.key`     | PEM private key of the client certificate                                           |

Setting any of the file keys enables TLS. Setting `nats.tls.cert` and `nats.tls.key` enables mutual TLS, which the
server can require with `verify: true` (or map certificates to users with `verify_and_map: true`). A `tls://` server URL
also enforces TLS against the system trust store.

## 3. Give each component least-privilege permissions

Authentication alone still lets every component publish anything. Give each component its own NATS user (or JWT user)
restricted to the subjects it needs. [`least-privilege.conf`](least-privilege.conf) is a complete authorization block
for a NATS server configuration that does this for the TM, the PM, an orchestration agent, the key management lifecycle
agent and IdentityHub, and for a second manager and its agent in their own namespace (see
[Namespaces](../namespaces.md)). The end-to-end tests `Test_VerifyLeastPrivilegeNatsUsers_KeyManagementEvents` and
`Test_VerifyLeastPrivilegeNatsUsers_NamespacesAreSeparated` in [`e2e/e2etests`](../../../e2e/e2etests) run the key event
flow and the namespace checks against exactly this file, so it stays in sync with the subjects the code uses. The
end-to-end tests of cfm-dataspace run the deploy and dispose flows of the Tenant Manager with the same permissions.

What the permissions enforce:

| Subject                                        | Published by                                        | Consumed by                         |
|------------------------------------------------|-----------------------------------------------------|-------------------------------------|
| `event.<namespace>.cfm-orchestration`          | The manager of the namespace only (TM for `default`) | PM                                  |
| `event.<namespace>.cfm-orchestration-response` | PM and the orchestration agents of the namespace    | The manager of the namespace        |
| `event.<namespace>.<activity-type>`            | PM and the orchestration agents of the namespace    | Agent for the type in the namespace |
| `$KV.<bucket>.<namespace>.>` (orchestration state) | PM and the orchestration agents of the namespace | PM (index watcher)                  |
| `events.keypair.>` (example of lifecycle events) | IdentityHub only                                  | Key management agent                |

Adapting the file to a deployment:

- Replace `cfm-stream`, `cfm-bucket` and `cfm-events` with the configured stream and bucket names.
- Add one manager user per namespace, and one orchestration agent user per namespace and activity type, replacing
  `second` and `test-activity` in their subjects. The durable consumer name is `<namespace>~<activity-type>` with `.`
  replaced by `-` in the activity type, and the durable consumer of a manager's responses is
  `<namespace>~cfm-orchestration-response`. An agent reads its namespace from the configuration key `namespace`.
- Add one lifecycle agent user per agent. Its durable consumer name is the lower-cased agent name with spaces, `.`, `/`,
  `*` and `>` replaced by `-`. Grant `$JS.API.STREAM.UPDATE.<events-stream>` only if the agent is configured to create
  the event stream itself.
- If the orchestration stream and the KV bucket are provisioned out of band (section 4), remove
  `$JS.API.STREAM.CREATE.<stream>` and `$JS.API.STREAM.CREATE.KV_<bucket>` from all users. Components only call them
  when the stream or bucket does not exist.

### Known limitations

The permissions narrow what a compromised component can do, but some trust remains by design:

- **Orchestration agents can publish responses and write orchestration state.** The executor that completes an
  orchestration runs inside the agent, so every agent may publish the responses of its namespace and update the
  orchestration state of its namespace. A manager should mitigate forged or replayed responses by only applying a
  response whose `manifestId` matches an orchestration it started (or the PM's auto-compensation of it), as the TM
  does, but an agent can still forge the outcome of an in-flight orchestration. Signing envelopes would close this
  gap.
- **All components may create the KV bucket.** The shared client setup creates the bucket on start when it is missing,
  so even managers and lifecycle agents need `$JS.API.STREAM.CREATE.KV_<bucket>` unless the bucket is provisioned out of
  band. No component needs to update an existing bucket, and only a stream created before namespaces were introduced
  needs to be updated once, see [Namespaces](../namespaces.md#upgrading).
- **Replies share the `_INBOX.>` namespace.** Components can subscribe to each other's request replies. NATS supports
  per-user inbox prefixes, but CFM does not configure them yet.

## 4. Make JetStream resources durable

CFM keeps orchestration messages in the orchestration stream (`<prefix>_STREAM`), orchestration state in the KV bucket
(`<prefix>_BUCKET`, backed by the stream `KV_<bucket>`) and lifecycle events in the event stream. A component creates the
orchestration stream and the bucket when they are missing, and a lifecycle agent creates the event stream only with
`createStream=true`. Existing streams and buckets are not changed, so their configuration is whatever they were created
with. The one exception is an orchestration stream created before namespaces were introduced: its subjects are widened
from `event.*` to `event.>`.

When a component creates one of them, it uses file storage and these settings:

| Key                               | Description                                                                 |
|-----------------------------------|-----------------------------------------------------------------------------|
| `nats.jetstream.replicas`         | Replicas of each created stream and of the KV bucket (default 1, at most 5) |
| `nats.jetstream.stream.maxAge`    | Maximum message age in created streams, e.g. `168h` (default unlimited)     |
| `nats.jetstream.stream.maxBytes`  | Maximum size of each created stream in bytes (default unlimited)            |
| `nats.jetstream.kv.maxBytes`      | Maximum size of the KV bucket in bytes (default unlimited)                  |

On a clustered NATS, use 3 replicas, so that orchestrations keep running and no messages or state are lost when a
server goes down. Set the same values on every component, because whichever starts first creates the resources.

Keep these in mind when setting limits:

- A message that exceeds `stream.maxAge` is removed even if it was never processed, so the age must exceed the longest
  time an orchestration activity may wait in the stream.
- The KV bucket holds the state of every orchestration and is not cleaned up. Once it reaches `kv.maxBytes`, writes
  fail and orchestrations stop. The bucket has no TTL, because the Provision Manager uses its entries to de-duplicate
  orchestration requests.

For production deployments, prefer provisioning the streams and the bucket out of band, for example with
[NACK](https://github.com/nats-io/nack) or a Helm job, so their configuration is under version control. Create them
with the same name, subjects and retention policy that CFM uses: a work queue stream on `event.>` for orchestrations, a
KV bucket with history 1, and an interest-based stream on the lifecycle event subjects. Changing replicas or limits
later is then done on the server, for example with `nats stream edit`.
