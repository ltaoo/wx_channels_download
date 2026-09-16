package wxchannels

import (
	"fmt"
	"os"
)

// EncryptedHeadLength is the number of leading bytes of a Channels video that
// are encrypted in place.
const EncryptedHeadLength uint32 = 131072

// DecryptFileInPlace decrypts the encrypted head of a local file and overwrites
// it with the decrypted bytes. The error text is part of the HTTP decrypt
// route's response body, so the wording is stable.
func DecryptFileInPlace(path string, key uint64) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	DecryptData(data, EncryptedHeadLength, key)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}
	return nil
}
