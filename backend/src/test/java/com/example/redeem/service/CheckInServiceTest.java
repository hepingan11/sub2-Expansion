package com.example.redeem.service;

import com.example.redeem.config.AppProperties;
import com.example.redeem.dto.CheckInResponse;
import com.example.redeem.model.RedeemCode;
import com.example.redeem.model.RedeemCodeStatus;
import com.example.redeem.repository.CheckInRecordRepository;
import com.example.redeem.repository.RedeemCodeRepository;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.orm.jpa.DataJpaTest;
import org.springframework.boot.test.context.TestConfiguration;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Import;

import java.math.BigDecimal;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

@DataJpaTest
@Import({CheckInService.class, CheckInSettingsService.class, PrizeDrawService.class, CheckInServiceTest.TestConfig.class})
class CheckInServiceTest {

    @Autowired
    private CheckInService checkInService;

    @Autowired
    private RedeemCodeRepository redeemCodeRepository;

    @Autowired
    private CheckInRecordRepository checkInRecordRepository;

    @Autowired
    private CheckInSettingsService checkInSettingsService;

    @Test
    void sameUserCanOnlyCheckInOncePerDay() {
        RedeemCode availableCode = new RedeemCode();
        availableCode.setCode("3FC57F0BB83B974B38C96CBBF7120451");
        availableCode.setAmount(new BigDecimal("1.00"));
        availableCode.setStatus(RedeemCodeStatus.AVAILABLE);
        redeemCodeRepository.save(availableCode);

        CheckInResponse first = checkInService.checkIn("10001");
        CheckInResponse second = checkInService.checkIn("10001");

        assertThat(first.alreadyCheckedIn()).isFalse();
        assertThat(second.alreadyCheckedIn()).isTrue();
        assertThat(second.code()).isEqualTo(first.code());
        assertThat(redeemCodeRepository.findAll())
                .extracting(RedeemCode::getUserId)
                .containsExactly("10001");
        assertThat(checkInRecordRepository.findAll()).hasSize(1);
    }

    @Test
    void newUsersCannotCheckInAfterDailyLimitIsReached() {
        checkInSettingsService.updateDailyMaxUsers(1);
        saveAvailableCode("CODE0000000000000000000000000001");
        saveAvailableCode("CODE0000000000000000000000000002");

        CheckInResponse first = checkInService.checkIn("10001");

        assertThat(first.alreadyCheckedIn()).isFalse();
        assertThatThrownBy(() -> checkInService.checkIn("10002"))
                .isInstanceOf(IllegalStateException.class)
                .hasMessage("今日签到名额已满");

        CheckInResponse repeat = checkInService.checkIn("10001");
        assertThat(repeat.alreadyCheckedIn()).isTrue();
        assertThat(checkInRecordRepository.findAll()).hasSize(1);
    }

    private void saveAvailableCode(String code) {
        RedeemCode availableCode = new RedeemCode();
        availableCode.setCode(code);
        availableCode.setAmount(new BigDecimal("1.00"));
        availableCode.setStatus(RedeemCodeStatus.AVAILABLE);
        redeemCodeRepository.save(availableCode);
    }

    @TestConfiguration
    static class TestConfig {

        @Bean
        AppProperties appProperties() {
            AppProperties appProperties = new AppProperties();
            appProperties.getCheckIn().setDailyMaxUsers(1000);
            return appProperties;
        }
    }
}
