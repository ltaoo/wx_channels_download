package douban

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
)

var (
	infoBlockRE  = regexp.MustCompile(`<div\s+id="info"[^>]*>([\s\S]*?)</div>`)
	brRE         = regexp.MustCompile(`<br\s*/?>`)
	stripTagRE   = regexp.MustCompile(`<[^>]+>`)
	dateRE       = regexp.MustCompile(`([0-9]{4}-[0-9]{2}-[0-9]{2})`)
	numberRE     = regexp.MustCompile(`([0-9]+)`)
	genreRE      = regexp.MustCompile(`<span property="v:genre">([^<]+)</span>`)
	imdbRE       = regexp.MustCompile(`IMDb:</span>\s*(tt[0-9]+)`)
	personLinkRE = regexp.MustCompile(`<a href="https://www\.douban\.com/(?:personage|celebrity)/([0-9]+)/"[^>]*>([^<]+)</a>`)
	reviewedRE   = regexp.MustCompile(`property="v:itemreviewed">([^<]+)<`)
	summaryRE    = regexp.MustCompile(`<span property="v:summary"[^>]*>([\s\S]*?)</span>`)
	introRE      = regexp.MustCompile(`<div class="intro">([\s\S]*?)</div>`)
	ratingRE     = regexp.MustCompile(`v:average[^>]*>([^<]+)<`)
	vImageTagRE  = regexp.MustCompile(`<img\b[^>]*\brel=["']v:(?:image|photo)["'][^>]*>`)
	seasonNameRE = regexp.MustCompile(`第[一二三四五六七八九十0-9]+季`)
)

type SplitNameResult struct {
	Name         string
	OriginalName string
}

func ParseProfilePageHTML(htmlText string) (*MediaProfile, error) {
	fields := infoBlockRE.FindStringSubmatch(htmlText)
	if len(fields) < 2 {
		return nil, fmt.Errorf("missing profile info block")
	}
	profile := &MediaProfile{
		Type: string(MediaTypeTV),
	}
	if isBookSubjectHTML(htmlText) {
		profile.Type = string(MediaTypeBook)
	}
	lines := brRE.Split(fields[1], -1)
	for _, line := range lines {
		switch {
		case strings.Contains(line, "集数"):
			if match := numberRE.FindStringSubmatch(line); len(match) > 1 {
				profile.SourceCount, _ = strconv.Atoi(match[1])
			}
		case strings.Contains(line, "首播") || strings.Contains(line, "上映日期"):
			if strings.Contains(line, "上映日期") {
				profile.Type = string(MediaTypeMovie)
			}
			if match := dateRE.FindStringSubmatch(line); len(match) > 1 {
				profile.AirDate = match[1]
			}
		case strings.Contains(line, "出版年"):
			if profile.Type == "" || profile.Type == string(MediaTypeTV) {
				profile.Type = string(MediaTypeBook)
			}
			profile.AirDate = valueAfterSpan(line)
		case strings.Contains(line, "又名"):
			profile.Alias = valueAfterSpan(line)
		case strings.Contains(line, "制片国家"):
			profile.OriginCountry = valueAfterSpan(line)
		case strings.Contains(line, "类型"):
			matches := genreRE.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 1 {
					text := strings.TrimSpace(match[1])
					if value := genreTextToValue[text]; value != 0 {
						profile.Genres = append(profile.Genres, Genre{ID: value, Text: text})
					}
				}
			}
		case strings.Contains(line, "IMDb"):
			if match := imdbRE.FindStringSubmatch(line); len(match) > 1 {
				profile.IMDB = strings.TrimSpace(match[1])
			}
		case strings.Contains(line, "主演"):
			profile.Actors = parsePersons(line)
		case strings.Contains(line, "导演"):
			profile.Director = parsePersons(line)
		case strings.Contains(line, "编剧"):
			profile.Author = parsePersons(line)
		}
	}
	if match := reviewedRE.FindStringSubmatch(htmlText); len(match) > 1 {
		names := SplitNameAndOriginalName(CleanName(match[1]))
		profile.Name = names.Name
		profile.OriginalName = names.OriginalName
	}
	if match := vImageTagRE.FindString(htmlText); match != "" {
		profile.CoverURL = attrValue(match, "src")
		profile.PosterPath = profile.CoverURL
	}
	if match := summaryRE.FindStringSubmatch(htmlText); len(match) > 1 {
		overview := regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(match[1], "\n")
		profile.Overview = strings.TrimSpace(stripTags(overview))
	}
	if profile.Overview == "" {
		for _, match := range introRE.FindAllStringSubmatch(htmlText, -1) {
			if len(match) > 1 {
				overview := strings.TrimSpace(stripTags(match[1]))
				if overview != "" && !strings.Contains(overview, "展开全部") {
					profile.Overview = overview
					break
				}
			}
		}
	}
	if match := ratingRE.FindStringSubmatch(htmlText); len(match) > 1 {
		profile.VoteAverage, _ = strconv.ParseFloat(strings.TrimSpace(match[1]), 64)
	}
	return profile, nil
}

