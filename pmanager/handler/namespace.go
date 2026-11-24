//  Copyright (c) 2025 Metaform Systems, Inc
//
//  This program and the accompanying materials are made available under the
//  terms of the Apache License, Version 2.0 which is available at
//  https://www.apache.org/licenses/LICENSE-2.0
//
//  SPDX-License-Identifier: Apache-2.0
//
//  Contributors:
//       Metaform Systems, Inc. - initial API and implementation
//

package handler

import (
	"context"
	"fmt"
	"net/http"

	cfmauth "github.com/eclipse-cfm/cfm/common/auth"
	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/go-chi/chi/v5"
)

// namespaceParam is the URL parameter of the routes under /namespaces/{namespace}, which selects the namespace a request
// reads and writes. The routes without it are in model.DefaultNamespace.
const namespaceParam = "namespace"

// scopePrefix starts every scope of the Provision Manager API. A scope is either "<scopePrefix><permission>", which
// grants the permission in model.DefaultNamespace, or "<scopePrefix><namespace>:<permission>", which grants it in the
// namespace.
const scopePrefix = "provision-manager-api:"

// permission is the last part of a scope of the Provision Manager API
type permission string

const (
	permissionRead  permission = "read"
	permissionWrite permission = "write"
	// permissionAdmin is needed to change activity and orchestration definitions, which decide what the agents do, and
	// to see the processing and output data of orchestrations.
	permissionAdmin permission = "admin"
)

// namespaceScope returns the scope that grants the permission in the namespace
func namespaceScope(namespace string, perm permission) string {
	return scopePrefix + namespace + ":" + string(perm)
}

// hasPermission returns true when the claims grant the permission in the namespace. In the default namespace, the scope
// without a namespace grants it too.
func hasPermission(claims *cfmauth.Claims, namespace string, perm permission) bool {
	if claims.HasScope(namespaceScope(namespace, perm)) {
		return true
	}
	return namespace == model.DefaultNamespace && claims.HasScope(scopePrefix+string(perm))
}

type namespaceContextKey struct{}

// authorize selects the namespace of the request and checks that the caller has the permission in it. It returns false
// and writes an error response if the request is not authorized. The namespace is attached to the request, see
// requestNamespace.
//
// The namespace is the namespaceParam URL parameter of the routes under /namespaces/{namespace}. The routes without it
// are in model.DefaultNamespace.
func (h *PMHandler) authorize(w http.ResponseWriter, req *http.Request, perm permission) (*http.Request, bool) {
	claims, ok := cfmauth.ClaimsFromContext(req.Context())
	if !ok || claims == nil {
		h.Monitor.Warnw("Request has no authentication claims, refusing it", "path", req.URL.Path)
		h.WriteError(w, "Not authenticated", http.StatusUnauthorized)
		return nil, false
	}

	namespace := chi.URLParam(req, namespaceParam)
	if namespace == "" {
		namespace = model.DefaultNamespace
	} else if err := model.ValidateNamespace(namespace); err != nil {
		h.WriteError(w, "invalid namespace: "+err.Error(), http.StatusBadRequest)
		return nil, false
	}

	if !hasPermission(claims, namespace, perm) {
		required := "'" + namespaceScope(namespace, perm) + "'"
		if namespace == model.DefaultNamespace {
			required = "'" + scopePrefix + string(perm) + "' or " + required
		}
		h.WriteError(w, "Access not granted: required scope "+required+" not present", http.StatusForbidden)
		return nil, false
	}
	return req.WithContext(context.WithValue(req.Context(), namespaceContextKey{}, namespace)), true
}

// namespacePath returns the path of a route relative to /api/v1alpha1, prefixed with /namespaces/{namespace} when the
// request was made on a namespaced route
func namespacePath(req *http.Request, path string) string {
	if namespace := chi.URLParam(req, namespaceParam); namespace != "" {
		return "/namespaces/" + namespace + path
	}
	return path
}

// requestNamespace returns the namespace that authorize selected for the request
func requestNamespace(req *http.Request) string {
	if namespace, ok := req.Context().Value(namespaceContextKey{}).(string); ok {
		return namespace
	}
	return model.DefaultNamespace
}

// payloadNamespace returns the namespace of the request, and writes a forbidden response if the payload names a
// different namespace. A payload without a namespace is in the namespace of the request.
func (h *PMHandler) payloadNamespace(w http.ResponseWriter, req *http.Request, payloadNamespace string) (string, bool) {
	namespace := requestNamespace(req)
	if payloadNamespace != "" && payloadNamespace != namespace {
		h.WriteError(w, fmt.Sprintf("namespace '%s' of the payload is not the namespace '%s' of the request", payloadNamespace, namespace), http.StatusForbidden)
		return "", false
	}
	return namespace, true
}
