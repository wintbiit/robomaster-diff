package main

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/schollz/progressbar/v3"
	lib "github.com/wintbiit/robomaster-diff"
)

func main() {
	singleId := flag.Int("id", 0, "Single ID to fetch")
	batchIds := flag.String("ids", "", "Comma separated list of IDs to fetch")
	beginId := flag.Int("begin", 0, "Begin ID")
	endId := flag.Int("end", 0, "End ID")
	qps := flag.Int("qps", 5, "QPS limit")
	ua := flag.String("ua", "", "User-Agent")
	storagePath := flag.String("storage", "./", "Storage path")

	flag.Parse()

	ids := lib.NewHashSet[int]()
	if *singleId != 0 {
		ids.Add(*singleId)
	}

	if *batchIds != "" {
		for _, id := range strings.Split(*batchIds, ",") {
			i, err := strconv.Atoi(id)
			if err != nil {
				log.Error().Err(err).Str("id", id).Msg("failed to parse ID")
				continue
			}

			ids.Add(i)
		}
	}

	if *beginId != 0 && *endId != 0 && *beginId < *endId {
		for i := *beginId; i <= *endId; i++ {
			ids.Add(i)
		}
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
	diffs := make(chan *lib.DiffRecord, len(ids))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	for _, id := range ids.ToSlice() {
		go func(id int) {
			defer wg.Done()
			defer pb.Add(1)
			content, rec, err := lib.Fetch(ctx, id)
			if err != nil {
				log.Error().Err(err).Int("id", id).Msg("failed to fetch")
				return
			}

			diff, err := lib.Diff(ctx, rec, content)
			if err != nil {
				log.Error().Err(err).Int("id", id).Msg("failed to diff")
				return
			}

			if diff != nil {
				diffs <- diff
			} else {
				log.Debug().Int("id", id).Msg("no diff")
			}
		}(id)
	}

	diffRecords := make([]*lib.DiffRecord, 0)
	go func() {
		for d := range diffs {
			diffRecords = append(diffRecords, d)
		}
	}()

	wg.Wait()
	pb.Finish()
	close(diffs)

	log.Info().
		Int("total", len(ids)).
		Int("diff_count", len(diffRecords)).
		Interface("diff_records", diffRecords).
		Msg("Diff done")
}
