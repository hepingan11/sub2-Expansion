package com.example.redeem.controller;

import com.example.redeem.dto.BatchImportCodesRequest;
import com.example.redeem.dto.BatchImportCodesResponse;
import com.example.redeem.dto.DashboardStatsResponse;
import com.example.redeem.dto.RedeemCodeRequest;
import com.example.redeem.dto.RedeemCodeResponse;
import com.example.redeem.model.RedeemCodeStatus;
import com.example.redeem.service.RedeemCodeService;
import jakarta.validation.Valid;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.PageRequest;
import org.springframework.data.domain.Sort;
import org.springframework.format.annotation.DateTimeFormat;
import org.springframework.web.bind.annotation.DeleteMapping;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.PathVariable;
import org.springframework.web.bind.annotation.PostMapping;
import org.springframework.web.bind.annotation.PutMapping;
import org.springframework.web.bind.annotation.RequestBody;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RequestParam;
import org.springframework.web.bind.annotation.RestController;

import java.time.LocalDate;

@RestController
@RequestMapping("/api/admin")
public class RedeemCodeAdminController {

    private final RedeemCodeService redeemCodeService;

    public RedeemCodeAdminController(RedeemCodeService redeemCodeService) {
        this.redeemCodeService = redeemCodeService;
    }

    @GetMapping("/codes")
    public Page<RedeemCodeResponse> list(
            @RequestParam(required = false) String keyword,
            @RequestParam(required = false) String userId,
            @RequestParam(required = false) RedeemCodeStatus status,
            @RequestParam(required = false) @DateTimeFormat(iso = DateTimeFormat.ISO.DATE) LocalDate startDate,
            @RequestParam(required = false) @DateTimeFormat(iso = DateTimeFormat.ISO.DATE) LocalDate endDate,
            @RequestParam(defaultValue = "0") int page,
            @RequestParam(defaultValue = "10") int size
    ) {
        PageRequest pageRequest = PageRequest.of(page, Math.min(size, 100), Sort.by(Sort.Direction.DESC, "createdAt"));
        return redeemCodeService.list(keyword, userId, status, startDate, endDate, pageRequest);
    }

    @GetMapping("/codes/{id}")
    public RedeemCodeResponse get(@PathVariable Long id) {
        return redeemCodeService.get(id);
    }

    @PostMapping("/codes")
    public RedeemCodeResponse create(@Valid @RequestBody RedeemCodeRequest request) {
        return redeemCodeService.create(request);
    }

    @PostMapping("/codes/batch-import")
    public BatchImportCodesResponse batchImport(@Valid @RequestBody BatchImportCodesRequest request) {
        return redeemCodeService.batchImport(request);
    }

    @PutMapping("/codes/{id}")
    public RedeemCodeResponse update(@PathVariable Long id, @Valid @RequestBody RedeemCodeRequest request) {
        return redeemCodeService.update(id, request);
    }

    @DeleteMapping("/codes/{id}")
    public void delete(@PathVariable Long id) {
        redeemCodeService.delete(id);
    }

    @GetMapping("/stats")
    public DashboardStatsResponse stats() {
        return redeemCodeService.stats();
    }
}
