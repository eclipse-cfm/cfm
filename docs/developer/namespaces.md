# Namespaces

Several managers can use one Provision Manager instance. Each manager works in its own namespace, which keeps its
orchestration definitions, activity definitions and orchestrations apart from those of the other managers.

## The namespace

A namespace is a single NATS subject token: letters, digits, `_` and `-`. Dots, which separate subject tokens, and the
wildcards `*` and `>` are not allowed.

A manifest, a definition or an orchestration without a namespace belongs to the default namespace, `default`. A manager
that does not use namespaces, such as the Tenant Manager of
[cfm-dataspace](https://github.com/eclipse-cfm/cfm-dataspace), therefore keeps working without a change.

A deployment can thus run the dataspace variant in `default` and other managers, with their own definitions and agents,
in further namespaces of the same Provision Manager. The [samples](../../samples/README.md#use-a-namespace) show the
calls of a manager and an agent in a namespace.

## Definitions

An orchestration definition and an activity definition are identified by the pair of namespace and type. Two managers can
register the same type, each in its own namespace. An orchestration definition can only use the activity definitions of
its own namespace.

The Postgres store keys a definition by `<namespace>/<type>` and keeps the namespace in its own `namespace` column. On
start, the store moves definitions that were stored before namespaces were introduced into the default namespace.

## Orchestrations

An orchestration runs with the definitions of the namespace of its manifest, and keeps that namespace. The compensation
of a failed orchestration runs in the namespace of the failed orchestration.

An orchestration is identified by the pair of namespace and ID, which is the ID of its manifest. Two managers can use the
same manifest ID, each in its own namespace. The state of an orchestration is stored in the KV bucket under the key
`<namespace>.<id>`, and its index entry in Postgres under the same key, with the ID in its own `orchestration_id`
column. De-duplication of manifests only looks at the orchestrations of the manifest's namespace. On start, the store
moves index entries that were stored before namespaces were introduced into the default namespace.

## Messaging

Every subject of an orchestration carries its namespace as the second token:

| Subject                                         | Purpose                                                          |
|-------------------------------------------------|------------------------------------------------------------------|
| `event.<namespace>.cfm-orchestration`           | A manager starts an orchestration of its namespace               |
| `event.<namespace>.<activity-type>`             | The engine asks the agent of the namespace to run an activity    |
| `event.<namespace>.cfm-orchestration-response`  | The engine reports the outcome to the manager of the namespace   |
| `event.<namespace>.<reply-to>`                  | The same, for a manifest that names a `replyTo` token            |

The Tenant Manager is the manager of the default namespace, so it sends manifests to `event.default.cfm-orchestration`
and receives responses on `event.default.cfm-orchestration-response`. The stream carries `event.>`.

The subject of a manifest decides its namespace. A manifest may name a namespace in its `namespace` field, and is
rejected with an error response if that is not the namespace of its subject. The Provision Manager receives the
manifests of every namespace with one durable consumer, `~cfm-orchestration`, on `event.*.cfm-orchestration`.

An agent runs the activities of one namespace, which it reads from the configuration key `namespace` (for example
`TESTAGENT_NAMESPACE`) or, if the key is not set, from `LauncherConfig.Namespace`, and which is `default` if neither
is set. The durable consumer of an activity type in a namespace is
`<namespace>~<activity-type>`, with `.` replaced by `-` in the activity type. A manager's consumer of its responses is
`<namespace>~cfm-orchestration-response`. A namespace, an activity type and a reply-to token cannot contain `~`, so the
consumers of different namespaces never share a name. An agent only reads and writes the orchestration state of its
namespace, and refuses to run an activity whose state names another namespace.

Since every subject and every key of a namespace starts with the namespace, NATS permissions can keep the namespaces apart:
a manager may only publish `event.<namespace>.cfm-orchestration`, and an agent only `event.<namespace>.>` and
`$KV.<bucket>.<namespace>.>`. [`least-privilege.conf`](nats/least-privilege.conf) has an example for a second namespace,
and the end-to-end test `Test_VerifyLeastPrivilegeNatsUsers_NamespacesAreSeparated` checks it.

## REST API

Every access token for the Provision Manager API names the namespaces of the caller in its standard `scope` claim, so
the identity provider does not need to issue a custom claim. A scope of the API has the form
`provision-manager-api:<namespace>:<permission>`, where the permission is `read`, `write` or `admin`. It grants the
permission in that namespace only. The scopes without a namespace, `provision-manager-api:read`, `:write` and `:admin`,
grant their permission in the default namespace, so the tokens of the Tenant Manager keep working.

A request selects its namespace with its path. Every route of the API is also available under
`/api/v1alpha1/namespaces/<namespace>/`, and then reads and writes that namespace. The routes without the prefix, such as
`/api/v1alpha1/orchestrations`, are in the default namespace, so the Tenant Manager keeps using them unchanged. A request
is rejected with `400 Bad Request` if the namespace in its path is not valid, and with `403 Forbidden` if the token does
not have the scope that the request needs in its namespace.

| Token scopes                                                            | Path                                                   | Result                |
|-------------------------------------------------------------------------|--------------------------------------------------------|-----------------------|
| `provision-manager-api:read`                                            | `/api/v1alpha1/orchestrations/query`                   | allowed, in `default` |
| `provision-manager-api:default:read`                                    | `/api/v1alpha1/orchestrations/query`                   | allowed, in `default` |
| `provision-manager-api:second:read`                                     | `/api/v1alpha1/namespaces/second/orchestrations/query` | allowed, in `second`  |
| `provision-manager-api:second:read`, `provision-manager-api:third:read` | `/api/v1alpha1/namespaces/third/orchestrations/query`  | allowed, in `third`   |
| `provision-manager-api:second:read`                                     | `/api/v1alpha1/namespaces/third/orchestrations/query`  | rejected, `403`       |
| `provision-manager-api:second:read`                                     | `/api/v1alpha1/orchestrations/query`                   | rejected, `403`       |
| `provision-manager-api:read`                                            | `/api/v1alpha1/namespaces/second/orchestrations/query` | rejected, `403`       |

Every read and every write of a request is limited to its namespace:

- lists and queries only return the definitions and orchestrations of the namespace
- an orchestration of another namespace is not found
- definitions are created in, and deleted from, the namespace
- the processing and output data of an orchestration are only returned with the admin scope of its namespace

A payload may name a namespace in its `namespace` field. A request whose payload names a namespace other than that of
the request is rejected with `403 Forbidden`.

When authentication is disabled, every scope is granted, so a request may use the routes of any namespace.

## Upgrading

The subjects, the consumers and the KV keys of the versions with namespaces differ from those of the versions before.
Messages in the stream on the old subjects are not delivered to the new consumers, and orchestrations stored under the
old keys are not continued. Drain the system before the upgrade:

1. Stop the managers, such as the Tenant Manager, so that no new orchestrations are started.
2. Wait until every orchestration has completed or failed: a query of the orchestrations in the states initialized (0) or
   running (1), for example `POST /api/v1alpha1/orchestrations/query` with the predicate `state = 0 OR state = 1`,
   returns none, and `nats stream info <stream>` shows no messages.
3. Stop the Provision Manager and the agents.
4. Widen the subjects of the stream from `event.*` to `event.>`, for example with
   `nats stream edit <stream> --subjects 'event.>'`. A component widens them on start if its NATS user may update the
   stream, and fails to start otherwise.
5. Change the NATS permissions to the new subjects and consumers, see [`least-privilege.conf`](nats/least-privilege.conf).
6. Start the new versions. Optionally delete the old durable consumers `cfm-orchestration`,
   `cfm-orchestration-response` and `<activity-type>` of the stream.

The index entries of orchestrations that finished before the upgrade move to the default namespace, so queries still
return them. Their state stays under the old keys in the KV bucket, so the Provision Manager no longer returns their
details, and no longer de-duplicates a manifest against them.

## Limits

Managers that share a Provision Manager instance share the Provision Manager process, which every manager trusts, and
the retry and timeout settings of the engine.
