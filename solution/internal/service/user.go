package service

import (
	"errors"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"solution/internal/repository"
	"solution/validate"
)

type UserService struct {
	repo repository.User
}

func NewUser(repo repository.User) *UserService {
	return &UserService{repo: repo}
}

func (s *UserService) UserSignUp(request validate.SignUpUserRequest) (string, error) {
	HashedPassword, err := HashPassword(request.Password)
	if err != nil {
		return "", errors.New("server error")
	}

	request.Password = HashedPassword

	ID, err := s.repo.UserSignUp(uuid.New().String(), request)
	if err != nil {
		return "", err
	}

	token, err := GenerateJWT(ID, request.Email, false)
	if err != nil {
		return "", errors.New("server error")
	}

	token, err = s.repo.AddJWTToRedis(ID, token)
	if err != nil {
		return "", errors.New("server error")
	}

	return token, nil
}

func (s *UserService) UserSignIn(request validate.SignInUserRequest) (string, error) {
	ID, err := s.repo.UserSignIn(request.Email, request.Password)
	if err != nil {
		return "", err
	}

	token, err := GenerateJWT(ID, request.Email, false)
	if err != nil {
		return "", errors.New("server error")
	}

	token, err = s.repo.AddJWTToRedis(ID, token)
	if err != nil {
		return "", errors.New("server error")
	}

	return token, nil
}

func (s *UserService) GetUserProfile(ID string) (validate.UserProfileResponse, error) {
	var user validate.UserProfileResponse
	var err error
	user, err = s.repo.GetUserProfile(ID)
	if err != nil {
		return user, err
	}

	return user, nil
}

func (s *UserService) ChangeProfileUser(request validate.ChangeUserProfileRequest, ID string) (validate.UserProfileResponse, error) {
	var user validate.UserProfileResponse
	var err error

	if request.Password != nil {
		request.Password, err = HashPasswordLink(*request.Password)
		if err != nil {
			return user, errors.New("server error")
		}
	}
	user, err = s.repo.ChangeProfile(request, ID)
	if err != nil {
		return user, err
	}

	return user, nil
}

func HashPasswordLink(password string) (*string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	str := string(bytes)
	return &str, err
}
