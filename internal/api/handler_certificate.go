package api

import (
	"github.com/gin-gonic/gin"

	"wx_channel/internal/config"
	result "wx_channel/internal/util"
	"wx_channel/pkg/certificate"
)

func (c *APIClient) handleRootCertificateStatus(ctx *gin.Context) {
	cert := config.LoadCertFiles(c.config_store)
	installed, err := certificate.CheckHasCertificate(cert.Name)
	if err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"name":      cert.Name,
		"installed": installed,
	})
}

func (c *APIClient) handleRootCertificateInstall(ctx *gin.Context) {
	cert := config.LoadCertFiles(c.config_store)
	if err := certificate.InstallCertificate(cert.Cert); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"name":      cert.Name,
		"installed": true,
	})
}

func (c *APIClient) handleRootCertificateUninstall(ctx *gin.Context) {
	cert := config.LoadCertFiles(c.config_store)
	if err := certificate.UninstallCertificate(cert.Name); err != nil {
		result.Err(ctx, 500, err.Error())
		return
	}
	result.Ok(ctx, gin.H{
		"name":      cert.Name,
		"installed": false,
	})
}
