package validate

type GetHistoryResponse struct {
	PromoID           string `json:"promo_id"`
	CompanyID         string `json:"company_id"`
	CompanyName       string `json:"company_name"`
	Description       string `json:"description"`
	ImageURL          string `json:"image_url"`
	Active            bool   `json:"active"`
	IsActivatedByUser bool   `json:"is_activated_by_user"`
	LikeCount         int    `json:"like_count"`
	IsLikedByUser     bool   `json:"is_liked_by_user"`
	CommentCount      int    `json:"comment_count"`
}

type GetStatsResponse struct {
	ActiveCount int       `json:"active_count"`
	Countries   []Country `json:"countries"`
}

type Country struct {
	Country          string `json:"country"`
	ActivationsCount int    `json:"activations_count"`
}
