package validate

type SignUpUserRequest struct {
	Password  string  `json:"password" binding:"required" required:"true" validator:"password_user,min=8,max=60"`
	Name      string  `json:"name" binding:"required" required:"true" validator:"min=1,max=100"`
	Surname   string  `json:"surname" binding:"required" required:"true" validator:"min=1,max=120"`
	Email     string  `json:"email" binding:"required" required:"true" validator:"email_custom,min=8,max=120"`
	AvatarUrl *string `json:"avatar_url" validator:"url,max=350"`
	Other     struct {
		Age     int    `json:"age" binding:"required" required:"true" validator:"required,min=0,max=100"`
		Country string `json:"country" binding:"required" required:"true" validator:"required,iso-3166-1-alpha2"`
	} `json:"other" binding:"required" required:"true"`
}

type SignInUserRequest struct {
	Email    string `json:"email" required:"true" validator:"email_custom,min=8,max=120"`
	Password string `json:"password" required:"true" validator:"password_user,min=8,max=60"`
}

type UserProfileResponse struct {
	Name      string  `json:"name"`
	Surname   string  `json:"surname"`
	Email     string  `json:"email"`
	AvatarURL *string `json:"avatar_url"`
	Other     struct {
		Age     int    `json:"age"`
		Country string `json:"country"`
	} `json:"other"`
}

type ChangeUserProfileRequest struct {
	Name      *string `json:"name" validator:"omitempty,min=1,max=100"`
	Surname   *string `json:"surname" validator:"omitempty,min=1,max=120"`
	AvatarUrl *string `json:"avatar_url" validator:"omitempty,url,min=1,max=120"`
	Password  *string `json:"password" validator:"omitempty,password_user,min=8,max=60"`
}
