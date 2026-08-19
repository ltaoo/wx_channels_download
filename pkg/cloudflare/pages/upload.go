package pages

// Asset upload follows Wrangler's content-addressed Pages upload protocol.

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

const (
	bulk_upload_concurrency = 4
	max_bucket_size         = 50 * 1024 * 1024
	max_bucket_file_count   = 100
)

// Bucket represents a file upload bucket.
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

// FileContainer describes one validated static asset.
type FileContainer struct {
	Path        string `json:"path"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeInBytes int    `json:"size_in_bytes"`
	Hash        string `json:"hash"`
}

// FileMetadata is sent to the Pages Assets upload API.
type FileMetadata struct {
	ContentType string `json:"contentType"`
}

// FilePayloadToUpload is one base64-encoded Pages asset.
type FilePayloadToUpload struct {
	Key      string       `json:"key"`
	Value    string       `json:"value"`
	Metadata FileMetadata `json:"metadata"`
	Base64   bool         `json:"base64"`
}

// UploadPayload is retained for compatibility with the original package API.
type UploadPayload struct {
	FilesMap map[string]FileContainer
	JWT      string
}

// UploadResp contains the Pages path-to-content-hash manifest.
type UploadResp struct {
	Files map[string]string `json:"files"`
}

func distribute_files_to_buckets(files []FileContainer) BucketResult {
	buckets := make([]Bucket, bulk_upload_concurrency)
	for bucket_index := range buckets {
		buckets[bucket_index] = Bucket{
			Files:         []FileContainer{},
			RemainingSize: max_bucket_size,
		}
	}
	bucket_offset := 0
	total_size := 0
	for _, file := range files {
		inserted := false
		for bucket_index := range buckets {
			candidate_index := (bucket_index + bucket_offset) % len(buckets)
			bucket := &buckets[candidate_index]
			if bucket.RemainingSize >= file.SizeInBytes && len(bucket.Files) < max_bucket_file_count {
				bucket.Files = append(bucket.Files, file)
				bucket.RemainingSize -= file.SizeInBytes
				inserted = true
				break
			}
		}
		if !inserted {
			buckets = append(buckets, Bucket{
				Files:         []FileContainer{file},
				RemainingSize: max_bucket_size - file.SizeInBytes,
			})
		}
		bucket_offset++
		total_size += file.SizeInBytes
	}
	non_empty_buckets := make([]Bucket, 0, len(buckets))
	for _, bucket := range buckets {
		if len(bucket.Files) > 0 {
			non_empty_buckets = append(non_empty_buckets, bucket)
		}
	}
	return BucketResult{
		Buckets:    non_empty_buckets,
		TotalFiles: len(files),
		TotalSize:  total_size,
	}
}

func select_missing_files(
	files_map map[string]FileContainer,
	missing_hashes []string,
) []FileContainer {
	missing_hash_set := make(map[string]struct{}, len(missing_hashes))
	for _, missing_hash := range missing_hashes {
		missing_hash_set[missing_hash] = struct{}{}
	}
	files_to_upload := make([]FileContainer, 0, len(missing_hash_set))
	for _, file := range files_map {
		if _, missing := missing_hash_set[file.Hash]; !missing {
			continue
		}
		files_to_upload = append(files_to_upload, file)
		// Pages assets are content-addressed. Identical hashes need one upload.
		delete(missing_hash_set, file.Hash)
	}
	sort.Slice(files_to_upload, func(left_index int, right_index int) bool {
		if files_to_upload[left_index].SizeInBytes != files_to_upload[right_index].SizeInBytes {
			return files_to_upload[left_index].SizeInBytes > files_to_upload[right_index].SizeInBytes
		}
		return files_to_upload[left_index].Path < files_to_upload[right_index].Path
	})
	return files_to_upload
}

func build_upload_bucket_payload(bucket Bucket) ([]FilePayloadToUpload, error) {
	files := make([]FilePayloadToUpload, 0, len(bucket.Files))
	for _, file := range bucket.Files {
		file_content, err := os.ReadFile(file.Path)
		if err != nil {
			return nil, fmt.Errorf("读取 Pages 资源 %s 失败: %w", file.Path, err)
		}
		files = append(files, FilePayloadToUpload{
			Key:   file.Hash,
			Value: base64.StdEncoding.EncodeToString(file_content),
			Metadata: FileMetadata{
				ContentType: file.ContentType,
			},
			Base64: true,
		})
	}
	return files, nil
}

type upload_batch_func func(context.Context, []FilePayloadToUpload, string) error

func upload_file_buckets(
	request_context context.Context,
	buckets []Bucket,
	jwt string,
	upload_batch upload_batch_func,
) error {
	if len(buckets) == 0 {
		return nil
	}
	worker_count := bulk_upload_concurrency
	if worker_count > len(buckets) {
		worker_count = len(buckets)
	}
	bucket_channel := make(chan Bucket)
	error_channel := make(chan error, len(buckets))
	var worker_group sync.WaitGroup
	worker_group.Add(worker_count)
	for worker_index := 0; worker_index < worker_count; worker_index++ {
		go func() {
			defer worker_group.Done()
			for bucket := range bucket_channel {
				files, err := build_upload_bucket_payload(bucket)
				if err == nil {
					err = upload_batch(request_context, files, jwt)
				}
				if err != nil {
					error_channel <- err
				}
			}
		}()
	}
	for _, bucket := range buckets {
		bucket_channel <- bucket
	}
	close(bucket_channel)
	worker_group.Wait()
	close(error_channel)
	for upload_err := range error_channel {
		return upload_err
	}
	return nil
}

func (c *Client) upload_assets(
	request_context context.Context,
	files_map map[string]FileContainer,
	jwt string,
) (*UploadResp, error) {
	file_hashes := make([]string, 0, len(files_map))
	seen_hashes := make(map[string]struct{}, len(files_map))
	manifest := make(map[string]string, len(files_map))
	for relative_path, file := range files_map {
		if _, seen := seen_hashes[file.Hash]; !seen {
			seen_hashes[file.Hash] = struct{}{}
			file_hashes = append(file_hashes, file.Hash)
		}
		manifest["/"+strings.TrimLeft(strings.ReplaceAll(relative_path, "\\", "/"), "/")] = file.Hash
	}
	sort.Strings(file_hashes)
	missing_hashes, err := c.fetch_missing_files(request_context, file_hashes, jwt)
	if err != nil {
		return nil, err
	}
	files_to_upload := select_missing_files(files_map, missing_hashes)
	buckets := distribute_files_to_buckets(files_to_upload)
	if err := upload_file_buckets(
		request_context,
		buckets.Buckets,
		jwt,
		c.upload_batch,
	); err != nil {
		return nil, err
	}
	if err := c.upsert_hashes(request_context, file_hashes, jwt); err != nil {
		return nil, err
	}
	return &UploadResp{Files: manifest}, nil
}

// Upload uploads assets using the default Cloudflare endpoint.
func Upload(payload UploadPayload) (*UploadResp, error) {
	return NewClient(ClientOptions{}).upload_assets(
		context.Background(),
		payload.FilesMap,
		payload.JWT,
	)
}
