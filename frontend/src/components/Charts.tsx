import { useEffect, useRef } from 'react';
import { CalendarCheck2 } from 'lucide-react';
import type { ECharts } from 'echarts';

import { CheckInStats, InvitationStats, RechargeRewardStats, Sub2APIGroupRateSeries } from '../api';

const rateChartColors = ['#2563eb', '#16a34a', '#dc2626', '#d18a2c', '#7c3aed', '#0891b2', '#4f46e5'];

function escapeTooltipHTML(value: string) {
  return value.replace(/[&<>"']/g, (character) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;'
  }[character] ?? character));
}

function formatRateChartTime(timestamp: number) {
  const date = new Date(timestamp);
  const pad = (value: number) => String(value).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

function toRateChartTimestamp(value: unknown) {
  if (typeof value === 'number') {
    return value;
  }
  const numeric = Number(value);
  if (Number.isFinite(numeric)) {
    return numeric;
  }
  return typeof value === 'string' ? new Date(value).getTime() : Number.NaN;
}

function effectiveRateAt(points: { timestamp: number; rate: number }[], timestamp: number) {
  let low = 0;
  let high = points.length - 1;
  let match = -1;
  while (low <= high) {
    const middle = Math.floor((low + high) / 2);
    if (points[middle].timestamp <= timestamp) {
      match = middle;
      low = middle + 1;
    } else {
      high = middle - 1;
    }
  }
  return match >= 0 ? points[match].rate : undefined;
}

export function RateLineChart({ series }: { series: Sub2APIGroupRateSeries[] }) {
  const chartRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!chartRef.current || series.length === 0) {
      return;
    }
    let disposed = false;
    let chart: ECharts | null = null;
    const resize = () => chart?.resize();
    const tooltipSeries = series.map((item, index) => ({
      name: `${item.groupName}${item.publicVisible ? ' · 公开' : ''}`,
      color: rateChartColors[index % rateChartColors.length],
      points: item.points
        .map((point) => ({ timestamp: new Date(point.time).getTime(), rate: point.rate }))
        .filter((point) => Number.isFinite(point.timestamp))
        .sort((left, right) => left.timestamp - right.timestamp)
    }));
    import('echarts').then((echarts) => {
      if (disposed || !chartRef.current) {
        return;
      }
      chart = echarts.init(chartRef.current);
      chart.setOption({
        color: rateChartColors,
        tooltip: {
          trigger: 'axis',
          confine: true,
          extraCssText: 'max-height: 60vh; overflow-y: auto;',
          axisPointer: { type: 'line' },
          formatter: (rawParams: unknown) => {
            const params = Array.isArray(rawParams) ? rawParams : [rawParams];
            const first = params[0] as { axisValue?: unknown; value?: unknown } | undefined;
            const pointValue = Array.isArray(first?.value) ? first.value[0] : undefined;
            const timestamp = toRateChartTimestamp(first?.axisValue ?? pointValue);
            if (!Number.isFinite(timestamp)) {
              return '';
            }
            const rows = tooltipSeries.map((item) => {
              const activeRate = effectiveRateAt(item.points, timestamp);
              const value = activeRate === undefined ? '暂无' : `${activeRate}x`;
              return `<div style="display:flex;align-items:center;gap:7px;min-width:210px;line-height:22px;">`
                + `<span style="width:9px;height:9px;border-radius:50%;background:${item.color};flex:0 0 auto;"></span>`
                + `<span style="flex:1;overflow-wrap:anywhere;">${escapeTooltipHTML(item.name)}</span>`
                + `<strong style="margin-left:12px;white-space:nowrap;">${value}</strong>`
                + '</div>';
            });
            return `<div style="font-weight:600;margin-bottom:5px;">${formatRateChartTime(timestamp)}</div>${rows.join('')}`;
          }
        },
        legend: { top: 0, type: 'scroll' },
        grid: { left: 44, right: 18, top: 48, bottom: 36 },
        xAxis: { type: 'time' },
        yAxis: {
          type: 'value',
          min: 'dataMin',
          axisLabel: { formatter: '{value}x' }
        },
        series: series.map((item) => ({
          name: `${item.groupName}${item.publicVisible ? ' · 公开' : ''}`,
          type: 'line',
          step: 'end',
          showSymbol: item.points.length < 18,
          data: item.points.map((point) => [point.time, point.rate])
        }))
      });
      window.addEventListener('resize', resize);
    });
    return () => {
      disposed = true;
      window.removeEventListener('resize', resize);
      chart?.dispose();
    };
  }, [series]);

  if (series.length === 0) {
    return <div className="amount-stats-empty">暂无倍率变化数据</div>;
  }

  return <div className="rate-chart" ref={chartRef} />;
}

