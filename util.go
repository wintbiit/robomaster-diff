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

type HashSet[T comparable] map[T]struct{}

func NewHashSet[T comparable]() HashSet[T] {
	return make(HashSet[T])
}

func (s HashSet[T]) Add(v T) {
	s[v] = struct{}{}
}

func (s HashSet[T]) AddAll(values ...T) {
	for _, v := range values {
		s.Add(v)
	}
}

func (s HashSet[T]) Contains(v T) bool {
	_, ok := s[v]
	return ok
}

func (s HashSet[T]) Remove(v T) {
	delete(s, v)
}

func (s HashSet[T]) ToSlice() []T {
	result := make([]T, 0, len(s))
	for v := range s {
		result = append(result, v)
	}

	return result
}
