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

Each API is protected by a read scope, a write scope and an admin scope:

- `tenant-manager-api:read`: scope needed by all read-only APIs exposed by TM
- `tenant-manager-api:write`: scope needed to create, change and delete tenants and to deploy and dispose participant
  profiles
- `tenant-manager-api:admin`: scope needed to create and delete cells, and to create, delete and deploy dataspace
  profiles. These are platform resources that all tenants share. The scope also lifts the tenant ownership restriction
  described below.
- `provision-manager-api:read`: scope needed by all read-only APIs exposed by PM
- `provision-manager-api:write`: scope needed to start orchestrations
- `provision-manager-api:admin`: scope needed to create and delete activity and orchestration definitions, which decide
  what the agents do. Only callers with this scope get the `processingData` and `outputData` of an orchestration, which
  carry the payload it was started with and the values its activities produced. Other callers get empty maps.

Note that sometimes read-only APIs are implemented as `POST` endpoints, e.g. when issuing a query. In these cases,
the `read` scope is sufficient. The admin scopes don't imply the read or write scopes: a client that manages tenants of
other clients needs both the write and the admin scope.

### Tenant ownership

A tenant is owned by the client that created it: TM records the subject (`sub`) of the caller's token as the tenant's
owner. A participant profile is owned by the owner of its tenant. A caller without `tenant-manager-api:admin` can only
see and change the tenants and participant profiles it owns:

- getting, changing or deleting another client's tenant, and any request below `/tenants/{id}/participant-profiles`
  for it, returns `404`, as if the tenant didn't exist;
- listing and querying tenants and participant profiles only returns the caller's own. The owner condition is added to
  the caller's query predicate on the server, so the predicate cannot widen the result.

A token without a subject is refused with `403` unless it has the admin scope. Tenants and participant profiles that
were created before owners were recorded have no owner, so only admin callers can reach them. When auth is disabled,
the anonymous claims carry every scope, so callers have full access and new tenants are owned by `anonymous`.

The handler attaches the caller's access to the request context, and the tenant and participant profile services read
it from there. A service call without an access in its context is refused rather than treated as unrestricted.

### Orchestration IDs

The ID of an orchestration is chosen by the caller and de-duplicates starts: submitting the same manifest again returns
the existing orchestration, which lets a caller retry a start, and resumes it when its start didn't complete. A manifest
that reuses the ID of an orchestration of a different type or correlation ID is refused with `409`.

### Implementation

Authentication and authorization are implemented as two distinct layers on top of a [chi](https://github.com/go-chi/chi)
HTTP router.

**Authentication** is handled by an `AuthValidator` middleware that is registered on the `/api/v1alpha1` route group. It
validates incoming JWT bearer tokens using OIDC. By default, the IdP's OIDC discovery endpoint (
`.well-known/openid-configuration`) is used to obtain the JWKS URL dynamically; alternatively an explicit JWKS URL can
be configured. On success, the middleware extracts a `Claims` struct (subject, scopes, roles) and stores it in the
request context. When auth is disabled (e.g. in tests), a no-op validator is registered instead. It attaches anonymous
claims that satisfy every rule, so all downstream authorization checks pass.

> Note: authentication is **enabled by default**.

When auth is enabled, `auth.audience` is mandatory, also when `auth.audienceClaim` names a claim other than `aud`.
Otherwise, a token the IdP issued to any client would be accepted. Rejected tokens get a generic `401`; the verifier's
reason is only logged.

In `production` mode (the default runtime mode), two settings are refused at startup:

- `auth.enabled=false`, unless `auth.allowDisabled=true` is also set. A single mis-set flag must not switch off API
  security. Whenever auth is disabled, a warning is logged on startup.
- `auth.skipIssuerCheck=true`. When the discovery URL differs from the issuer in the tokens, set `auth.expectedIssuer`
  instead.

**Authorization** is enforced inside each handler via `handler.IsAuthorized(w, req, rules...)`. The function retrieves
the `Claims` from the request context and evaluates each `ClaimsRule` against them. Two rules exist:

- `RequireScope(scope)` – the token's `scope` claim must contain the given value.
- `RequireRole(roles...)` – the token must carry at least one of the given roles (OR logic). _This is not used yet_.

Authorization fails closed: when no claims are present, the request did not pass through the auth middleware (for
example, a route registered outside the protected group), so `IsAuthorized` writes a `401` and returns `false`. When
claims are present but a rule is not satisfied, it writes a `403`. In both cases the handler aborts.

Health check endpoints (`/health`) sit outside the auth-protected route group and are always unauthenticated.

The auth subsystem is wired together via a dependency-injection registry: `AuthServiceAssembly` resolves the OIDC
configuration, creates the validator, and registers it under `cfmauth.ValidatorKey`. Route assemblies for TM and PM both
declare a `Requires()` dependency on that key, ensuring the validator is always available before routes are registered.

