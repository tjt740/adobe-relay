package service

import "context"

type freshAccountSnapshotKey struct{}

// WithFreshAccountSnapshot bypasses only scheduler snapshots. All normal
// group/model/permission/exclusion and manual-pause checks still apply.
func WithFreshAccountSnapshot(ctx context.Context) context.Context {
	return context.WithValue(ctx, freshAccountSnapshotKey{}, true)
}
func freshAccountSnapshotRequested(ctx context.Context) bool {
	v, _ := ctx.Value(freshAccountSnapshotKey{}).(bool)
	return v
}
