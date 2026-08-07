package pages

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func Api_fetch_missing_files(hashes []string, jwt string) ([]string, error) {
	// CLOUDFLARE_API_TOKEN := "d1zCvKDV_wyGepEW4rFkyY6ueXXuzURqrUisIQIb"
	CLOUDFLARE_API_BASE_URL := "https://api.cloudflare.com/client/v4"

	methods := "POST"
	request_url := CLOUDFLARE_API_BASE_URL + "/pages/assets/check-missing"
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + jwt,
	}
	body, err := json.Marshal(map[string]interface{}{
		"hashes": hashes,
	})
	if err != nil {
		return []string{}, err
	}
	req, err := http.NewRequest(methods, request_url, strings.NewReader(string(body)))
	if err != nil {
		return []string{}, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return []string{}, err
	}
	defer resp.Body.Close()
	resp_bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return []string{}, err
	}
	fmt.Println("Api_fetch_missing_files", string(resp_bytes))
	// Return the specified type
	var result struct {
		Success bool     `json:"success"`
		Result  []string `json:"result"`
	}
	if err := json.Unmarshal(resp_bytes, &result); err != nil {
		return []string{}, err
	}
	if !result.Success {
		return []string{}, errors.New("fetch missing files failed")
	}
	return result.Result, nil
}

type UploadTokenResp struct {
	Result struct {
		JWT string `json:"jwt"`
	} `json:"result"`
	Success  bool     `json:"success"`
	Errors   []string `json:"errors"`
	Messages []string `json:"messages"`
}

func Api_fetch_upload_token(account_id string, project_name string) (*UploadTokenResp, error) {
	// CF_PAGES_UPLOAD_JWT := ""

	CLOUDFLARE_API_TOKEN := "d1zCvKDV_wyGepEW4rFkyY6ueXXuzURqrUisIQIb"
	CLOUDFLARE_API_BASE_URL := "https://api.cloudflare.com/client/v4"

	methods := "GET"
	request_url := CLOUDFLARE_API_BASE_URL + "/accounts/" + account_id + "/pages/projects/" + project_name + "/upload-token"
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + CLOUDFLARE_API_TOKEN,
	}
	body := map[string][]string{}
	req, err := http.NewRequest(methods, request_url, strings.NewReader(url.Values(body).Encode()))
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// Read raw response body
	resp_bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var r UploadTokenResp
	if err := json.Unmarshal(resp_bytes, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func Api_upload(files []FilePayloadToUpload, jwt string) (string, error) {
	CLOUDFLARE_API_BASE_URL := "https://api.cloudflare.com/client/v4"

	methods := "POST"
	request_url := CLOUDFLARE_API_BASE_URL + "/pages/assets/upload"
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + jwt,
	}
	body, err := json.Marshal(files)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(methods, request_url, strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	resp_bytes, err := io.ReadAll(resp.Body)
	fmt.Println("Api_upload", string(resp_bytes))
	if err != nil {
		return "", err
	}
	return string(resp_bytes), nil
}

func Api_upsert_hashes(hashes []string, jwt string) (string, error) {
	CLOUDFLARE_API_BASE_URL := "https://api.cloudflare.com/client/v4"

	request_method := "POST"
	request_url := CLOUDFLARE_API_BASE_URL + "/pages/assets/upsert-hashes"
	request_headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + jwt,
	}
	request_body, err := json.Marshal(map[string]interface{}{
		"hashes": hashes,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(request_method, request_url, strings.NewReader(string(request_body)))
	if err != nil {
		return "", err
	}
	for key, value := range request_headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	resp_bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	fmt.Println("Api_upsert_hashes", string(resp_bytes))
	return string(resp_bytes), nil
}

type DeploymentBody struct {
	AccountId   string            `json:"account_id"`
	ProjectName string            `json:"project_name"`
	Manifest    map[string]string `json:"manifest"`
	// Buffer      *bytes.Buffer `json:""buffer`
}

func Api_create_deployment(body DeploymentBody) (string, error) {
	CLOUDFLARE_API_TOKEN := "d1zCvKDV_wyGepEW4rFkyY6ueXXuzURqrUisIQIb"
	CLOUDFLARE_API_BASE_URL := "https://api.cloudflare.com/client/v4"

	request_method := "POST"
	request_url := CLOUDFLARE_API_BASE_URL + "/accounts/" + body.AccountId + "/pages/projects/" + body.ProjectName + "/deployments"

	var request_body bytes.Buffer
	writer := multipart.NewWriter(&request_body)
	request_body_manifest, err := json.Marshal(body.Manifest)
	if err != nil {
		return "", err
	}
	writer.WriteField("manifest", string(request_body_manifest))
	writer.Close()
	request_headers := map[string]string{
		"Content-Type":  writer.FormDataContentType(),
		"Authorization": "Bearer " + CLOUDFLARE_API_TOKEN,
	}
	req, err := http.NewRequest(request_method, request_url, &request_body)
	if err != nil {
		return "", err
	}
	for key, value := range request_headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	resp_bytes, err := io.ReadAll(resp.Body)
	fmt.Println("Api_create_deployment", string(resp_bytes))
	if err != nil {
		return "", err
	}
	return string(resp_bytes), nil
}

// IsJwtExpired checks if a JWT token is expired
// Returns true if expired, false if not expired, and error if token is invalid
func IsJwtExpired(token string) (bool, error) {
	// During testing we don't use valid JWTs, so don't try and parse them
	if token == "<<funfetti-auth-jwt>>" ||
		token == "<<funfetti-auth-jwt2>>" ||
		token == "<<aus-completion-token>>" {
		return false, nil
	}

	// Split the JWT token by dots
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false, errors.New("invalid token format: expected 3 parts separated by dots")
	}

	// Decode the payload (second part)
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false, fmt.Errorf("invalid token: failed to decode payload: %v", err)
	}

	// Parse the JSON payload
	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return false, fmt.Errorf("invalid token: failed to parse payload JSON: %v", err)
	}

	// Get the expiration time
	expInterface, exists := payload["exp"]
	if !exists {
		return false, errors.New("invalid token: missing expiration claim")
	}

	// Convert expiration to float64 (JWT exp is typically a number)
	var exp float64
	switch v := expInterface.(type) {
	case float64:
		exp = v
	case int:
		exp = float64(v)
	case int64:
		exp = float64(v)
	case string:
		// Try to parse as string
		expFloat, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return false, fmt.Errorf("invalid token: expiration claim is not a valid number: %v", err)
		}
		exp = expFloat
	default:
		return false, fmt.Errorf("invalid token: expiration claim has unexpected type: %T", expInterface)
	}

	// Get current time in seconds since epoch
	now := time.Now().Unix()

	// Check if token is expired
	return int64(exp) <= now, nil
}

