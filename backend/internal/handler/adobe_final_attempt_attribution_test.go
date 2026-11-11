package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAdobeFinalAttemptAccountMatchesFinalProxy(t *testing.T) {
	first, pid := int64(1), int64(88)
	entry := &service.OpsInsertErrorLogInput{AccountID: &first}
	events := []*service.OpsUpstreamErrorEvent{{Platform: service.PlatformAdobe, AccountID: 1, UpstreamStatusCode: 403}, {Platform: service.PlatformAdobe, AccountID: 2, ProxyID: &pid, UpstreamStatusCode: 403, Message: "access denied", Stage: "submit"}}
	applyOpsUpstreamErrorEvents(entry, events)
	require.EqualValues(t, 2, *entry.AccountID)
	require.Equal(t, 403, *entry.UpstreamStatusCode)
	require.Len(t, entry.UpstreamErrors, 2)
	other := &service.OpsInsertErrorLogInput{AccountID: &first}
	applyOpsUpstreamErrorEvents(other, []*service.OpsUpstreamErrorEvent{{Platform: "openai", AccountID: 99, UpstreamStatusCode: 403}})
	require.EqualValues(t, 1, *other.AccountID)
}
