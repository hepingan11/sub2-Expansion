package com.example.redeem.dto;

import jakarta.validation.Valid;
import jakarta.validation.constraints.Min;

import java.util.List;

public record UpdateCheckInSettingsRequest(
        @Min(0) int dailyMaxUsers,
        @Valid List<PrizeTierSetting> prizeTiers
) {
}
