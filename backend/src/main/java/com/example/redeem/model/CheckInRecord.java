package com.example.redeem.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.PrePersist;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;

import java.time.LocalDate;
import java.time.LocalDateTime;

@Entity
@Table(
        name = "check_in_records",
        uniqueConstraints = {
                @UniqueConstraint(name = "uk_check_in_records_user_date", columnNames = {"user_id", "sign_date"})
        },
        indexes = {
                @Index(name = "idx_check_in_records_code_id", columnList = "redeem_code_id")
        }
)
public class CheckInRecord {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    private Long id;

    @Column(name = "user_id", nullable = false, length = 64)
    private String userId;

    @Column(name = "sign_date", nullable = false)
    private LocalDate signDate;

    @Column(name = "redeem_code_id", nullable = false)
    private Long redeemCodeId;

    @Column(name = "created_at", nullable = false)
    private LocalDateTime createdAt;

    @PrePersist
    public void prePersist() {
        createdAt = LocalDateTime.now();
    }

    public String getUserId() {
        return userId;
    }

    public void setUserId(String userId) {
        this.userId = userId;
    }

    public LocalDate getSignDate() {
        return signDate;
    }

    public void setSignDate(LocalDate signDate) {
        this.signDate = signDate;
    }

    public Long getRedeemCodeId() {
        return redeemCodeId;
    }

    public void setRedeemCodeId(Long redeemCodeId) {
        this.redeemCodeId = redeemCodeId;
    }
}
