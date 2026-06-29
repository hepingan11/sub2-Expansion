package com.example.redeem.dto;

import jakarta.validation.constraints.DecimalMax;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotNull;

import java.math.BigDecimal;

public record PrizeTierSetting(
        @NotNull @DecimalMin("0.01") BigDecimal amount,
        @NotNull @DecimalMin("0.01") @DecimalMax("100.00") BigDecimal probability
) {
}
