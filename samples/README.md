# Samples

The samples show how to use CFM as a generic orchestration engine, without any dataspace component. They deploy and
dispose a made-up resource, the storage of a "workspace":

| Sample                                                                             | Shows                                                                                   |
|------------------------------------------------------------------------------------|-----------------------------------------------------------------------------------------|
| [`agent/`](agent/main.go)                                                          | A custom activity agent built on [`pmanager/natsagent`](../pmanager/natsagent)          |
| [`manager/`](manager/main.go)                                                      | A minimal manager that starts an orchestration over NATS and waits for its response     |
| [`definitions/activity-definition.json`](definitions/activity-definition.json)     | An activity definition with an input and an output schema                               |
| [`definitions/orchestration-template.json`](definitions/orchestration-template.json) | An orchestration template with two orchestration types, a schema and a compensation   |
| [`definitions/manifest.json`](definitions/manifest.json)                           | An orchestration manifest for the REST API                                              |
| [`nats/nats-auth.conf`](nats/nats-auth.conf)                                       | A NATS server configuration with one user and least-privilege permissions per component |

For a complete, dataspace-specific set of a manager and agents on the same engine, see
[cfm-dataspace](https://github.com/eclipse-cfm/cfm-dataspace).

## Run the engine and the agent

The commands are run from the root of the repository. They need Go and Docker.

1. Start NATS with JetStream:

   ```sh
   docker run --rm -p 4222:4222 nats:2 -js
   ```

2. Start the Provision Manager. Without `PM_POSTGRES` it keeps its definitions in memory. Development mode allows the
   anonymous NATS connection and the disabled API authentication, which production mode refuses:

   ```sh
   PM_URI=nats://localhost:4222 PM_BUCKET=cfm-bucket PM_STREAM=cfm-stream PM_AUTH_ENABLED=false \
     go run ./pmanager/cmd/server -mode development
   ```

   The API is served on port `8181` (`PM_HTTPPORT`).

   The Provision Manager also reads a configuration file named `pm` from `/etc/appname/`, `$HOME/.appname` and the
   current directory. If such a file sets `postgres`, the Provision Manager uses Postgres. Set `CFM_CONFIG_DIR` to an
   empty directory to ignore these files.

3. Start the sample agent. It reads its configuration with the prefix `SAMPLEAGENT_`:

   ```sh
   SAMPLEAGENT_URI=nats://localhost:4222 SAMPLEAGENT_BUCKET=cfm-bucket SAMPLEAGENT_STREAM=cfm-stream \
     go run ./samples/agent -mode development
   ```

## Register the definitions

An activity definition must exist before an orchestration template can use it. Both calls need the `admin` permission
when authentication is enabled:

```sh
PM=http://localhost:8181/api/v1alpha1

curl -X POST $PM/activity-definitions \
  -H 'Content-Type: application/json' -d @samples/definitions/activity-definition.json

curl -X POST $PM/orchestration-definitions \
  -H 'Content-Type: application/json' -d @samples/definitions/orchestration-template.json
```

The template creates one orchestration definition per entry of `activities`, here `sample.workspace.deploy` and
`sample.workspace.dispose`. List them, or get the definitions of one template:

```sh
curl $PM/activity-definitions
curl $PM/orchestration-definitions
curl $PM/orchestration-definitions/sample-workspace
```

The template is accepted because every required input of `sample-storage` (`workspace`) is declared in its `schema`.
Remove `workspace` from the `schema` and the request is rejected.

## Start an orchestration

### With the sample manager, over NATS

A manager sends an orchestration manifest to `event.<namespace>.cfm-orchestration` and receives the response on
`event.<namespace>.cfm-orchestration-response`:

```sh
go run ./samples/manager -workspace acme
```

```text
Started orchestration 6f0c... of type sample.workspace.deploy in namespace default
Orchestration 6f0c... completed with the properties map[bucketName:bucket-acme]
```

`bucketName` is the output value the agent set with `SetOutputValue`. Dispose the workspace with
`-type sample.workspace.dispose`.

### With the REST API

```sh
curl -X POST $PM/orchestrations \
  -H 'Content-Type: application/json' -d @samples/definitions/manifest.json
```

The call returns `202 Accepted` with the orchestration. Posting the same manifest again returns the existing
orchestration and does not start a second one. Its outcome is still published to the response subject of the
namespace, and its state can be read from the API:

```sh
curl $PM/orchestrations/workspace-acme-deploy-1

curl -X POST $PM/orchestrations/query \
  -H 'Content-Type: application/json' \
  -d '{"predicate": "correlationId = '\''acme'\'' AND state = 2"}'
```

The states are `0` initialized, `1` running, `2` completed and `3` errored. The predicate syntax is described in
[The CFM Query Language](../docs/developer/architecture/query/cfm.query.language.md).

## Use a namespace

Several managers can share one Provision Manager, each in its own [namespace](../docs/developer/namespaces.md). The
calls above use the routes without a namespace, which are in the namespace `default`. Every route is also available
under `/namespaces/<namespace>`, and then only reads and writes that namespace:

```sh
NS=$PM/namespaces/team-a

curl -X POST $NS/activity-definitions \
  -H 'Content-Type: application/json' -d @samples/definitions/activity-definition.json
curl -X POST $NS/orchestration-definitions \
  -H 'Content-Type: application/json' -d @samples/definitions/orchestration-template.json

curl $NS/orchestration-definitions   # the definitions of team-a
curl $PM/orchestration-definitions   # the definitions of default
```

An agent executes the activities of one namespace, so `team-a` needs its own agent:

```sh
SAMPLEAGENT_NAMESPACE=team-a SAMPLEAGENT_URI=nats://localhost:4222 SAMPLEAGENT_BUCKET=cfm-bucket \
  SAMPLEAGENT_STREAM=cfm-stream go run ./samples/agent -mode development
```

```sh
go run ./samples/manager -namespace team-a -workspace acme
curl $NS/orchestrations/query -X POST -H 'Content-Type: application/json' -d '{"predicate": "true"}'
```

The same manifest ID, definition type and template ID can be used in `default` and in `team-a` without a conflict.

### With authentication

When authentication is enabled (`PM_AUTH_ENABLED` unset or `true`, with `PM_AUTH_JWKSURL` or `PM_AUTH_DISCOVERYURL` and
`PM_AUTH_AUDIENCE`), the `scope` claim of the access token decides which namespaces a caller may use:

| Call                                              | Scope the token needs                                                    |
|---------------------------------------------------|--------------------------------------------------------------------------|
| `GET  $PM/orchestration-definitions`              | `provision-manager-api:read` or `provision-manager-api:default:read`     |
| `POST $PM/orchestrations`                         | `provision-manager-api:write` or `provision-manager-api:default:write`   |
| `GET  $NS/orchestration-definitions`              | `provision-manager-api:team-a:read`                                      |
| `POST $NS/orchestrations`                         | `provision-manager-api:team-a:write`                                     |
| `POST $NS/activity-definitions`                   | `provision-manager-api:team-a:admin`                                     |

```sh
curl -H "Authorization: Bearer $TOKEN" $NS/orchestration-definitions
```

A token with the scopes of `team-a` only gets `403 Forbidden` on the routes of `default` and of any other namespace.
The processing and output data of an orchestration are only returned with the `admin` scope of its namespace.

## Use authenticated NATS

The steps above connect to NATS anonymously, which a component only accepts in development mode. In production mode,
which is the default, every component must authenticate, see
[Securing the NATS control plane](../docs/developer/nats/README.md). This section runs the same samples with one NATS
user per component.

1. Stop the Provision Manager, the agent and NATS, and start NATS with [`nats/nats-auth.conf`](nats/nats-auth.conf).
   It defines the users `pmanager`, `sample-agent` and `sample-manager`, each limited to the subjects it needs:

   ```sh
   docker run --rm -p 4222:4222 -v "$PWD/samples/nats/nats-auth.conf:/etc/nats/nats.conf" nats:2 -c /etc/nats/nats.conf
   ```

2. Start the Provision Manager with its user. The keys are `nats.auth.method`, `nats.auth.username` and
   `nats.auth.password` under the prefix of the component:

   ```sh
   PM_URI=nats://localhost:4222 PM_BUCKET=cfm-bucket PM_STREAM=cfm-stream PM_AUTH_ENABLED=false \
     PM_NATS_AUTH_METHOD=userinfo PM_NATS_AUTH_USERNAME=pmanager PM_NATS_AUTH_PASSWORD=pmanager-secret \
     go run ./pmanager/cmd/server -mode development
   ```

   `PM_AUTH_ENABLED` is about the REST API and is independent of the NATS authentication. Development mode is only
   kept here because the sample disables the authentication of the REST API.

3. Start the agent with its user. An agent built on `pmanager/natsagent` reads the same keys under its own prefix
   without any code. It authenticates, so it also starts in production mode, without `-mode development`:

   ```sh
   SAMPLEAGENT_URI=nats://localhost:4222 SAMPLEAGENT_BUCKET=cfm-bucket SAMPLEAGENT_STREAM=cfm-stream \
     SAMPLEAGENT_NATS_AUTH_METHOD=userinfo SAMPLEAGENT_NATS_AUTH_USERNAME=sample-agent \
     SAMPLEAGENT_NATS_AUTH_PASSWORD=sample-agent-secret \
     go run ./samples/agent
   ```

4. Register the definitions as in [Register the definitions](#register-the-definitions), then run the manager with its
   user. The sample manager resolves the same keys with `natsclient.AuthFromConfig`, under the prefix
   `SAMPLEMANAGER_`:

   ```sh
   SAMPLEMANAGER_NATS_AUTH_METHOD=userinfo SAMPLEMANAGER_NATS_AUTH_USERNAME=sample-manager \
     SAMPLEMANAGER_NATS_AUTH_PASSWORD=sample-manager-secret \
     go run ./samples/manager -workspace acme
   ```

Without the credentials, the components cannot use NATS any more:

- `go run ./samples/manager -workspace acme`, without the variables or with a wrong password, fails with
  `unable to access jetstream key value store with bucket cfm-bucket: nats: connection closed` instead of starting an
  orchestration. The log of the NATS server shows the `authentication error`.
- The agent without the variables does not start in production mode:
  `anonymous NATS connections are not allowed in production mode`.

The permissions also separate the components. The user `sample-manager` can start orchestrations, but it cannot publish
activity messages or responses, and it cannot use another namespace. With `-namespace team-a` it prints
`nats: permissions violation: Permissions Violation for Publish to "$JS.API.CONSUMER.CREATE.cfm-stream.team-a~cfm-orchestration-response..."`
and fails when its timeout of one minute ends, because the server does not answer a request it does not permit. To add
a namespace, add a manager user and an agent user for it, as
[`least-privilege.conf`](../docs/developer/nats/least-privilege.conf) does for the namespace `second`.

## Clean up

Orchestration definitions are deleted by the ID of their template, and before the activity definitions they use:

```sh
curl -X DELETE $PM/orchestration-definitions/sample-workspace
curl -X DELETE $PM/activity-definitions/sample-storage
```

An orchestration definition that orchestrations were started for cannot be deleted. After the steps above, stop the
Provision Manager and NATS instead: without Postgres, nothing is kept.
