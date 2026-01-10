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

// HashType 定义支持的哈希类型
type HashType string

const (
	MD5    HashType = "md5"
	SHA1   HashType = "sha1"
	SHA256 HashType = "sha256"
	BLAKE3 HashType = "blake3"
)

// FileHash 计算文件的哈希值
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

// StringHash 计算字符串的哈希值
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

// BytesHash 计算字节数组的哈希值
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

// FileHashAll 计算文件的所有支持的哈希值
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

// ValidateHash 验证哈希值格式是否正确
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

	// 检查是否为有效的十六进制字符串
	_, err := hex.DecodeString(hash)
	return err == nil
}

// FileHashWithExtension 参考 TypeScript 实现：计算文件内容的 base64 + 扩展名的 blake3 哈希
// 返回前 32 个字符的十六进制字符串
func FileHashWithExtension(filePath string) (string, error) {
	// 读取文件内容
	contents, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// 转换为 base64
	base64Contents := base64.StdEncoding.EncodeToString(contents)

	// 获取文件扩展名（不包含点）
	extension := filepath.Ext(filePath)
	if len(extension) > 0 && extension[0] == '.' {
		extension = extension[1:] // 去掉点
	}

	// 拼接 base64 内容和扩展名
	data := base64Contents + extension

	// 使用 blake3 计算哈希
	hasher := blake3.New()
	_, err = hasher.Write([]byte(data))
	if err != nil {
		return "", fmt.Errorf("failed to calculate blake3 hash: %w", err)
	}

	// 获取哈希值并转换为十六进制字符串
	hashBytes := hasher.Sum(nil)
	hashHex := hex.EncodeToString(hashBytes)

	// 返回前 32 个字符
	if len(hashHex) > 32 {
		return hashHex[:32], nil
	}
	return hashHex, nil
}

// StringHashWithExtension 计算字符串内容 + 扩展名的 blake3 哈希
func StringHashWithExtension(content, extension string) (string, error) {
	// 转换为 base64
	base64Contents := base64.StdEncoding.EncodeToString([]byte(content))

	// 清理扩展名（去掉点）
	cleanExtension := strings.TrimPrefix(extension, ".")

	// 拼接 base64 内容和扩展名
	data := base64Contents + cleanExtension

	// 使用 blake3 计算哈希
	hasher := blake3.New()
	_, err := hasher.Write([]byte(data))
	if err != nil {
		return "", fmt.Errorf("failed to calculate blake3 hash: %w", err)
	}

	// 获取哈希值并转换为十六进制字符串
	hashBytes := hasher.Sum(nil)
	hashHex := hex.EncodeToString(hashBytes)

	// 返回前 32 个字符
	if len(hashHex) > 32 {
		return hashHex[:32], nil
	}
	return hashHex, nil
}
