//go:build windows

package secrets

import (
	"fmt"
	"golang.org/x/sys/windows"
	"unsafe"
)

type blob struct {
	size uint32
	data *byte
}

var crypt32 = windows.NewLazySystemDLL("crypt32.dll")
var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

func crypt(raw []byte, method string) ([]byte, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty credential data")
	}
	in := blob{uint32(len(raw)), &raw[0]}
	var out blob
	r, _, err := crypt32.NewProc(method).Call(uintptr(unsafe.Pointer(&in)), 0, 0, 0, 0, 1, uintptr(unsafe.Pointer(&out)))
	if r == 0 {
		return nil, fmt.Errorf("Windows credential protection failed: %w", err)
	}
	defer kernel32.NewProc("LocalFree").Call(uintptr(unsafe.Pointer(out.data)))
	return append([]byte(nil), unsafe.Slice(out.data, int(out.size))...), nil
}
func protect(raw []byte) ([]byte, error)   { return crypt(raw, "CryptProtectData") }
func unprotect(raw []byte) ([]byte, error) { return crypt(raw, "CryptUnprotectData") }
