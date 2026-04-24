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

package e2efixtures

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ApiClient struct {
	pmanagerBaseUrl string
	tmanagerBaseUrl string
	cfmAgentBaseUrl string
	client          http.Client
}

func NewApiClient(tmanagerBaseUrl string, pmanagerBaseUrl string) *ApiClient {
	return &ApiClient{
		tmanagerBaseUrl: tmanagerBaseUrl,
		pmanagerBaseUrl: pmanagerBaseUrl,
		client:          http.Client{},
	}
}

// PostToPManager makes a POST request to Provision Manager API
func (c *ApiClient) PostToPManager(endpoint string, payload any) error {
	url := fmt.Sprintf("%s/%s", c.pmanagerBaseUrl, endpoint)
	_, err := c.bodyRequestWithResponse(url, http.MethodPost, payload, nil)
	return err
}

// DeleteToPManager makes a DELETE request to Provision Manager API
func (c *ApiClient) DeleteToPManager(endpoint string) error {
	url := fmt.Sprintf("%s/%s", c.pmanagerBaseUrl, endpoint)
	return c.deleteRequest(url)
}

func (c *ApiClient) PostToTManager(endpoint string, payload any) error {
	url := fmt.Sprintf("%s/%s", c.tmanagerBaseUrl, endpoint)
	_, err := c.bodyRequestWithResponse(url, http.MethodPost, payload, nil)
	return err
}

func (c *ApiClient) PostToTManagerWithResponse(endpoint string, payload any, result any) error {
	url := fmt.Sprintf("%s/%s", c.tmanagerBaseUrl, endpoint)
	return c.postWithResponse(url, payload, result)
}

func (c *ApiClient) PostToPManagerWithResponse(endpoint string, payload any, result any) error {
	url := fmt.Sprintf("%s/%s", c.pmanagerBaseUrl, endpoint)
	return c.postWithResponse(url, payload, result)
}

func (c *ApiClient) PatchToTManager(endpoint string, payload any) error {
	url := fmt.Sprintf("%s/%s", c.tmanagerBaseUrl, endpoint)
	return c.bodyRequest(url, http.MethodPatch, payload, nil)
}

func (c *ApiClient) GetTManager(endpoint string, result any) error {
	return c.getRequest(c.tmanagerBaseUrl, endpoint, result)
}

func (c *ApiClient) GetPManager(endpoint string, result any) error {
	return c.getRequest(c.pmanagerBaseUrl, endpoint, result)
}

// DeleteToTManager makes a DELETE request to Tenant Manager API
func (c *ApiClient) DeleteToTManager(endpoint string) error {
	url := fmt.Sprintf("%s/%s", c.tmanagerBaseUrl, endpoint)
	return c.deleteRequest(url)
}

func (c *ApiClient) postWithResponse(url string, payload any, result any) error {
	ser, err := c.bodyRequestWithResponse(url, http.MethodPost, payload, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(ser, result)
}

// bodyRequestWithResponse handles POST requests with JSON payload
func (c *ApiClient) bodyRequestWithResponse(url string, method string, payload any, headers map[string]string) ([]byte, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusCreated &&
		resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusAccepted {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// bodyRequestWithResponse handles POST requests with JSON payload
func (c *ApiClient) bodyRequest(url string, method string, payload any, headers map[string]string) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusCreated &&
		resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return nil
}

// deleteRequest handles DELETE requests
func (c *ApiClient) deleteRequest(url string) error {

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusCreated &&
		resp.StatusCode != http.StatusNoContent &&
		resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *ApiClient) getRequest(baseUrl string, endpoint string, result any) error {
	url := fmt.Sprintf("%s/%s", baseUrl, endpoint)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return json.Unmarshal(body, result)
}
