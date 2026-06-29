package com.example.redeem.dto;

import java.math.BigDecimal;
import java.time.LocalDate;

public record CheckInResponse(
        boolean success,
        boolean alreadyCheckedIn,
        String userId,
        LocalDate signDate,
        String code,
        BigDecimal amount,
        String message
) {
}
