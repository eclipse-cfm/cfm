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
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeProvider issues numbered tokens that expire after lifetime, and fails while err is set
type fakeProvider struct {
	mu       sync.Mutex
	now      func() time.Time
	lifetime time.Duration
	err      error
	calls    int
}

func (f *fakeProvider) GetExpiringToken(_ context.Context, scope string, participantId string) (*ExpiringToken, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	token := &ExpiringToken{Value: fmt.Sprintf("%s|%s|%d", scope, participantId, f.calls)}
	if f.lifetime > 0 {
		expiresAt := f.now().Add(f.lifetime)
		token.ExpiresAt = &expiresAt
	}
	return token, nil
}

func (f *fakeProvider) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func setup(lifetime time.Duration) (*CachingTokenProvider, *fakeProvider, *fakeClock) {
	clock := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	delegate := &fakeProvider{now: clock.now, lifetime: lifetime}
	return NewCachingTokenProvider(delegate, WithClock(clock.now)), delegate, clock
}

func TestCachingTokenProvider_ReusesTokenUntilRefreshMargin(t *testing.T) {
	provider, delegate, clock := setup(time.Hour)

	first, err := provider.GetToken(t.Context(), "scope", "p1")
	require.NoError(t, err)

	clock.advance(time.Hour - DefaultRefreshMargin - time.Second)
	second, err := provider.GetToken(t.Context(), "scope", "p1")
	require.NoError(t, err)
	assert.Equal(t, first, second)
	assert.Equal(t, 1, delegate.callCount())

	clock.advance(time.Second)
	third, err := provider.GetToken(t.Context(), "scope", "p1")
	require.NoError(t, err)
	assert.NotEqual(t, first, third)
	assert.Equal(t, 2, delegate.callCount())
}

func TestCachingTokenProvider_SeparateEntriesPerScopeAndParticipant(t *testing.T) {
	provider, delegate, _ := setup(time.Hour)

	for range 2 {
		for _, scope := range []string{"read", "write"} {
			for _, participant := range []string{"p1", "p2"} {
				token, err := provider.GetToken(t.Context(), scope, participant)
				require.NoError(t, err)
				assert.Contains(t, token, scope+"|"+participant+"|")
			}
		}
	}
	assert.Equal(t, 4, delegate.callCount())
}

func TestCachingTokenProvider_DoesNotCacheFailures(t *testing.T) {
	provider, delegate, _ := setup(time.Hour)
	delegate.err = errors.New("token endpoint unavailable")

	_, err := provider.GetToken(t.Context(), "scope", "p1")
	require.ErrorContains(t, err, "token endpoint unavailable")

	delegate.err = nil
	token, err := provider.GetToken(t.Context(), "scope", "p1")
	require.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Equal(t, 2, delegate.callCount())
}

func TestCachingTokenProvider_ReportsFailedRefresh(t *testing.T) {
	provider, delegate, clock := setup(time.Hour)

	_, err := provider.GetToken(t.Context(), "scope", "p1")
	require.NoError(t, err)

	// the cached token is past its refresh point but not yet expired; a failed refresh is reported instead of
	// serving a token that is about to expire
	clock.advance(time.Hour - DefaultRefreshMargin)
	delegate.err = errors.New("token endpoint unavailable")
	_, err = provider.GetToken(t.Context(), "scope", "p1")
	require.Error(t, err)
}

func TestCachingTokenProvider_DoesNotCacheTokensWithoutExpiry(t *testing.T) {
	provider, delegate, _ := setup(0)

	for range 3 {
		_, err := provider.GetToken(t.Context(), "scope", "p1")
		require.NoError(t, err)
	}
	assert.Equal(t, 3, delegate.callCount())
}

func TestCachingTokenProvider_ShortLivedTokensUseFractionOfLifetime(t *testing.T) {
	// a 20s lifetime is shorter than the default margin, so a tenth of it (2s) is used instead
	provider, delegate, clock := setup(20 * time.Second)

	_, err := provider.GetToken(t.Context(), "scope", "p1")
	require.NoError(t, err)

	clock.advance(17 * time.Second)
	_, err = provider.GetToken(t.Context(), "scope", "p1")
	require.NoError(t, err)
	assert.Equal(t, 1, delegate.callCount())

	clock.advance(time.Second)
	_, err = provider.GetToken(t.Context(), "scope", "p1")
	require.NoError(t, err)
	assert.Equal(t, 2, delegate.callCount())
}

func TestCachingTokenProvider_DoesNotCacheAlreadyExpiredTokens(t *testing.T) {
	expired := &expiredProvider{}
	provider := NewCachingTokenProvider(expired)

	for range 2 {
		_, err := provider.GetToken(t.Context(), "scope", "p1")
		require.NoError(t, err)
	}
	assert.Equal(t, 2, expired.calls)
}

func TestCachingTokenProvider_DropsExpiredEntries(t *testing.T) {
	provider, _, clock := setup(time.Hour)

	_, err := provider.GetToken(t.Context(), "scope", "p1")
	require.NoError(t, err)
	_, err = provider.GetToken(t.Context(), "scope", "p2")
	require.NoError(t, err)

	clock.advance(time.Hour)
	_, err = provider.GetToken(t.Context(), "scope", "p3")
	require.NoError(t, err)

	provider.mu.Lock()
	defer provider.mu.Unlock()
	assert.Len(t, provider.entries, 1)
	assert.Contains(t, provider.entries, cacheKey{scope: "scope", participantId: "p3"})
}

func TestCachingTokenProvider_ConcurrentUse(t *testing.T) {
	provider, delegate, _ := setup(time.Hour)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Go(func() {
			participant := fmt.Sprintf("p%d", i%5)
			token, err := provider.GetToken(t.Context(), "scope", participant)
			assert.NoError(t, err)
			assert.Contains(t, token, "|"+participant+"|")
		})
	}
	wg.Wait()

	// concurrent misses for the same key may each fetch, but afterwards every key is served from the cache
	fetched := delegate.callCount()
	assert.GreaterOrEqual(t, fetched, 5)
	for i := range 5 {
		_, err := provider.GetToken(t.Context(), "scope", fmt.Sprintf("p%d", i))
		require.NoError(t, err)
	}
	assert.Equal(t, fetched, delegate.callCount())
}

func TestCachingTokenProvider_NilTokenIsAnError(t *testing.T) {
	provider := NewCachingTokenProvider(nilProvider{})

	_, err := provider.GetToken(t.Context(), "scope", "p1")
	require.ErrorContains(t, err, "returned no token")
}

type nilProvider struct{}

func (nilProvider) GetExpiringToken(context.Context, string, string) (*ExpiringToken, error) {
	return nil, nil
}

// expiredProvider issues tokens that expired in the past
type expiredProvider struct {
	calls int
}

func (e *expiredProvider) GetExpiringToken(context.Context, string, string) (*ExpiringToken, error) {
	e.calls++
	expiresAt := time.Now().Add(-time.Second)
	return &ExpiringToken{Value: "expired", ExpiresAt: &expiresAt}, nil
}
