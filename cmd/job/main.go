package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	lib "github.com/wintbiit/robomaster-diff"
)

func main() {
	// find git executable
	if _, err := exec.LookPath("git"); err != nil {
		log.Error().Err(err).Msg("git not found")
		return
	}

	beginId := getEnvInt("BEGIN_ID", 0)
	endId := getEnvInt("END_ID", 0)

	if beginId >= endId {
		log.Error().Msg("invalid range, please define BEGIN_ID and END_ID")
	}

	ids := make([]int, endId-beginId+1)
	for i := 0; i < len(ids); i++ {
		ids[i] = beginId + i
	}

	log.Info().Int("begin_id", beginId).Int("end_id", endId).Msg("fetching IDs")

	var wg sync.WaitGroup
	wg.Add(len(ids))
	diffs := make(chan *lib.DiffRecord, len(ids))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	for _, id := range ids {
		go func(id int) {
			defer wg.Done()
			hash, rec, err := lib.Sum(ctx, id)
			if err != nil {
				log.Error().Err(err).Int("id", id).Msg("failed to fetch")
				return
			}

			diff, err := lib.Diff(ctx, id, hash)
			if err != nil {
				log.Error().Err(err).Int("id", id).Msg("failed to diff")
				return
			}

			if diff {
				diffs <- rec
			}
		}(id)
	}

	diffRecords := make([]fmt.Stringer, 0)
	go func() {
		for d := range diffs {
			diffRecords = append(diffRecords, d)
		}
	}()

	wg.Wait()
	close(diffs)

	log.Info().
		Int("total", len(ids)).
		Int("diff_count", len(diffRecords)).
		Stringers("diffs", diffRecords).
		Msg("Diff done")

	if len(diffRecords) == 0 {
		return
	}

	dryRun := os.Getenv("DRY_RUN") == "true"

	log.Info().Msg("running git add")
	cmd := exec.Command("git", "add", lib.GetStoragePath())
	cmd.Stdout = log.Logger
	cmd.Stderr = log.Logger
	log.Debug().Str("cmd", cmd.String()).Msg("add command")
	if !dryRun {
		if err := cmd.Run(); err != nil {
			log.Error().Err(err).Msg("failed to run git add")
			return
		}
	}

	log.Info().Msg("running git commit")
	commitTitle := fmt.Sprintf("diff %d records", len(diffRecords))
	commitMessage := strings.Builder{}
	for _, rec := range diffRecords {
		commitMessage.WriteString(rec.String())
		commitMessage.WriteString("\n")
	}

	cmd = exec.Command("git", "commit", "-m", commitTitle, "-m", commitMessage.String())
	cmd.Stdout = log.Logger
	cmd.Stderr = log.Logger
	log.Debug().Str("cmd", cmd.String()).Msg("commit command")
	if !dryRun {
		if err := cmd.Run(); err != nil {
			log.Error().Err(err).Msg("failed to run git commit")
			return
		}
	}
}

func getEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(value)
		if err != nil {
			return fallback
		}
		return i
	}

	return fallback
}
