package robomaster_diff

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

type DiffType string

const (
	AnnouncementUrlZh          = "https://www.robomaster.com/zh-CN/resource/pages/announcement/%d"
	AnnouncementUrlEn          = "https://www.robomaster.com/en-US/resource/pages/announcement/%d"
	DiffTypeAdd       DiffType = "add"
	DiffTypeChange    DiffType = "change"
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

func init() {
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"
	}

	os.MkdirAll(storagePath, 0o755)
}

func GetAnnouncementUrl(id int) string {
	if os.Getenv("LANG") == "EN" {
		return fmt.Sprintf(AnnouncementUrlEn, id)
	}

	return fmt.Sprintf(AnnouncementUrlZh, id)
}

func GetStoragePath() string {
	return storagePath
}

type ItemInfo struct {
	Id    int    `json:"id"`
	Title string `json:"title"`
	Url   string `json:"url"`
}

type DiffDetail struct {
	Operation string `json:"operation"`
	Content   string `json:"content"`
}

type DiffRecord struct {
	*ItemInfo
	DiffType    DiffType     `json:"diff_type"`
	DiffDetails []DiffDetail `json:"diff_details"`
}

func (d *DiffRecord) String() string {
	return fmt.Sprintf("[%s] %s", d.Title, GetAnnouncementUrl(d.Id))
}

func (d *DiffRecord) RichString() string {
	j, err := json.Marshal(d)
	if err != nil {
		panic(err)
	}

	return string(j)
}

func Fetch(ctx context.Context, id int) ([]byte, *ItemInfo, error) {
	url := GetAnnouncementUrl(id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil, errors.Wrap(err, "creating request")
	}

	req.Header.Set("User-Agent", ua)

	if err = httpLimiter.Wait(ctx); err != nil {
		return nil, nil, errors.Wrap(err, "waiting for rate limiter")
	}

	log.Debug().
		Str("url", url).
		Int("id", id).
		Str("user_agent", ua).
		Str("storage_path", storagePath).
		Int64("qps", qps).
		Msg("fetching url")

	resp, err := sumClient.Do(req)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed doing request")
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, errors.Errorf("http status code %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, errors.Wrap(err, "reading response body")
	}

	title, err := extractHtmlTitle(content)
	if err != nil {
		return nil, nil, errors.Wrap(err, "extracting title")
	}

	log.Debug().
		Str("url", url).
		Int("id", id).
		Str("title", title).
		Msg("got content")
	return content, &ItemInfo{
		Id:    id,
		Title: title,
		Url:   url,
	}, nil
}

// Diff compares the hash of the content with the one stored in the file.
func Diff(ctx context.Context, item *ItemInfo, compare []byte) (*DiffRecord, error) {
	p := fmt.Sprintf("%s/%d.html", storagePath, item.Id)
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, errors.Wrap(err, "opening file")
	}

	defer f.Close()
	current, err := io.ReadAll(f)
	if err != nil {
		return nil, errors.Wrap(err, "reading file")
	}

	rec := &DiffRecord{
		ItemInfo: item,
	}

	defer func() {
		if rec.DiffType != DiffTypeChange {
			return
		}

		log.Info().Str("path", p).Int("id", item.Id).Msg("updating file")

		if _, err = f.Seek(0, 0); err != nil {
			log.Error().Err(err).Int("id", item.Id).Msg("failed to seek file")
		}

		if _, err = f.Write(compare); err != nil {
			log.Error().Err(err).Int("id", item.Id).Msg("failed to write file")
		}
	}()

	if len(current) == 0 {
		rec.DiffType = DiffTypeAdd
		return rec, nil
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(string(current), string(compare), false)
	if len(diffs) == 1 && diffs[0].Type == diffmatchpatch.DiffEqual {
		return nil, nil
	}

	rec.DiffType = DiffTypeChange
	details := make([]DiffDetail, 0)
	for _, diff := range diffs {
		if diff.Type == diffmatchpatch.DiffEqual {
			continue
		}

		var d DiffDetail
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			d.Operation = "+"
		case diffmatchpatch.DiffDelete:
			d.Operation = "-"
		}

		d.Content = diff.Text
		details = append(details, d)
	}

	rec.DiffDetails = details
	return rec, nil
}
