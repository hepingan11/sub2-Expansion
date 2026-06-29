package com.example.redeem.dto;

import com.example.redeem.model.RedeemCode;
import com.example.redeem.model.RedeemCodeStatus;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.time.LocalDateTime;

public record RedeemCodeResponse(
        Long id,
        String code,
        String userId,
        LocalDate signDate,
        BigDecimal amount,
        RedeemCodeStatus status,
        LocalDateTime createdAt,
        LocalDateTime updatedAt
) {

    public static RedeemCodeResponse from(RedeemCode redeemCode) {
        return new RedeemCodeResponse(
                redeemCode.getId(),
                redeemCode.getCode(),
                redeemCode.getUserId(),
                redeemCode.getSignDate(),
                redeemCode.getAmount(),
                redeemCode.getStatus(),
                redeemCode.getCreatedAt(),
                redeemCode.getUpdatedAt()
        );
    }
}
