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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"strconv"
	"strings"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
	"github.com/eclipse-cfm/cfm/common/types"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const contentType = "application/json"

// ErrorResponse represents a generic JSON error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code,omitempty"`
	ID      string `json:"id,omitempty"`
}

type HttpHandler struct {
	Monitor system.LogMonitor
}

func (h HttpHandler) WriteError(w http.ResponseWriter, message string, statusCode int) {
	h.WriteErrorWithID(w, message, statusCode, "")
}

// WriteErrorWithID writes a JSON error response to the response writer
func (h HttpHandler) WriteErrorWithID(w http.ResponseWriter, message string, statusCode int, errorID string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
		Code:    statusCode,
	}

	if errorID != "" {
		response.ID = errorID
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.Monitor.Infow("Error encoding JSON error response: %v", err)
	}
}

func (h HttpHandler) InvalidMethod(w http.ResponseWriter, req *http.Request, expectedMethod string) bool {
	if req.Method != expectedMethod {
		h.WriteError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return true
	}
	return false
}

func (h HttpHandler) ReadPayload(w http.ResponseWriter, req *http.Request, payload any) bool {
	// Read the request body
	body, err := io.ReadAll(req.Body)
	if err != nil {
		h.WriteError(w, "Failed to read request body", http.StatusBadRequest)
		return false
	}

	defer req.Body.Close()

	if err := json.Unmarshal(body, payload); err != nil {
		h.WriteError(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return false
	}

	if err := model.Validator.Struct(payload); err != nil {
		h.WriteError(w, "Invalid payload: "+err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func (h HttpHandler) HandleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, types.ErrNotFound):
		h.WriteError(w, "Not found", http.StatusNotFound)
	case errors.Is(err, types.ErrConflict):
		h.WriteError(w, "Conflict", http.StatusConflict)
	case errors.Is(err, types.ErrInvalidInput):
		h.WriteError(w, "Invalid input", http.StatusBadRequest)
	case types.IsClientError(err):
		var clientErr types.ClientError
		errors.As(err, &clientErr)
		if badReq, ok := clientErr.(types.BadRequestError); ok {
			h.WriteError(w, fmt.Sprintf("Bad request: %s", badReq.Message), http.StatusBadRequest)
		} else {
			h.WriteError(w, fmt.Sprintf("Client error: %v", clientErr), http.StatusBadRequest)
		}
	case types.IsFatal(err):
		id := uuid.New().String()
		h.Monitor.Infow("Internal Error [%s]: %v", id, err)
		h.WriteError(w, fmt.Sprintf("Internal server error occurred [%s]", id), http.StatusInternalServerError)
	default:
		h.WriteError(w, fmt.Sprintf("Operation failed: %s", err.Error()), http.StatusInternalServerError)
	}
}

func (h HttpHandler) Created(w http.ResponseWriter) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusCreated)
}

func (h HttpHandler) Accepted(w http.ResponseWriter) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusAccepted)
}

func (h HttpHandler) ResponseAccepted(w http.ResponseWriter, response any) {
	h.Accepted(w)
	h.write(w, response)
}

func (h HttpHandler) OK(w http.ResponseWriter) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
}

func (h HttpHandler) ResponseOK(w http.ResponseWriter, response any) {
	h.OK(w)
	h.write(w, response)
}

func (h HttpHandler) ResponseCreated(w http.ResponseWriter, response any) {
	h.Created(w)
	h.write(w, response)
}

func (h HttpHandler) write(w http.ResponseWriter, response any) {
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.Monitor.Infow("Error encoding response: %v", err)
	}
}

func (h HttpHandler) ExtractPathVariable(w http.ResponseWriter, req *http.Request, key string) (string, bool) {
	value := chi.URLParam(req, key)
	if value == "" {
		h.WriteError(w, fmt.Sprintf("Missing %s parameter", key), http.StatusBadRequest)
		return "", false
	}
	return value, true
}

func (h HttpHandler) WriteLinkHeaders(w http.ResponseWriter, path string, offset int64, limit int64, totalCount int64) {
	var links []string
	selfLink := fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"self\"", path, offset, limit)
	links = append(links, selfLink)

	if offset+limit < totalCount {
		nextLink := fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"next\"", path, offset+limit, limit)
		links = append(links, nextLink)
	}

	if offset > 0 {
		prevOffset := offset - limit
		if prevOffset < 0 {
			prevOffset = 0
		}
		prevLink := fmt.Sprintf("<%s?offset=%d&limit=%d>; rel=\"prev\"", path, prevOffset, limit)
		links = append(links, prevLink)
	}

	firstLink := fmt.Sprintf("<%s?offset=0&limit=%d>; rel=\"first\"", path, limit)
	links = append(links, firstLink)

	if limit > 0 {
		lastOffset := ((totalCount - 1) / limit) * limit
		if lastOffset < 0 {
			lastOffset = 0
		}
		lastLink := fmt.Sprintf("<%s=%d&limit=%d>; rel=\"last\"", path, lastOffset, limit)
		links = append(links, lastLink)
	}

	w.Header().Set("Link", strings.Join(links, ", "))
	w.Header().Set("X-Total-Count", fmt.Sprintf("%d", totalCount))
}

