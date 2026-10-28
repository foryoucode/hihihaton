package validate

import (
	"github.com/dlclark/regexp2"
	"github.com/go-playground/validator/v10"
	"log"
	"regexp"
	"time"
)

var (
	validModes = []string{"COMMON", "UNIQUE"}
	rePassword = regexp2.MustCompile(`^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[@$!%*?&])[A-Za-z\d@$!%*?&]{8,}$`, 0)
	reEmail    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
)

func CreateValidator() *validator.Validate {
	var err error

	validate := validator.New()
	if err = validate.RegisterValidation("mode", ModeValidator); err != nil {
		log.Fatal(err)
	}
	if err = validate.RegisterValidation("password_user", PasswordValidator); err != nil {
		log.Fatal(err)
	}
	if err = validate.RegisterValidation("date", DateValidate); err != nil {
		log.Fatal(err)
	}
	if err = validate.RegisterValidation("email_custom", EmailCustomValidate); err != nil {
		log.Fatal(err)
	}
	return validate
}

func PasswordValidator(fl validator.FieldLevel) bool {
	password := fl.Field().String()

	match, _ := rePassword.MatchString(password)
	return match
}

func ModeValidator(fl validator.FieldLevel) bool {
	status := fl.Field().String()

	for _, v := range validModes {
		if v == status {
			return true
		}
	}

	return false
}

func DateValidate(fl validator.FieldLevel) bool {
	date := fl.Field().String()
	//fmt.Println("Validating date:", date)
	if date == "0000-00-00" || date == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", date)
	return err == nil
}

func EmailCustomValidate(fl validator.FieldLevel) bool {
	email := fl.Field().String()

	match := reEmail.MatchString(email)
	return match
}
