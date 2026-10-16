//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type rescueAccountRepo struct {
	AccountRepository
	group   int64
	calls   int
	account Account
}

func (r *rescueAccountRepo) ListSchedulableByGroupIDAndPlatform(_ context.Context, group int64, platform string) ([]Account, error) {
	r.group = group
	r.calls++
	return []Account{r.account}, nil
}
func (r *rescueAccountRepo) GetByID(context.Context, int64) (*Account, error) {
	r.calls++
	return &r.account, nil
}
func TestFreshSnapshotRescuesEmptyCacheWithinSameGroup(t *testing.T) {
	repo := &rescueAccountRepo{account: Account{ID: 44, Platform: PlatformAdobe, Status: StatusActive, Schedulable: true}}
	cache := &snapshotHydrationCache{accounts: map[int64]*Account{44: {ID: 44, Schedulable: false}}}
	s := NewSchedulerSnapshotService(cache, nil, repo, nil, nil)
	groupID := int64(7)
	accounts, _, err := s.ListSchedulableAccounts(context.Background(), &groupID, PlatformAdobe, false)
	require.NoError(t, err)
	require.Empty(t, accounts)
	require.Zero(t, repo.calls)
	ctx := WithFreshAccountSnapshot(context.Background())
	accounts, mixed, err := s.ListSchedulableAccounts(ctx, &groupID, PlatformAdobe, false)
	require.NoError(t, err)
	require.False(t, mixed)
	require.Len(t, accounts, 1)
	require.EqualValues(t, 7, repo.group)
	hydrated, err := s.GetAccount(ctx, 44)
	require.NoError(t, err)
	require.True(t, hydrated.Schedulable, "must not rehydrate rescue from stale account cache")
	for i := 0; i < 4; i++ {
		_, _, err = s.ListSchedulableAccounts(ctx, &groupID, PlatformAdobe, false)
		require.NoError(t, err)
	}
	calls := repo.calls
	_, _, err = s.ListSchedulableAccounts(ctx, &groupID, PlatformAdobe, false)
	require.ErrorIs(t, err, ErrSchedulerFallbackLimited)
	require.Equal(t, calls, repo.calls, "fallback floods are rejected before SQL")
}
func TestFreshSnapshotStopsOnCancellation(t *testing.T) {
	repo := &rescueAccountRepo{}
	s := NewSchedulerSnapshotService(&snapshotHydrationCache{}, nil, repo, nil, nil)
	ctx, cancel := context.WithCancel(WithFreshAccountSnapshot(context.Background()))
	cancel()
	_, _, err := s.ListSchedulableAccounts(ctx, nil, PlatformAdobe, false)
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, repo.calls)
}
