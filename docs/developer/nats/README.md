# Securing the NATS control plane

CFM components coordinate exclusively through NATS: the Tenant Manager (TM) starts orchestrations, the Provision
Manager (PM) and orchestration agents execute them and keep their state in a JetStream key-value bucket, and lifecycle
agents react to CloudEvents such as IdentityHub key pair events. Every component trusts the messages it receives, so
anyone who can publish to the bus can deploy or dispose participants, mark deployments as active or swap signing keys.
The broker is therefore the security boundary of the control plane and must be locked down in every non-development
deployment.

## 1. Authenticate every connection

Each component reads its NATS authentication from configuration, relative to its configuration prefix (for example
`TM_`, `PM_` or the agent's prefix in environment variables):

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
agent and IdentityHub. The end-to-end tests `Test_VerifyE2E_LeastPrivilegeNatsUsers` and
`Test_VerifyLeastPrivilegeNatsUsers_KeyManagementEvents` run the deploy, dispose and key event flows against exactly
this file, so it stays in sync with the subjects the code uses.

What the permissions enforce:

| Subject                              | Published by                          | Consumed by        |
|--------------------------------------|---------------------------------------|--------------------|
| `event.cfm-orchestration`            | TM only                               | PM                 |
| `event.cfm-orchestration-response`   | PM and orchestration agents           | TM                 |
| `event.<activity-type>`              | PM and orchestration agents           | Agent for the type |
| `$KV.<bucket>.>` (orchestration state) | PM and orchestration agents         | PM (index watcher) |
| `events.keypair.>`                   | IdentityHub only                      | Key management agent |

Adapting the file to a deployment:

- Replace `cfm-stream`, `cfm-bucket` and `cfm-events` with the configured stream and bucket names.
- Add one orchestration agent user per activity type, replacing `test-activity` in its consumer subjects. The durable
  consumer name is the activity type with `.` replaced by `-`.
- Add one lifecycle agent user per agent. Its durable consumer name is the lower-cased agent name with spaces, `.`, `/`,
  `*` and `>` replaced by `-`. Grant `$JS.API.STREAM.UPDATE.<events-stream>` only if the agent is configured to create
  the event stream itself.
- If the orchestration stream and the KV bucket are provisioned out of band (section 4), remove
  `$JS.API.STREAM.CREATE.<stream>` and `$JS.API.STREAM.CREATE.KV_<bucket>` from all users. Components only call them
  when the stream or bucket does not exist.

### Known limitations

The permissions narrow what a compromised component can do, but some trust remains by design:

- **Orchestration agents can publish responses and write orchestration state.** The executor that completes an
  orchestration runs inside the agent, so every agent may publish `event.cfm-orchestration-response` and update the KV
  bucket. The TM mitigates forged or replayed responses by only applying a response whose `manifestId` matches the
  orchestration it started for that profile (or the PM's auto-compensation of it), but an agent can still forge the
  outcome of an in-flight orchestration. Signing envelopes would close this gap.
- **All components may create the KV bucket.** The shared client setup creates the bucket on start when it is missing,
  so even the TM and lifecycle agents need `$JS.API.STREAM.CREATE.KV_<bucket>` unless the bucket is provisioned out of
  band. No component needs to update an existing bucket or the orchestration stream.
- **Replies share the `_INBOX.>` namespace.** Components can subscribe to each other's request replies. NATS supports
  per-user inbox prefixes, but CFM does not configure them yet.

## 4. Make JetStream resources durable

CFM keeps orchestration messages in the orchestration stream (`<prefix>_STREAM`), orchestration state in the KV bucket
(`<prefix>_BUCKET`, backed by the stream `KV_<bucket>`) and lifecycle events in the event stream. A component creates the
orchestration stream and the bucket when they are missing, and a lifecycle agent creates the event stream only with
`createStream=true`. Existing streams and buckets are never changed, so their configuration is whatever they were created
with.

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
with the same name, subjects and retention policy that CFM uses: a work queue stream on `event.*` for orchestrations, a
KV bucket with history 1, and an interest-based stream on the lifecycle event subjects. Changing replicas or limits
later is then done on the server, for example with `nats stream edit`.
