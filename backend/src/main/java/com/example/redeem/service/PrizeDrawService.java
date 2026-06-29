package com.example.redeem.service;

import org.springframework.stereotype.Service;

import java.math.BigDecimal;
import java.security.SecureRandom;
import java.util.List;

@Service
public class PrizeDrawService {

    private final SecureRandom random = new SecureRandom();
    private final List<PrizeTier> tiers = List.of(
            new PrizeTier(new BigDecimal("1.00"), 70),
            new PrizeTier(new BigDecimal("3.00"), 20),
            new PrizeTier(new BigDecimal("5.00"), 8),
            new PrizeTier(new BigDecimal("10.00"), 2)
    );

    public BigDecimal drawAmount() {
        int roll = random.nextInt(100) + 1;
        int cumulative = 0;
        for (PrizeTier tier : tiers) {
            cumulative += tier.weight();
            if (roll <= cumulative) {
                return tier.amount();
            }
        }
        return tiers.get(tiers.size() - 1).amount();
    }

    private record PrizeTier(BigDecimal amount, int weight) {
    }
}
