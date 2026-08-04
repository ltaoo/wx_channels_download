package pages

// https://github.com/cloudflare/workers-sdk/blob/main/packages/wrangler/src/pages/upload.ts

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/samber/lo"
)

// Bucket distribution algorithm constants
const (
	BULK_UPLOAD_CONCURRENCY = 4                // Concurrent upload count
	MAX_BUCKET_SIZE         = 50 * 1024 * 1024 // 50MB max size per bucket
	MAX_BUCKET_FILE_COUNT   = 100              // Max files per bucket
)

// Bucket represents a file bucket.
type Bucket struct {
	Files         []FileContainer `json:"files"`
	RemainingSize int             `json:"remaining_size"`
}

// BucketResult holds the bucket distribution result.
type BucketResult struct {
	Buckets    []Bucket `json:"buckets"`
	TotalFiles int      `json:"total_files"`
	TotalSize  int      `json:"total_size"`
}

type FileContainer struct {
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeInBytes int    `json:"size_in_bytes"`
	Hash        string `json:"hash"`
}

var MAX_ASSET_COUNT = 50

// should_ignore checks whether a file path should be ignored.
func should_ignore(path string, ignorePatterns []string) bool {
	for _, pattern := range ignorePatterns {
		// Handle ** wildcard pattern
		if strings.Contains(pattern, "**") {
			// Convert ** to wildcard match
			pattern = strings.ReplaceAll(pattern, "**", "*")
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				return true
			}
		} else {
			// Exact filename match
			if filepath.Base(path) == pattern {
				return true
			}
		}
	}
	return false
}

type FileMetadata struct {
	ContentType string `json:"contentType"`
}
type FilePayloadToUpload struct {
	Key      string       `json:"key"`
	Value    string       `json:"value"`
	Metadata FileMetadata `json:"metadata"`
	Base64   bool
}

func distribute_files_to_buckets(files []FileContainer) BucketResult {
	// Initialize bucket array
	buckets := make([]Bucket, BULK_UPLOAD_CONCURRENCY)
	for i := range buckets {
		buckets[i] = Bucket{
			Files:         []FileContainer{},
			RemainingSize: MAX_BUCKET_SIZE,
		}
	}

	bucket_offset := 0
	total_files := 0
	total_size := 0

	// Iterate over all files for bucket distribution
	for _, file := range files {
		inserted := false

		// Try to insert file into an existing bucket
		for i := 0; i < len(buckets); i++ {
			bucket_index := (i + bucket_offset) % len(buckets)
			bucket := &buckets[bucket_index]

			// Check if bucket has enough space and is within file count limit
			if bucket.RemainingSize >= file.SizeInBytes &&
				len(bucket.Files) < MAX_BUCKET_FILE_COUNT {
				bucket.Files = append(bucket.Files, file)
				bucket.RemainingSize -= file.SizeInBytes
				inserted = true
				break
			}
		}

		// If no existing bucket can fit, create a new one
		if !inserted {
			newBucket := Bucket{
				Files:         []FileContainer{file},
				RemainingSize: MAX_BUCKET_SIZE - file.SizeInBytes,
			}
			buckets = append(buckets, newBucket)
		}

		bucket_offset++
		total_files++
		total_size += file.SizeInBytes
	}

	// Filter out empty buckets
	non_empty_buckets := []Bucket{}
	for _, bucket := range buckets {
		if len(bucket.Files) > 0 {
			non_empty_buckets = append(non_empty_buckets, bucket)
		}
	}

	return BucketResult{
		Buckets:    non_empty_buckets,
		TotalFiles: total_files,
		TotalSize:  total_size,
	}
}

// print_bucket_result prints bucket distribution results (for debugging).
func print_bucket_result(result BucketResult) {
	fmt.Printf("\n=== Bucket Results ===\n")
	fmt.Printf("Total files: %d\n", result.TotalFiles)
	fmt.Printf("Total size: %d bytes (%.2f MB)\n", result.TotalSize, float64(result.TotalSize)/(1024*1024))
	fmt.Printf("Bucket count: %d\n", len(result.Buckets))

	for i, bucket := range result.Buckets {
		bucketSize := MAX_BUCKET_SIZE - bucket.RemainingSize
		fmt.Printf("Bucket %d: %d files, %d bytes (%.2f MB), remaining: %d bytes\n",
			i+1, len(bucket.Files), bucketSize, float64(bucketSize)/(1024*1024), bucket.RemainingSize)
		for _, f := range bucket.Files {
			fmt.Println(f.Path, f.SizeInBytes)
		}
	}
	fmt.Println("================")
}

