package com.example.redeem.controller;

import com.example.redeem.dto.CheckInSettingsResponse;
import com.example.redeem.dto.UpdateCheckInSettingsRequest;
import com.example.redeem.service.CheckInSettingsService;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/admin/settings")
public class AdminSettingsController {

    private final CheckInSettingsService checkInSettingsService;

    public AdminSettingsController(CheckInSettingsService checkInSettingsService) {
        this.checkInSettingsService = checkInSettingsService;
    }

    @GetMapping("/check-in")
    public CheckInSettingsResponse getCheckInSettings() {
        return checkInSettingsService.getCheckInSettings();
    }

    @PutMapping("/check-in")
    public CheckInSettingsResponse updateCheckInSettings(@Valid @RequestBody UpdateCheckInSettingsRequest request) {
        return checkInSettingsService.updateCheckInSettings(request.dailyMaxUsers(), request.prizeTiers());
    }
}
