package robomaster_diff

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

const (
	AnnouncementUrlZh = "https://www.robomaster.com/zh-CN/resource/pages/announcement/%s"
	AnnouncementUrlEn = "https://www.robomaster.com/en-US/resource/pages/announcement/%s"
)

var sumClient = &http.Client{
	Timeout: time.Second * 10,
}

var (
	qps         = getEnvInt64("QPS", 5)
	httpLimiter = rate.NewLimiter(rate.Every(time.Second/time.Duration(qps)), 1)
	ua          = getEnv("USER_AGENT", "")
	storagePath = getEnv("STORAGE_PATH", "./")
)

func getAnnouncementUrl() string {
	if os.Getenv("LANG") == "EN" {
		return AnnouncementUrlEn
	}

	return AnnouncementUrlZh
}

func Sum(ctx context.Context, id string) ([]byte, string, error) {
	url := fmt.Sprintf(getAnnouncementUrl(), id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", errors.Wrap(err, "creating request")
	}

	req.Header.Set("User-Agent", ua)

	if err = httpLimiter.Wait(ctx); err != nil {
		return nil, "", errors.Wrap(err, "waiting for rate limiter")
	}

	log.Debug().
		Str("url", url).
		Str("id", id).
		Str("user_agent", ua).
		Str("storage_path", storagePath).
		Int64("qps", qps).
		Msg("fetching url")

	resp, err := sumClient.Do(req)
	if err != nil {
		return nil, "", errors.Wrap(err, "failed doing request")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", errors.Errorf("http status code %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", errors.Wrap(err, "reading response body")
	}

	title, err := extractHtmlTitle(content)
	if err != nil {
		return nil, "", errors.Wrap(err, "extracting title")
	}

	hasher := sha256.New()
	if _, err = hasher.Write(content); err != nil {
		return nil, "", errors.Wrap(err, "writing content")
	}

	log.Debug().
		Str("url", url).
		Str("id", id).
		Str("hash", fmt.Sprintf("%x", hasher.Sum(nil))).
		Str("title", title).
		Msg("hashed content")
	return hasher.Sum(nil), title, nil
}

// Diff compares the hash of the content with the one stored in the file.
func Diff(ctx context.Context, id string, compareHash []byte) (bool, error) {
	p := fmt.Sprintf("%s/%s.sha256", storagePath, id)
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return false, errors.Wrap(err, "opening file")
	}

	defer f.Close()
	currentHash, err := io.ReadAll(f)
	if err != nil {
		return false, errors.Wrap(err, "reading file")
	}

	if bytes.Equal(currentHash, compareHash) {
		log.Debug().
			Str("path", p).
			Str("id", id).
			Str("hash", fmt.Sprintf("%x", currentHash)).
			Msg("hashes are equal")

		return false, nil
	}

	log.Debug().
		Str("path", p).
		Str("id", id).
		Str("existing_hash", fmt.Sprintf("%x", currentHash)).
		Str("compare_hash", fmt.Sprintf("%x", compareHash)).
		Msg("hashes are different, updating hash")

	if _, err = f.Seek(0, 0); err != nil {
		return false, errors.Wrap(err, "seeking file")
	}

	if _, err = f.Write(compareHash); err != nil {
		return false, errors.Wrap(err, "writing hash")
	}

	return true, nil
}
