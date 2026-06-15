package officialaccount

import (
	"html"
	"strings"
	"unicode"
)

type officialAccountArticleVariable struct {
	Title string `json:"title,omitempty"`
}

type officialAccountPublisherVariable struct {
	AvatarURL string `json:"avatar_url,omitempty"`
	Nickname  string `json:"nickname,omitempty"`
	Biz       string `json:"biz,omitempty"`
	Username  string `json:"username,omitempty"`
}

type officialAccountPageVariable struct {
	Publisher officialAccountPublisherVariable `json:"publisher,omitempty"`
	Article   officialAccountArticleVariable   `json:"article,omitempty"`
}

func buildOfficialAccountVariables(htmlText string) map[string]interface{} {
	page := extractOfficialAccountPageVariable(htmlText)
	variables := map[string]interface{}{}
	if page.Publisher.AvatarURL == "" &&
		page.Publisher.Nickname == "" &&
		page.Publisher.Biz == "" &&
		page.Publisher.Username == "" &&
		page.Article.Title == "" {
		return variables
	}
	variables["officialAccount"] = page
	return variables
}

func extractOfficialAccountPageVariable(htmlText string) officialAccountPageVariable {
	block, ok := extractWindowObjectBlock(htmlText, "cgiDataNew")
	if !ok {
		return officialAccountPageVariable{}
	}

	nickname := decodeWechatJSString(topLevelStringProperty(block, "nick_name"))
	avatarURL := firstNonEmpty(
		decodeWechatJSString(topLevelStringProperty(block, "round_head_img")),
		decodeWechatJSString(topLevelStringProperty(block, "ori_head_img_url")),
		decodeWechatJSString(topLevelStringProperty(block, "hd_head_img")),
	)

	return officialAccountPageVariable{
		Publisher: officialAccountPublisherVariable{
			AvatarURL: avatarURL,
			Nickname:  nickname,
			Biz:       decodeWechatJSString(topLevelStringProperty(block, "bizuin")),
			Username:  decodeWechatJSString(topLevelStringProperty(block, "user_name")),
		},
		Article: officialAccountArticleVariable{
			Title: decodeWechatJSString(topLevelStringProperty(block, "title")),
		},
	}
}

func extractWindowObjectBlock(source string, name string) (string, bool) {
	marker := "window." + name
	idx := strings.Index(source, marker)
	if idx < 0 {
		return "", false
	}
	rest := source[idx+len(marker):]
	assignIdx := strings.Index(rest, "=")
	if assignIdx < 0 {
		return "", false
	}
	rest = rest[assignIdx+1:]
	startRel := strings.Index(rest, "{")
	if startRel < 0 {
		return "", false
	}
	start := idx + len(marker) + assignIdx + 1 + startRel

	var quote rune
	escaped := false
	depth := 0
	for i, r := range source[start:] {
		pos := start + i
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == '{' {
			depth++
			continue
		}
		if r == '}' {
			depth--
			if depth == 0 {
				return source[start+1 : pos], true
			}
		}
	}

	return "", false
}

func topLevelStringProperty(objectBody string, key string) string {
	var quote rune
	escaped := false
	depth := 0
	for i := 0; i < len(objectBody); i++ {
		ch := rune(objectBody[i])
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		if ch == '\'' || ch == '"' {
			quote = ch
			continue
		}
		switch ch {
		case '{', '[':
			depth++
			continue
		case '}', ']':
			if depth > 0 {
				depth--
			}
			continue
		}
		if depth != 0 || !isIdentStart(ch) {
			continue
		}
		end := i + 1
		for end < len(objectBody) && isIdentPart(rune(objectBody[end])) {
			end++
		}
		if objectBody[i:end] != key {
			i = end - 1
			continue
		}
		pos := skipSpace(objectBody, end)
		if pos >= len(objectBody) || objectBody[pos] != ':' {
			i = end - 1
			continue
		}
		return readStringLikeExpression(objectBody, skipSpace(objectBody, pos+1))
	}
	return ""
}

func readStringLikeExpression(source string, start int) string {
	if start >= len(source) {
		return ""
	}
	if strings.HasPrefix(source[start:], "htmlDecode(") {
		innerStart := skipSpace(source, start+len("htmlDecode("))
		return readQuotedStringLiteral(source, innerStart)
	}
	return readQuotedStringLiteral(source, start)
}

func readQuotedStringLiteral(source string, start int) string {
	if start >= len(source) || (source[start] != '\'' && source[start] != '"') {
		return ""
	}
	quote := source[start]
	escaped := false
	for i := start + 1; i < len(source); i++ {
		if escaped {
			escaped = false
			continue
		}
		if source[i] == '\\' {
			escaped = true
			continue
		}
		if source[i] == quote {
			return source[start : i+1]
		}
	}
	return ""
}

func decodeWechatJSString(literal string) string {
	if len(literal) < 2 {
		return ""
	}
	quote := literal[0]
	if quote != '\'' && quote != '"' {
		return ""
	}
	var b strings.Builder
	for i := 1; i < len(literal)-1; i++ {
		ch := literal[i]
		if ch != '\\' || i+1 >= len(literal)-1 {
			b.WriteByte(ch)
			continue
		}
		i++
		switch literal[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '0':
			b.WriteByte(0)
		case 'x':
			if i+2 < len(literal)-1 {
				if value, ok := parseHexByte(literal[i+1 : i+3]); ok {
					b.WriteByte(value)
					i += 2
					break
				}
			}
			b.WriteByte('x')
		case '\\', '\'', '"':
			b.WriteByte(literal[i])
		default:
			b.WriteByte(literal[i])
		}
	}
	return strings.ReplaceAll(html.UnescapeString(b.String()), "\u00a0", " ")
}

func parseHexByte(s string) (byte, bool) {
	if len(s) != 2 {
		return 0, false
	}
	hi, ok := hexValue(s[0])
	if !ok {
		return 0, false
	}
	lo, ok := hexValue(s[1])
	if !ok {
		return 0, false
	}
	return hi<<4 | lo, true
}

func hexValue(ch byte) (byte, bool) {
	switch {
	case ch >= '0' && ch <= '9':
		return ch - '0', true
	case ch >= 'a' && ch <= 'f':
		return ch - 'a' + 10, true
	case ch >= 'A' && ch <= 'F':
		return ch - 'A' + 10, true
	default:
		return 0, false
	}
}

func skipSpace(source string, start int) int {
	for start < len(source) && unicode.IsSpace(rune(source[start])) {
		start++
	}
	return start
}

func isIdentStart(ch rune) bool {
	return ch == '_' || ch == '$' || unicode.IsLetter(ch)
}

func isIdentPart(ch rune) bool {
	return isIdentStart(ch) || unicode.IsDigit(ch)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
