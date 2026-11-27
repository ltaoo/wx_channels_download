package flowengine

import (
	"testing"
	"time"
)

func TestNextScheduleAt(t *testing.T) {
	parse := func(value string) time.Time {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	for _, test_case := range []struct {
		name   string
		policy SchedulePolicy
		from   string
		want   string
	}{
		{"interval keeps seconds", SchedulePolicy{Mode: "interval", Interval: 300}, "2026-11-27T00:00:35Z", "2026-11-27T00:05:35Z"},
		{"cron timezone", SchedulePolicy{Mode: "cron", Cron: "0 8 * * 1-5", Timezone: "Asia/Shanghai"}, "2026-11-27T00:00:00Z", "2026-11-30T00:00:00Z"},
		{"cron descriptor", SchedulePolicy{Mode: "cron", Cron: "@daily", Timezone: "UTC"}, "2026-11-27T00:00:00Z", "2026-11-28T00:00:00Z"},
		{"daily times", SchedulePolicy{Mode: "times", Times: []string{"08:00", "12:30"}}, "2026-11-27T00:00:00Z", "2026-11-27T04:30:00Z"},
		{"weekday skips missed slots", SchedulePolicy{Mode: "times", Days: []int{1}, Times: []string{"08:00"}}, "2026-11-27T00:00:00Z", "2026-11-30T00:00:00Z"},
		{"DST missing time skipped", SchedulePolicy{Mode: "times", Timezone: "America/New_York", Times: []string{"02:30"}}, "2026-03-08T06:59:00Z", "2026-03-09T06:30:00Z"},
		{"manual", SchedulePolicy{Mode: "manual"}, "2026-11-27T00:00:00Z", ""},
	} {
		t.Run(test_case.name, func(t *testing.T) {
			next, err := NextScheduleAt(test_case.policy, parse(test_case.from))
			if err != nil {
				t.Fatal(err)
			}
			if test_case.want == "" {
				if !next.IsZero() {
					t.Fatalf("manual returned %v", next)
				}
			} else if !next.Equal(parse(test_case.want)) {
				t.Fatalf("got %v, want %s", next, test_case.want)
			}
		})
	}
}

func TestScheduleWindows(t *testing.T) {
	policy := SchedulePolicy{Mode: "windows", Timezone: "UTC", Windows: []ScheduleWindow{
		{Days: []int{6}, Start: "23:00", End: "01:00", Interval: 600},
		{Days: []int{0}, Start: "01:00", End: "02:00", Interval: 1800},
	}}
	for _, test_case := range []struct{ from, want string }{
		{"2026-11-28T22:59:35Z", "2026-11-28T23:00:00Z"},
		{"2026-11-28T23:55:35Z", "2026-11-29T00:05:35Z"},
		{"2026-11-29T00:55:35Z", "2026-11-29T01:00:00Z"},
		{"2026-11-29T01:55:35Z", "2026-12-05T23:00:00Z"},
	} {
		from, _ := time.Parse(time.RFC3339, test_case.from)
		want, _ := time.Parse(time.RFC3339, test_case.want)
		next, err := NextScheduleAt(policy, from)
		if err != nil || !next.Equal(want) {
			t.Fatalf("from %s: got %v, %v; want %v", test_case.from, next, err, want)
		}
	}
	policy.OutsideInterval = 3600
	from, _ := time.Parse(time.RFC3339, "2026-11-29T01:55:35Z")
	next, err := NextScheduleAt(policy, from)
	if err != nil || next.Format(time.RFC3339) != "2026-11-29T03:00:00Z" {
		t.Fatalf("outside boundary: %v, %v", next, err)
	}
}

func TestScheduleValidation(t *testing.T) {
	for _, policy := range []SchedulePolicy{
		{Mode: "unknown"},
		{Mode: "interval", Interval: 299},
		{Mode: "interval", Interval: 86401},
		{Mode: "interval", Interval: 300, Timezone: "Bad/Zone"},
		{Mode: "cron", Cron: "0 0 31 2 *"},
		{Mode: "cron", Cron: "TZ=UTC 0 0 * * *"},
		{Mode: "times", Times: []string{"24:00"}},
		{Mode: "times", Days: []int{}, Times: []string{"08:00"}},
		{Mode: "times", Days: []int{7}, Times: []string{"08:00"}},
		{Mode: "windows", Windows: []ScheduleWindow{{Days: []int{1}, Start: "08:00", End: "08:00", Interval: 600}}},
		{Mode: "windows", Windows: []ScheduleWindow{{Days: []int{6}, Start: "23:00", End: "01:00", Interval: 600}, {Days: []int{0}, Start: "00:30", End: "02:00", Interval: 600}}},
	} {
		if _, err := NextScheduleAt(policy, time.Now()); err == nil {
			t.Fatalf("accepted invalid policy: %+v", policy)
		}
	}
}
