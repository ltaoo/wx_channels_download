package application

import (
	"net"
	"strconv"

	"wx_channel/internal/api"
	"wx_channel/internal/config"
	"wx_channel/internal/mcpserver"
	"wx_channel/internal/services"
	"wx_channel/pkg/cookies"
	"wx_channel/pkg/flowengine"
	"wx_channel/pkg/scraper/zhihu"
)

func new_mcp_service(
	api_config *api.APIConfig,
	data_service *services.DataQueryService,
	download_task_service *services.DownloadTaskService,
	scraper_job_service *services.ScraperJobService,
	automation_service *services.AutomationService,
	enabled bool,
) (*services.MCPService, error) {
	cookie_reader := cookies.NewPersistentReader(api_config.WorkDir)
	service_config := mcpserver.Config{
		APIBaseURL:          mcp_api_base_url(api_config),
		Version:             api_config.Version,
		DataReader:          new_mcp_data_reader(data_service),
		ScraperJobs:         new_mcp_scraper_job_backend(scraper_job_service),
		DownloadTaskCreator: new_mcp_download_task_creator(download_task_service),
		DownloadTaskDeleter: new_mcp_download_task_deleter(download_task_service),
		WXMP:                new_mcp_wxmp_runtime(),
		WXChannels:          new_mcp_wxchannels_backend(),
		SphDeployer:         NewMCPSphDeployer(api_config.Original),
		ZhihuCollections:    zhihu.NewClient(cookie_reader, api_config.Original.Logger()),
		ZhihuCredentials:    cookie_reader,
		Automation:          new_mcp_automation_backend(automation_service),
	}
	var mcp_service *services.MCPService
	var err error
	if !enabled {
		mcp_service = services.NewLazyMCPService(service_config)
	} else {
		mcp_service, err = services.NewMCPService(service_config)
	}
	if err != nil {
		return nil, err
	}
	flowengine.RegisterServiceNode(automation_service.FlowEngine(), mcp_service.ExecuteTool)
	return mcp_service, nil
}

func mcp_api_base_url(api_config *api.APIConfig) string {
	hostname := config.APIClientHostname(api_config.Hostname)
	port := api_config.Port
	if port <= 0 {
		port = 2022
	}
	return "http://" + net.JoinHostPort(hostname, strconv.Itoa(port))
}
