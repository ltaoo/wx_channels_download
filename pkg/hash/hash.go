package hash

// https://github.com/cloudflare/workers-sdk/blob/main/packages/wrangler/src/pages/hash.ts

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/blake3"
)

// HashType defines the supported hash types
type HashType string

const (
	MD5    HashType = "md5"
	SHA1   HashType = "sha1"
	SHA256 HashType = "sha256"
	BLAKE3 HashType = "blake3"
)

// FileHash calculates the hash of a file
func FileHash(filePath string, hashType HashType) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", filePath, err)
	}
	defer file.Close()

	var hash interface{}
	switch hashType {
	case MD5:
		hash = md5.New()
	case SHA1:
		hash = sha1.New()
	case SHA256:
		hash = sha256.New()
	case BLAKE3:
		hash = blake3.New()
	default:
		return "", fmt.Errorf("unsupported hash type: %s", hashType)
	}

	_, err = io.Copy(hash.(io.Writer), file)
	if err != nil {
		return "", fmt.Errorf("failed to calculate hash for %s: %w", filePath, err)
	}

	hashBytes := hash.(interface{ Sum([]byte) []byte }).Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}

// StringHash calculates the hash of a string
func StringHash(data string, hashType HashType) (string, error) {
	var hash interface{}
	switch hashType {
	case MD5:
		hash = md5.New()
	case SHA1:
		hash = sha1.New()
	case SHA256:
		hash = sha256.New()
	case BLAKE3:
		hash = blake3.New()
	default:
		return "", fmt.Errorf("unsupported hash type: %s", hashType)
	}

	_, err := io.WriteString(hash.(io.Writer), data)
	if err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	hashBytes := hash.(interface{ Sum([]byte) []byte }).Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}

// BytesHash calculates the hash of a byte array
func BytesHash(data []byte, hashType HashType) (string, error) {
	var hash interface{}
	switch hashType {
	case MD5:
		hash = md5.New()
	case SHA1:
		hash = sha1.New()
	case SHA256:
		hash = sha256.New()
	case BLAKE3:
		hash = blake3.New()
	default:
		return "", fmt.Errorf("unsupported hash type: %s", hashType)
	}

	_, err := hash.(io.Writer).Write(data)
	if err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	hashBytes := hash.(interface{ Sum([]byte) []byte }).Sum(nil)
	return hex.EncodeToString(hashBytes), nil
}

// FileHashAll computes all supported hash values for a file.
func FileHashAll(filePath string) (map[HashType]string, error) {
	hashes := make(map[HashType]string)

	for _, hashType := range []HashType{MD5, SHA1, SHA256, BLAKE3} {
		hash, err := FileHash(filePath, hashType)
		if err != nil {
			return nil, err
		}
		hashes[hashType] = hash
	}

	return hashes, nil
}

// ValidateHash validates whether a hash string has the correct format
func ValidateHash(hash string, hashType HashType) bool {
	expectedLength := 0
	switch hashType {
	case MD5:
		expectedLength = 32
	case SHA1:
		expectedLength = 40
	case SHA256:
		expectedLength = 64
	case BLAKE3:
		expectedLength = 64
	default:
		return false
	}

	if len(hash) != expectedLength {
		return false
	}

	// Check if valid hex string
	_, err := hex.DecodeString(hash)
	return err == nil
}

// FileHashWithExtension is based on the TypeScript implementation: calculates the blake3 hash
// of the file's base64 content + extension, returning the first 32 characters of the hex string
func FileHashWithExtension(file_path string) (string, error) {
	file, err := os.Open(file_path)
	if err != nil {
		return "", fmt.Errorf("failed to open file %s: %w", file_path, err)
	}
	defer file.Close()

	// Cloudflare hashes the Base64 representation followed by the extension.
	// Stream the encoded bytes into BLAKE3 to avoid retaining the file and
	// multiple Base64-sized copies in memory at the same time.
	hasher := blake3.New()
	base64_encoder := base64.NewEncoder(base64.StdEncoding, hasher)
	if _, err := io.Copy(base64_encoder, file); err != nil {
		_ = base64_encoder.Close()
		return "", fmt.Errorf("failed to read file %s: %w", file_path, err)
	}
	if err := base64_encoder.Close(); err != nil {
		return "", fmt.Errorf("failed to encode file %s: %w", file_path, err)
	}

	extension := strings.TrimPrefix(filepath.Ext(file_path), ".")
	if _, err := io.WriteString(hasher, extension); err != nil {
		return "", fmt.Errorf("failed to calculate blake3 hash: %w", err)
	}

	hash_bytes := hasher.Sum(nil)
	hash_hex := hex.EncodeToString(hash_bytes)

	if len(hash_hex) > 32 {
		return hash_hex[:32], nil
	}
	return hash_hex, nil
}

// StringHashWithExtension calculates the blake3 hash of string content + extension
func StringHashWithExtension(content, extension string) (string, error) {
	// Convert to base64
	base64Contents := base64.StdEncoding.EncodeToString([]byte(content))

	// Clean extension (remove dot)
	cleanExtension := strings.TrimPrefix(extension, ".")

	// Concatenate base64 content and extension
	data := base64Contents + cleanExtension

	// Calculate hash using blake3
	hasher := blake3.New()
	_, err := hasher.Write([]byte(data))
	if err != nil {
		return "", fmt.Errorf("failed to calculate blake3 hash: %w", err)
	}

	// Get hash bytes and convert to hex string
	hashBytes := hasher.Sum(nil)
	hashHex := hex.EncodeToString(hashBytes)

	// Return first 32 characters
	if len(hashHex) > 32 {
		return hashHex[:32], nil
	}
	return hashHex, nil
}
