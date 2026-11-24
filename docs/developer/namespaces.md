# Namespaces

Several managers can use one Provision Manager instance. Each manager works in its own namespace, which keeps its
orchestration definitions, activity definitions and orchestrations apart from those of the other managers.

## The namespace

A namespace is a single NATS subject token: letters, digits, `_` and `-`. Dots, which separate subject tokens, and the
wildcards `*` and `>` are not allowed.

A manifest, a definition or an orchestration without a namespace belongs to the default namespace, `default`. A manager
that does not use namespaces, such as the Tenant Manager, therefore keeps working without a change.

## Definitions

An orchestration definition and an activity definition are identified by the pair of namespace and type. Two managers can
register the same type, each in its own namespace. An orchestration definition can only use the activity definitions of
its own namespace.

The Postgres store keys a definition by `<namespace>/<type>` and keeps the namespace in its own `namespace` column. On
start, the store moves definitions that were stored before namespaces were introduced into the default namespace.

## Orchestrations

An orchestration runs with the definitions of the namespace of its manifest, and keeps that namespace. Its index entry
has the namespace too. The compensation of a failed orchestration runs in the namespace of the failed orchestration.
Orchestration entries that were stored before namespaces were introduced are in the default namespace.

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

## Limits

The namespace does not yet separate the NATS subjects, the consumers, the keys of the orchestration state in the KV
bucket or the NATS permissions. Managers that share a Provision Manager instance must use different orchestration and
manifest IDs, and their agents can read each other's messages and orchestration state.
