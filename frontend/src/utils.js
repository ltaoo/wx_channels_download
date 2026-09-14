export function format_time(
  value,
  fallback_message = "时间未知",
  format_options = {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  },
) {
  const timestamp = Number(value);
  let date;
  if (Number.isFinite(timestamp)) {
    if (timestamp <= 0) {
      return fallback_message;
    }
    const normalized = timestamp < 1000000000000 ? timestamp * 1000 : timestamp;
    date = new Date(normalized);
  } else {
    date = new Date(value);
  }
  return Number.isNaN(date.getTime())
    ? fallback_message
    : new Intl.DateTimeFormat("zh-CN", format_options).format(date);
}
