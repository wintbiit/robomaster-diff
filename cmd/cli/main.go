package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	lib "github.com/wintbiit/robomaster-diff"
)

type diffRecord struct {
	id    string
	title string
}

func (d *diffRecord) String() string {
	return fmt.Sprintf("[%s] %s", d.id, d.title)
}

func main() {
	singleId := flag.String("id", "", "Single ID to fetch")
	batchIds := flag.String("ids", "", "Comma separated list of IDs to fetch")
	qps := flag.Int("qps", 5, "QPS limit")
	ua := flag.String("ua", "", "User-Agent")
	storagePath := flag.String("storage", "./", "Storage path")

	flag.Parse()

	ids := make([]string, 0)
	if *singleId != "" {
		ids = append(ids, *singleId)
	}

	if *batchIds != "" {
		ids = append(ids, strings.Split(*batchIds, ",")...)
	}

	if len(ids) == 0 {
		flag.PrintDefaults()
		return
	}

	os.Setenv("QPS", strconv.Itoa(*qps))
	os.Setenv("USER_AGENT", *ua)
	os.Setenv("STORAGE_PATH", *storagePath)

	var wg sync.WaitGroup
	pb := progressbar.NewOptions(len(ids), progressbar.OptionSetWriter(log.Logger))
	wg.Add(len(ids))
	diffs := make(chan diffRecord, len(ids))
	for _, id := range ids {
		go func(id string) {
			defer wg.Done()
			defer pb.Add(1)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()
			hash, title, err := lib.Sum(ctx, id)
			if err != nil {
				log.Error().Err(err).Str("id", id).Msg("failed to fetch")
				return
			}

			diff, err := lib.Diff(ctx, id, hash)
			if err != nil {
				log.Error().Err(err).Str("id", id).Msg("failed to diff")
				return
			}

			if diff {
				diffs <- diffRecord{id: id, title: title}
			}
		}(id)
	}

	diffRecords := make([]fmt.Stringer, 0)
	go func() {
		for d := range diffs {
			diffRecords = append(diffRecords, &d)
		}
	}()

	wg.Wait()
	pb.Finish()
	close(diffs)

	log.Info().
		Int("total", len(ids)).
		Int("diff_count", len(diffRecords)).
		Stringers("diffs", diffRecords).
		Msg("Diff done")
}
