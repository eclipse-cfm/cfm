# Running CFM on Postgres

The Tenant Manager (TM) and the Provision Manager (PM) store their data in Postgres when `postgres` is set to `true` and
`dsn` holds the connection string (for example `TM_POSTGRES` and `TM_DSN` in environment variables).

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
Postgres advisory lock, so replicas, and TM and PM sharing a database, can start at the same time.
