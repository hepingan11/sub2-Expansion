package com.example.redeem.service;

import com.example.redeem.dto.BatchImportCodesRequest;
import com.example.redeem.dto.BatchImportCodesResponse;
import com.example.redeem.model.RedeemCode;
import com.example.redeem.repository.RedeemCodeRepository;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.orm.jpa.DataJpaTest;
import org.springframework.context.annotation.Import;

import java.math.BigDecimal;

import static org.assertj.core.api.Assertions.assertThat;

@DataJpaTest
@Import({RedeemCodeService.class, CodeGenerator.class})
class RedeemCodeServiceTest {

    @Autowired
    private RedeemCodeService redeemCodeService;

    @Autowired
    private RedeemCodeRepository redeemCodeRepository;

    @Test
    void batchImportKeepsCodeCaseSensitive() {
        BatchImportCodesRequest request = new BatchImportCodesRequest();
        request.setCodesText("abc ABC abc");
        request.setAmount(new BigDecimal("1.00"));

        BatchImportCodesResponse response = redeemCodeService.batchImport(request);

        assertThat(response.totalParsed()).isEqualTo(2);
        assertThat(response.imported()).isEqualTo(2);
        assertThat(response.duplicated()).isZero();
        assertThat(redeemCodeRepository.findAll())
                .extracting(RedeemCode::getCode)
                .containsExactlyInAnyOrder("abc", "ABC");
    }
}
