package validate

//import "database/sql"

type BusinessSignUpRequest struct {
	Name     string `json:"name" required:"true"`
	Email    string `json:"email" required:"true"`
	Password string `json:"password" required:"true"`
}

type BusinessSignInRequest struct {
	Email    string `json:"email" required:"true"`
	Password string `json:"password" required:"true"`
}

type Target struct {
	AgeFrom  int    `json:"age_from" validate:"omitempty,min=0,max=100"`
	AgeUntil int    `json:"age_until" validate:"omitempty,min=0,max=100"`
	Country  string `json:"country" validate:"omitempty,min=2,max=2"`
	//Country    string   `json:"country" validate:"iso3166_1_alpha2"`
	Categories []string `json:"categories" validate:"omitempty,max=20,dive,min=2,max=20"`
}

func (t *Target) IsEmpty() bool {
	return t.AgeFrom == 0 && t.AgeUntil == 0 && t.Country == "" && len(t.Categories) == 0
}

type AddPromoRequest struct {
	Mode              string   `json:"mode" required:"true" validate:"mode"`
	PromoCommon       string   `json:"promo_common" validate:"omitempty,required_if=Mode COMMON,min=5,max=30"`
	PromoUnique       []string `json:"promo_unique" validate:"omitempty,required_if=Mode UNIQUE,min=1,max=5000,dive,min=3,max=30"`
	Description       string   `json:"description" required:"true" validate:"min=10,max=300"`
	ImageUrl          string   `json:"image_url" required:"false" validate:"omitempty,url,max=350"`
	Target            *Target  `json:"target" validate:"required"`
	MaxCount          int      `json:"max_count" required:"true"`
	ActiveFrom        string   `json:"active_from" validate:"omitempty,date,min=10,max=10"`
	ActiveUntil       string   `json:"active_until" validate:"omitempty,date,min=10,max=10"`
	TargetWithoutLink Target
	ActiveFromNormal  *string
	ActiveUntilNormal *string
}

type GetListParams struct {
	Limit      int
	Offset     int
	SortedBy   string
	Countries  []string
	IDBusiness string
}

type GetPromoResponse struct {
	PromoId     string   `json:"promo_id"`
	CompanyId   string   `json:"company_id"`
	CompanyName string   `json:"company_name"`
	LikeCount   int      `json:"like_count"`
	UsedCount   int      `json:"used_count"`
	Active      bool     `json:"active"`
	Mode        string   `json:"mode"`
	PromoCommon string   `json:"promo_common,omitempty"`
	PromoUnique []string `json:"promo_unique,omitempty"`
	Description string   `json:"description"`
	ImageUrl    string   `json:"image_url,omitempty"`
	Target      TargetN  `json:"target"`
	MaxCount    int      `json:"max_count"`
	ActiveFrom  *string  `json:"active_from,omitempty"`
	ActiveUntil *string  `json:"active_until,omitempty"`
}

type TargetN struct {
	AgeFrom    int      `json:"age_from"`
	AgeUntil   int      `json:"age_until"`
	Country    string   `json:"country,omitempty"`
	Categories []string `json:"categories,omitempty"`
}

type ChangeOnePromoRequest struct {
	Description       string  `json:"description" validate:"omitempty,min=10,max=300"`
	ImageUrl          string  `json:"image_url" validate:"omitempty,url,max=350"`
	Target            *Target `json:"target"`
	MaxCountL         *int    `json:"max_count"`
	ActiveFrom        string  `json:"active_from" validate:"omitempty,date,min=10,max=10"`
	ActiveUntil       string  `json:"active_until" validate:"omitempty,date,min=10,max=10"`
	TargetWithoutLink Target
	ActiveFromNormal  *string
	ActiveUntilNormal *string
	MaxCount          int
}
