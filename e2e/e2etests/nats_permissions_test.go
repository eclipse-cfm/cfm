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

package e2etests

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eclipse-cfm/cfm/common/fixtures"
	"github.com/eclipse-cfm/cfm/common/lifecycleagent"
	"github.com/eclipse-cfm/cfm/common/natsclient"
	"github.com/eclipse-cfm/cfm/common/natsfixtures"
	"github.com/eclipse-cfm/cfm/common/sqlstore"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// leastPrivilegeConfig is the documented sample NATS authorization block. Running the deployment flow against it keeps
// the documentation in sync with the subjects the components actually use.
const leastPrivilegeConfig = "../../docs/developer/nats/least-privilege.conf"

// Test_VerifyE2E_LeastPrivilegeNatsUsers runs the deploy and dispose flow with every component connected as its own
// NATS user, restricted by the documented least-privilege permissions.
func Test_VerifyE2E_LeastPrivilegeNatsUsers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// The fixture client only verifies connectivity, so it connects with the Provision Manager's permissions
	nt, err := natsfixtures.SetupNatsContainerWithConfig(ctx, cfmBucket, readLeastPrivilegeConfig(t, false), nil,
		natsclient.ClientConfig{Auth: natsclient.UserInfoAuth{Username: "pmanager", Password: "pmanager-secret"}})
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)
	defer cleanup()

	pg, dsn, err := sqlstore.SetupTestContainer(t)
	require.NoError(t, err)
	defer pg.Terminate(context.Background())

	client := launchPlatformWithAgentEnv(t, nt.URI, dsn, map[string]string{
		"TM_NATS_AUTH_METHOD":          natsclient.AuthMethodUserInfo,
		"TM_NATS_AUTH_USERNAME":        "tmanager",
		"TM_NATS_AUTH_PASSWORD":        "tmanager-secret",
		"PM_NATS_AUTH_METHOD":          natsclient.AuthMethodUserInfo,
		"PM_NATS_AUTH_USERNAME":        "pmanager",
		"PM_NATS_AUTH_PASSWORD":        "pmanager-secret",
		"TESTAGENT_NATS_AUTH_METHOD":   natsclient.AuthMethodUserInfo,
		"TESTAGENT_NATS_AUTH_USERNAME": "test-activity-agent",
		"TESTAGENT_NATS_AUTH_PASSWORD": "test-activity-agent-secret",
	})

	verifyDeployAndDispose(t, client)
}

