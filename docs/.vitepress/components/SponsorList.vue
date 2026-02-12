<template>
  <div class="sponsor-list-container">
    <div class="sponsor-header">
      <h3 class="thank-you-text">感谢您的支持</h3>
      <img
        src="../../assets/wechat_qrcode.png"
        alt="WeChat QR Code"
        class="qr-code"
      />
    </div>
    <div v-if="loading" class="loading">正在加载赞赏列表...</div>
    <div v-else-if="error" class="error">加载失败: {{ error }}</div>
    <div v-else-if="sponsors.length === 0" class="empty">暂无赞赏数据</div>
    <div v-else class="content-wrapper">
      <div class="sponsor-grid">
        <div
          v-for="(item, index) in sponsors"
          :key="index"
          class="sponsor-card"
        >
          <div class="card-header">
            <span class="name" :title="item.text">{{
              item.text || "匿名"
            }}</span>
            <span class="amount">¥{{ item.amount }}</span>
          </div>
          <div class="time">{{ formatDate(item.time) }}</div>
        </div>
      </div>

      <div class="divider"></div>
      <div class="stats-section">
        <h3 class="section-title">赞赏统计</h3>
        <div class="section-body stats-grid">
          <div class="stat-card">
            <div class="stat-label">累计赞赏金额</div>
            <div class="stat-value">¥{{ totalAmount.toFixed(2) }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">累计赞赏次数</div>
            <div class="stat-value">{{ sponsors.length }}</div>
          </div>
          <div class="stat-card">
            <div class="stat-label">单笔最高金额</div>
            <div class="stat-value">¥{{ maxAmount.toFixed(2) }}</div>
          </div>
        </div>
      </div>
      <div class="chart-container" v-if="chartData.length > 1">
        <h3 class="section-title">每月赞赏趋势</h3>
        <div class="section-body svg-wrapper" ref="chartContainer">
          <svg viewBox="0 0 800 300" class="line-chart">
            <!-- Grid Lines -->
            <g class="grid">
              <line
                v-for="i in 5"
                :key="i"
                x1="50"
                :y1="50 + (i - 1) * 50"
                x2="780"
                :y2="50 + (i - 1) * 50"
                stroke="var(--vp-c-divider)"
                stroke-dasharray="4"
              />
            </g>

            <!-- Y Axis Labels -->
            <g class="y-labels">
              <text
                v-for="i in 5"
                :key="i"
                x="40"
                :y="55 + (i - 1) * 50"
                text-anchor="end"
                font-size="12"
                fill="var(--vp-c-text-2)"
              >
                {{ Math.round(maxY - (maxY / 4) * (i - 1)) }}
              </text>
            </g>

            <!-- Path -->
            <path
              :d="linePath"
              fill="none"
              stroke="var(--vp-c-brand)"
              stroke-width="2"
            />
            <path
              :d="areaPath"
              fill="var(--vp-c-brand)"
              fill-opacity="0.1"
              stroke="none"
            />

            <!-- Data Points -->
            <g class="points">
              <circle
                v-for="(point, index) in chartPoints"
                :key="index"
                :cx="point.x"
                :cy="point.y"
                r="4"
                fill="var(--vp-c-bg)"
                stroke="var(--vp-c-brand)"
                stroke-width="2"
                @mouseenter="hoveredPoint = index"
                @mouseleave="hoveredPoint = null"
              />
            </g>

            <!-- X Axis Labels (Show every nth label to avoid clutter) -->
            <g class="x-labels">
              <text
                v-for="(point, index) in chartPoints"
                :key="index"
                :x="point.x"
                y="270"
                text-anchor="middle"
                font-size="12"
                fill="var(--vp-c-text-2)"
                v-show="
                  index === 0 ||
                  index === chartPoints.length - 1 ||
                  index % Math.ceil(chartPoints.length / 8) === 0
                "
              >
                {{ point.label }}
              </text>
            </g>

            <!-- Tooltip -->
            <g
              v-if="hoveredPoint !== null && chartPoints[hoveredPoint]"
              pointer-events="none"
            >
              <rect
                :x="chartPoints[hoveredPoint].x - 60"
                :y="chartPoints[hoveredPoint].y - 50"
                width="120"
                height="40"
                rx="4"
                fill="var(--vp-c-bg-soft)"
                stroke="var(--vp-c-divider)"
              />
              <text
                :x="chartPoints[hoveredPoint].x"
                :y="chartPoints[hoveredPoint].y - 32"
                text-anchor="middle"
                font-size="12"
                font-weight="bold"
                fill="var(--vp-c-text-1)"
              >
                {{ chartPoints[hoveredPoint].label }}
              </text>
              <text
                :x="chartPoints[hoveredPoint].x"
                :y="chartPoints[hoveredPoint].y - 18"
                text-anchor="middle"
                font-size="12"
                fill="var(--vp-c-brand)"
              >
                ¥{{ chartPoints[hoveredPoint].value.toFixed(2) }}
              </text>
            </g>
          </svg>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from "vue";

const sponsors = ref([]);
const loading = ref(true);
const error = ref(null);
const hoveredPoint = ref(null);

// Stats Computed Properties
const totalAmount = computed(() => {
  return sponsors.value.reduce(
    (sum, item) => sum + Number(item.amount || 0),
    0,
  );
});

const maxAmount = computed(() => {
  return Math.max(...sponsors.value.map((item) => Number(item.amount || 0)), 0);
});

// Chart Computed Properties
const chartData = computed(() => {
  if (sponsors.value.length === 0) return [];

  const monthlyData = {};

  sponsors.value.forEach((item) => {
    if (!item.time) return;
    const date = new Date(item.time);
    const key = `${date.getFullYear()}-${(date.getMonth() + 1).toString().padStart(2, "0")}`;
    monthlyData[key] = (monthlyData[key] || 0) + Number(item.amount || 0);
  });

  const keys = Object.keys(monthlyData).sort();
  if (keys.length === 0) return [];

  const startKey = keys[0];
  const endKey = keys[keys.length - 1];

  const result = [];
  let [year, month] = startKey.split("-").map(Number);
  const [endYear, endMonth] = endKey.split("-").map(Number);

  while (year < endYear || (year === endYear && month <= endMonth)) {
    const key = `${year}-${month.toString().padStart(2, "0")}`;
    result.push({
      date: key,
      amount: monthlyData[key] || 0,
    });

    month++;
    if (month > 12) {
      month = 1;
      year++;
    }
  }

  return result;
});

const maxY = computed(() => {
  if (chartData.value.length === 0) return 100;
  const max = Math.max(...chartData.value.map((d) => d.amount));
  return Math.ceil(max / 100) * 100; // Round up to nearest 100
});

const chartPoints = computed(() => {
  const data = chartData.value;
  if (data.length === 0) return [];

  const width = 700; // 800 - 50 - 50 (padding)
  const height = 200; // 250 - 50 (drawing area height)
  const startX = 50;
  const startY = 250;

  const stepX = width / (data.length - 1 || 1);

  return data.map((d, i) => ({
    x: startX + i * stepX,
    y: startY - (d.amount / maxY.value) * height,
    value: d.amount,
    label: d.date,
  }));
});

const linePath = computed(() => {
  const points = chartPoints.value;
  if (points.length === 0) return "";
  return `M ${points.map((p) => `${p.x},${p.y}`).join(" L ")}`;
});

const areaPath = computed(() => {
  const points = chartPoints.value;
  if (points.length === 0) return "";
  const first = points[0];
  const last = points[points.length - 1];
  return `M ${points.map((p) => `${p.x},${p.y}`).join(" L ")} L ${last.x},250 L ${first.x},250 Z`;
});

const formatDate = (dateStr) => {
  if (!dateStr) return "-";
  try {
    const date = new Date(dateStr);
    const year = date.getFullYear();
    const month = (date.getMonth() + 1).toString().padStart(2, "0");
    const day = date.getDate().toString().padStart(2, "0");
    const hours = date.getHours().toString().padStart(2, "0");
    const minutes = date.getMinutes().toString().padStart(2, "0");
    return `${year}-${month}-${day} ${hours}:${minutes}`;
  } catch (e) {
    return dateStr;
  }
};

const fetchData = async () => {
  try {
    const res = await fetch(
      "https://sponsorkit.funzm.com/api/sponsor/list?pageSize=999",
    );
    if (!res.ok) {
      throw new Error(`HTTP error! status: ${res.status}`);
    }
    const data = await res.json();
    if (data && data.code === 0 && data.data && Array.isArray(data.data.list)) {
      sponsors.value = data.data.list;
    } else {
      sponsors.value = [];
      console.warn("Unexpected API response structure:", data);
    }
  } catch (err) {
    error.value = err.message;
    console.error("Failed to fetch sponsors:", err);
  } finally {
    loading.value = false;
  }
};

onMounted(() => {
  fetchData();
});
</script>

<style scoped>
.sponsor-list-container {
  margin-top: 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.sponsor-header {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  margin-bottom: 32px;
}

.thank-you-text {
  margin-bottom: 16px;
  font-weight: 600;
  font-size: 1.2em;
}

.qr-code {
  margin-top: 8px;
  max-width: 200px;
  border-radius: 8px;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
}

.loading,
.error,
.empty {
  padding: 20px;
  text-align: center;
  color: var(--vp-c-text-2);
}

.error {
  color: var(--vp-c-danger);
}

.sponsor-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 16px;
  width: 100%;
}

.sponsor-card {
  background-color: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition:
    transform 0.2s,
    box-shadow 0.2s;
}

.sponsor-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
}

