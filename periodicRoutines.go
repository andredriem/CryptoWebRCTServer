package main

import "time"

// scheduleHalfPastHour will call fn() once at the next :30 mark, then once every hour thereafter.
func ScheduleHalfPastHour(fn func(*RedisClientConnectionCache), redisCleintCache *RedisClientConnectionCache) {
    // figure out when the next XX:30:00 is
    now := time.Now()
    // construct the :30 of the current hour
    next := time.Date(now.Year(), now.Month(), now.Day(),
        now.Hour(), 30, 0, 0, now.Location(),
    )
    // if we're already past :30, bump to the next hour's :30
    if now.After(next) {
        next = next.Add(time.Hour)
    }
    // sleep until that moment
    time.Sleep(time.Until(next))

    // fire once immediately at the half‑past mark
    fn(redisCleintCache)

    // then tick every hour thereafter
    ticker := time.NewTicker(time.Hour)
    defer ticker.Stop()
    for range ticker.C {
        fn(redisCleintCache)
    }
}