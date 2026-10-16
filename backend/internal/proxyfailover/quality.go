package proxyfailover

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/clash"
)

const slowLatencyMS int64 = 2000
const softSwitchCooldown = 5 * time.Minute

func qualityObservation(old, h health) health {
	if old.fingerprint != h.fingerprint || h.CheckedAt.Sub(old.CheckedAt) > 2*clash.ProbeFreshness(512) {
		old = health{}
	}
	h.SlowChecks = 0
	h.LatencyEWMA = old.LatencyEWMA
	h.SuccessPermille = old.SuccessPermille
	if h.Status == "healthy" {
		if old.SuccessPermille == 0 && old.Status != "unhealthy" {
			h.SuccessPermille = 1000
		} else {
			h.SuccessPermille = (3*old.SuccessPermille + 1000) / 4
		}
		if old.LatencyEWMA <= 0 {
			h.LatencyEWMA = h.LatencyMS
		} else {
			h.LatencyEWMA = (3*old.LatencyEWMA + h.LatencyMS) / 4
		}
		if h.LatencyMS > slowLatencyMS {
			h.SlowChecks = min(100, old.SlowChecks+1)
		}
	} else if h.Status == "unhealthy" {
		h.SuccessPermille = 3 * old.SuccessPermille / 4
	}
	return h
}
func routeLatency(h health) int64 {
	if h.LatencyEWMA > 0 {
		return h.LatencyEWMA
	}
	return max(1, h.LatencyMS)
}
func routeScore(h health, used int) int64 {
	rate := h.SuccessPermille
	if rate <= 0 {
		rate = 1000
	}
	return routeLatency(h) * int64(100+15*used) * 1000 / (100 * int64(max(rate, 100)))
}
func betterLatencyRoute(current, candidate health) bool {
	before, after := routeLatency(current), routeLatency(candidate)
	return current.SlowChecks >= 3 && after*10 <= before*7 && before-after >= 500
}