.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-right: 8px;
  flex: 1;
}

.amount {
  color: var(--vp-c-brand);
  white-space: nowrap;
}

.time {
  font-size: 0.8em;
  color: var(--vp-c-text-2);
  text-align: right;
}

.content-wrapper {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 32px;
}

.divider {
  height: 1px;
  background-color: var(--vp-c-divider);
  width: 100%;
}

.stats-section {
}

.section-title {
  text-align: center;
  font-size: 1.2em;
  font-weight: 600;
  margin: 0;
}
.section-body {
  margin-top: 24px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
}

.stat-card {
  background-color: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  padding: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 8px;
}

.stat-label {
  font-size: 0.9em;
  color: var(--vp-c-text-2);
}

.stat-value {
  font-size: 1.4em;
  font-weight: bold;
  color: var(--vp-c-brand);
}

.chart-container {
  margin-top: 24px;
}

.chart-title {
  text-align: center;
  font-size: 1.1em;
  font-weight: 600;
  margin: 0;
}

.svg-wrapper {
  width: 100%;
  overflow-x: auto;
  background-color: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  border-radius: 8px;
  padding: 16px;
}

.line-chart {
  width: 100%;
  min-width: 600px; /* Ensure chart is readable on small screens */
  height: auto;
}

.points circle {
  transition:
    r 0.2s,
    stroke-width 0.2s;
  cursor: pointer;
}

.points circle:hover {
  r: 6;
  stroke-width: 3;
}
</style>
