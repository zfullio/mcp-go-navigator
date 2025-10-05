package tools

import (
	"time"

	"github.com/rs/zerolog/log"
)

func logStart(tool string, fields map[string]string) time.Time {
	e := log.Info().Str("tool", tool)
	for k, v := range fields {
		e = e.Str(k, v)
	}

	e.Msg("started")

	return time.Now()
}

func logEnd(tool string, start time.Time, count int) {
	log.Info().
		Str("tool", tool).
		Int("count", count).
		Dur("elapsed", time.Since(start)).
		Msg("completed")
}

func logError(tool string, err error, msg string) {
	log.Error().
		Err(err).
		Str("tool", tool).
		Msg(msg)
}
