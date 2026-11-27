package flowengine

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // Packaged apps must resolve IANA zones without system zoneinfo.

	"github.com/robfig/cron/v3"
)

// SchedulePolicy describes a timed trigger. Intervals are in seconds; weekdays
// use 0 for Sunday. A nil policy lets callers retain their legacy cron rule.
type SchedulePolicy struct {
	Mode            string           `json:"mode"`
	Timezone        string           `json:"timezone,omitempty"`
	Interval        int              `json:"interval,omitempty"`
	Cron            string           `json:"cron,omitempty"`
	Days            []int            `json:"days,omitempty"`
	Times           []string         `json:"times,omitempty"`
	Windows         []ScheduleWindow `json:"windows,omitempty"`
	OutsideInterval int              `json:"outside_interval,omitempty"`
}

type ScheduleWindow struct {
	Days     []int  `json:"days"`
	Start    string `json:"start"`
	End      string `json:"end"`
	Interval int    `json:"interval"`
}

func schedule_interval_valid(interval int) bool { return interval >= 300 && interval <= 86400 }

func schedule_minute(value string) (int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil || len(value) != 5 {
		return 0, fmt.Errorf("时间应为 HH:mm，例如 08:30")
	}
	return parsed.Hour()*60 + parsed.Minute(), nil
}

func schedule_days_valid(days []int) bool {
	if len(days) == 0 || len(days) > 7 {
		return false
	}
	for _, day := range days {
		if day < 0 || day > 6 {
			return false
		}
	}
	return true
}

func schedule_has_day(days []int, day int) bool {
	for _, value := range days {
		if value == day {
			return true
		}
	}
	return false
}

// NextScheduleAt validates a policy and returns its next occurrence strictly
// after from. Manual policies return the zero time; missed slots are skipped.
func NextScheduleAt(policy SchedulePolicy, from time.Time) (time.Time, error) {
	if policy.Mode == "manual" {
		return time.Time{}, nil
	}
	timezone := policy.Timezone
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil || len(timezone) > 128 {
		return time.Time{}, fmt.Errorf("请输入有效的 IANA 时区，例如 Asia/Shanghai 或 UTC")
	}
	switch policy.Mode {
	case "interval":
		if !schedule_interval_valid(policy.Interval) {
			return time.Time{}, fmt.Errorf("定时间隔应为 5 分钟至 24 小时")
		}
		return from.Add(time.Duration(policy.Interval) * time.Second), nil
	case "cron":
		// The timezone belongs to the policy; embedded overrides are ambiguous.
		if strings.Contains(policy.Cron, "TZ=") || len(policy.Cron) > 128 {
			return time.Time{}, fmt.Errorf("请通过 timezone 设置 Cron 时区")
		}
		plan, err := cron.ParseStandard(policy.Cron)
		if err != nil {
			return time.Time{}, fmt.Errorf("cron 表达式无效: %w", err)
		}
		next := plan.Next(from.In(location))
		if next.IsZero() {
			return time.Time{}, fmt.Errorf("Cron 没有可执行时间")
		}
		return next, nil
	case "times":
		if policy.Days == nil {
			policy.Days = []int{0, 1, 2, 3, 4, 5, 6}
		}
		if !schedule_days_valid(policy.Days) || len(policy.Times) == 0 || len(policy.Times) > 48 {
			return time.Time{}, fmt.Errorf("请选择有效的星期及 1 至 48 个时间点")
		}
		days := make([]string, 0, len(policy.Days))
		for _, day := range policy.Days {
			days = append(days, strconv.Itoa(day))
		}
		var next time.Time
		for _, value := range policy.Times {
			minute, err := schedule_minute(value)
			if err != nil {
				return time.Time{}, err
			}
			plan, err := cron.ParseStandard(fmt.Sprintf("%d %d * * %s", minute%60, minute/60, strings.Join(days, ",")))
			if err != nil {
				return time.Time{}, err
			}
			candidate := plan.Next(from.In(location))
			if next.IsZero() || candidate.Before(next) {
				next = candidate
			}
		}
		return next, nil
	case "windows":
		if len(policy.Windows) == 0 || len(policy.Windows) > 8 || (policy.OutsideInterval != 0 && !schedule_interval_valid(policy.OutsideInterval)) {
			return time.Time{}, fmt.Errorf("请设置 1 至 8 个时段；时段外间隔为 0 或 300 至 86400 秒")
		}
		ranges := [][2]int{}
		for _, period := range policy.Windows {
			start, start_err := schedule_minute(period.Start)
			end, end_err := schedule_minute(period.End)
			if start_err != nil || end_err != nil || start == end || !schedule_days_valid(period.Days) || !schedule_interval_valid(period.Interval) {
				return time.Time{}, fmt.Errorf("时段需包含有效星期、不同的 HH:mm 起止时间和 300 至 86400 秒间隔")
			}
			for _, day := range period.Days {
				left := day*1440 + start
				right := left + (end-start+1440)%1440
				if right > 10080 {
					ranges = append(ranges, [2]int{0, right - 10080})
					right = 10080
				}
				ranges = append(ranges, [2]int{left, right})
			}
		}
		sort.Slice(ranges, func(left, right int) bool { return ranges[left][0] < ranges[right][0] })
		for index := 1; index < len(ranges); index++ {
			if ranges[index][0] < ranges[index-1][1] {
				return time.Time{}, fmt.Errorf("定时时段不能重叠")
			}
		}
		previous := schedule_window_at(policy, from.In(location))
		delay := policy.OutsideInterval
		if previous >= 0 {
			delay = policy.Windows[previous].Interval
		}
		var target time.Time
		if delay > 0 {
			target = from.Add(time.Duration(delay) * time.Second)
		}
		for candidate, end := from.Truncate(time.Minute).Add(time.Minute), from.Add(9*24*time.Hour); !candidate.After(end); candidate = candidate.Add(time.Minute) {
			if !target.IsZero() && target.Before(candidate) {
				return target, nil
			}
			current := schedule_window_at(policy, candidate.In(location))
			if current != previous {
				if current >= 0 {
					return candidate, nil
				}
				target = time.Time{}
				if policy.OutsideInterval > 0 {
					target = candidate.Add(time.Duration(policy.OutsideInterval) * time.Second)
				}
				previous = current
			}
			if !target.IsZero() && !target.After(candidate) {
				return target, nil
			}
		}
	default:
		return time.Time{}, fmt.Errorf("未知的定时方式: %s", policy.Mode)
	}
	return time.Time{}, fmt.Errorf("定时策略没有可执行时间")
}

func schedule_window_at(policy SchedulePolicy, at time.Time) int {
	minute, day := at.Hour()*60+at.Minute(), int(at.Weekday())
	for index, period := range policy.Windows {
		start, _ := schedule_minute(period.Start)
		end, _ := schedule_minute(period.End)
		if start < end && schedule_has_day(period.Days, day) && minute >= start && minute < end {
			return index
		}
		if start > end && ((schedule_has_day(period.Days, day) && minute >= start) || (schedule_has_day(period.Days, (day+6)%7) && minute < end)) {
			return index
		}
	}
	return -1
}
