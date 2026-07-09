package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"

	douyinpkg "wx_channel/pkg/scraper/douyin"
)

const defaultURL = "https://www.douyin.com/user/MS4wLjABAAAAOE57npukdzs0SH_Fk5RHB-qfUlnZ5jQT2R_KPH4Sd8s"

func main() {
	rawURL := flag.String("url", defaultURL, "douyin author homepage URL")
	configFile := flag.String("config", "config.yaml", "optional config file for douyin.cookie")
	curlFile := flag.String("curl-file", "", "optional markdown/shell file containing a signed douyin aweme/post curl")
	cookie := flag.String("cookie", "", "douyin cookie; falls back to DOUYIN_COOKIE, curl-file, and config douyin.cookie")
	apiURL := flag.String("api-url", "", "signed /aweme/v1/web/aweme/post/ URL; falls back to curl-file")
	userAgent := flag.String("user-agent", "", "user-agent; falls back to curl-file and a desktop Chrome UA")
	userHTMLFile := flag.String("user-html-file", "", "optional local user.html fixture instead of fetching homepage HTML")
	postJSONFile := flag.String("post-json-file", "", "optional local JSON/markdown fixture instead of fetching aweme/post")
	skipPage := flag.Bool("skip-page", false, "skip homepage HTML fetch and use aweme/post author data only")
	timeout := flag.Duration("timeout", 30*time.Second, "request timeout")
	count := flag.Int("count", 18, "aweme/post count")
	limit := flag.Int("limit", 5, "number of posts to print")
	flag.Parse()

	target, ok := douyinpkg.ParseProfileURL(*rawURL)
	if !ok {
		log.Fatalf("unsupported douyin author URL: %s", *rawURL)
	}

	curlText := readOptionalFile(*curlFile)
	resolvedCookie := firstNonEmpty(*cookie, os.Getenv("DOUYIN_COOKIE"), curlFileCookie(curlText), configCookie(*configFile))
	resolvedAPIURL := firstNonEmpty(*apiURL, curlFileAPIURL(curlText))
	resolvedUserAgent := firstNonEmpty(*userAgent, curlFileHeader(curlText, "user-agent"), douyinpkg.DefaultWebUserAgent)
	pageHTML := readOptionalFile(*userHTMLFile)

	fmt.Println("Request URL:", *rawURL)
	fmt.Println("Canonical:", target.Canonical)
	fmt.Println("Has cookie:", resolvedCookie != "")
	fmt.Println("Has signed API URL:", resolvedAPIURL != "")
	fmt.Println()

	var page *douyinpkg.ProfilePage
	if strings.TrimSpace(*postJSONFile) != "" {
		response, err := parsePostJSONFile(*postJSONFile)
		if err != nil {
			log.Fatal(err)
		}
		page = &douyinpkg.ProfilePage{
			URL:       target,
			SourceURL: *rawURL,
			User:      firstProfile(response.AwemeList, target.SecUserID),
			Response:  *response,
			Posts:     douyinpkg.SummarizeAwemes(response.AwemeList),
			HasMore:   response.HasMore > 0,
			MinCursor: response.MinCursor,
			MaxCursor: response.MaxCursor,
		}
		if strings.TrimSpace(pageHTML) != "" {
			if parsed, err := douyinpkg.ParseProfilePageHTML(target.Canonical, pageHTML); err == nil {
				page.User = parsed.User
			}
		}
	} else {
		client := douyinpkg.NewClient()
		client.Cookie = resolvedCookie
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		var err error
		page, err = client.FetchUserProfile(ctx, *rawURL, douyinpkg.ProfileOptions{
			Count:     *count,
			Cookie:    resolvedCookie,
			UserAgent: resolvedUserAgent,
			APIURL:    resolvedAPIURL,
			PageHTML:  pageHTML,
			SkipPage:  *skipPage,
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	if page == nil || page.User.SecUID == "" {
		log.Fatalf("未抓取到抖音作者信息: %s", target.SecUserID)
	}
	if len(page.Posts) == 0 {
		log.Fatalf("未抓取到抖音作品列表: sec_uid=%s warnings=%v", page.User.SecUID, page.Warnings)
	}

	fmt.Println("API URL:", redactedURL(page.APIURL))
	fmt.Println("Author:", firstNonEmpty(page.User.Nickname, page.User.UniqueID, page.User.UID))
	fmt.Println("UID:", firstNonEmpty(page.User.UID, "-"))
	fmt.Println("Sec UID:", page.User.SecUID)
	fmt.Println("Unique ID:", firstNonEmpty(page.User.UniqueID, "-"))
	fmt.Println("Signature:", singleLine(firstNonEmpty(page.User.Signature, "-"), 120))
	fmt.Println("Followers:", page.User.FollowerCount)
	fmt.Println("Following:", page.User.FollowingCount)
	fmt.Println("Aweme count:", page.User.AwemeCount)
	fmt.Println("Posts fetched:", len(page.Posts))
	fmt.Println("Has more:", page.HasMore)
	fmt.Println("Max cursor:", page.MaxCursor)
	for _, warning := range page.Warnings {
		fmt.Println("Warning:", warning)
	}
	fmt.Println()

	maxPosts := *limit
	if maxPosts <= 0 || maxPosts > len(page.Posts) {
		maxPosts = len(page.Posts)
	}
	for i, post := range page.Posts[:maxPosts] {
		fmt.Printf("%d. [%s] %s %s\n", i+1, post.ContentType, post.ID, firstNonEmpty(post.URL, "-"))
		fmt.Printf("   %s\n", singleLine(post.Description, 160))
		fmt.Printf("   author=%s created=%d\n", firstNonEmpty(post.Author.Nickname, post.Author.UniqueID, "-"), post.CreateTime)
		if post.VideoURL != "" {
			fmt.Printf("   video=%s\n", post.VideoURL)
		}
		if len(post.ImageURLs) > 0 {
			fmt.Printf("   images=%d first=%s\n", len(post.ImageURLs), post.ImageURLs[0])
		}
		fmt.Printf("   counts: digg=%d comment=%d collect=%d share=%d\n", post.DiggCount, post.CommentCount, post.CollectCount, post.ShareCount)
	}
}

func configCookie(configFile string) string {
	configFile = strings.TrimSpace(configFile)
	if configFile == "" {
		return ""
	}
	v := viper.New()
	v.SetConfigFile(configFile)
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if errors.As(err, &notFound) {
			return ""
		}
		return ""
	}
	return strings.TrimSpace(v.GetString("douyin.cookie"))
}

func parsePostJSONFile(path string) (*douyinpkg.AwemePostResponse, error) {
	body, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return nil, err
	}
	text := string(body)
	if strings.Contains(text, "```json") {
		text, err = markdownJSON(text)
		if err != nil {
			return nil, err
		}
	}
	return douyinpkg.ParseAwemePostResponse([]byte(text))
}

func markdownJSON(markdown string) (string, error) {
	idx := strings.Index(markdown, "```json")
	if idx < 0 {
		return "", errors.New("missing json fence")
	}
	body := markdown[idx+len("```json"):]
	start := strings.Index(body, "{")
	end := strings.LastIndex(body, "```")
	if start < 0 || end < 0 || end <= start {
		return "", errors.New("invalid json fence")
	}
	return body[start:end], nil
}

func readOptionalFile(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(body)
}

func curlFileCookie(text string) string {
	for _, pattern := range []string{
		`(?is)(?:^|\s)(?:-b|--cookie)\s+'([^']*)'`,
		`(?is)(?:^|\s)(?:-b|--cookie)\s+"([^"]*)"`,
		`(?im)-H\s+'cookie:\s*([^']*)'`,
		`(?im)-H\s+"cookie:\s*([^"]*)"`,
	} {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(text); len(match) > 1 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func curlFileAPIURL(text string) string {
	match := regexp.MustCompile("https://www\\.douyin\\.com/aweme/v1/web/aweme/post/\\?[^'\"`\\s]+").FindString(text)
	return strings.TrimSpace(match)
}

func curlFileHeader(text string, name string) string {
	name = regexp.QuoteMeta(strings.ToLower(strings.TrimSpace(name)))
	for _, pattern := range []string{
		`(?is)-H\s+'` + name + `:\s*([^']*)'`,
		`(?is)-H\s+"` + name + `:\s*([^"]*)"`,
	} {
		if match := regexp.MustCompile(pattern).FindStringSubmatch(text); len(match) > 1 {
			return strings.TrimSpace(match[1])
		}
	}
	return ""
}

func firstProfile(awemes []douyinpkg.Aweme, fallbackSecUID string) douyinpkg.UserProfile {
	for _, aweme := range awemes {
		profile := aweme.Author.Profile()
		if profile.SecUID != "" || profile.UID != "" || profile.Nickname != "" {
			if profile.SecUID == "" {
				profile.SecUID = fallbackSecUID
			}
			return profile
		}
	}
	return douyinpkg.UserProfile{SecUID: fallbackSecUID}
}

func redactedURL(rawURL string) string {
	if strings.TrimSpace(rawURL) == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	query := parsed.Query()
	for _, key := range []string{"msToken", "a_bogus", "verifyFp", "fp", "webid", "uifid", "x-secsdk-web-signature", "timestamp"} {
		if query.Has(key) {
			query.Set(key, "<redacted>")
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func singleLine(value string, max int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if max <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max]) + "..."
}
