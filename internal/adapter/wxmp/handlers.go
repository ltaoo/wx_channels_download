package wxmpadapter

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	result "wx_channel/internal/apiresult"
	"wx_channel/pkg/scraper/wxmp"
)

var proxy_image_url_reg = regexp.MustCompile(`https?://mmbiz\.qpic\.cn/(?:[a-zA-Z0-9_]+/)*[a-zA-Z0-9_]+/[^\s"']+`)

func (r *Routes) handle_websocket(ctx *gin.Context) {
	r.server.ServeWebsocket(ctx.Writer, ctx.Request)
}

func (r *Routes) handle_fetch_message_list(ctx *gin.Context) {
	if !r.server.ValidateToken(ctx.Query("token")) {
		write_api_error(ctx, api_code_token_invalid)
		return
	}
	offset, parse_err := strconv.Atoi(ctx.Query("offset"))
	if parse_err != nil {
		offset = 0
	}
	data, err := r.server.FetchMessageList(wxmp.FetchMessageListParams{
		Biz:        ctx.Query("biz"),
		Offset:     offset,
		Uin:        ctx.Query("uin"),
		Key:        ctx.Query("key"),
		PassTicket: ctx.Query("pass_ticket"),
	})
	if err != nil {
		write_client_error(ctx, err, api_code_fetch_message)
		return
	}
	result.Ok(ctx, data)
}

func (r *Routes) handle_fetch_article_list(ctx *gin.Context) {
	biz := ctx.Query("biz")
	if biz == "" {
		write_api_error(ctx, api_code_invalid_params)
		return
	}
	data, err := r.server.FetchArticleList(biz)
	if err != nil {
		write_client_error(ctx, err, api_code_fetch_message)
		return
	}
	result.Ok(ctx, data)
}

func (r *Routes) handle_fetch_list(ctx *gin.Context) {
	token := ctx.Query("token")
	if !r.server.ValidateToken(token) {
		write_api_error(ctx, api_code_token_invalid)
		return
	}
	page, page_err := strconv.Atoi(ctx.Query("page"))
	if page_err != nil {
		page = 1
	}
	page_size, page_size_err := strconv.Atoi(ctx.Query("page_size"))
	if page_size_err != nil {
		page_size = 10
	}
	var effective *bool
	if raw_effective := ctx.Query("is_effective"); raw_effective != "" {
		value := raw_effective == "1" || strings.EqualFold(raw_effective, "true")
		effective = &value
	}
	data := r.server.ListOfficialAccounts(wxmp.ListOfficialAccountsParams{
		Page:      page,
		PageSize:  page_size,
		Keyword:   ctx.Query("keyword"),
		Effective: effective,
		Token:     token,
	})
	result.Ok(ctx, data)
}

func (r *Routes) handle_delete(ctx *gin.Context) {
	if !r.server.ValidateRefreshToken(ctx.Query("token")) {
		write_api_error(ctx, api_code_token_invalid)
		return
	}
	var body wxmp.OfficialAccountBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		write_api_error(ctx, api_code_invalid_params)
		return
	}
	if body.Biz == "" {
		write_api_error(ctx, api_code_missing_biz)
		return
	}
	r.server.DeleteOfficialAccount(body.Biz)
	result.Ok(ctx, nil)
}

func (r *Routes) handle_refresh_event(ctx *gin.Context) {
	if !r.server.ValidateRefreshToken(ctx.Query("token")) {
		write_api_error(ctx, api_code_token_invalid)
		return
	}
	var body wxmp.OfficialAccount
	if err := ctx.ShouldBindJSON(&body); err != nil {
		write_api_error(ctx, api_code_invalid_params)
		return
	}
	if body.Biz == "" || body.Key == "" {
		write_api_error(ctx, api_code_missing_biz)
		return
	}
	r.server.RefreshOfficialAccount(body)
	result.Ok(ctx, nil)
}

func (r *Routes) handle_refresh_with_frontend(ctx *gin.Context) {
	if err := r.server.Validate(); err != nil {
		write_api_error(ctx, api_code_client_not_ready)
		return
	}
	var body struct {
		BizList []string `json:"biz_list"`
	}
	_ = ctx.ShouldBindJSON(&body)
	if err := r.server.RefreshSpecifiedOfficialAccountList(body.BizList); err != nil {
		write_client_error(ctx, err, 1001)
		return
	}
	result.Ok(ctx, nil)
}

func (r *Routes) handle_official_account_rss(ctx *gin.Context) {
	if !r.server.ValidateToken(ctx.Query("token")) {
		write_api_error(ctx, api_code_token_invalid)
		return
	}
	offset, parse_err := strconv.Atoi(ctx.Query("offset"))
	if parse_err != nil {
		offset = 0
	}
	feed, err := r.server.BuildRSS(wxmp.RSSParams{
		Biz:            ctx.Query("biz"),
		Offset:         offset,
		IncludeContent: ctx.Query("content") == "1",
		ProxyLinks:     ctx.Query("proxy") == "1",
		ProxyCover:     ctx.Query("proxy_cover") == "1",
		SelfURL:        "http://" + ctx.Request.Host + ctx.Request.RequestURI,
	})
	if err != nil {
		write_client_error(ctx, err, api_code_fetch_message)
		return
	}
	ctx.Header("Content-Type", "application/atom+xml; charset=utf-8")
	ctx.XML(http.StatusOK, feed)
}

func (r *Routes) handle_official_account_proxy(ctx *gin.Context) {
	if !r.server.ValidateToken(ctx.Query("token")) {
		write_api_error(ctx, api_code_token_invalid)
		return
	}
	target_url := ctx.Query("url")
	if target_url == "" {
		write_api_error(ctx, api_code_missing_url)
		return
	}
	response, err := r.server.FetchOfficialAccountProxy(target_url)
	if err != nil {
		write_client_error(ctx, err, api_code_proxy_dispatch)
		return
	}
	defer response.Body.Close()
	for key, values := range response.Header {
		if key == "Content-Length" {
			continue
		}
		for _, value := range values {
			ctx.Header(key, value)
		}
	}
	ctx.Status(response.StatusCode)
	if !strings.Contains(response.Header.Get("Content-Type"), "text/html") {
		_, _ = io.Copy(ctx.Writer, response.Body)
		return
	}
	body, read_err := io.ReadAll(response.Body)
	if read_err != nil {
		return
	}
	remote_server := r.server.RemoteServerAddress()
	rewritten := proxy_image_url_reg.ReplaceAllStringFunc(string(body), func(match string) string {
		image_url := html.UnescapeString(match)
		return fmt.Sprintf("%s/mp/proxy?url=%s", remote_server, url.QueryEscape(image_url))
	})
	_, _ = ctx.Writer.Write([]byte(rewritten))
}

func (r *Routes) handle_fetch_official_account_clients(ctx *gin.Context) {
	result.Ok(ctx, gin.H{"list": r.server.ClientStatuses()})
}

func write_client_error(ctx *gin.Context, err error, fallback_code int) {
	code := fallback_code
	message := err.Error()
	if error_kind, error_message, location, ok := wxmp.ErrorDetails(err); ok {
		code = api_code_for_error_kind(error_kind, fallback_code)
		message = error_message
		if location != "" {
			message = fmt.Sprintf("%s (loc=%s)", message, location)
		}
	}
	result.Err(ctx, code, message)
}
