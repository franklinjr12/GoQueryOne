//go:build !windows

package securestore

import "errors"

func protectData(plain []byte) ([]byte, error) {
	return nil, errors.New("secure credential storage is supported on Windows only")
}

func unprotectData(ciphertext []byte) ([]byte, error) {
	return nil, errors.New("secure credential storage is supported on Windows only")
}
