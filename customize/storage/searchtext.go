package storage

import (
	"encoding/base64"
	"io"
	"mime"
	"mime/quotedprintable"
	"regexp"
	"strings"

	"github.com/mailhog/data"
	"golang.org/x/text/encoding/htmlindex"
)

var (
	htmlBlockRE = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	htmlTagRE   = regexp.MustCompile(`(?s)<[^>]*>`)
)

// messageMatchesQuery reports whether query (already lower-cased) appears in
// the visible headers or the decoded text body, including MIME parts.
func messageMatchesQuery(m *data.Message, query string) bool {
	if m == nil || query == "" {
		return false
	}
	if m.Content != nil && contentMatches(m.Content, query, true) {
		return true
	}
	if m.MIME != nil {
		for _, part := range m.MIME.Parts {
			if contentMatches(part, query, false) {
				return true
			}
		}
	}
	return false
}

func contentMatches(c *data.Content, query string, topLevel bool) bool {
	if c == nil {
		return false
	}
	if topLevel && headersMatch(c, query) {
		return true
	}
	if c.IsMIME() {
		if c.MIME != nil {
			for _, part := range c.MIME.Parts {
				if contentMatches(part, query, false) {
					return true
				}
			}
		}
		return false
	}
	if !searchableText(c) {
		return false
	}
	return strings.Contains(strings.ToLower(decodePartText(c)), query)
}

func decodedHeader(value string) string {
	if unpacked, err := new(mime.WordDecoder).DecodeHeader(value); err == nil && unpacked != "" {
		return unpacked
	}
	return value
}

func headerFieldContains(headers map[string][]string, key, query string) bool {
	if headers == nil {
		return false
	}
	for _, value := range headers[key] {
		if strings.Contains(strings.ToLower(decodedHeader(value)), query) {
			return true
		}
	}
	return false
}

func headersMatch(c *data.Content, query string) bool {
	for _, key := range []string{"Subject", "From", "To", "Cc", "Bcc"} {
		if headerFieldContains(c.Headers, key, query) {
			return true
		}
	}
	return false
}

func searchableText(c *data.Content) bool {
	media := mediaType(c)
	if media == "" || strings.HasPrefix(media, "text/") {
		return true
	}
	return false
}

func mediaType(c *data.Content) string {
	raw := headerFirst(c, "Content-Type")
	if raw == "" {
		return ""
	}
	media, _, err := mime.ParseMediaType(raw)
	if err != nil {
		if i := strings.Index(raw, ";"); i >= 0 {
			return strings.ToLower(strings.TrimSpace(raw[:i]))
		}
		return strings.ToLower(strings.TrimSpace(raw))
	}
	return strings.ToLower(media)
}

func headerFirst(c *data.Content, key string) string {
	if c == nil || c.Headers == nil {
		return ""
	}
	if values, ok := c.Headers[key]; ok && len(values) > 0 {
		return values[0]
	}
	for k, values := range c.Headers {
		if strings.EqualFold(k, key) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func decodePartText(c *data.Content) string {
	decoded := decodeTransfer(c.Body, headerFirst(c, "Content-Transfer-Encoding"))
	text := decodeCharset(decoded, charsetParam(c))
	if strings.Contains(mediaType(c), "html") {
		text = stripHTML(text)
	}
	return text
}

func decodeTransfer(body, encoding string) []byte {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "base64":
		cleaned := strings.Map(func(r rune) rune {
			switch r {
			case '\r', '\n', ' ', '\t':
				return -1
			default:
				return r
			}
		}, body)
		b, err := base64.StdEncoding.DecodeString(cleaned)
		if err != nil {
			return []byte(body)
		}
		return b
	case "quoted-printable":
		b, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
		if err != nil {
			return []byte(body)
		}
		return b
	default:
		return []byte(body)
	}
}

func charsetParam(c *data.Content) string {
	raw := headerFirst(c, "Content-Type")
	if raw == "" {
		return ""
	}
	_, params, err := mime.ParseMediaType(raw)
	if err != nil {
		return ""
	}
	return params["charset"]
}

func decodeCharset(body []byte, charset string) string {
	charset = strings.Trim(strings.ToLower(strings.TrimSpace(charset)), "\"")
	if charset == "" || charset == "utf-8" || charset == "utf8" || charset == "us-ascii" || charset == "ascii" {
		return string(body)
	}
	enc, err := htmlindex.Get(charset)
	if err != nil {
		return string(body)
	}
	decoded, err := enc.NewDecoder().Bytes(body)
	if err != nil {
		return string(body)
	}
	return string(decoded)
}

func stripHTML(s string) string {
	s = htmlBlockRE.ReplaceAllString(s, " ")
	s = htmlTagRE.ReplaceAllString(s, " ")
	replacer := strings.NewReplacer(
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
	)
	return replacer.Replace(s)
}
