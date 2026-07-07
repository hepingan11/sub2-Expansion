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
	UserID string `json:"userId"`
}

type CheckInResponse struct {
	Success          bool       `json:"success"`
	AlreadyCheckedIn bool       `json:"alreadyCheckedIn"`
	UserID           *string    `json:"userId"`
	SignDate         *LocalDate `json:"signDate"`
	Code             string     `json:"code"`
	Amount           Amount     `json:"amount"`
	Message          string     `json:"message"`
}

type PrizeTier struct {
	Amount      Amount `json:"amount"`
	Probability Amount `json:"probability"`
}

type CheckInSettingsResponse struct {
	DailyMaxUsers int           `json:"dailyMaxUsers"`
	PrizeTiers    []PrizeTier   `json:"prizeTiers"`
	Sub2API       Sub2APIConfig `json:"sub2api"`
}

type UpdateCheckInSettingsRequest struct {
	DailyMaxUsers int           `json:"dailyMaxUsers"`
	PrizeTiers    []PrizeTier   `json:"prizeTiers"`
	Sub2API       Sub2APIConfig `json:"sub2api"`
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
