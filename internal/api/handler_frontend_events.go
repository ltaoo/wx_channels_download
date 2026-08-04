package api

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/fatih/color"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	scraper_wxchannels "wx_channel/pkg/scraper/wxchannels"
	scraper_wxmp "wx_channel/pkg/scraper/wxmp"
	result "wx_channel/internal/util"
)

// handleFrontendTip handles frontend log/tip messages posted from injected pages.
func (c *APIClient) handleFrontendTip(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}
	var data scraper_wxchannels.FrontendTip
	if err := json.Unmarshal(body, &data); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	prefixText := "[FRONTEND]"
	prefix := data.Prefix
	if prefix == nil {
		prefix = &prefixText
	}
	if data.End == 1 {
		fmt.Println()
	} else if data.Replace == 1 {
		fmt.Printf("\r\033[K%v%s", *prefix, data.Msg)
	} else if data.IgnorePrefix == 1 {
		fmt.Printf("%s\n", data.Msg)
	} else {
		fmt.Printf("%v%s\n", *prefix, data.Msg)
	}
	result.Ok(ctx, nil)
}

// handleFrontendError handles frontend error messages posted from injected pages.
func (c *APIClient) handleFrontendError(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}
	var data scraper_wxchannels.FrontendErrorTip
	if err := json.Unmarshal(body, &data); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	color.Red(fmt.Sprintf("[FRONTEND ERROR]%s\n", data.Msg))
	result.Ok(ctx, nil)
}

// handleFrontendReport handles unified frontend reports, level is "info" or "error".
func (c *APIClient) handleFrontendReport(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}
	var data scraper_wxchannels.FrontendReport
	if err := json.Unmarshal(body, &data); err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}

	// Write to log file -- parse all fields to support arbitrary key=value passed by the frontend fluent logger
	var extraFields map[string]interface{}
	json.Unmarshal(body, &extraFields)
	delete(extraFields, "level")
	delete(extraFields, "msg")
	delete(extraFields, "end")
	delete(extraFields, "replace")
	delete(extraFields, "ignore_prefix")
	delete(extraFields, "prefix")

	evt := c.logger.WithLevel(zerologLevel(data.Level)).
		Str("source", "frontend").
		Str("msg", data.Msg)
	for k, v := range extraFields {
		evt = evt.Interface(k, v)
	}
	evt.Msg("frontend report")

	// Terminal display
	if data.Level == "error" {
		color.Red(fmt.Sprintf("[FRONTEND ERROR]%s\n", data.Msg))
	} else {
		prefixText := "[FRONTEND]"
		prefix := data.Prefix
		if prefix == nil {
			prefix = &prefixText
		}
		if data.End == 1 {
			fmt.Println()
		} else if data.Replace == 1 {
			fmt.Printf("\r\033[K%v%s", *prefix, data.Msg)
		} else if data.IgnorePrefix == 1 {
			fmt.Printf("%s\n", data.Msg)
		} else {
			fmt.Printf("%v%s\n", *prefix, data.Msg)
		}
	}
	result.Ok(ctx, nil)
}

func zerologLevel(level string) zerolog.Level {
	switch level {
	case "error":
		return zerolog.ErrorLevel
	case "warn":
		return zerolog.WarnLevel
	case "debug":
		return zerolog.DebugLevel
	default:
		return zerolog.InfoLevel
	}
}

// handleFrontendArticle handles official account article metadata posted from injected pages.
func (c *APIClient) handleFrontendArticle(ctx *gin.Context) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		result.Err(ctx, 400, "read body failed")
		return
	}
	profile, err := scraper_wxmp.NewOfficialAccountArticleProfile(json.RawMessage(body))
	if err != nil {
		result.Err(ctx, 400, err.Error())
		return
	}
	if profile != nil {
		fmt.Printf("\nOpened official account article\n%s\n", profile.Title)
	}
	result.Ok(ctx, nil)
}
