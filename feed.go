package main

import (
	"html"
	"regexp"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
)

type Feed struct {
	URL      string
	Title    string
	Articles []Article
}

type Article struct {
	Title     string
	Link      string
	Content   string
	Published string
	GUID      string
	Read      bool
}

func fetchFeed(url string) (*Feed, error) {
	fp := gofeed.NewParser()
	parsed, err := fp.ParseURL(url)
	if err != nil {
		return nil, err
	}

	f := &Feed{
		URL:   url,
		Title: parsed.Title,
	}

	for _, item := range parsed.Items {
		content := item.Content
		if content == "" {
			content = item.Description
		}
		content = cleanHTML(content)

		pub := time.Now().Format("Jan 2, 2006")
		if item.PublishedParsed != nil {
			pub = item.PublishedParsed.Format("Jan 2, 2006")
		} else if item.UpdatedParsed != nil {
			pub = item.UpdatedParsed.Format("Jan 2, 2006")
		}

		guid := item.GUID
		if guid == "" {
			guid = item.Link
		}

		f.Articles = append(f.Articles, Article{
			Title:     strings.TrimSpace(item.Title),
			Link:      item.Link,
			Content:   content,
			Published: pub,
			GUID:      guid,
		})
	}

	return f, nil
}

var (
	blockTagRe = regexp.MustCompile(`(?i)<(br|p|div|h[1-6]|li|tr|blockquote)[^>]*>`)
	htmlTagRe  = regexp.MustCompile(`<[^>]+>`)
)

func cleanHTML(s string) string {
	s = blockTagRe.ReplaceAllString(s, "\n")
	s = htmlTagRe.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			lines = append(lines, t)
		}
	}
	return strings.Join(lines, "\n")
}
