# DatePicker / DateRangePicker / TimePicker

源文件：`packages/shadcn/src/date-picker.ts`, `date-range-picker.ts`, `time-picker.ts`
Core：`packages/ui/src/date-picker/index.ts`, `date-range-picker/index.ts`, `time-picker/index.ts`

三者均为工厂函数（非 class），内部组合 PresenceCore + PopperCore + CalendarCore。

## DatePicker

```js
const dp$ = Timeless.ui.DatePickerCore({ today: new Date() });

DatePicker({ store: dp$ });

dp$.setValue(new Date(2024, 0, 15));
dp$.onChange((date) => { console.log(date); });
```

### API

```ts
DatePickerCore({ today: Date })

dp$.state → { date: string, value: Date | null }  // date 格式 "YYYY/MM/DD"
dp$.setValue(v: Date)
dp$.value → Date | null
dp$.onChange(fn)
dp$.onStateChange(fn)
// 内部子 core：dp$.$presence, dp$.$popper, dp$.$calendar, dp$.$btn
```

## DateRangePicker

```js
const drp$ = Timeless.ui.DateRangePickerCore({ today: new Date() });

DateRangePicker({ store: drp$ });

drp$.setValue(new Date(2024, 0, 1), new Date(2024, 0, 31));
drp$.clear();
```

### API

```ts
DateRangePickerCore({ today: Date })

drp$.state → { dateText: string, value: { start: Date, end: Date } | null }
drp$.setValue(start, end)
drp$.clear()
drp$.onChange(fn) / drp$.onStateChange(fn)
```

## TimePicker

```js
const tp$ = Timeless.ui.TimePickerCore({
  showSeconds: false,
  hourStep: 1,
  minuteStep: 5,
});

TimePicker({ store: tp$ });

tp$.setValue({ hour: 14, minute: 30 });
```

### API

```ts
TimePickerCore({
  defaultValue?: { hour, minute, second? },
  showSeconds?: boolean, use12Hours?: boolean,
  hourStep?, minuteStep?, secondStep?,
})

tp$.state → { time: string | null, value: TimeValue | null, showSeconds, use12Hours, tempHour, tempMinute, tempSecond }
tp$.selectHour(h) / tp$.selectMinute(m) / tp$.selectSecond(s)
tp$.confirm() / tp$.clear()
tp$.setValue(v)
tp$.onChange(fn) / tp$.onStateChange(fn)
```
