package com.example.redeem.service;

import com.example.redeem.config.AppProperties;
import com.example.redeem.dto.AdminLoginRequest;
import com.example.redeem.dto.AdminLoginResponse;
import com.example.redeem.security.TokenService;
import org.springframework.security.crypto.bcrypt.BCryptPasswordEncoder;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;

@Service
public class AdminAuthService {

    private final AppProperties appProperties;
    private final TokenService tokenService;
    private final BCryptPasswordEncoder passwordEncoder = new BCryptPasswordEncoder();

    public AdminAuthService(AppProperties appProperties, TokenService tokenService) {
        this.appProperties = appProperties;
        this.tokenService = tokenService;
    }

    public AdminLoginResponse login(AdminLoginRequest request) {
        String configuredUsername = appProperties.getAdmin().getUsername();
        String configuredPassword = appProperties.getAdmin().getPassword();

        if (!request.getUsername().equals(configuredUsername) || !matchesPassword(request.getPassword(), configuredPassword)) {
            throw new IllegalArgumentException("用户名或密码错误");
        }

        return new AdminLoginResponse(tokenService.issue(configuredUsername), appProperties.getAuth().getTokenTtlHours());
    }

    private boolean matchesPassword(String rawPassword, String configuredPassword) {
        if (!StringUtils.hasText(configuredPassword)) {
            return false;
        }
        if (configuredPassword.startsWith("$2a$") || configuredPassword.startsWith("$2b$") || configuredPassword.startsWith("$2y$")) {
            return passwordEncoder.matches(rawPassword, configuredPassword);
        }
        return rawPassword.equals(configuredPassword);
    }
}
