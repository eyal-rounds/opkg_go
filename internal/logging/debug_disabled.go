//go:build !debug

package logging

// Debugf is a no-op when the debug build tag is not enabled.
func Debugf(string, ...interface{}) {}
