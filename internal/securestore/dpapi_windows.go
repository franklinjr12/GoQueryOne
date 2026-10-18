//go:build windows

package securestore

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

// dpapiProtector implements Protector using Windows DPAPI
// (CryptProtectData/CryptUnprotectData), scoped to the current user.
type dpapiProtector struct{}

func defaultProtector() Protector {
	return dpapiProtector{}
}

func (dpapiProtector) Protect(plain []byte) ([]byte, error) {
	if len(plain) == 0 {
		return []byte{}, nil
	}
	in := windows.DataBlob{
		Size: uint32(len(plain)),
		Data: &plain[0],
	}
	var out windows.DataBlob
	if err := windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	if out.Size == 0 || out.Data == nil {
		return nil, errors.New("dpapi returned empty payload")
	}
	buf := unsafe.Slice(out.Data, out.Size)
	result := make([]byte, out.Size)
	copy(result, buf)
	return result, nil
}

func (dpapiProtector) Unprotect(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) == 0 {
		return []byte{}, nil
	}
	in := windows.DataBlob{
		Size: uint32(len(ciphertext)),
		Data: &ciphertext[0],
	}
	var out windows.DataBlob
	if err := windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out); err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	if out.Size == 0 || out.Data == nil {
		return nil, errors.New("dpapi returned empty payload")
	}
	buf := unsafe.Slice(out.Data, out.Size)
	result := make([]byte, out.Size)
	copy(result, buf)
	return result, nil
}
