package main

import "time"

func msSinceMidnight(t time.Time) uint32 {
	seconds := t.Unix() % (24 * 60 * 60)
	return uint32(seconds*1000 + int64(t.Nanosecond()/1e6))
}
