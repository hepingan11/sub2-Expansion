package com.example.redeem.dto;

import com.example.redeem.model.RedeemCodeStatus;
import jakarta.validation.constraints.DecimalMin;
import jakarta.validation.constraints.NotNull;

import java.math.BigDecimal;
import java.time.LocalDate;

public class RedeemCodeRequest {

    private String code;

    private String userId;

    private LocalDate signDate;

    @NotNull
    @DecimalMin(value = "0.01")
    private BigDecimal amount;

    private RedeemCodeStatus status = RedeemCodeStatus.AVAILABLE;

    public String getCode() {
        return code;
    }

    public void setCode(String code) {
        this.code = code;
    }

    public String getUserId() {
        return userId;
    }

    public void setUserId(String userId) {
        this.userId = userId;
    }

    public LocalDate getSignDate() {
        return signDate;
    }

    public void setSignDate(LocalDate signDate) {
        this.signDate = signDate;
    }

    public BigDecimal getAmount() {
        return amount;
    }

    public void setAmount(BigDecimal amount) {
        this.amount = amount;
    }

    public RedeemCodeStatus getStatus() {
        return status;
    }

    public void setStatus(RedeemCodeStatus status) {
        this.status = status;
    }
}
