package source

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var tag_pattern = regexp.MustCompile(`<[^>]*>`)

// JSON IDs must retain all digits, including when the server sends a number.
type TextValue string

func (value *TextValue) UnmarshalJSON(raw []byte) error {
	if string(raw) == "null" {
		*value = ""
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		*value = TextValue(text)
		return nil
	}
	var number json.Number
	if err := json.Unmarshal(raw, &number); err != nil {
		return err
	}
	*value = TextValue(number.String())
	return nil
}

func CleanText(value string) string {
	return strings.Join(strings.Fields(html.UnescapeString(tag_pattern.ReplaceAllString(value, ""))), " ")
}

func FirstText(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func RandomUUID() string {
	var data [16]byte
	_, _ = rand.Read(data[:])
	data[6], data[8] = (data[6]&15)|64, (data[8]&63)|128
	return fmt.Sprintf("%x-%x-%x-%x-%x", data[:4], data[4:6], data[6:8], data[8:10], data[10:])
}
func Numeric(value TextValue) float64 {
	number, err := strconv.ParseFloat(string(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0
	}
	return number
}
func DecodeBase64(value string) ([]byte, error) {
	value = strings.NewReplacer("-", "+", "_", "/").Replace(strings.TrimSpace(value))
	return base64.RawStdEncoding.DecodeString(strings.TrimRight(value, "="))
}
func CookieValue(cookie, name string) string {
	req := &http.Request{Header: http.Header{"Cookie": []string{cookie}}}
	value, err := req.Cookie(name)
	if err != nil {
		return ""
	}
	return value.Value
}
func (client *RequestClient) GetText(ctx context.Context, address string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	response, err := client.Response(ctx, address, true)
	if err != nil {
		return "", err
	}
	return ReadVideoResponse(response)
}
