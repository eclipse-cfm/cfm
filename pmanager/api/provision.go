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
	"fmt"
	"iter"
	"reflect"

	"time"

	"github.com/eclipse-cfm/cfm/common/model"
	"github.com/eclipse-cfm/cfm/common/query"
	"github.com/eclipse-cfm/cfm/common/store"
	"github.com/eclipse-cfm/cfm/common/system"
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

// ActivityProcessor defines an interface for processing various activities within an orchestrated workflow.
// Process handles a generic activity, taking an ActivityContext and returning an ActivityResult.
// ProcessDeploy handles deployment-specific activities, taking an ActivityContext and returning an ActivityResult.
// ProcessDispose handles disposal-specific activities, taking an ActivityContext and returning an ActivityResult.
type ActivityProcessor interface {
	Process(activityContext ActivityContext) ActivityResult
	ProcessDeploy(activityContext ActivityContext) ActivityResult
	ProcessDispose(activityContext ActivityContext) ActivityResult
}

// BaseActivityProcessor provides default implementations for processing activity lifecycle methods in an orchestration system.
type BaseActivityProcessor struct {
}

func (p BaseActivityProcessor) Process(ctx ActivityContext) ActivityResult {
	return ActivityResult{Result: ActivityResultFatalError, Error: fmt.Errorf("the '%s' discriminator is not supported", ctx.Discriminator())}
}

type ActivityResultType int

func (r ActivityResultType) String() string {
	switch r {
	case ActivityResultWait:
		return "wait"
	case ActivityResultComplete:
		return "complete"
	case ActivityResultSchedule:
		return "schedule"
	case ActivityResultRetryError:
		return "retry_error"
	case ActivityResultFatalError:
		return "fatal_error"
	default:
		return "unknown"
	}
}

const (
	ActivityResultWait       = 0
	ActivityResultComplete   = 1
	ActivityResultSchedule   = 2
	ActivityResultRetryError = -1
	ActivityResultFatalError = -2

	DeployDiscriminator  = Discriminator(model.VPADeployType)
	DisposeDiscriminator = Discriminator(model.VPADisposeType)
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

	// VpaProperties returns the properties for the given VPA type from the activity context.
	VpaProperties(vpaType model.VPAType) (map[string]any, error)
}

type DefinitionManager interface {
	CreateOrchestrationDefinition(ctx context.Context, definition *OrchestrationDefinition) (*OrchestrationDefinition, error)
	DeleteOrchestrationDefinition(ctx context.Context, templateRef string) error
	GetOrchestrationDefinitions(ctx context.Context) ([]OrchestrationDefinition, error)
	GetOrchestrationDefinitionsByTemplate(ctx context.Context, templateRef string) ([]OrchestrationDefinition, error)
	QueryOrchestrationDefinitions(ctx context.Context, predicate query.Predicate) ([]OrchestrationDefinition, error)

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

func (d defaultActivityContext) VpaProperties(vpaType model.VPAType) (map[string]any, error) {
	vpaData, ok := d.processingData[model.VPAData]
	if !ok {
		return nil, fmt.Errorf("error reading %s", model.VPAData)
	}
	if vpaData == nil {
		return nil, fmt.Errorf("vpa data ('%s') not found in activity context", model.VPAData)
	}

	vpaList, ok := vpaData.([]any)
	if !ok {
		return nil, fmt.Errorf("vpa data is not a slice")
	}

	for _, item := range vpaList {
		vpaEntry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if entryType, exists := vpaEntry["vpaType"]; exists && entryType == vpaType.String() {
			if properties, ok := vpaEntry["properties"].(map[string]any); ok {
				return properties, nil
			}
			return make(map[string]any), nil
		}
		return nil, fmt.Errorf("no vpa entry for type '%s' found", vpaType)
	}

	return nil, fmt.Errorf("vpa entry with type '%s' not found", vpaType)
}
