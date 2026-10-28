package service

import (
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
	"os"
	"solution/internal/repository"
	validate "solution/validate"
	"time"
)

var secret = os.Getenv("RANDOM_SECRET")

type Business interface {
	BusinessSignUp(request validate.BusinessSignUpRequest) (string, string, error)
	BusinessSignIn(request validate.BusinessSignInRequest) (string, error)
	JWTAuthMiddleware(token string) (string, error)
	AddPromo(request validate.AddPromoRequest, IDBusiness string) (string, error)
	GetPromoList(params validate.GetListParams) ([]validate.GetPromoResponse, int, error)
	GetOnePromo(ID, IDBusiness string) (validate.GetPromoResponse, error)
	ChangeOnePromo(request validate.ChangeOnePromoRequest, ID, IDBusiness string) (validate.GetPromoResponse, error)
}

type User interface {
	UserSignUp(request validate.SignUpUserRequest) (string, error)
	UserSignIn(request validate.SignInUserRequest) (string, error)
	GetUserProfile(ID string) (validate.UserProfileResponse, error)
	ChangeProfileUser(request validate.ChangeUserProfileRequest, ID string) (validate.UserProfileResponse, error)
}

type Service struct {
	Business
	User
}

func NewService(repo *repository.Repository) *Service {
	return &Service{
		Business: NewBusinessService(repo.Business),
		User:     NewUser(repo.User),
	}
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func GetRandomSecret() (string, error) {
	if secret == "" {
		return "", fmt.Errorf("переменная окружения RANDOM_SECRET не установлена")
	}
	return secret, nil
}

func GenerateJWT(id, email string, isBusiness bool) (string, error) {
	secretKey, err := GetRandomSecret()
	if err != nil {
		return "", err
	}

	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["exp"] = time.Now().Add(time.Hour * 72).Unix()
	claims["iat"] = time.Now().Unix()
	claims["id"] = id
	claims["email"] = email
	claims["isBusiness"] = isBusiness

	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}
