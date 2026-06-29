package com.example.redeem.service;

import com.example.redeem.config.AppProperties;
import com.example.redeem.dto.PrizeTierSetting;
import com.example.redeem.repository.SystemSettingRepository;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.orm.jpa.DataJpaTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Import;

import java.math.BigDecimal;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

@DataJpaTest
@Import({CheckInSettingsService.class, CheckInSettingsServiceTest.TestConfig.class})
class CheckInSettingsServiceTest {

    @Autowired
    private CheckInSettingsService checkInSettingsService;

    @Autowired
    private SystemSettingRepository systemSettingRepository;

    @Test
    void updatesPrizeTiersWhenProbabilityTotalIsOneHundred() {
        List<PrizeTierSetting> saved = checkInSettingsService.updatePrizeTiers(List.of(
                new PrizeTierSetting(new BigDecimal("5"), new BigDecimal("25")),
                new PrizeTierSetting(new BigDecimal("1"), new BigDecimal("75"))
        ));

        assertThat(saved)
                .extracting(PrizeTierSetting::amount)
                .containsExactly(new BigDecimal("1.00"), new BigDecimal("5.00"));
        assertThat(checkInSettingsService.getPrizeTiers())
                .extracting(PrizeTierSetting::probability)
                .containsExactly(new BigDecimal("75.00"), new BigDecimal("25.00"));
        assertThat(systemSettingRepository.findById("check_in.prize_tiers")).isPresent();
    }

    @Test
    void rejectsPrizeTiersWhenProbabilityTotalIsNotOneHundred() {
        assertThatThrownBy(() -> checkInSettingsService.updatePrizeTiers(List.of(
                new PrizeTierSetting(new BigDecimal("1"), new BigDecimal("60")),
                new PrizeTierSetting(new BigDecimal("5"), new BigDecimal("30"))
        )))
                .isInstanceOf(IllegalArgumentException.class)
                .hasMessage("所有金额概率之和必须等于 100%");
    }

    @TestConfiguration
    static class TestConfig {

        @Bean
        AppProperties appProperties() {
            return new AppProperties();
        }

        @Bean
        ObjectMapper objectMapper() {
            return new ObjectMapper();
        }
    }
}
