package com.example.redeem.service;

import com.example.redeem.dto.BatchImportCodesRequest;
import com.example.redeem.dto.BatchImportCodesResponse;
import com.example.redeem.dto.DashboardStatsResponse;
import com.example.redeem.dto.RedeemCodeRequest;
import com.example.redeem.dto.RedeemCodeResponse;
import com.example.redeem.model.RedeemCode;
import com.example.redeem.model.RedeemCodeStatus;
import com.example.redeem.repository.RedeemCodeRepository;
import jakarta.persistence.EntityNotFoundException;
import jakarta.persistence.criteria.Predicate;
import org.springframework.data.domain.Page;
import org.springframework.data.domain.Pageable;
import org.springframework.data.jpa.domain.Specification;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.util.StringUtils;

import java.time.LocalDate;
import java.util.ArrayList;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;

@Service
public class RedeemCodeService {

    private final RedeemCodeRepository redeemCodeRepository;
    private final CodeGenerator codeGenerator;

    public RedeemCodeService(RedeemCodeRepository redeemCodeRepository, CodeGenerator codeGenerator) {
        this.redeemCodeRepository = redeemCodeRepository;
        this.codeGenerator = codeGenerator;
    }

    @Transactional(readOnly = true)
    public Page<RedeemCodeResponse> list(String keyword, String userId, RedeemCodeStatus status, LocalDate startDate, LocalDate endDate, Pageable pageable) {
        return redeemCodeRepository.findAll(buildSpec(keyword, userId, status, startDate, endDate), pageable)
                .map(RedeemCodeResponse::from);
    }

    @Transactional(readOnly = true)
    public RedeemCodeResponse get(Long id) {
        return RedeemCodeResponse.from(findById(id));
    }

    @Transactional
    public RedeemCodeResponse create(RedeemCodeRequest request) {
        RedeemCode redeemCode = new RedeemCode();
        applyRequest(redeemCode, request, true);
        return RedeemCodeResponse.from(redeemCodeRepository.save(redeemCode));
    }

    @Transactional
    public RedeemCodeResponse update(Long id, RedeemCodeRequest request) {
        RedeemCode redeemCode = findById(id);
        applyRequest(redeemCode, request, false);
        return RedeemCodeResponse.from(redeemCodeRepository.save(redeemCode));
    }

    @Transactional
    public void delete(Long id) {
        redeemCodeRepository.delete(findById(id));
    }

    @Transactional(readOnly = true)
    public DashboardStatsResponse stats() {
        return new DashboardStatsResponse(
                redeemCodeRepository.count(),
                redeemCodeRepository.countByStatus(RedeemCodeStatus.AVAILABLE),
                redeemCodeRepository.countByStatus(RedeemCodeStatus.ASSIGNED),
                redeemCodeRepository.countByStatus(RedeemCodeStatus.USED),
                redeemCodeRepository.countByStatus(RedeemCodeStatus.VOIDED)
        );
    }

    @Transactional
    public BatchImportCodesResponse batchImport(BatchImportCodesRequest request) {
        List<String> parsedCodes = parseCodes(request.getCodesText());
        if (parsedCodes.isEmpty()) {
            throw new IllegalArgumentException("请至少粘贴一个兑换码");
        }

        Set<String> existingCodes = redeemCodeRepository.findByCodeIn(parsedCodes).stream()
                .map(RedeemCode::getCode)
                .collect(Collectors.toSet());

        List<RedeemCode> newCodes = parsedCodes.stream()
                .filter(code -> !existingCodes.contains(code))
                .map(code -> {
                    RedeemCode redeemCode = new RedeemCode();
                    redeemCode.setCode(code);
                    redeemCode.setAmount(request.getAmount());
                    redeemCode.setStatus(RedeemCodeStatus.AVAILABLE);
                    return redeemCode;
                })
                .toList();

        redeemCodeRepository.saveAll(newCodes);
        return new BatchImportCodesResponse(parsedCodes.size(), newCodes.size(), existingCodes.size(), new ArrayList<>(existingCodes));
    }

    private void applyRequest(RedeemCode redeemCode, RedeemCodeRequest request, boolean creating) {
        String code = normalizeCode(request.getCode());
        if (creating && !StringUtils.hasText(code)) {
            code = codeGenerator.uniqueCode();
        }

        if (StringUtils.hasText(code)) {
            redeemCode.setCode(code);
        }
        redeemCode.setAmount(request.getAmount());
        RedeemCodeStatus status = request.getStatus() == null ? RedeemCodeStatus.AVAILABLE : request.getStatus();
        redeemCode.setStatus(status);
        if (status == RedeemCodeStatus.AVAILABLE) {
            redeemCode.setUserId(null);
            redeemCode.setSignDate(null);
        } else {
            redeemCode.setUserId(StringUtils.hasText(request.getUserId()) ? request.getUserId().trim() : null);
            redeemCode.setSignDate(request.getSignDate());
        }
    }

    private RedeemCode findById(Long id) {
        return redeemCodeRepository.findById(id)
                .orElseThrow(() -> new EntityNotFoundException("Redeem code not found: " + id));
    }

    private Specification<RedeemCode> buildSpec(String keyword, String userId, RedeemCodeStatus status, LocalDate startDate, LocalDate endDate) {
        return (root, query, criteriaBuilder) -> {
            List<Predicate> predicates = new ArrayList<>();

            if (StringUtils.hasText(keyword)) {
                String pattern = "%" + keyword.trim() + "%";
                predicates.add(criteriaBuilder.or(
                        criteriaBuilder.like(root.get("code"), pattern),
                        criteriaBuilder.like(root.get("userId"), pattern)
                ));
            }
            if (StringUtils.hasText(userId)) {
                predicates.add(criteriaBuilder.equal(root.get("userId"), userId.trim()));
            }
            if (status != null) {
                predicates.add(criteriaBuilder.equal(root.get("status"), status));
            }
            if (startDate != null) {
                predicates.add(criteriaBuilder.greaterThanOrEqualTo(root.get("signDate"), startDate));
            }
            if (endDate != null) {
                predicates.add(criteriaBuilder.lessThanOrEqualTo(root.get("signDate"), endDate));
            }

            return criteriaBuilder.and(predicates.toArray(Predicate[]::new));
        };
    }

    private List<String> parseCodes(String codesText) {
        Set<String> codes = new LinkedHashSet<>();
        for (String rawCode : codesText.split("[\\r\\n,;\\s]+")) {
            String code = normalizeCode(rawCode);
            if (StringUtils.hasText(code)) {
                codes.add(code);
            }
        }
        return new ArrayList<>(codes);
    }

    private String normalizeCode(String code) {
        return StringUtils.hasText(code) ? code.trim() : null;
    }
}
