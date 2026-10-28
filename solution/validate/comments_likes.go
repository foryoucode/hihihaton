package validate

type CreateCommentRequest struct {
	Text string `json:"text" validate:"required,min=10,max=1000"`
}

type PutCommentResponse struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Author struct {
		Name      string `json:"name"`
		Surname   string `json:"surname"`
		AvatarURL string `json:"avatar_url"`
	} `json:"author"`
}
