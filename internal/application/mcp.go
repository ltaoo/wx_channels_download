package application

import (
	"net"
	"strconv"

	"wx_channel/internal/api"
	"wx_channel/internal/config"
	"wx_channel/internal/services"
)

func new_mcp_service(
	api_config *api.APIConfig,
	data_service *services.DataQueryService,
	download_task_service *services.DownloadTaskService,
	scraper_job_service *services.ScraperJobService,
	enabled bool,
) (*services.MCPService, error) {
	service_config := services.MCPServiceConfig{
		APIBaseURL:          mcp_api_base_url(api_config),
		Version:             api_config.Version,
		DataReader:          new_mcp_data_reader(data_service),
		ScraperJobs:         new_mcp_scraper_job_backend(scraper_job_service),
		DownloadTaskDeleter: new_mcp_download_task_deleter(download_task_service),
	}
	if !enabled {
		return services.NewLazyMCPService(service_config), nil
	}
	return services.NewMCPService(service_config)
}

func mcp_api_base_url(api_config *api.APIConfig) string {
	hostname := config.APIClientHostname(api_config.Hostname)
	port := api_config.Port
	if port <= 0 {
		port = 2022
	}
	return "http://" + net.JoinHostPort(hostname, strconv.Itoa(port))
}
