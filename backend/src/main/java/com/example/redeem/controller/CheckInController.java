package com.example.redeem.controller;

import com.example.redeem.dto.CheckInRequest;
import com.example.redeem.dto.CheckInResponse;
import com.example.redeem.service.CheckInService;
import jakarta.validation.Valid;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;

@RestController
@RequestMapping("/api/checkins")
public class CheckInController {

    private final CheckInService checkInService;

    public CheckInController(CheckInService checkInService) {
        this.checkInService = checkInService;
    }

    @PostMapping
    public CheckInResponse checkIn(@Valid @RequestBody CheckInRequest request) {
        return checkInService.checkIn(request.getUserId());
    }
}
