//  Copyright (c) 2026 Metaform Systems, Inc
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

package lifecycleagent

import "time"

// SpecVersion is the CloudEvents specification version produced and expected by lifecycle events.
const SpecVersion = "1.0"

// CloudEvent is a CloudEvents v1.0 structured-mode envelope (https://cloudevents.io). Lifecycle events are delivered as
// CloudEvents, with the domain-specific payload carried in the Data field. The type parameter T is the type of that
// payload.
type CloudEvent[T any] struct {
	// SpecVersion is the CloudEvents specification version, e.g. "1.0".
	SpecVersion string `json:"specversion"`
	// ID uniquely identifies the event within the scope of its Source.
	ID string `json:"id"`
	// Source identifies the context in which the event occurred (a URI-reference).
	Source string `json:"source"`
	// Type describes the kind of event, e.g. "io.cfm.contract.definition.created".
	Type string `json:"type"`
	// Subject describes the subject of the event in the context of the Source (optional).
	Subject string `json:"subject,omitempty"`
	// Time is the timestamp of when the occurrence happened (optional).
	Time time.Time `json:"time,omitzero"`
	// DataContentType is the media type of the Data value, e.g. "application/json" (optional).
	DataContentType string `json:"datacontenttype,omitempty"`
	// Data is the domain-specific event payload.
	Data T `json:"data"`
}
