//go:build !linux

package tabby

func KeyringLookup() (Lookup, func()) { return nil, func() {} }
