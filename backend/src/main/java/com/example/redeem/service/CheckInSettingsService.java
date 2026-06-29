package com.example.redeem.service;

import com.example.redeem.config.AppProperties;
import com.example.redeem.model.SystemSetting;
import com.example.redeem.repository.SystemSettingRepository;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

@Service
public class CheckInSettingsService {

    private static final String DAILY_MAX_USERS_KEY = "check_in.daily_max_users";

    private final SystemSettingRepository systemSettingRepository;
    private final AppProperties appProperties;

    public CheckInSettingsService(SystemSettingRepository systemSettingRepository, AppProperties appProperties) {
        this.systemSettingRepository = systemSettingRepository;
        this.appProperties = appProperties;
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
}
