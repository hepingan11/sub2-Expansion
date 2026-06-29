package com.example.redeem.service;

import com.example.redeem.repository.RedeemCodeRepository;
import org.springframework.stereotype.Component;

import java.security.SecureRandom;

@Component
public class CodeGenerator {

    private static final char[] ALPHABET = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789".toCharArray();
    private final SecureRandom secureRandom = new SecureRandom();
    private final RedeemCodeRepository redeemCodeRepository;

    public CodeGenerator(RedeemCodeRepository redeemCodeRepository) {
        this.redeemCodeRepository = redeemCodeRepository;
    }

    public String uniqueCode() {
        for (int attempts = 0; attempts < 10; attempts++) {
            String code = randomCode();
            if (!redeemCodeRepository.existsByCode(code)) {
                return code;
            }
        }
        throw new IllegalStateException("Unable to generate a unique redeem code");
    }

    private String randomCode() {
        StringBuilder builder = new StringBuilder("RC");
        for (int i = 0; i < 14; i++) {
            builder.append(ALPHABET[secureRandom.nextInt(ALPHABET.length)]);
        }
        return builder.toString();
    }
}
