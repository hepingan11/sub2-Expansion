package com.example.redeem.config;

import org.springframework.boot.ApplicationArguments;
import org.springframework.boot.ApplicationRunner;
import org.springframework.jdbc.core.JdbcTemplate;
import org.springframework.stereotype.Component;

@Component
public class DatabaseMigrationRunner implements ApplicationRunner {

    private final JdbcTemplate jdbcTemplate;

    public DatabaseMigrationRunner(JdbcTemplate jdbcTemplate) {
        this.jdbcTemplate = jdbcTemplate;
    }

    @Override
    public void run(ApplicationArguments args) {
        migrateRedeemCodeTable();
    }

    private void migrateRedeemCodeTable() {
        jdbcTemplate.execute("update redeem_codes set status = 'ASSIGNED' where status = 'ISSUED'");
        jdbcTemplate.execute("alter table redeem_codes modify column user_id varchar(64) null");
        jdbcTemplate.execute("alter table redeem_codes modify column sign_date date null");
        jdbcTemplate.execute("alter table redeem_codes modify column status enum('AVAILABLE','ASSIGNED','USED','VOIDED') not null");
        jdbcTemplate.execute("""
                create table if not exists daily_checkin_limits (
                    sign_date date not null,
                    checked_count int not null,
                    created_at datetime(6) not null,
                    updated_at datetime(6) not null,
                    primary key (sign_date)
                ) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
                """);
        jdbcTemplate.execute("""
                create table if not exists system_settings (
                    setting_key varchar(100) not null,
                    setting_value text not null,
                    created_at datetime(6) not null,
                    updated_at datetime(6) not null,
                    primary key (setting_key)
                ) engine=InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
                """);
        jdbcTemplate.execute("alter table system_settings modify column setting_value text not null");
    }
}
