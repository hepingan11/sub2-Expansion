package com.example.redeem.service;

import com.example.redeem.dto.CheckInResponse;
import com.example.redeem.model.CheckInRecord;
import com.example.redeem.model.DailyCheckInLimit;
import com.example.redeem.model.RedeemCode;
import com.example.redeem.repository.CheckInRecordRepository;
import com.example.redeem.repository.DailyCheckInLimitRepository;
import com.example.redeem.repository.RedeemCodeRepository;
import org.springframework.dao.DataIntegrityViolationException;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.time.LocalDate;

@Service
public class CheckInService {

    private final RedeemCodeRepository redeemCodeRepository;
    private final CheckInRecordRepository checkInRecordRepository;
    private final DailyCheckInLimitRepository dailyCheckInLimitRepository;
    private final PrizeDrawService prizeDrawService;
    private final CheckInSettingsService checkInSettingsService;

    public CheckInService(
            RedeemCodeRepository redeemCodeRepository,
            CheckInRecordRepository checkInRecordRepository,
            DailyCheckInLimitRepository dailyCheckInLimitRepository,
            PrizeDrawService prizeDrawService,
            CheckInSettingsService checkInSettingsService
    ) {
        this.redeemCodeRepository = redeemCodeRepository;
        this.checkInRecordRepository = checkInRecordRepository;
        this.dailyCheckInLimitRepository = dailyCheckInLimitRepository;
        this.prizeDrawService = prizeDrawService;
        this.checkInSettingsService = checkInSettingsService;
    }

    @Transactional
    public CheckInResponse checkIn(String userId) {
        String trimmedUserId = userId.trim();
        LocalDate today = LocalDate.now();

        return checkInRecordRepository.findByUserIdAndSignDate(trimmedUserId, today)
                .flatMap(record -> redeemCodeRepository.findById(record.getRedeemCodeId()))
                .map(code -> toCheckInResponse(code, true, "今日已签到"))
                .orElseGet(() -> createCheckIn(trimmedUserId, today));
    }

    private CheckInResponse createCheckIn(String userId, LocalDate today) {
        try {
            consumeDailyQuota(today);
            int assigned = redeemCodeRepository.assignRandomAvailableByAmount(prizeDrawService.drawAmount(), userId, today);
            if (assigned == 0) {
                assigned = redeemCodeRepository.assignRandomAvailable(userId, today);
            }
            if (assigned == 0) {
                throw new IllegalStateException("兑换码库存不足，请先在后台导入兑换码");
            }

            RedeemCode savedCode = redeemCodeRepository.findByUserIdAndSignDate(userId, today)
                    .orElseThrow(() -> new IllegalStateException("签到成功但未找到绑定兑换码"));
            CheckInRecord record = new CheckInRecord();
            record.setUserId(userId);
            record.setSignDate(today);
            record.setRedeemCodeId(savedCode.getId());
            checkInRecordRepository.save(record);
            return toCheckInResponse(savedCode, false, "签到成功");
        } catch (DataIntegrityViolationException ex) {
            return checkInRecordRepository.findByUserIdAndSignDate(userId, today)
                    .flatMap(record -> redeemCodeRepository.findById(record.getRedeemCodeId()))
                    .map(code -> toCheckInResponse(code, true, "今日已签到"))
                    .orElseThrow(() -> ex);
        }
    }

    private void consumeDailyQuota(LocalDate today) {
        int dailyMaxUsers = checkInSettingsService.getDailyMaxUsers();
        if (dailyMaxUsers <= 0) {
            throw new IllegalStateException("今日签到名额已满");
        }

        DailyCheckInLimit limit = dailyCheckInLimitRepository.findBySignDateForUpdate(today)
                .orElseGet(() -> {
                    DailyCheckInLimit created = new DailyCheckInLimit();
                    created.setSignDate(today);
                    created.setCheckedCount(0);
                    return dailyCheckInLimitRepository.saveAndFlush(created);
                });

        if (limit.getCheckedCount() >= dailyMaxUsers) {
            throw new IllegalStateException("今日签到名额已满");
        }
        limit.setCheckedCount(limit.getCheckedCount() + 1);
        dailyCheckInLimitRepository.save(limit);
    }

    private CheckInResponse toCheckInResponse(RedeemCode redeemCode, boolean alreadyCheckedIn, String message) {
        return new CheckInResponse(
                true,
                alreadyCheckedIn,
                redeemCode.getUserId(),
                redeemCode.getSignDate(),
                redeemCode.getCode(),
                redeemCode.getAmount(),
                message
        );
    }
}
