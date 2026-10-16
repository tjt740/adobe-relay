package repository

import (
	"context"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestConcurrencyRollingDeploymentKeepsBothProcessSlots(t *testing.T) {
	server := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	ctx := context.Background()
	c := NewConcurrencyCache(rdb, 15, 900).(*concurrencyCache)
	require.NoError(t, rdb.ZAdd(ctx, "concurrency:user:7", redis.Z{Member: "old-process-1", Score: 1}).Err())
	require.NoError(t, rdb.ZAdd(ctx, userActiveIndexKey, redis.Z{Member: "7", Score: 1}).Err())
	require.NoError(t, rdb.Set(ctx, rollingDeploymentKey, "operator-handoff", 30*time.Minute).Err())
	require.NoError(t, c.CleanupStaleProcessSlots(ctx, "new-process-"))
	members, err := rdb.ZRange(ctx, "concurrency:user:7", 0, -1).Result()
	require.NoError(t, err)
	require.Equal(t, []string{"old-process-1"}, members)
	require.NoError(t, rdb.Set(ctx, legacyWaitSweepMarkerKey, "1", 0).Err())
	server.FastForward(31 * time.Minute)
	require.NoError(t, c.CleanupStaleProcessSlots(ctx, "new-process-"))
	require.Zero(t, rdb.ZCard(ctx, "concurrency:user:7").Val(), "expired handoff must restore normal stale-process cleanup")
}
