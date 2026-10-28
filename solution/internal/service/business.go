package service

import (
	"errors"
	"github.com/google/uuid"
	"log"
	"regexp"
	"solution/internal/repository"
	"solution/validate"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	reEmail = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

const layout = "2006-01-02"

type BusinessService struct {
	repo repository.Business
}

func NewBusinessService(repo repository.Business) *BusinessService {
	return &BusinessService{repo: repo}
}

func (s *BusinessService) BusinessSignUp(businessInfo validate.BusinessSignUpRequest) (string, string, error) {
	if isValidPassword(businessInfo.Password) && isValidName(businessInfo.Name) && isValidEmail(businessInfo.Email) {
		hashedPassword, err := HashPassword(businessInfo.Password)
		if err != nil {
			return "Server Error", "", err
		}
		id, err := s.repo.BusinessSignUp(businessInfo.Name, hashedPassword, businessInfo.Email, uuid.New().String())
		if err != nil {
			if id == "" {
				return "Server Error", "", err
			} else {
				return "Conflict", "", err
			}
		}
		token, err := GenerateJWT(id, businessInfo.Email, true)
		if err != nil {
			return "Server Error", "", err
		}
		token, err = s.repo.AddJWTToRedis(id, token)
		if err != nil {
			return "Server Error", "", err
		}
		return id, token, nil
	} else {
		return "", "", errors.New("bad request")
	}
}

func (s *BusinessService) BusinessSignIn(businessInfo validate.BusinessSignInRequest) (string, error) {
	if isValidPassword(businessInfo.Password) && isValidEmail(businessInfo.Email) {
		id, err := s.repo.BusinessSignIn(businessInfo.Email, businessInfo.Password)
		if err != nil {
			return "", err
		}

		token, err := GenerateJWT(id, businessInfo.Email, true)
		if err != nil {
			return "", errors.New("server error")
		}

		token, err = s.repo.AddJWTToRedis(id, token)
		if err != nil {
			return "", errors.New("server error")
		}

		return token, nil
	}

	return "", errors.New("bad request")

}

func (s *BusinessService) JWTAuthMiddleware(token string) (string, error) {
	return s.repo.GetIDByJWTFromRedis(token)
}

func (s *BusinessService) AddPromo(info validate.AddPromoRequest, IDBusiness string) (string, error) {
	//until, err := time.Parse(layout, info.ActiveUntil)
	//if err != nil {
	//	return "", errors.New("server error")
	//}
	//
	//from, err := time.Parse(layout, info.ActiveFrom)
	//if err != nil {
	//	return "", errors.New("server error")
	//}
	//
	//info.ActiveFrom = fmt.Sprintf("%d", from.Unix())
	//info.ActiveUntil = fmt.Sprintf("%d", until.Unix())

	id, err := s.repo.AddPromo(info, uuid.New().String(), IDBusiness)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (s *BusinessService) GetPromoList(Params validate.GetListParams) ([]validate.GetPromoResponse, int, error) {
	Results, TotalCount, err := s.repo.GetPromoList(Params)
	if err != nil {
		return nil, 0, err
	}

	return Results, TotalCount, nil
}

func isValidPassword(password string) bool {
	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSpecial := false

	specialChars := "@$!%*?&"

	for _, char := range password {
		switch {
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsDigit(char):
			hasDigit = true
		case strings.ContainsRune(specialChars, char):
			hasSpecial = true
		}
	}

	lenPassword := utf8.RuneCountInString(password)
	isValidLengthPassword := lenPassword >= 8 && lenPassword <= 60

	return hasLower && hasUpper && hasDigit && hasSpecial && isValidLengthPassword
}

func isValidEmail(email string) bool {
	isEmail := reEmail.MatchString(email)
	lenEmail := utf8.RuneCountInString(email)
	isValidLengthEmail := lenEmail >= 8 && lenEmail <= 120

	return isEmail && isValidLengthEmail
}

func isValidName(name string) bool {
	lenName := utf8.RuneCountInString(name)
	return lenName >= 5 && lenName <= 50
}

func (s *BusinessService) GetOnePromo(ID, IDBusiness string) (validate.GetPromoResponse, error) {
	var promo validate.GetPromoResponse
	var err error

	promo, err = s.repo.GetOnePromo(ID, IDBusiness)
	if err != nil {
		return promo, err
	}

	return promo, nil
}

func (s *BusinessService) ChangeOnePromo(request validate.ChangeOnePromoRequest, ID, IDBusiness string) (validate.GetPromoResponse, error) {
	var promo validate.GetPromoResponse
	var err error

	promo, err = s.repo.ChangeOnePromo(request, ID, IDBusiness)
	if err != nil {
		log.Println(err.Error())
		return promo, err
	}

	return promo, nil
}