// Example: send simple form data
func Api_send_simple_formdata() error {
	url := "https://api.example.com/upload"

	// Create form data
	form_date := make(map[string][]string)
	form_date["name"] = []string{"test file"}
	form_date["description"] = []string{"这是一个测试文件"}
	form_date["category"] = []string{"document"}

	// Send request
	resp, err := http.PostForm(url, form_date)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println("Api_send_simple_formdata", string(body))
	return nil
}

// Send FormData containing files
func Api_send_multipart_formdata() error {
	url := "https://api.example.com/upload"

	// Create multipart writer
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add text field
	writer.WriteField("name", "test file")
	writer.WriteField("description", "这是一个测试文件")

	// Add file field
	fileWriter, err := writer.CreateFormFile("file", "test.txt")
	if err != nil {
		return err
	}

	// Write file content
	fileContent := []byte("this is the file content")
	fileWriter.Write(fileContent)

	// Close writer
	writer.Close()

	// Create request
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}

	// Set Content-Type header (includes boundary)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println("Api_send_multipart_formdata", string(body))
	return nil
}

// Upload local file
func Api_upload_local_file(filePath string) error {
	url := "https://api.example.com/upload"

	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Create multipart writer
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add text field
	writer.WriteField("name", "uploaded file")
	writer.WriteField("description", "从本地文件上传")

	// Create file field
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return err
	}

	// Copy file content to form
	_, err = io.Copy(part, file)
	if err != nil {
		return err
	}

	// Close writer
	writer.Close()

	// Create request
	req, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return err
	}

	// Set Content-Type header
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Println("Api_upload_local_file", string(body))
	return nil
}
