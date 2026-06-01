# Mandatory Authn/Authz on TM and PM APIs

## Decision

All client-facing APIs require access control, specifically authentication and authorization using OAuth2. This includes
all APIs exposed by the Tenant Manager (TM) and Provision Manager (PM).

## Rationale

APIs exposed by TM and PM should be protected by authentication and authorization to ensure that only authorized clients
can access them. Typically, different clients (or types of clients) need to access the PM and TM APIs. While the PM APIs
are likely to be used by internal clients when setting up or inspecting the CFM deployment, the TM APIs are expected to
be used by client applications such as onboarding apps to manipulate tenant- or participant-related information.

## Approach

### Scope-based access control

In an initial implementation, each API is protected by a read-scope and a write-scope:

- `tenant-manager-api:read`: scope needed by all read-only APIs exposed by TM
- `tenant-manager-api:write`: scope needed by all write APIs exposed by TM
- `provision-manager-api:read`: scope needed by all read-only APIs exposed by PM
- `provision-manager-api:write`: scope needed by all write APIs exposed by PM

Note that sometimes read-only APIs are implemented as `POST` endpoints, e.g. when issuing a query. In these cases,
the `read` scope is sufficient.

### Implementation

Authentication and authorization are implemented as two distinct layers on top of a [chi](https://github.com/go-chi/chi)
HTTP router.

**Authentication** is handled by an `AuthValidator` middleware that is registered on the `/api/v1alpha1` route group. It
validates incoming JWT bearer tokens using OIDC. By default, the IdP's OIDC discovery endpoint (
`.well-known/openid-configuration`) is used to obtain the JWKS URL dynamically; alternatively an explicit JWKS URL can
be configured. On success, the middleware extracts a `Claims` struct (subject, scopes, roles) and stores it in the
request context. When auth is disabled (e.g. in tests), a no-op validator is registered instead, and all downstream
authorization checks pass automatically.

> Note: authentication is **enabled by default**.

**Authorization** is enforced inside each handler via `handler.IsAuthorized(w, req, rules...)`. The function retrieves
the `Claims` from the request context and evaluates each `ClaimsRule` against them. Two rules exist:

- `RequireScope(scope)` – the token's `scope` claim must contain the given value.
- `RequireRole(roles...)` – the token must carry at least one of the given roles (OR logic). _This is not used yet_.

When no claims are present (auth disabled), the call succeeds unconditionally. When claims are present but a rule is not
satisfied, `IsAuthorized` writes the appropriate HTTP error (`401` for missing claims, `403` for insufficient claims)
and returns `false`, causing the handler to abort.

Health check endpoints (`/health`) sit outside the auth-protected route group and are always unauthenticated.

The auth subsystem is wired together via a dependency-injection registry: `AuthServiceAssembly` resolves the OIDC
configuration, creates the validator, and registers it under `cfmauth.ValidatorKey`. Route assemblies for TM and PM both
declare a `Requires()` dependency on that key, ensuring the validator is always available before routes are registered.

