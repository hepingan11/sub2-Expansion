package com.example.redeem.dto;

import java.util.List;

public record BatchImportCodesResponse(int totalParsed, int imported, int duplicated, List<String> duplicatedCodes) {
}
