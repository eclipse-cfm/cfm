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
- If the orchestration stream is provisioned out of band, remove `$JS.API.STREAM.CREATE.<stream>` and
  `$JS.API.STREAM.UPDATE.<stream>` from all users. Components only call them when the stream does not exist.

### Known limitations

The permissions narrow what a compromised component can do, but some trust remains by design:

- **Orchestration agents can publish responses and write orchestration state.** The executor that completes an
  orchestration runs inside the agent, so every agent may publish `event.cfm-orchestration-response` and update the KV
  bucket. The TM mitigates forged or replayed responses by only applying a response whose `manifestId` matches the
  orchestration it started for that profile (or the PM's auto-compensation of it), but an agent can still forge the
  outcome of an in-flight orchestration. Signing envelopes would close this gap.
- **All components may create and reconfigure the KV bucket.** The shared client setup calls create-or-update on the
  bucket on start, so even the TM and lifecycle agents need `$JS.API.STREAM.CREATE`/`UPDATE.KV_<bucket>`, which lets them
  change the bucket's configuration (but not its contents).
- **Replies share the `_INBOX.>` namespace.** Components can subscribe to each other's request replies. NATS supports
  per-user inbox prefixes, but CFM does not configure them yet.
- **The PM's index watcher replies to KV writers.** It acknowledges the core NATS messages it receives on
  `$KV.<bucket>.>`, which sends `+ACK` to the publisher's reply inbox. The permissions above deny those publishes, and the
  server logs them as permission violations. This is harmless and is expected to disappear when the watcher is moved to
  a JetStream consumer.