func QueryEntities[T any](
	h *HttpHandler,
	w http.ResponseWriter,
	req *http.Request,
	path string,
	countFn func(context.Context, query.Predicate) (int64, error),
	queryFn func(context.Context, query.Predicate, store.PaginationOptions) iter.Seq2[T, error],
	transformFn func(T) any,
	txContext store.TransactionContext) {

	if h.InvalidMethod(w, req, http.MethodPost) {
		return
	}

	txContext.Execute(req.Context(), func(ctx context.Context) error {
		var queryMessage model.Query
		if !h.ReadPayload(w, req, &queryMessage) {
			return nil
		}
		offset := queryMessage.Offset
		limit := queryMessage.Limit
		if limit == 0 || limit > 10000 {
			limit = 10000
		}

		predicate, err := query.ParsePredicate(queryMessage.Predicate)
		if err != nil {
			h.WriteError(w, fmt.Sprintf("Client error: %v", err), http.StatusBadRequest)
			return nil
		}

		totalCount, err := countFn(ctx, predicate)
		if err != nil {
			h.HandleError(w, err)
			return nil
		}

		h.WriteLinkHeaders(w, path, offset, limit, totalCount)

		h.OK(w)
		_, err = w.Write([]byte("["))
		if err != nil {
			h.Monitor.Infow("Error writing response: %v", err)
			return nil
		}
		first := true

		for entity, err := range queryFn(ctx, predicate, store.PaginationOptions{
			Offset: offset,
			Limit:  limit,
		}) {
			if err != nil {
				h.Monitor.Infow("Error streaming results: %v", err)
				break
			}

			if !first {
				_, err = w.Write([]byte(","))
				if err != nil {
					h.Monitor.Infow("Error writing response: %v", err)
					return nil
				}
			}
			first = false

			response := transformFn(entity)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				h.Monitor.Infow("Error encoding response: %v", err)
				break
			}

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		_, err = w.Write([]byte("]"))
		if err != nil {
			h.Monitor.Infow("Error writing response: %v", err)
			return nil
		}
		return nil
	})
}

func ListEntities[T any](
	h *HttpHandler,
	w http.ResponseWriter,
	req *http.Request,
	path string,
	countFn func(context.Context) (int64, error),
	listFn func(context.Context, store.PaginationOptions) iter.Seq2[T, error],
	transformFn func(T) any,
	txContext store.TransactionContext) {

	if h.InvalidMethod(w, req, http.MethodGet) {
		return
	}

	txContext.Execute(req.Context(), func(ctx context.Context) error {

		totalCount, err := countFn(ctx)
		if err != nil {
			h.HandleError(w, err)
			return nil
		}
		offset := req.URL.Query().Get("offset")
		limit := req.URL.Query().Get("limit")
		options := store.PaginationOptions{}
		if offset != "" {
			offsetVal, err := strconv.Atoi(offset) // Safe conversion as prevision will not be lost
			if err != nil {
				h.WriteError(w, fmt.Sprintf("Invalid offset: %s", err), http.StatusBadRequest)
				return nil
			}
			options.Offset = int64(offsetVal) // Safe conversion as prevision will not be lost
		}
		if limit != "" {
			limitVal, err := strconv.Atoi(limit)
			if err != nil {
				h.WriteError(w, fmt.Sprintf("Invalid limit: %s", err), http.StatusBadRequest)
				return nil
			}
			options.Limit = int64(limitVal)
		}

		h.WriteLinkHeaders(w, path, options.Offset, options.Limit, totalCount)

		h.OK(w)
		_, err = w.Write([]byte("["))
		if err != nil {
			h.Monitor.Infow("Error writing response: %v", err)
			return nil
		}
		first := true

		for entity, err := range listFn(ctx, options) {
			if err != nil {
				h.Monitor.Infow("Error streaming results: %v", err)
				break
			}

			if !first {
				_, err = w.Write([]byte(","))
				if err != nil {
					h.Monitor.Infow("Error writing response: %v", err)
					return nil
				}
			}
			first = false

			response := transformFn(entity)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				h.Monitor.Infow("Error encoding response: %v", err)
				break
			}

			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		_, err = w.Write([]byte("]"))
		if err != nil {
			h.Monitor.Infow("Error writing response: %v", err)
			return nil
		}
		return nil
	})
}
