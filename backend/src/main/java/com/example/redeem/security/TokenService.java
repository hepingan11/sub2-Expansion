package com.example.redeem.security;

import com.example.redeem.config.AppProperties;
import org.springframework.stereotype.Service;

import javax.crypto.Mac;
import javax.crypto.spec.SecretKeySpec;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.Base64;

@Service
public class TokenService {

    private static final String HMAC_SHA256 = "HmacSHA256";

    private final AppProperties appProperties;

    public TokenService(AppProperties appProperties) {
        this.appProperties = appProperties;
    }

    public String issue(String username) {
        long expiresAt = Instant.now().plusSeconds(appProperties.getAuth().getTokenTtlHours() * 3600).getEpochSecond();
        String payload = base64Url(username) + "." + expiresAt;
        return payload + "." + sign(payload);
    }

    public boolean verify(String token) {
        String[] parts = token.split("\\.");
        if (parts.length != 3) {
            return false;
        }

        String payload = parts[0] + "." + parts[1];
        if (!constantTimeEquals(sign(payload), parts[2])) {
            return false;
        }

        try {
            long expiresAt = Long.parseLong(parts[1]);
            return expiresAt > Instant.now().getEpochSecond();
        } catch (NumberFormatException ex) {
            return false;
        }
    }

    private String sign(String payload) {
        try {
            Mac mac = Mac.getInstance(HMAC_SHA256);
            mac.init(new SecretKeySpec(appProperties.getAuth().getSecret().getBytes(StandardCharsets.UTF_8), HMAC_SHA256));
            return Base64.getUrlEncoder().withoutPadding().encodeToString(mac.doFinal(payload.getBytes(StandardCharsets.UTF_8)));
        } catch (Exception ex) {
            throw new IllegalStateException("Unable to sign admin token", ex);
        }
    }

    private String base64Url(String value) {
        return Base64.getUrlEncoder().withoutPadding().encodeToString(value.getBytes(StandardCharsets.UTF_8));
    }

    private boolean constantTimeEquals(String first, String second) {
        if (first == null || second == null || first.length() != second.length()) {
            return false;
        }

        int result = 0;
        for (int i = 0; i < first.length(); i++) {
            result |= first.charAt(i) ^ second.charAt(i);
        }
        return result == 0;
    }
}
