package com.example.redeem.security;

import com.example.redeem.config.AppProperties;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class TokenServiceTest {

    @Test
    void issuedTokenCanBeVerified() {
        AppProperties properties = new AppProperties();
        properties.getAuth().setSecret("test-secret-with-enough-length");
        properties.getAuth().setTokenTtlHours(1);
        TokenService tokenService = new TokenService(properties);

        String token = tokenService.issue("admin");

        assertThat(tokenService.verify(token)).isTrue();
        assertThat(tokenService.verify(token + "tampered")).isFalse();
    }
}
