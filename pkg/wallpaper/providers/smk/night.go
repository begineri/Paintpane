package smk

import "time"

// SMK's image servers (iip.smk.dk and iip-thumb.smk.dk) stop answering altogether for hours during
// the Danish night, while its API keeps working (docs/smk-download.md §3.4 and §6.2). Nothing on
// our side helps then. So when an SMK image fails to download during the Danish night, SMK's
// downloads pause until the morning (provider.OutagePauseProvider); other sources go on as usual.
// A night when the server works is not affected: nothing pauses until an image actually fails.
const (
	// nightStart and nightEnd bound the Danish night, in Danish local time.
	nightStart = 23 * time.Hour
	nightEnd   = 7 * time.Hour
)

var (
	cet  = time.FixedZone("CET", 1*60*60)
	cest = time.FixedZone("CEST", 2*60*60)
)

// danishTime returns t in Danish local time: CET (UTC+1), or CEST (UTC+2) from 01:00 UTC on the
// last Sunday of March to 01:00 UTC on the last Sunday of October (EU summer time). A fixed rule
// instead of time.LoadLocation, because Windows has no tz database and embedding one adds 450 KB.
func danishTime(t time.Time) time.Time {
	u := t.UTC()
	start := lastSunday(u.Year(), time.March).Add(time.Hour)
	end := lastSunday(u.Year(), time.October).Add(time.Hour)
	if !u.Before(start) && u.Before(end) {
		return u.In(cest)
	}
	return u.In(cet)
}

// lastSunday returns 00:00 UTC on the last Sunday of month m in year y.
func lastSunday(y int, m time.Month) time.Time {
	last := time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC)
	return last.AddDate(0, 0, -int(last.Weekday()))
}

// nightLeft returns, during the Danish night, how long it still lasts; otherwise 0.
func nightLeft(now time.Time) time.Duration {
	local := danishTime(now)
	clock := time.Duration(local.Hour())*time.Hour + time.Duration(local.Minute())*time.Minute +
		time.Duration(local.Second())*time.Second + time.Duration(local.Nanosecond())
	if clock < nightStart && clock >= nightEnd {
		return 0
	}
	day := local.Day()
	if clock >= nightStart {
		day++
	}
	// nightEnd on that day, in whichever offset applies then (the clocks change at 02:00 or 03:00,
	// before the night ends).
	end := time.Date(local.Year(), local.Month(), day, 0, 0, 0, 0, cet).Add(nightEnd)
	if danishTime(end).Location() == cest {
		end = end.Add(-time.Hour)
	}
	return end.Sub(now)
}

// OutagePause implements provider.OutagePauseProvider: during the Danish night a failed image
// pauses SMK's downloads until the night ends.
func (p *Provider) OutagePause(now time.Time) time.Duration {
	return nightLeft(now)
}
