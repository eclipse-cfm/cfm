/*
 *  Copyright (c) 2026 Metaform Systems, Inc.
 *
 *  This program and the accompanying materials are made available under the
 *  terms of the Apache License, Version 2.0 which is available at
 *  https://www.apache.org/licenses/LICENSE-2.0
 *
 *  SPDX-License-Identifier: Apache-2.0
 *
 *  Contributors:
 *       Metaform Systems, Inc. - initial API and implementation
 *
 */

package natsorchestration

import "github.com/eclipse-cfm/cfm/pmanager/api"

type DefaultTestProcessor struct {
}

func (d DefaultTestProcessor) ProcessDeploy(activityContext api.ActivityContext) api.ActivityResult {
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (d DefaultTestProcessor) ProcessDispose(activityContext api.ActivityContext) api.ActivityResult {
	return api.ActivityResult{Result: api.ActivityResultComplete}
}

func (d DefaultTestProcessor) Process(activityContext api.ActivityContext) api.ActivityResult {
	return api.ActivityResult{Result: api.ActivityResultComplete}
}
