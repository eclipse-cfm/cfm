# Running CFM on Postgres

The Provision Manager (PM) stores its definitions and its orchestration index in Postgres when the key `postgres` is
set and `dsn` holds the connection string (`PM_POSTGRES=true` and `PM_DSN` in environment variables). Without
`postgres`, it keeps them in memory, which is only suitable for development. The state of running orchestrations is
kept in the NATS key-value bucket in both cases.

Managers that build on the engine can use the same storage library (`common/sqlstore`) and the same keys. The Tenant
Manager of [cfm-dataspace](https://github.com/eclipse-cfm/cfm-dataspace), for example, is configured with `TM_POSTGRES`
and `TM_DSN`.

## Connection pool

Each replica keeps a bounded pool of connections. Size it so that the pools of all replicas of all components that use
the database stay below the server's `max_connections`, leaving room for administrative connections.

| Key                   | Description                                                                 | Default |
|-----------------------|-----------------------------------------------------------------------------|---------|
| `db.maxOpenConns`     | Maximum number of open connections, in use or idle                          | 25      |
| `db.maxIdleConns`     | Maximum number of idle connections kept for reuse, at most `maxOpenConns`   | 10      |
| `db.connMaxLifetime`  | Maximum time a connection is reused, for example `30m`                      | 30m     |
| `db.connMaxIdleTime`  | Maximum time a connection stays idle before it is closed                    | 5m      |

A request uses a single connection: store calls made while a transaction is open join that transaction.

## Schema

Each component creates its tables on startup if they don't exist. The statements run in one transaction under a
Postgres advisory lock, so replicas, and a manager and the PM sharing a database, can start at the same time.
