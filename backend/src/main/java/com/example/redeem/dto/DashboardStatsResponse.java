package com.example.redeem.dto;

public record DashboardStatsResponse(long total, long available, long assigned, long used, long voided) {
}
