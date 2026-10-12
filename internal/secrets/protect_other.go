//go:build !windows

package secrets

// Non-Windows credentials use the private 0600 file written by Vault.Save.
func protect(raw []byte) ([]byte, error)   { return raw, nil }
func unprotect(raw []byte) ([]byte, error) { return raw, nil }
