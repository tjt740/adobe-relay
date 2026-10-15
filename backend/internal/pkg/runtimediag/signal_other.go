//go:build !linux

package runtimediag

func StartMemoryDiagnostics() func() { return func() {} }
