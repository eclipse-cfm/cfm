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

//go:generate mockery

package api

import (
	"context"
	"encoding/json"
	"iter"
	"reflect"

	"time"

	"github.com/metaform/connector-fabric-manager/common/model"
	"github.com/metaform/connector-fabric-manager/common/query"
	"github.com/metaform/connector-fabric-manager/common/store"
	"github.com/metaform/connector-fabric-manager/common/system"
)

const (
	ProvisionManagerKey  system.ServiceType = "pmapi:ProvisionManager"
	DefinitionStoreKey   system.ServiceType = "pmapi:DefinitionStore"
	OrchestratorKey      system.ServiceType = "pmapi:Orchestrator"
	DefinitionManagerKey system.ServiceType = "pmapi:DefinitionManager"
)

// ProvisionManager handles orchestration execution and resource management.
type ProvisionManager interface {

	// Start initializes a new orchestration and starts its execution.
	// If a recoverable error is encountered one of model.RecoverableError, model.ClientError, or model.FatalError will be returned.
	Start(ctx context.Context, manifest *model.OrchestrationManifest) (*Orchestration, error)

	// Cancel terminates an orchestration execution.
	// If a recoverable error is encountered one of model.RecoverableError, model.ClientError, or model.FatalError will be returned.
	Cancel(ctx context.Context, orchestrationID string) error

	// GetOrchestration returns an orchestration by its ID or nil if not found.
	GetOrchestration(ctx context.Context, orchestrationID string) (*Orchestration, error)

	// QueryOrchestrations returns a sequence of orchestration entries matching the given predicate.
	QueryOrchestrations(
		ctx context.Context,
		predicate query.Predicate,
		options store.PaginationOptions) iter.Seq2[*OrchestrationEntry, error]

	// CountOrchestrations returns the number of orchestrations matching the given predicate.
	CountOrchestrations(ctx context.Context, predicate query.Predicate) (int64, error)
}

// Orchestrator manages asynchronous execution of orchestrations.
// Implementations must support idempotent behavior.
type Orchestrator interface {

	// Execute starts the execution of the specified orchestration, processing its steps and activities.
	Execute(ctx context.Context, orchestration *Orchestration) error

	// GetOrchestration retrieves an Orchestration by its ID or nil if not found.
	GetOrchestration(ctx context.Context, id string) (*Orchestration, error)
}

// ActivityProcessor executes activities for a given type.
//
// If the execution completes successfully, the processor returns ActivityResultComplete.
//
// If the processor returns ActivityResultWait, the activity will remain outstanding until completion is asynchronously signaled.
//
// If the processor returns ActivityResultSchedule, the orchestration engine will reschedule message delivery in the duration
// defined by WaitOnReschedule.
//
// If the processor encounters an error, it returns an ActivityResultRetryError or an ActivityResultFatalError.
type ActivityProcessor interface {
	Process(activityContext ActivityContext) ActivityResult
}

type ActivityResultType int

const (
	ActivityResultWait       = 0
	ActivityResultComplete   = 1
	ActivityResultSchedule   = 2
	ActivityResultRetryError = -1
	ActivityResultFatalError = -2

	DeployDiscriminator  Discriminator = "deploy"
	DisposeDiscriminator Discriminator = "dispose"
)

type ActivityResult struct {
	Result           ActivityResultType
	WaitOnReschedule time.Duration
	Error            error
}

type Discriminator string

func (ad Discriminator) String() string {
	return string(ad)
}

// ActivityContext provides context to current activity, including access to persistent storage.
type ActivityContext interface {
	// OID returns the ID of the orchestration that owns the activity
	OID() string

	// ID returns the ID of the current activity
	ID() string

	// Discriminator returns the discriminator of the current activity; may be empty
	Discriminator() Discriminator

	// SetValue sets a value in the persistent context that can be accessed by other activities
	SetValue(key string, value any)

	// Value retrieves a value from the context
	Value(key string) (any, bool)

	// Values returns the map of persistent context values
	Values() map[string]any

	// ReadValues deserializes the payload into the given result object; must be a pointer. Use JSON tags to control field names and validation.
	ReadValues(result any) error

	// Delete removes a persistent value from the context
	Delete(key string)

	// SetOutputValue sets a value to be returned by the orchestrator
	SetOutputValue(key string, value any)

	// OutputValues returns the map of output values to be returned by the orchestrator
	OutputValues() map[string]any

	// Context returns the underlying context
	Context() context.Context
}

type DefinitionManager interface {
	CreateOrchestrationDefinition(ctx context.Context, definition *OrchestrationDefinition) (*OrchestrationDefinition, error)
	DeleteOrchestrationDefinition(ctx context.Context, templateRef string) error
	GetOrchestrationDefinitions(ctx context.Context) ([]OrchestrationDefinition, error)

	CreateActivityDefinition(ctx context.Context, definition *ActivityDefinition) (*ActivityDefinition, error)
	DeleteActivityDefinition(ctx context.Context, atype ActivityType) error
	GetActivityDefinitions(ctx context.Context) ([]ActivityDefinition, error)
}

type defaultActivityContext struct {
	activity       Activity
	oID            string
	context        context.Context
	processingData map[string]any
	outputData     map[string]any
}

func NewActivityContext(
	ctx context.Context,
	oID string,
	activity Activity,
	processingData map[string]any,
	outputData map[string]any) ActivityContext {
	return defaultActivityContext{
		activity:       activity,
		oID:            oID,
		context:        ctx,
		processingData: processingData,
		outputData:     outputData,
	}
}

func (d defaultActivityContext) Context() context.Context {
	return d.context
}

func (d defaultActivityContext) Discriminator() Discriminator {
	return d.activity.Discriminator
}

func (d defaultActivityContext) ID() string {
	return d.activity.ID
}

func (d defaultActivityContext) OID() string {
	return d.oID
}

func (d defaultActivityContext) SetValue(key string, value any) {
	d.processingData[key] = value
}

func (d defaultActivityContext) Value(key string) (any, bool) {
	value, ok := d.processingData[key]
	return value, ok
}

func (d defaultActivityContext) ReadValues(result any) error {
	input, err := json.Marshal(d.processingData)
	if err != nil {
		return err
	}
	err = json.Unmarshal(input, result)
	if err != nil {
		return err
	}

	kind := reflect.TypeOf(result).Kind()
	if kind == reflect.Ptr {
		kind = reflect.TypeOf(result).Elem().Kind()
	}
	if kind == reflect.Struct || kind == reflect.Interface {
		if err := model.Validator.Struct(result); err != nil {
			return err
		}
	}
	return nil
}

func (d defaultActivityContext) Values() map[string]any {
	return d.processingData
}

func (d defaultActivityContext) Delete(key string) {
	delete(d.processingData, key)
}

func (d defaultActivityContext) SetOutputValue(key string, value any) {
	d.outputData[key] = value
}

func (d defaultActivityContext) OutputValues() map[string]any {
	return d.outputData
}
