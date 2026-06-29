package com.example.redeem.service;

import org.springframework.stereotype.Service;

import java.math.BigDecimal;
import java.security.SecureRandom;

@Service
public class PrizeDrawService {

    private final SecureRandom random = new SecureRandom();
    private final CheckInSettingsService checkInSettingsService;

    public PrizeDrawService(CheckInSettingsService checkInSettingsService) {
        this.checkInSettingsService = checkInSettingsService;
    }

    public BigDecimal drawAmount() {
        var tiers = checkInSettingsService.getPrizeTiers();
        int roll = random.nextInt(10000) + 1;
        int cumulative = 0;
        for (var tier : tiers) {
            cumulative += tier.probability().movePointRight(2).intValueExact();
            if (roll <= cumulative) {
                return tier.amount();
            }
        }
        return tiers.get(tiers.size() - 1).amount();
    }
}
