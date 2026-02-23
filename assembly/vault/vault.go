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

package vault

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	hvault "github.com/hashicorp/vault-client-go"
	"github.com/hashicorp/vault-client-go/schema"
	"github.com/metaform/connector-fabric-manager/common/system"
)

const (
	contentKey = "content"
)

type VaultOptions func(*vaultClient)

func (f VaultOptions) apply(vc *vaultClient) {
	f(vc)
}

// WithMountPath sets the mount path
func WithMountPath(path string) VaultOptions {
	return func(vc *vaultClient) {
		vc.mountPath = path
	}
}

// vaultClient implements a client to Hashicorp Vault supporting token renewal.
type vaultClient struct {
	vaultURL     string
	clientID     string
	clientSecret string
	tokenUrl     string
	mountPath    string
	softDelete   bool
	monitor      system.LogMonitor
	client       *hvault.Client
	stopCh       chan struct{}
	lastCreated  time.Time // When the token was last renewed; will be the zero value if the token has never been renewed or there was an error.
	lastRenew    time.Time // When the token was last renewed; will be the zero value if the token has never been renewed or there was an error.
}

func newVaultClient(vaultURL string, clientID string, clientSecret string, tokenUrl string, monitor system.LogMonitor, opts ...VaultOptions) (*vaultClient, error) {
	client := &vaultClient{
		vaultURL:     vaultURL,
		clientID:     clientID,
		clientSecret: clientSecret,
		tokenUrl:     tokenUrl,
		monitor:      monitor,
		stopCh:       make(chan struct{}),
	}
	for _, opt := range opts {
		opt.apply(client)
	}
	return client, nil
}

func (v *vaultClient) ResolveSecret(ctx context.Context, path string) (string, error) {
	secret, err := v.client.Secrets.KvV2Read(
		ctx,
		path,
		v.getOptions()...,
	)
	if err != nil {
		return "", fmt.Errorf("unable to read secret: %w", err)
	}
	if value, ok := secret.Data.Data["content"].(string); ok {
		return value, nil
	}
	return "", fmt.Errorf("content field not found or not a string")

}

func (v *vaultClient) StoreSecret(ctx context.Context, path string, value string) error {
	_, err := v.client.Secrets.KvV2Write(
		ctx,
		path,
		schema.KvV2WriteRequest{
			Data: map[string]any{
				contentKey: value,
			},
		},
		v.getOptions()...,
	)
	if err != nil {
		return fmt.Errorf("unable to write secret to path %s: %w", path, err)
	}

	return nil
}

func (v *vaultClient) DeleteSecret(ctx context.Context, path string) error {
	var err error

	_, err = v.client.Secrets.KvV2Delete(
		ctx,
		path,
		v.getOptions()...,
	)
	if err != nil {
		return fmt.Errorf("unable to delete secret at path %s: %w", path, err)
	}

	if !v.softDelete {
		_, err := v.client.Secrets.KvV2DeleteMetadataAndAllVersions(
			ctx,
			path,
			v.getOptions()...,
		)
		if err != nil {
			return fmt.Errorf("unable to purge metadata and all versions at path %s: %w", path, err)
		}
	}

	return nil
}

func (v *vaultClient) init(ctx context.Context) error {
	var err error
	v.client, err = hvault.New(
		hvault.WithAddress(v.vaultURL),
		hvault.WithRequestTimeout(10*time.Second), // TODO configure
	)
	if err != nil {
		return fmt.Errorf("unable to initialize Vault client: %w", err)
	}

	// Authenticate using AppRole
	resp, err := v.createToken(ctx)
	if err != nil {
		return fmt.Errorf("unable to initialize Vault client: %w", err)
	}
	go v.renewTokenPeriodically(time.Duration(resp.Auth.LeaseDuration) * time.Second)
	return nil
}

func (v *vaultClient) createToken(ctx context.Context) (*hvault.Response[map[string]any], error) {
	jwt, err := getVaultAccessToken(v.clientID, v.clientSecret, v.tokenUrl)
	if err != nil {
		return nil, fmt.Errorf("unable to get Vault access token: %w", err)
	}
	loginResult, err := v.client.Auth.JwtLogin(
		ctx,
		schema.JwtLoginRequest{
			Jwt:  jwt,
			Role: "provisioner",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("unable to authenticate with JWT: %w", err)
	}

	// Set the token obtained from AppRole login
	err = v.client.SetToken(loginResult.Auth.ClientToken)
	v.lastCreated = time.Now()
	return loginResult, err
}

func getVaultAccessToken(clientId string, secret string, tokenUrl string) (string, error) {

	formData := strings.NewReader(fmt.Sprintf("client_id=%s&client_secret=%s&grant_type=client_credentials",
		clientId, secret))

	req, err := http.NewRequest("POST", tokenUrl, formData)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResponse struct {
		AccessToken string `json:"access_token"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal token response: %w", err)
	}

	if tokenResponse.AccessToken == "" {
		return "", fmt.Errorf("access token not found in response")
	}

	return tokenResponse.AccessToken, nil
}

// leaseDuration specifies the token lease duration and supports any time.Duration unit (milliseconds, seconds, minutes, etc.)
func (v *vaultClient) renewTokenPeriodically(leaseDuration time.Duration) {
	// Renew at 80% of lease duration
	renewInterval := time.Duration(float64(leaseDuration) * 0.8)

	ticker := time.NewTicker(renewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_, err := v.client.Auth.TokenRenewSelf(context.Background(), schema.TokenRenewSelfRequest{
				Increment: fmt.Sprintf("%ds", int(leaseDuration.Seconds())),
			})
			if err != nil {
				if strings.Contains(err.Error(), "invalid token") {
					// Token cannot be renewed further because it has expired so create a new one
					_, err2 := v.createToken(context.Background())
					if err2 != nil {
						v.monitor.Severef("Error creating token after expiration: %v. Will attempt renewal at next interval", err2)
						continue
					}
				}
				v.lastRenew = time.Time{}
				v.monitor.Severef("Error renewing token: %v. Will attempt renewal at next interval", err)
				continue
			}
			v.lastRenew = time.Now()
		case <-v.stopCh:
			return
		}
	}
}

// Close gracefully shuts down the vaultClient and stops token renewal
func (v *vaultClient) Close() error {
	close(v.stopCh)
	return nil
}

func (v *vaultClient) getOptions() []hvault.RequestOption {
	var opts []hvault.RequestOption
	if v.mountPath != "" {
		opts = append(opts, hvault.WithMountPath(v.mountPath))
	}
	return opts
}