type UploadPayload struct {
	FilesMap map[string]FileContainer
	JWT      string
	// AccountId   string
	// ProjectName string
}

type UploadResp struct {
	Files map[string]string `json:"files"`
}

func Upload(payload UploadPayload) (*UploadResp, error) {
	files_map := payload.FilesMap
	jwt := payload.JWT
	// account_id := payload.AccountId
	// project_name := payload.ProjectName
	// jwt := "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJwYWdlcy1idWlsZC1tYWVzdHJvIiwiZXhwIjoxNzU0MzY5MDE0LCJmZWF0dXJlcyI6WyJmaWxlcyJdLCJpYXQiOjE3NTQzNjcyMTQsImlzcyI6ImZ1bmZldHRpIiwicHJvamVjdE5hbWVzcGFjZSI6IjI3Y2FjMGE0YWRkMTRkNDlhY2UzMDQxMWU5MzRkZWZlIn0.SZ8nSoM5Hv2T2ONaa4SZJvRZpJFpEZ0n4fLPKfwCaK2G-A-sTXD8IKEej7sgn9Vebx2evDt8zPczMSgQVS-UgxTizf1z9PYVHuwNqmFd4W0iOk8Hkas_PS0kKg78o1tpa8DsUESE0nnwzouf08P_luKIAOfo_YiS3fAMLZeen6G1gph4OdyrGoMV72sW8YmI-cDkOKaoX8CDSYsbyldxbGsUaIscJammvrsaHt_3C9nLPStD3PWcE6PE4qNZ7y_XNdFJ0SiRx0a9xtzePDeuZ1YpdcBwwDGY__J2sgLhambF-MbQlcLFmBMHeJr-3-IHLj1BhQo25_7E-IVKIfFFtIUGEUPOSVBP_UasXRHDwOodxv5BUGjEVceVlTwSbGuN6wjUL0yPOjzeYYIH_Wh52bLMulNlLM52OtN_3XSTnfLYxJL9YD2wgjPJS65BrgLA054XmO0y9TzCHTuucg16ecdk_gdbTnPikqcXNJcJxB4FRIh1kuSK-qvO_WG62SPGI8m2eCKzrlvOAiaY5BMu4ymirB7wpdowUKIjVRUjiwHoAXHDuJ-qHR8TWLVmKm14tPZP7VZGX0m9ED-r41xepVuT2r2BuGIYmA6THo2ug7kzGJJ0t5XkUrN2wXot3NzGiplklDwRcaMGJDxsjOF8giYlvo1kKe7g3Wrqdn7b31w"
	// jwt, err := Api_fetch_upload_token(account_id, project_name)
	// if err != nil {
	// 	log.Fatalf("Error fetching upload token: %v", err)
	// 	return err
	// }
	// fmt.Println(jwt)
	files_path := []string{}
	files_hash := []string{}
	files_path_with_hash := map[string]string{}
	for _, file := range files_map {
		files_path = append(files_path, file.Path)
		files_hash = append(files_hash, file.Hash)
		key := "/" + file.Filename
		files_path_with_hash[key] = file.Hash
	}
	missing_files, err := Api_fetch_missing_files(files_path, jwt)
	if err != nil {
		log.Fatalf("Error fetching missing files: %v", err)
		return nil, err
	}
	sorted_files := []FileContainer{}
	for _, file := range files_map {
		present := lo.Contains(missing_files, file.Hash)
		if !present {
			sorted_files = append(sorted_files, file)
		}
		sorted_files = append(sorted_files, file)
	}
	buckets := distribute_files_to_buckets(sorted_files)

	for _, bucket := range buckets.Buckets {
		if len(bucket.Files) == 0 {
			continue
		}

		var files []FilePayloadToUpload
		for _, f := range bucket.Files {
			fileContent, err := os.ReadFile(f.Path)
			if err != nil {
				log.Printf("Error reading file %s: %v", f.Path, err)
				continue
			}
			base64Content := base64.StdEncoding.EncodeToString(fileContent)

			files = append(files, FilePayloadToUpload{
				Key:   f.Hash,
				Value: base64Content,
				Metadata: FileMetadata{
					ContentType: f.ContentType,
				},
				Base64: true,
			})
		}
		fmt.Println("[PAGES]Upload - before Api_upload", files)
		Api_upload(files, jwt)
	}
	// print_bucket_result(buckets)
	_, err = Api_upsert_hashes(files_hash, jwt)
	if err != nil {
		return nil, err
	}
	return &UploadResp{
		Files: files_path_with_hash,
	}, nil
}
