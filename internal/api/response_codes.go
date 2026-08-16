package api

// These codes are part of the application API contract. Feature-specific
// adapters maintain their own code sets instead of adding them here.
const (
	api_code_success                  = 0
	api_code_invalid_params           = 400
	api_code_duplicate_download_task  = 409
	api_code_missing_url              = 4002
	api_code_scraper_operation_failed = 2002
)

const api_success_message = "成功"