export function CheckInTrendChart({
  title,
  daily,
  valueKey,
  unit,
  color
}: {
  title: string;
  daily: CheckInStats['daily'];
  valueKey: 'amount' | 'users';
  unit: string;
  color: string;
}) {
  const chartRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!chartRef.current || daily.length === 0) {
      return;
    }
    let disposed = false;
    let chart: ECharts | null = null;
    const resize = () => chart?.resize();
    import('echarts').then((echarts) => {
      if (disposed || !chartRef.current) {
        return;
      }
      chart = echarts.init(chartRef.current);
      chart.setOption({
        color: [color],
        tooltip: { trigger: 'axis' },
        grid: { left: 48, right: 18, top: 34, bottom: 34 },
        xAxis: {
          type: 'category',
          data: daily.map((item) => item.signDate.slice(5)),
          boundaryGap: false
        },
        yAxis: {
          type: 'value',
          minInterval: valueKey === 'users' ? 1 : 0,
          axisLabel: { formatter: `{value}${unit}` }
        },
        series: [{
          name: title,
          type: 'line',
          smooth: true,
          areaStyle: { opacity: 0.08 },
          showSymbol: daily.length < 18,
          data: daily.map((item) => Number(item[valueKey] ?? 0))
        }]
      });
      window.addEventListener('resize', resize);
    });
    return () => {
      disposed = true;
      window.removeEventListener('resize', resize);
      chart?.dispose();
    };
  }, [color, daily, title, unit, valueKey]);

  if (daily.length === 0) {
    return <div className="amount-stats-empty">暂无签到统计数据</div>;
  }

  return (
    <div className="checkin-chart-card">
      <div className="settings-title">
        <CalendarCheck2 size={17} />
        <span>{title}</span>
      </div>
      <div className="checkin-chart" ref={chartRef} />
    </div>
  );
}

export function RechargeRewardTrendChart({ daily }: { daily: RechargeRewardStats['daily'] }) {
  const chartRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!chartRef.current || daily.length === 0) {
      return;
    }
    let disposed = false;
    let chart: ECharts | null = null;
    const resize = () => chart?.resize();
    import('echarts').then((echarts) => {
      if (disposed || !chartRef.current) {
        return;
      }
      chart = echarts.init(chartRef.current);
      chart.setOption({
        color: ['#d97706'],
        tooltip: { trigger: 'axis', valueFormatter: (value: number | string) => `${Number(value).toFixed(2)}` },
        grid: { left: 48, right: 18, top: 34, bottom: 34 },
        xAxis: {
          type: 'category',
          data: daily.map((item) => item.rewardDate.slice(5)),
          boundaryGap: false
        },
        yAxis: {
          type: 'value',
          min: 0,
          axisLabel: { formatter: '{value}' }
        },
        series: [{
          name: '返利金额',
          type: 'line',
          smooth: true,
          areaStyle: { opacity: 0.08 },
          showSymbol: daily.length < 18,
          data: daily.map((item) => Number(item.amount ?? 0))
        }]
      });
      window.addEventListener('resize', resize);
    });
    return () => {
      disposed = true;
      window.removeEventListener('resize', resize);
      chart?.dispose();
    };
  }, [daily]);

  return (
    <div className="recharge-reward-chart-card">
      <div className="settings-title">
        <CalendarCheck2 size={17} />
        <span>近 30 日返利金额</span>
      </div>
      <div className="checkin-chart" ref={chartRef} />
    </div>
  );
}

export function InvitationTrendChart({ daily }: { daily: InvitationStats['daily'] }) {
  const chartRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!chartRef.current || daily.length === 0) {
      return;
    }
    let disposed = false;
    let chart: ECharts | null = null;
    const resize = () => chart?.resize();
    import('echarts').then((echarts) => {
      if (disposed || !chartRef.current) {
        return;
      }
      chart = echarts.init(chartRef.current);
      chart.setOption({
        color: ['#2563eb', '#d97706'],
        tooltip: { trigger: 'axis' },
        legend: { top: 0 },
        grid: { left: 48, right: 48, top: 42, bottom: 34 },
        xAxis: {
          type: 'category',
          data: daily.map((item) => item.rewardDate.slice(5)),
          boundaryGap: false
        },
        yAxis: [
          { type: 'value', minInterval: 1, axisLabel: { formatter: '{value} 人' } },
          { type: 'value', min: 0, axisLabel: { formatter: '{value}' } }
        ],
        series: [
          {
            name: '成功邀请人数',
            type: 'line',
            smooth: true,
            areaStyle: { opacity: 0.06 },
            showSymbol: daily.length < 18,
            data: daily.map((item) => Number(item.users ?? 0))
          },
          {
            name: '发放金额',
            type: 'line',
            yAxisIndex: 1,
            smooth: true,
            showSymbol: daily.length < 18,
            data: daily.map((item) => Number(item.amount ?? 0))
          }
        ]
      });
      window.addEventListener('resize', resize);
    });
    return () => {
      disposed = true;
      window.removeEventListener('resize', resize);
      chart?.dispose();
    };
  }, [daily]);

  return (
    <section className="invitation-trend-chart-card">
      <div className="settings-title">
        <CalendarCheck2 size={17} />
        <span>近 30 日邀请奖励</span>
      </div>
      <div className="checkin-chart" ref={chartRef} />
    </section>
  );
}
