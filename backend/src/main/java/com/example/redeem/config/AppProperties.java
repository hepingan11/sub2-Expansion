package com.example.redeem.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "app")
public class AppProperties {

    private final Admin admin = new Admin();
    private final Auth auth = new Auth();
    private final Cors cors = new Cors();
    private final CheckIn checkIn = new CheckIn();

    public Admin getAdmin() {
        return admin;
    }

    public Auth getAuth() {
        return auth;
    }

    public Cors getCors() {
        return cors;
    }

    public CheckIn getCheckIn() {
        return checkIn;
    }

    public static class Admin {
        private String username;
        private String password;

        public String getUsername() {
            return username;
        }

        public void setUsername(String username) {
            this.username = username;
        }

        public String getPassword() {
            return password;
        }

        public void setPassword(String password) {
            this.password = password;
        }
    }

    public static class Auth {
        private String secret;
        private long tokenTtlHours = 12;

        public String getSecret() {
            return secret;
        }

        public void setSecret(String secret) {
            this.secret = secret;
        }

        public long getTokenTtlHours() {
            return tokenTtlHours;
        }

        public void setTokenTtlHours(long tokenTtlHours) {
            this.tokenTtlHours = tokenTtlHours;
        }
    }

    public static class Cors {
        private String allowedOrigins = "http://localhost:5173";

        public String getAllowedOrigins() {
            return allowedOrigins;
        }

        public void setAllowedOrigins(String allowedOrigins) {
            this.allowedOrigins = allowedOrigins;
        }
    }

    public static class CheckIn {
        private int dailyMaxUsers = 1000;

        public int getDailyMaxUsers() {
            return dailyMaxUsers;
        }

        public void setDailyMaxUsers(int dailyMaxUsers) {
            this.dailyMaxUsers = dailyMaxUsers;
        }
    }
}
