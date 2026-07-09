package app

type AdminLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AdminLoginResponse struct {
	Token          string `json:"token"`
	ExpiresInHours int64  `json:"expiresInHours"`
}

type RedeemCodeRequest struct {
	Code     string     `json:"code"`
	UserID   string     `json:"userId"`
	SignDate *LocalDate `json:"signDate"`
	Amount   Amount     `json:"amount"`
	Status   string     `json:"status"`
}

type BatchImportCodesRequest struct {
	CodesText string `json:"codesText"`
	Amount    Amount `json:"amount"`
}

type BatchImportCodesResponse struct {
	TotalParsed     int      `json:"totalParsed"`
	Imported        int      `json:"imported"`
	Duplicated      int      `json:"duplicated"`
	DuplicatedCodes []string `json:"duplicatedCodes"`
}

type FavoriteSiteRequest struct {
	Icon        string `json:"icon"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sort        int    `json:"sort"`
	Group       string `json:"group"`
}

type DashboardStatsResponse struct {
	Total       int64             `json:"total"`
	Available   int64             `json:"available"`
	Assigned    int64             `json:"assigned"`
	Used        int64             `json:"used"`
	Voided      int64             `json:"voided"`
	AmountStats []AmountStatEntry `json:"amountStats"`
}

type AmountStatEntry struct {
	Amount    Amount `json:"amount"`
	Total     int64  `json:"total"`
	Available int64  `json:"available"`
}

type CheckInRequest struct {
	UserID       string `json:"userId"`
	PlatformType string `json:"platformType"`
	Platform     string `json:"platform"`
}

type CheckInResponse struct {
	Success          bool       `json:"success"`
	AlreadyCheckedIn bool       `json:"alreadyCheckedIn"`
	UserID           *string    `json:"userId"`
	SignDate         *LocalDate `json:"signDate"`
	Code             string     `json:"code"`
	Amount           Amount     `json:"amount"`
	CheckInMethod    string     `json:"checkInMethod"`
	PlatformType     string     `json:"platformType,omitempty"`
	Message          string     `json:"message"`
}

type SocialBindingRequiredResponse struct {
	Message        string `json:"message"`
	Code           string `json:"code"`
	Platform       string `json:"platform"`
	UserID         string `json:"userId"`
	ExternalUserID string `json:"externalUserId"`
	BindingURL     string `json:"bindingUrl"`
}

type CheckInStatsResponse struct {
	TodayAmount Amount                  `json:"todayAmount"`
	TodayUsers  int64                   `json:"todayUsers"`
	Daily       []DailyCheckInStatEntry `json:"daily"`
}

type DailyCheckInStatEntry struct {
	SignDate LocalDate `json:"signDate"`
	Amount   Amount    `json:"amount"`
	Users    int64     `json:"users"`
}

type PrizeTier struct {
	Amount      Amount `json:"amount"`
	Probability Amount `json:"probability"`
}

type CheckInSettingsResponse struct {
	DailyMaxUsers       int           `json:"dailyMaxUsers"`
	DailyLimitMode      string        `json:"dailyLimitMode"`
	DirectDailyMaxUsers int           `json:"directDailyMaxUsers"`
	SocialDailyMaxUsers int           `json:"socialDailyMaxUsers"`
	PrizeTiers          []PrizeTier   `json:"prizeTiers"`
	DirectPrizeTiers    []PrizeTier   `json:"directPrizeTiers"`
	SocialPrizeTiers    []PrizeTier   `json:"socialPrizeTiers"`
	Admin               AdminConfig   `json:"admin"`
	Sub2API             Sub2APIConfig `json:"sub2api"`
}

type UpdateCheckInSettingsRequest struct {
	DailyMaxUsers       int           `json:"dailyMaxUsers"`
	DailyLimitMode      string        `json:"dailyLimitMode"`
	DirectDailyMaxUsers int           `json:"directDailyMaxUsers"`
	SocialDailyMaxUsers int           `json:"socialDailyMaxUsers"`
	PrizeTiers          []PrizeTier   `json:"prizeTiers"`
	DirectPrizeTiers    []PrizeTier   `json:"directPrizeTiers"`
	SocialPrizeTiers    []PrizeTier   `json:"socialPrizeTiers"`
	Admin               AdminConfig   `json:"admin"`
	Sub2API             Sub2APIConfig `json:"sub2api"`
}

type AdminConfig struct {
	Username    string `json:"username"`
	Password    string `json:"password,omitempty"`
	PasswordSet bool   `json:"passwordSet"`
}

type Sub2APIConfig struct {
	BaseURL          string `json:"baseUrl"`
	AuthMode         string `json:"authMode"`
	AdminAPIKey      string `json:"adminApiKey,omitempty"`
	AdminAPIKeySet   bool   `json:"adminApiKeySet"`
	AdminEmail       string `json:"adminEmail"`
	AdminPassword    string `json:"adminPassword,omitempty"`
	AdminPasswordSet bool   `json:"adminPasswordSet"`
	TimeoutSeconds   int    `json:"timeoutSeconds"`
}

type RechargeActivityRequest struct {
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	Enabled     bool                        `json:"enabled"`
	StartAt     string                      `json:"startAt"`
	EndAt       string                      `json:"endAt"`
	Tiers       []RechargeRewardTierRequest `json:"tiers"`
}

type RechargeRewardTierRequest struct {
	ID              uint64 `json:"id"`
	ThresholdAmount Amount `json:"thresholdAmount"`
	RewardAmount    Amount `json:"rewardAmount"`
	Sort            int    `json:"sort"`
}

type RechargeActivityResponse struct {
	ID          uint64                       `json:"id"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Enabled     bool                         `json:"enabled"`
	StartAt     *JSONTime                    `json:"startAt"`
	EndAt       *JSONTime                    `json:"endAt"`
	CreatedAt   JSONTime                     `json:"createdAt"`
	UpdatedAt   JSONTime                     `json:"updatedAt"`
	Tiers       []RechargeRewardTierResponse `json:"tiers"`
}

