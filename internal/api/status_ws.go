package api

import "github.com/gin-gonic/gin"

func (c *APIClient) channelsStatusData() gin.H {
	available := false
	if c.channels != nil {
		available = c.channels.Available()
	}
	return gin.H{
		"available": available,
		"channels": gin.H{
			"available": available,
		},
	}
}
