//go:build !windows

package log

func outputDebugString(s string) {}
func messageBox(s string)        {}
