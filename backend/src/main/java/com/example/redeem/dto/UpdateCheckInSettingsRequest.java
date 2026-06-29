package com.example.redeem.dto;

import jakarta.validation.constraints.Min;

public record UpdateCheckInSettingsRequest(
        @Min(0) int dailyMaxUsers
) {
}
