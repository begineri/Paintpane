package smk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

func utc(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestDanishTimeFollowsSummerTime(t *testing.T) {
	for _, c := range []struct {
		utc, local string
	}{
		{"2026-11-08 22:00", "2026-11-08 23:00"}, // winter, CET
		{"2026-07-01 22:00", "2026-07-02 00:00"}, // summer, CEST
		{"2026-03-29 00:59", "2026-03-29 01:59"}, // last Sunday of March, just before the change
		{"2026-03-29 01:00", "2026-03-29 03:00"}, // clocks go forward
		{"2026-10-25 00:59", "2026-10-25 02:59"}, // last Sunday of October, just before the change
		{"2026-10-25 01:00", "2026-10-25 02:00"}, // clocks go back
		{"2027-03-28 01:00", "2027-03-28 03:00"},
	} {
		assert.Equal(t, c.local, danishTime(utc(c.utc)).Format("2006-01-02 15:04"), c.utc)
	}
}

func TestNightLeft(t *testing.T) {
	for _, c := range []struct {
		utc  string
		left time.Duration
	}{
		{"2026-11-08 12:00", 0},                            // Danish afternoon
		{"2026-11-08 21:59", 0},                            // 22:59 in Copenhagen
		{"2026-11-08 22:00", 8 * time.Hour},                // 23:00 → 07:00
		{"2026-11-08 23:30", 6*time.Hour + 30*time.Minute}, // 00:30
		{"2026-11-09 02:39", 3*time.Hour + 21*time.Minute}, // 03:39, when A2 phase 4 saw it down
		{"2026-11-09 05:59", time.Minute},                  // 06:59
		{"2026-11-09 06:00", 0},                            // 07:00, morning
		{"2026-07-01 21:00", 8 * time.Hour},                // summer: 23:00 CEST → 07:00 CEST
		{"2026-07-02 05:00", 0},                            // 07:00 CEST
		{"2026-07-02 04:59", time.Minute},
		{"2026-03-28 22:00", 7 * time.Hour}, // the night the clocks go forward is an hour shorter
		{"2026-10-24 21:00", 9 * time.Hour}, // and the night they go back an hour longer
		{"2026-10-25 05:59", time.Minute},   // 06:59 CET after the change
		{"2026-12-31 23:00", 7 * time.Hour}, // across the new year: 00:00 → 07:00
		{"2026-12-31 22:00", 8 * time.Hour}, // 23:00 on 31 December
		{"2026-02-28 22:30", 7*time.Hour + 30*time.Minute},
	} {
		assert.Equal(t, c.left, nightLeft(utc(c.utc)), c.utc)
	}
}

func TestProviderPausesForTheNight(t *testing.T) {
	var p provider.ImageProvider = NewProvider(nil, nil)
	op, ok := p.(provider.OutagePauseProvider)
	if assert.True(t, ok, "SMK tells the downloader about its night outage") {
		assert.Equal(t, 8*time.Hour, op.OutagePause(utc("2026-11-08 22:00")))
		assert.Zero(t, op.OutagePause(utc("2026-11-08 12:00")))
	}
}
