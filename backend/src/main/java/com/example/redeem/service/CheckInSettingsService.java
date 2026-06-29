package com.example.redeem.service;

import com.example.redeem.dto.CheckInSettingsResponse;
import com.example.redeem.dto.PrizeTierSetting;
import com.example.redeem.config.AppProperties;
import com.example.redeem.model.SystemSetting;
import com.example.redeem.repository.SystemSettingRepository;
import com.fasterxml.jackson.core.JsonProcessingException;
import com.fasterxml.jackson.core.type.TypeReference;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.math.BigDecimal;
import java.math.RoundingMode;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

@Service
public class CheckInSettingsService {

    private static final String DAILY_MAX_USERS_KEY = "check_in.daily_max_users";
    private static final String PRIZE_TIERS_KEY = "check_in.prize_tiers";
    private static final BigDecimal ONE_HUNDRED = new BigDecimal("100.00");
    private static final List<PrizeTierSetting> DEFAULT_PRIZE_TIERS = List.of(
            new PrizeTierSetting(new BigDecimal("1.00"), new BigDecimal("70.00")),
            new PrizeTierSetting(new BigDecimal("3.00"), new BigDecimal("20.00")),
            new PrizeTierSetting(new BigDecimal("5.00"), new BigDecimal("8.00")),
            new PrizeTierSetting(new BigDecimal("10.00"), new BigDecimal("2.00"))
    );

    private final SystemSettingRepository systemSettingRepository;
    private final AppProperties appProperties;
    private final ObjectMapper objectMapper;

    public CheckInSettingsService(
            SystemSettingRepository systemSettingRepository,
            AppProperties appProperties,
            ObjectMapper objectMapper
    ) {
        this.systemSettingRepository = systemSettingRepository;
        this.appProperties = appProperties;
        this.objectMapper = objectMapper;
    }

    @Transactional
    public CheckInSettingsResponse getCheckInSettings() {
        return new CheckInSettingsResponse(getDailyMaxUsers(), getPrizeTiers());
    }

    @Transactional
    public int getDailyMaxUsers() {
        return systemSettingRepository.findById(DAILY_MAX_USERS_KEY)
                .map(SystemSetting::getSettingValue)
                .map(this::parseDailyMaxUsers)
                .orElseGet(this::createDefaultDailyMaxUsers);
    }

    @Transactional
    public int updateDailyMaxUsers(int dailyMaxUsers) {
        if (dailyMaxUsers < 0) {
            throw new IllegalArgumentException("每日签到上限不能小于 0");
        }

        SystemSetting setting = systemSettingRepository.findById(DAILY_MAX_USERS_KEY)
                .orElseGet(() -> {
                    SystemSetting created = new SystemSetting();
                    created.setSettingKey(DAILY_MAX_USERS_KEY);
                    return created;
                });
        setting.setSettingValue(String.valueOf(dailyMaxUsers));
        systemSettingRepository.save(setting);
        return dailyMaxUsers;
    }

    @Transactional
    public List<PrizeTierSetting> getPrizeTiers() {
        return systemSettingRepository.findById(PRIZE_TIERS_KEY)
                .map(SystemSetting::getSettingValue)
                .map(this::parsePrizeTiers)
                .orElseGet(this::createDefaultPrizeTiers);
    }

    @Transactional
    public CheckInSettingsResponse updateCheckInSettings(int dailyMaxUsers, List<PrizeTierSetting> prizeTiers) {
        List<PrizeTierSetting> savedPrizeTiers = prizeTiers == null ? getPrizeTiers() : updatePrizeTiers(prizeTiers);
        return new CheckInSettingsResponse(updateDailyMaxUsers(dailyMaxUsers), savedPrizeTiers);
    }

    @Transactional
    public List<PrizeTierSetting> updatePrizeTiers(List<PrizeTierSetting> prizeTiers) {
        List<PrizeTierSetting> normalized = normalizePrizeTiers(prizeTiers);
        SystemSetting setting = systemSettingRepository.findById(PRIZE_TIERS_KEY)
                .orElseGet(() -> {
                    SystemSetting created = new SystemSetting();
                    created.setSettingKey(PRIZE_TIERS_KEY);
                    return created;
                });
        setting.setSettingValue(toJson(normalized));
        systemSettingRepository.save(setting);
        return normalized;
    }

    private int createDefaultDailyMaxUsers() {
        int dailyMaxUsers = Math.max(appProperties.getCheckIn().getDailyMaxUsers(), 0);
        updateDailyMaxUsers(dailyMaxUsers);
        return dailyMaxUsers;
    }

    private int parseDailyMaxUsers(String value) {
        try {
            return Math.max(Integer.parseInt(value), 0);
        } catch (NumberFormatException ex) {
            return createDefaultDailyMaxUsers();
        }
    }

    private List<PrizeTierSetting> createDefaultPrizeTiers() {
        return updatePrizeTiers(DEFAULT_PRIZE_TIERS);
    }

    private List<PrizeTierSetting> parsePrizeTiers(String value) {
        try {
            return normalizePrizeTiers(objectMapper.readValue(value, new TypeReference<>() {
            }));
        } catch (JsonProcessingException | IllegalArgumentException ex) {
            return createDefaultPrizeTiers();
        }
    }

    private List<PrizeTierSetting> normalizePrizeTiers(List<PrizeTierSetting> prizeTiers) {
        if (prizeTiers == null || prizeTiers.isEmpty()) {
            throw new IllegalArgumentException("请至少配置一个兑换码金额概率");
        }

        Map<BigDecimal, BigDecimal> merged = new LinkedHashMap<>();
        for (PrizeTierSetting tier : prizeTiers) {
            if (tier == null || tier.amount() == null || tier.probability() == null) {
                throw new IllegalArgumentException("金额和概率不能为空");
            }

            BigDecimal amount = tier.amount().setScale(2, RoundingMode.HALF_UP);
            BigDecimal probability = tier.probability().setScale(2, RoundingMode.HALF_UP);
            if (amount.compareTo(BigDecimal.ZERO) <= 0) {
                throw new IllegalArgumentException("金额必须大于 0");
            }
            if (probability.compareTo(BigDecimal.ZERO) <= 0 || probability.compareTo(ONE_HUNDRED) > 0) {
                throw new IllegalArgumentException("概率必须大于 0 且不超过 100");
            }
            merged.merge(amount, probability, BigDecimal::add);
        }

        List<PrizeTierSetting> normalized = merged.entrySet().stream()
                .map(entry -> new PrizeTierSetting(entry.getKey(), entry.getValue().setScale(2, RoundingMode.HALF_UP)))
                .sorted(Comparator.comparing(PrizeTierSetting::amount))
                .toList();

        BigDecimal total = normalized.stream()
                .map(PrizeTierSetting::probability)
                .reduce(BigDecimal.ZERO, BigDecimal::add)
                .setScale(2, RoundingMode.HALF_UP);
        if (total.compareTo(ONE_HUNDRED) != 0) {
            throw new IllegalArgumentException("所有金额概率之和必须等于 100%");
        }

        return normalized;
    }

    private String toJson(List<PrizeTierSetting> prizeTiers) {
        try {
            return objectMapper.writeValueAsString(prizeTiers);
        } catch (JsonProcessingException ex) {
            throw new IllegalStateException("保存概率配置失败", ex);
        }
    }
}
