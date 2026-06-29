package com.example.redeem.dto;

import jakarta.validation.constraints.NotBlank;

public class CheckInRequest {

    @NotBlank
    private String userId;

    public String getUserId() {
        return userId;
    }

    public void setUserId(String userId) {
        this.userId = userId;
    }
}
