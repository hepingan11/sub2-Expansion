package com.example.redeem.dto;

import java.util.List;

public record CheckInSettingsResponse(
        int dailyMaxUsers,
        List<PrizeTierSetting> prizeTiers
) {
}
