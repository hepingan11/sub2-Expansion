package com.example.redeem.dto;

import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;

import java.math.BigDecimal;

public class BatchImportCodesRequest {

    @NotBlank
    private String codesText;

    @NotNull
    @DecimalMin(value = "0.01")
    private BigDecimal amount;

    public String getCodesText() {
        return codesText;
    }

    public void setCodesText(String codesText) {
        this.codesText = codesText;
    }

    public BigDecimal getAmount() {
        return amount;
    }

    public void setAmount(BigDecimal amount) {
        this.amount = amount;
    }
}
