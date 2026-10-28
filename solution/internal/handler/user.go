package handler

import (
	"errors"
	"github.com/dlclark/regexp2"
	"github.com/gin-gonic/gin"
	"log"
	"log/slog"
	"net/http"
	"regexp"
	"solution/validate"
	"strings"
	"unicode/utf8"
)

var (
	reEmail         = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	conflictErr     = errors.New("conflict")
	unauthorizedErr = errors.New("unauth")
	rePassword      = regexp2.MustCompile(`^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{8,}$`, 0)
)

func (h *Handler) UserSignUp(c *gin.Context) {
	var Req validate.SignUpUserRequest

	if err := c.ShouldBindJSON(&Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if !isValidEmail(Req.Email) || !isValidPasswordNotLink(Req.Password) || !isValidLink(Req.AvatarUrl) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if err := h.validator.Struct(Req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	token, err := h.service.UserSignUp(Req)
	if err != nil {
		if err.Error() == conflictErr.Error() {
			c.JSON(http.StatusConflict, gin.H{
				"status":  "error",
				"message": "Такой email уже зарегистрирован.",
			})
			c.Abort()
			return
		}
		slog.Error("Unexpected error: ", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})
}

func (h *Handler) UserSignIn(c *gin.Context) {
	var Req validate.SignInUserRequest

	if err := c.ShouldBindJSON(&Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if !isValidEmail(Req.Email) || !isValidPasswordNotLink(Req.Password) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if err := h.validator.Struct(Req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	token, err := h.service.UserSignIn(Req)
	if err != nil {
		slog.Error(err.Error())
		if err.Error() == unauthorizedErr.Error() {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Неверный email или пароль.",
			})
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})
}

func isValidPasswordNotLink(password string) bool {
	if password == "" {
		return false
	}
	match, _ := rePassword.MatchString(password)
	return match
}

func (h *Handler) GetProfile(c *gin.Context) {
	var response validate.UserProfileResponse
	var err error

	userID, _ := c.Get("userID")
	response, err = h.service.GetUserProfile(userID.(string))
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{})
		c.Abort()
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *Handler) ChangeProfile(c *gin.Context) {
	var Req validate.ChangeUserProfileRequest
	var response validate.UserProfileResponse
	var err error

	if err = c.ShouldBindJSON(&Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if !isValidPassword(Req.Password) || !isValidLink(Req.AvatarUrl) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if err = h.validator.Struct(Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if !isValidName(Req.Name) || !isValidSurName(Req.Surname) {
		log.Println("name/surname:")
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}
	userID, _ := c.Get("userID")

	response, err = h.service.ChangeProfileUser(Req, userID.(string))
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
	}

	c.JSON(http.StatusOK, response)
}

func isValidEmail(email string) bool {
	return reEmail.MatchString(email)
}

func isValidPassword(password *string) bool {
	if password == nil {
		return true
	}
	match, _ := rePassword.MatchString(*password)
	return match
}

func isValidLink(link *string) bool {
	if link == nil {
		return true
	}
	re := regexp.MustCompile(`^(https?://|)([a-zA-Z0-9.-]+)(:[0-9]+)?(/.*)?$`)
	return re.MatchString(*link) && strings.Contains(*link, ".")
}

func isValidName(name *string) bool {
	if name == nil {
		return true
	}
	length := utf8.RuneCountInString(*name)
	return length > 0 && length <= 100
}

func isValidSurName(Surname *string) bool {
	if Surname == nil {
		return true
	}
	length := utf8.RuneCountInString(*Surname)
	return length > 0 && length <= 120
}
func isValidLinkA(link string) bool {
	if link == "" {
		return true
	}
	re := regexp.MustCompile(`^(https?://|)([a-zA-Z0-9.-]+)(:[0-9]+)?(/.*)?$`)
	return re.MatchString(link) && strings.Contains(link, ".")
}
