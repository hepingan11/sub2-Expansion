package com.example.redeem.dto;

public record AdminLoginResponse(String token, long expiresInHours) {
}
