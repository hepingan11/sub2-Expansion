package com.example.redeem.repository;

import com.example.redeem.model.RedeemCode;
import com.example.redeem.model.RedeemCodeStatus;
import org.springframework.data.jpa.repository.JpaRepository;
import org.springframework.data.jpa.repository.JpaSpecificationExecutor;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.repository.Modifying;
import org.springframework.data.jpa.repository.Query;
import org.springframework.data.repository.query.Param;

import java.math.BigDecimal;
import java.time.LocalDate;
import java.util.List;
import java.util.Optional;

public interface RedeemCodeRepository extends JpaRepository<RedeemCode, Long>, JpaSpecificationExecutor<RedeemCode> {

    Optional<RedeemCode> findByUserIdAndSignDate(String userId, LocalDate signDate);

    boolean existsByCode(String code);

    long countByStatus(RedeemCodeStatus status);

    @Query("select r from RedeemCode r where r.status = 'AVAILABLE' and r.amount = ?1 order by function('RAND')")
    List<RedeemCode> findRandomAvailableByAmount(BigDecimal amount, Pageable pageable);

    @Query("select r from RedeemCode r where r.status = 'AVAILABLE' order by function('RAND')")
    List<RedeemCode> findRandomAvailable(Pageable pageable);

    List<RedeemCode> findByCodeIn(List<String> codes);

    @Modifying(clearAutomatically = true, flushAutomatically = true)
    @Query(value = """
            update redeem_codes
            set user_id = :userId,
                sign_date = :signDate,
                status = 'ASSIGNED',
                updated_at = current_timestamp(6)
            where status = 'AVAILABLE' and amount = :amount
            order by rand()
            limit 1
            """, nativeQuery = true)
    int assignRandomAvailableByAmount(@Param("amount") BigDecimal amount, @Param("userId") String userId, @Param("signDate") LocalDate signDate);

    @Modifying(clearAutomatically = true, flushAutomatically = true)
    @Query(value = """
            update redeem_codes
            set user_id = :userId,
                sign_date = :signDate,
                status = 'ASSIGNED',
                updated_at = current_timestamp(6)
            where status = 'AVAILABLE'
            order by rand()
            limit 1
            """, nativeQuery = true)
    int assignRandomAvailable(@Param("userId") String userId, @Param("signDate") LocalDate signDate);
}