func CleanName(name string) string {
	name = strings.ReplaceAll(name, "\u200e", "")
	name = strings.ReplaceAll(name, "&lrm;", "")
	name = strings.ReplaceAll(name, "&#x200e;", "")
	return strings.ReplaceAll(name, "  ", " ")
}

func SplitNameAndOriginalName(name string) SplitNameResult {
	parts := strings.Split(name, " ")
	if len(parts) == 1 {
		return SplitNameResult{Name: parts[0]}
	}
	first := parts[0]
	second := parts[1]
	rest := parts[2:]
	if seasonNameRE.MatchString(second) {
		original := ""
		if len(rest) > 0 {
			original = strings.Join(rest, " ")
		}
		return SplitNameResult{
			Name:         strings.Join([]string{first, second}, " "),
			OriginalName: original,
		}
	}
	return SplitNameResult{
		Name:         first,
		OriginalName: strings.Join(parts[1:], " "),
	}
}

func MatchExactMedia(media MatchMedia, list []SearchItem) (*SearchItem, error) {
	if len(list) == 0 {
		return nil, fmt.Errorf("empty search result")
	}
	namesProcessed := []string{media.Name, strings.ReplaceAll(media.Name, "：", "·")}
	var candidates []string
	switch media.Type {
	case MediaTypeMovie:
		candidates = appendNameCandidates(namesProcessed, []string{media.OriginalName})
	case MediaTypeSeason:
		seasonTexts := []string{"", strconv.Itoa(media.Order), " 第" + strconv.Itoa(media.Order) + "季", " 第" + numToChinese(media.Order) + "季", " Season " + strconv.Itoa(media.Order)}
		var chineseNames []string
		for _, suffix := range seasonTexts {
			for _, name := range namesProcessed {
				chineseNames = append(chineseNames, strings.TrimSpace(name+suffix))
			}
		}
		var originalNames []string
		if media.OriginalName != "" {
			for _, suffix := range seasonTexts {
				originalNames = append(originalNames, strings.TrimSpace(media.OriginalName+suffix))
			}
		}
		candidates = appendNameCandidates(chineseNames, originalNames)
	default:
		return nil, fmt.Errorf("unsupported media type %q", media.Type)
	}
	candidates = uniqStrings(candidates)
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		for i := range list {
			if candidate != list[i].Name {
				continue
			}
			if media.AirDate == "" || year(media.AirDate) == year(list[i].AirDate) {
				return &list[i], nil
			}
		}
	}
	return nil, fmt.Errorf("no exact match")
}

func appendNameCandidates(chineseNames []string, originalNames []string) []string {
	out := append([]string{}, chineseNames...)
	out = append(out, originalNames...)
	for _, c := range chineseNames {
		for _, o := range originalNames {
			if c != "" && o != "" {
				out = append(out, c+" "+o)
			}
		}
	}
	return out
}

func parsePersons(line string) []Person {
	matches := personLinkRE.FindAllStringSubmatch(line, -1)
	persons := make([]Person, 0, len(matches))
	for i, match := range matches {
		if len(match) > 2 {
			persons = append(persons, Person{
				ID:    match[1],
				Name:  html.UnescapeString(match[2]),
				Order: i + 1,
			})
		}
	}
	return persons
}

func valueAfterSpan(line string) string {
	parts := strings.SplitN(line, "/span>", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(stripTags(parts[1]))
}

func stripTags(value string) string {
	return strings.TrimSpace(html.UnescapeString(stripTagRE.ReplaceAllString(value, "")))
}

func attrValue(tag string, name string) string {
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(name) + `\s*=\s*("[^"]*"|'[^']*'|[^\s>]+)`)
	match := re.FindStringSubmatch(tag)
	if len(match) < 2 {
		return ""
	}
	value := strings.Trim(match[1], `"'`)
	return strings.TrimSpace(html.UnescapeString(value))
}

func isBookSubjectHTML(htmlText string) bool {
	return strings.Contains(htmlText, `id="db-nav-book"`) ||
		strings.Contains(htmlText, `book.douban.com/subject`) ||
		strings.Contains(htmlText, `<span class="pl">出版社:</span>`) ||
		strings.Contains(htmlText, `<span class="pl"> 作者</span>`)
}

func splitAndTrim(value string, sep string) []string {
	parts := strings.Split(value, sep)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func idString(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func year(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 4 {
		return value[:4]
	}
	return value
}

func uniqStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func numToChinese(num int) string {
	values := map[int]string{
		1: "一", 2: "二", 3: "三", 4: "四", 5: "五", 6: "六", 7: "七", 8: "八", 9: "九", 10: "十",
		11: "十一", 12: "十二", 13: "十三", 14: "十四", 15: "十五", 16: "十六", 17: "十七", 18: "十八", 19: "十九", 20: "二十",
		21: "二十一", 22: "二十二", 23: "二十三", 24: "二十四", 25: "二十五", 26: "二十六", 27: "二十七", 28: "二十八", 29: "二十九", 30: "三十",
	}
	if value := values[num]; value != "" {
		return value
	}
	return strconv.Itoa(num)
}