// Test_VerifyLeastPrivilegeNatsUsers_KeyManagementEvents verifies that the documented permissions let IdentityHub
// deliver key pair events to the key management agent, and that no other component can publish them or start
// orchestrations.
func Test_VerifyLeastPrivilegeNatsUsers_KeyManagementEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// An unrestricted admin user stands in for the out-of-band provisioning of the event stream
	nt, err := natsfixtures.SetupNatsContainerWithConfig(ctx, cfmBucket, readLeastPrivilegeConfig(t, true), nil,
		natsclient.ClientConfig{Auth: natsclient.UserInfoAuth{Username: "admin", Password: "admin-secret"}})
	require.NoError(t, err)
	defer natsfixtures.TeardownNatsContainer(ctx, nt)

	const agentName = "keymanagement-agent"
	const eventStream = "cfm-events"
	_, err = nt.Client.JetStream.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      eventStream,
		Retention: jetstream.InterestPolicy,
		Storage:   jetstream.MemoryStorage,
		Subjects:  []string{"events.keypair.>"},
	})
	require.NoError(t, err)

	fixtures.IsolateConfig(t)
	p := strings.ToUpper(strings.ReplaceAll(agentName, "-", ""))
	t.Setenv(p+"_URI", nt.URI)
	t.Setenv(p+"_BUCKET", cfmBucket)
	t.Setenv(p+"_STREAM", eventStream)
	t.Setenv(p+"_NATS_AUTH_METHOD", natsclient.AuthMethodUserInfo)
	t.Setenv(p+"_NATS_AUTH_USERNAME", agentName)
	t.Setenv(p+"_NATS_AUTH_PASSWORD", agentName+"-secret")

	proc := &captureProcessor{received: make(chan lifecycleagent.EventContext[lifecycleagent.CloudEvent[itemCreated]], 1)}
	shutdown := make(chan struct{})
	go lifecycleagent.LaunchAgent(shutdown, lifecycleagent.LauncherConfig[lifecycleagent.CloudEvent[itemCreated]]{
		AgentName:    agentName,
		ServiceName:  "cfm.agent." + agentName,
		ConfigPrefix: strings.ToLower(p),
		Subjects:     []string{"events.keypair.>"},
		NewProcessor: func(*lifecycleagent.AgentContext) lifecycleagent.EventProcessor[lifecycleagent.CloudEvent[itemCreated]] {
			return proc
		},
	})
	defer close(shutdown)

	require.Eventually(t, func() bool {
		stream, err := nt.Client.JetStream.Stream(ctx, eventStream)
		if err != nil {
			return false
		}
		_, err = stream.Consumer(ctx, agentName)
		return err == nil
	}, testTimeout, eventsPoll, "agent durable consumer should be created")

	identityHub := connectAs(t, nt.URI, "identityhub")
	_, err = identityHub.Publish(ctx, "events.keypair.activated",
		[]byte(`{"specversion":"1.0","id":"evt-1","source":"/identityhub","type":"keypair.activated","data":{"itemId":"42"}}`))
	require.NoError(t, err)

	select {
	case evt := <-proc.received:
		assert.Equal(t, "events.keypair.activated", evt.Subject)
	case <-time.After(testTimeout):
		t.Fatal("timed out waiting for the key pair event to be dispatched")
	}

	denied := []struct{ user, subject string }{
		{"identityhub", natsclient.CFMOrchestrationSubject},
		{"pmanager", natsclient.CFMOrchestrationSubject},
		{"test-activity-agent", natsclient.CFMOrchestrationSubject},
		{"keymanagement-agent", natsclient.CFMOrchestrationSubject},
		{"pmanager", "events.keypair.revoked"},
		{"tmanager", "events.keypair.revoked"},
		{"test-activity-agent", "events.keypair.revoked"},
		{"tmanager", natsclient.CFMOrchestrationResponseSubject},
		{"tmanager", "$KV." + cfmBucket + ".some-orchestration"},
		{"keymanagement-agent", "$KV." + cfmBucket + ".some-orchestration"},
	}
	for _, d := range denied {
		t.Run(fmt.Sprintf("%s cannot publish %s", d.user, d.subject), func(t *testing.T) {
			publishCtx, cancelPublish := context.WithTimeout(ctx, 2*time.Second)
			defer cancelPublish()
			_, err := connectAs(t, nt.URI, d.user).Publish(publishCtx, d.subject, []byte("{}"))
			assert.Error(t, err)
		})
	}
}

// readLeastPrivilegeConfig returns the documented authorization block, optionally with an extra unrestricted admin user
// for test setup.
func readLeastPrivilegeConfig(t *testing.T, withAdmin bool) string {
	t.Helper()
	authorization, err := os.ReadFile(leastPrivilegeConfig)
	require.NoError(t, err)
	block := string(authorization)
	if withAdmin {
		require.Contains(t, block, "users = [")
		block = strings.Replace(block, "users = [", `users = [
    { user: "admin", password: "admin-secret" }`, 1)
	}
	return block
}

// connectAs opens a JetStream context for one of the documented users, whose password is "<user>-secret".
func connectAs(t *testing.T, uri string, user string) jetstream.JetStream {
	t.Helper()
	connection, err := nats.Connect(uri, nats.UserInfo(user, user+"-secret"))
	require.NoError(t, err)
	t.Cleanup(connection.Close)
	js, err := jetstream.New(connection)
	require.NoError(t, err)
	return js
}
