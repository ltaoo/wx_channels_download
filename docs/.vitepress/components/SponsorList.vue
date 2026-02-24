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
          <div class="card-body">
            <div class="avatar-container">
              <img
                :src="formatAvatar(item.image)"
                class="avatar"
                alt="Avatar"
              />
            </div>
            <div class="info-container">
              <div class="card-header">
                <span class="name" :title="item.text">{{
                  item.text || "匿名"
                }}</span>
                <span class="amount">¥{{ item.amount }}</span>
              </div>
              <div class="time">{{ formatDate(item.time) }}</div>
            </div>
          </div>
        </div>
      </div>

      <div class="top-sponsors-section" v-if="topSponsors.length > 0">
        <h3 class="section-title">✨ 赞赏最多 ✨</h3>
        <div class="section-body top-sponsors-grid">
          <div
            v-for="item in topSponsors"
            :key="item.id"
            :class="['top-sponsor-card', `rank-${item.rank}`]"
          >
            <div class="rank-badge">{{ item.rank }}</div>
            <div class="avatar-container">
              <img
                :src="formatAvatar(item.image)"
                class="avatar"
                alt="Avatar"
              />
            </div>
            <div class="top-info">
              <div class="name" :title="item.text">{{ item.text || "匿名" }}</div>
              <div class="amount">¥{{ item.amount }}</div>
            </div>
          </div>
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
    <div class="footer-section">
      <p class="powered-by">
        该赞赏列表由
        <a
          href="https://github.com/ltaoo/sponsorkit"
          target="_blank"
          rel="noopener noreferrer"
          class="sponsor-link"
          >sponsorkit</a
        >
        驱动
      </p>
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

const topSponsors = computed(() => {
  if (sponsors.value.length === 0) return [];

  const sponsorMap = new Map();

  sponsors.value.forEach((item) => {
    // 优先使用 id 作为唯一标识，如果没有则使用 text（昵称）
    const id = item.id || item.text || "anonymous";
    const currentAmount = Number(item.amount || 0);

    if (sponsorMap.has(id)) {
      const existing = sponsorMap.get(id);
      existing.amount += currentAmount;
      // 保持最新的信息（如头像、昵称、最后一次赞赏时间）
      if (!existing.time || new Date(item.time) > new Date(existing.time)) {
        existing.text = item.text || existing.text;
        existing.image = item.image || existing.image;
        existing.time = item.time;
      }
    } else {
      sponsorMap.set(id, {
        id,
        text: item.text,
        image: item.image,
        amount: currentAmount,
        time: item.time,
      });
    }
  });

  return Array.from(sponsorMap.values())
    .sort((a, b) => b.amount - a.amount)
    .slice(0, 3)
    .map((item, index) => ({ ...item, rank: index + 1 }));
});

const formatAvatar = (url) => {
  if (typeof window === "undefined") {
    return url;
  }
  const prefix = window.location.href.replace(/\/$/, '');
  if (!url) {
    
    return `${prefix}/sponsors/wechat.svg`;
  }
  return `${prefix}${url}`;
};

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
  transition:
    transform 0.2s,
    box-shadow 0.2s;
}

.sponsor-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.05);
}

.card-body {
  display: flex;
  align-items: center;
  gap: 12px;
}

.avatar-container {
  flex-shrink: 0;
}

.avatar {
  width: 40px;
  height: 40px;
  border-radius: 50%;
  border: 2px solid var(--vp-c-divider);
  object-fit: cover;
}

.info-container {
  flex: 1;
  min-width: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-weight: 600;
  margin-bottom: 4px;
}

.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-right: 8px;
  flex: 1;
}

.amount {
  background: linear-gradient(135deg, #b8860b 0%, #daa520 50%, #ffd700 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  font-weight: 800;
  white-space: nowrap;
  font-size: 1.1em;
}

.time {
  font-size: 0.8em;
  color: var(--vp-c-text-2);
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

.top-sponsors-section {
  margin-top: 24px;
}

.top-sponsors-grid {
  display: flex;
  justify-content: center;
  align-items: flex-end;
  gap: 16px;
  margin-top: 48px !important;
  flex-wrap: nowrap;
}

@media (max-width: 640px) {
  .top-sponsors-grid {
    flex-direction: column;
    align-items: center;
    gap: 24px;
  }
  .top-sponsor-card,
  .top-sponsor-card.rank-1 {
    order: unset;
    max-width: 100%;
    width: 100%;
  }
}

.top-sponsor-card {
  position: relative;
  background-color: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  border-radius: 12px;
  padding: 24px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  transition: transform 0.2s;
  flex: 1;
  max-width: 200px;
}

.top-sponsor-card.rank-1 {
  order: 1;
  padding: 48px 24px;
  z-index: 2;
  border-width: 2px;
  max-width: 240px;
}

.top-sponsor-card.rank-2 {
  order: 0;
  z-index: 1;
  padding: 36px 20px;
}

.top-sponsor-card.rank-3 {
  order: 2;
  z-index: 1;
  padding: 24px 16px;
}

.top-sponsor-card .avatar {
  width: 60px;
  height: 60px;
}

.top-sponsor-card.rank-1 .avatar {
  width: 96px;
  height: 96px;
  border-width: 3px;
}

.top-sponsor-card.rank-2 .avatar {
  width: 72px;
  height: 72px;
}

.top-sponsor-card:hover {
  transform: translateY(-8px);
}

.rank-badge {
  position: absolute;
  top: -12px;
  left: 50%;
  transform: translateX(-50%);
  width: 24px;
  height: 24px;
  background-color: var(--vp-c-brand);
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: bold;
  font-size: 14px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.rank-1 {
  border-color: #ffd700;
  background: linear-gradient(
    to bottom right,
    var(--vp-c-bg-soft),
    rgba(255, 215, 0, 0.05)
  );
}
.rank-1 .rank-badge {
  background-color: #ffd700;
  color: #8b4513;
}
.rank-2 {
  border-color: #c0c0c0;
  background: linear-gradient(
    to bottom right,
    var(--vp-c-bg-soft),
    rgba(192, 192, 192, 0.05)
  );
}
.rank-2 .rank-badge {
  background-color: #c0c0c0;
  color: #4a4a4a;
}
.rank-3 {
  border-color: #cd7f32;
  background: linear-gradient(
    to bottom right,
    var(--vp-c-bg-soft),
    rgba(205, 127, 50, 0.05)
  );
}
.rank-3 .rank-badge {
  background-color: #cd7f32;
  color: white;
}

.top-info {
  text-align: center;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.top-info .name {
  font-weight: 600;
  font-size: 1.1em;
}

.top-info .amount {
  font-size: 1.3em;
  font-weight: 800;
  background: linear-gradient(135deg, #b8860b 0%, #daa520 50%, #ffd700 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
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
  font-weight: 800;
  background: linear-gradient(135deg, #b8860b 0%, #daa520 50%, #ffd700 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
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

.footer-section {
  margin-top: 48px;
  padding-bottom: 24px;
  text-align: center;
  width: 100%;
  border-top: 1px solid var(--vp-c-divider);
}

.powered-by {
  margin-top: 24px;
  font-size: 0.9em;
  color: var(--vp-c-text-2);
}

.sponsor-link {
  color: var(--vp-c-brand);
  font-weight: 500;
  text-decoration: none;
  transition: color 0.2s;
}

.sponsor-link:hover {
  color: var(--vp-c-brand-dark);
  text-decoration: underline;
}
</style>
