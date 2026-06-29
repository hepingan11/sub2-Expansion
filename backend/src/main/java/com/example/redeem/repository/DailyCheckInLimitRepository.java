package com.example.redeem.repository;

import com.example.redeem.model.DailyCheckInLimit;
import jakarta.persistence.LockModeType;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.Lock;
import org.springframework.data.jpa.repository.Query;

import java.time.LocalDate;
import java.util.Optional;

public interface DailyCheckInLimitRepository extends JpaRepository<DailyCheckInLimit, LocalDate> {

    @Lock(LockModeType.PESSIMISTIC_WRITE)
    @Query("select d from DailyCheckInLimit d where d.signDate = ?1")
    Optional<DailyCheckInLimit> findBySignDateForUpdate(LocalDate signDate);
}
