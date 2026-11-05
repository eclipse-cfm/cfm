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

package token

import (
	"context"
	"errors"
	"sync"
	"time"
)

// DefaultRefreshMargin is how long before its expiry a cached token is considered stale and fetched again.
const DefaultRefreshMargin = 30 * time.Second

// ExpiringToken is a token together with the point in time after which it is no longer valid.
// A nil ExpiresAt means the expiry is unknown, and such a token is never cached.
type ExpiringToken struct {
	Value     string
	ExpiresAt *time.Time
}

// ExpiringTokenProvider gets tokens and reports when they expire. Implementations can be wrapped in a
// CachingTokenProvider.
type ExpiringTokenProvider interface {
	GetExpiringToken(ctx context.Context, scope string, participantId string) (*ExpiringToken, error)
}

type cacheKey struct {
	scope         string
	participantId string
}

type cacheEntry struct {
	value     string
	refreshAt time.Time
}

// CachingTokenProvider is a TokenProvider that caches the tokens of a delegate per scope and participant until shortly
// before they expire. It is safe for concurrent use and is meant to be shared by all clients of a component.
// Failures are never cached.
type CachingTokenProvider struct {
	delegate      ExpiringTokenProvider
	refreshMargin time.Duration
	now           func() time.Time

	mu      sync.Mutex
	entries map[cacheKey]cacheEntry
}

// CacheOption is a functional option for configuring a CachingTokenProvider
type CacheOption func(*CachingTokenProvider)

// WithRefreshMargin sets how long before expiry a token is fetched again. For tokens whose lifetime is shorter than
// ten times the margin, a tenth of the lifetime is used instead, so short-lived tokens are still cached.
func WithRefreshMargin(margin time.Duration) CacheOption {
	return func(c *CachingTokenProvider) {
		c.refreshMargin = margin
	}
}

// WithClock sets the function used to get the current time. Intended for tests.
func WithClock(now func() time.Time) CacheOption {
	return func(c *CachingTokenProvider) {
		c.now = now
	}
}

// NewCachingTokenProvider creates a CachingTokenProvider that gets tokens from the given delegate.
func NewCachingTokenProvider(delegate ExpiringTokenProvider, opts ...CacheOption) *CachingTokenProvider {
	c := &CachingTokenProvider{
		delegate:      delegate,
		refreshMargin: DefaultRefreshMargin,
		now:           time.Now,
		entries:       make(map[cacheKey]cacheEntry),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *CachingTokenProvider) GetToken(ctx context.Context, scope string, participantId string) (string, error) {
	key := cacheKey{scope: scope, participantId: participantId}

	c.mu.Lock()
	entry, found := c.entries[key]
	c.mu.Unlock()
	if found && c.now().Before(entry.refreshAt) {
		return entry.value, nil
	}

	// the lock is not held while fetching, so a slow token endpoint does not block other keys. Concurrent misses for
	// the same key may fetch more than once, and the last one wins.
	token, err := c.delegate.GetExpiringToken(ctx, scope, participantId)
	if err != nil {
		return "", err
	}
	if token == nil {
		return "", errors.New("token provider returned no token")
	}
	c.store(key, token)
	return token.Value, nil
}

func (c *CachingTokenProvider) store(key cacheKey, token *ExpiringToken) {
	now := c.now()

	c.mu.Lock()
	defer c.mu.Unlock()

	// drop expired entries, so participants that are no longer used do not accumulate
	for k, e := range c.entries {
		if !now.Before(e.refreshAt) {
			delete(c.entries, k)
		}
	}

	if token.ExpiresAt == nil {
		return
	}
	lifetime := token.ExpiresAt.Sub(now)
	margin := min(c.refreshMargin, lifetime/10)
	refreshAt := token.ExpiresAt.Add(-margin)
	if !now.Before(refreshAt) {
		return
	}
	c.entries[key] = cacheEntry{value: token.Value, refreshAt: refreshAt}
}
