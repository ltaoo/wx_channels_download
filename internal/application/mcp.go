package application

import (
	"net"
	"strconv"
	"strings"

	"wx_channel/internal/api"
	"wx_channel/internal/services"
)

func new_mcp_service(
	api_config *api.APIConfig,
	data_service *services.DataQueryService,
	scraper_job_service *services.ScraperJobService,
) (*services.MCPService, error) {
	return services.NewMCPService(services.MCPServiceConfig{
		APIBaseURL:  mcp_api_base_url(api_config),
		Version:     api_config.Version,
		DataReader:  new_mcp_data_reader(data_service),
		ScraperJobs: new_mcp_scraper_job_backend(scraper_job_service),
	})
}

func mcp_api_base_url(api_config *api.APIConfig) string {
	hostname := strings.TrimSpace(api_config.Hostname)
	parsed_ip := net.ParseIP(hostname)
	if hostname == "" || (parsed_ip != nil && parsed_ip.IsUnspecified()) {
		hostname = "127.0.0.1"
	}
	port := api_config.Port
	if port <= 0 {
		port = 2022
	}
	return "http://" + net.JoinHostPort(hostname, strconv.Itoa(port))
}
