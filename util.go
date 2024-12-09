package robomaster_diff

import (
	"bytes"
	"fmt"
	"os"
	"strconv"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/html"
)

var DEBUG = os.Getenv("DEBUG") == "true"

func init() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if DEBUG {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	value := getEnv(key, fmt.Sprintf("%d", fallback))
	i, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}

	return i
}

func extractHtmlTitle(content []byte) (string, error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return "", err
	}

	var title string
	var findTitle func(*html.Node)
	findTitle = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			title = n.FirstChild.Data
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTitle(c)
		}
	}

	var findHead func(*html.Node)
	findHead = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "head" {
			findTitle(n)
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findHead(c)
		}
	}

	findHead(doc)
	return title, nil
}