type RechargeRewardTierResponse struct {
	ID              uint64   `json:"id"`
	ActivityID      uint64   `json:"activityId"`
	ThresholdAmount Amount   `json:"thresholdAmount"`
	RewardAmount    Amount   `json:"rewardAmount"`
	Sort            int      `json:"sort"`
	CreatedAt       JSONTime `json:"createdAt"`
	UpdatedAt       JSONTime `json:"updatedAt"`
}

type UserRechargeRewardsResponse struct {
	TotalRecharged Amount                         `json:"totalRecharged"`
	Activities     []UserRechargeActivityResponse `json:"activities"`
}

type UserRechargeActivityResponse struct {
	ID          uint64                           `json:"id"`
	Name        string                           `json:"name"`
	Description string                           `json:"description"`
	StartAt     *JSONTime                        `json:"startAt"`
	EndAt       *JSONTime                        `json:"endAt"`
	Tiers       []UserRechargeRewardTierResponse `json:"tiers"`
}

type UserRechargeRewardTierResponse struct {
	ID              uint64   `json:"id"`
	ThresholdAmount Amount   `json:"thresholdAmount"`
	RewardAmount    Amount   `json:"rewardAmount"`
	Eligible        bool     `json:"eligible"`
	Claimed         bool     `json:"claimed"`
	ClaimStatus     string   `json:"claimStatus"`
	RedeemCode      string   `json:"redeemCode,omitempty"`
	ClaimedAt       JSONTime `json:"claimedAt,omitempty"`
}

type ClaimRechargeRewardResponse struct {
	ClaimID      uint64 `json:"claimId"`
	RedeemCode   string `json:"redeemCode"`
	RewardAmount Amount `json:"rewardAmount"`
}

type AdminRechargeRewardClaimResponse struct {
	ID              uint64   `json:"id"`
	ActivityID      uint64   `json:"activityId"`
	ActivityName    string   `json:"activityName"`
	TierID          uint64   `json:"tierId"`
	TierSort        int      `json:"tierSort"`
	UserID          int64    `json:"userId"`
	ThresholdAmount Amount   `json:"thresholdAmount"`
	RewardAmount    Amount   `json:"rewardAmount"`
	Status          string   `json:"status"`
	RedeemCode      string   `json:"redeemCode"`
	ErrorMessage    string   `json:"errorMessage"`
	CreatedAt       JSONTime `json:"createdAt"`
	UpdatedAt       JSONTime `json:"updatedAt"`
}

type PageResponse[T any] struct {
	Content       []T   `json:"content"`
	TotalElements int64 `json:"totalElements"`
	TotalPages    int   `json:"totalPages"`
	Number        int   `json:"number"`
	Size          int   `json:"size"`
}

type APIError struct {
	Message string `json:"message"`
}

type sub2APIResponse[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type sub2APIRedeemCode struct {
	Code string `json:"code"`
}

type sub2APILoginData struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type BusinessConflict string
