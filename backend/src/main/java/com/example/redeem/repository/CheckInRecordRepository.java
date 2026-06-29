package com.example.redeem.repository;

import com.example.redeem.model.CheckInRecord;
import org.springframework.data.jpa.repository.JpaRepository;

import java.time.LocalDate;
import java.util.Optional;

public interface CheckInRecordRepository extends JpaRepository<CheckInRecord, Long> {

    Optional<CheckInRecord> findByUserIdAndSignDate(String userId, LocalDate signDate);
}
