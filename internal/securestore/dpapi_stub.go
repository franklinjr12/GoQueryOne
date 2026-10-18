//go:build !windows

package securestore

import "errors"

// unsupportedProtector is the default Protector on non-Windows platforms,
// where no DPAPI-equivalent secure storage is wired up.
type unsupportedProtector struct{}

func defaultProtector() Protector {
	return unsupportedProtector{}
}

func (unsupportedProtector) Protect(plain []byte) ([]byte, error) {
	return nil, errors.New("secure credential storage is supported on Windows only")
}

func (unsupportedProtector) Unprotect(ciphertext []byte) ([]byte, error) {
	return nil, errors.New("secure credential storage is supported on Windows only")
}
