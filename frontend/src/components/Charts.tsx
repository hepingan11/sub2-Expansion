import { useEffect, useRef } from 'react';
import { CalendarCheck2 } from 'lucide-react';
import type { ECharts } from 'echarts';

import { CheckInStats, Sub2APIGroupRateSeries } from '../api';

export function RateLineChart({ series }: { series: Sub2APIGroupRateSeries[] }) {
  const chartRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!chartRef.current || series.length === 0) {
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
        color: ['#2563eb', '#16a34a', '#dc2626', '#d18a2c', '#7c3aed', '#0891b2', '#4f46e5'],
        tooltip: { trigger: 'axis' },
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
          smooth: true,
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
