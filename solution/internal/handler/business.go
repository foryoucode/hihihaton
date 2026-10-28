package handler

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/text/language"
	"log"
	"log/slog"
	"net/http"
	"solution/validate"
	"strconv"
	"strings"
	"time"
)

const layout = "2006-01-02"

var (
	ErrServerError = errors.New("server error")
	validSorted    = []string{"active_from", "active_until"}
)

func (h *Handler) BusinessSignUp(c *gin.Context) {
	var Req validate.BusinessSignUpRequest

	if err := c.ShouldBindJSON(&Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Ошибка в данных запроса."})
		c.Abort()
		return
	}

	id, token, err := h.service.BusinessSignUp(Req)
	if err != nil {
		slog.Error(err.Error())
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Ошибка в данных запроса.",
			})
			c.Abort()
			return
		} else if id == "Conflict" {
			c.JSON(http.StatusConflict, gin.H{
				"status":  "error",
				"message": "Такой email уже зарегистрирован.",
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
		"token":      token,
		"company_id": id,
	})

}

func (h *Handler) BusinessSignIn(c *gin.Context) {
	var Req validate.BusinessSignInRequest

	if err := c.ShouldBindJSON(&Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	token, err := h.service.BusinessSignIn(Req)
	if err != nil {
		slog.Error(err.Error())
		if err.Error() == ErrServerError.Error() {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": err.Error(),
			})
			c.Abort()
			return
		} else if err.Error() == "bad request" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Ошибка в данных запроса.",
			})
			c.Abort()
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Неверный email или пароль.",
		})
		c.Abort()
		return

	}
	c.JSON(http.StatusOK, gin.H{
		"token": token,
	})

}

func (h *Handler) AddPromo(c *gin.Context) {
	var Req validate.AddPromoRequest

	if err := c.ShouldBindJSON(&Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}
	switch Req.Mode {
	case "COMMON":
		if Req.PromoCommon == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Ошибка в данных запроса.",
			})
			c.Abort()
			return
		}
	case "UNIQUE":
		if len(Req.PromoUnique) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Ошибка в данных запроса.",
			})
			c.Abort()
			return
		}
	}
	switch Req.Mode {
	case "UNIQUE":
		Req.PromoCommon = ""
	case "COMMON":
		Req.PromoUnique = nil
	}

	if Req.ActiveFrom == "" {
		Req.ActiveFrom = "1970-01-01"
		Req.ActiveFromNormal = nil
	} else {
		Req.ActiveFromNormal = &Req.ActiveFrom
	}

	if Req.ActiveUntil == "" {
		Req.ActiveUntil = "9999-12-31"
		Req.ActiveUntilNormal = nil
	} else {
		Req.ActiveUntilNormal = &Req.ActiveUntil
	}

	normal, err := checkDates(Req.ActiveFrom, Req.ActiveUntil)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if !normal {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if err := h.validator.Struct(Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if Req.Target == nil {
		slog.Error(errors.New("no target").Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	} else {
		Req.TargetWithoutLink = *Req.Target
		log.Println(Req.TargetWithoutLink)
	}

	if !((Req.Mode == "COMMON" && Req.MaxCount >= 0 && Req.MaxCount <= 100000000) || (Req.Mode == "UNIQUE" && Req.MaxCount == 1)) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if !isValidLinkA(Req.ImageUrl) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if !isValidISO3166Alpha2(Req.TargetWithoutLink.Country) && Req.TargetWithoutLink.Country != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}
	if Req.TargetWithoutLink.AgeFrom > Req.TargetWithoutLink.AgeUntil && Req.TargetWithoutLink.AgeFrom != 0 && Req.TargetWithoutLink.AgeUntil != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	businessID, _ := c.Get("businessID")
	id, err := h.service.AddPromo(Req, businessID.(string))
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		c.Abort()
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func (h *Handler) GetList(c *gin.Context) {
	limit := c.Query("limit")
	offset := c.Query("offset")
	sortBy := c.Query("sortBy")
	country := c.QueryArray("country")

	log.Println(limit, offset, sortBy, country)

	var limitNum, offsetNum int
	if limit == "" {
		limitNum = 10
	} else {
		limitNum, _ = strconv.Atoi(limit)
		if limitNum < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Ошибка в данных запроса.",
			})
			c.Abort()
			return
		}
	}
	if offset == "" {
		offsetNum = 0
	} else {
		offsetNum, _ = strconv.Atoi(offset)
		if offsetNum < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Ошибка в данных запроса.",
			})
			c.Abort()
			return
		}
	}

	if !checkValidSorted(&sortBy) || !checkIsValidCounty(country) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	idBusiness, _ := c.Get("businessID")

	Params := validate.GetListParams{
		Limit:      limitNum,
		Offset:     offsetNum,
		SortedBy:   sortBy,
		Countries:  country,
		IDBusiness: idBusiness.(string),
	}
	Results, TotalCount, err := h.service.GetPromoList(Params)
	if err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		c.Abort()
		return
	}

	if Results == nil {
		c.Header("X-Total-Count", strconv.Itoa(TotalCount))
		c.JSON(http.StatusOK, []validate.GetPromoResponse{})
		c.Abort()
		return
	}

	c.Header("X-Total-Count", strconv.Itoa(TotalCount))
	c.JSON(http.StatusOK, Results)
}

func checkValidSorted(sorted *string) bool {
	if *sorted == "" {
		*sorted = "default"
		return true
	}
	log.Println(sorted)
	log.Println(*sorted)
	for _, v := range validSorted {
		if v == *sorted {
			return true
		}
	}

	return false
}

func checkIsValidCounty(countries []string) bool {
	if len(countries) == 0 {
		return true
	}
	log.Println(len(countries))
	for _, v := range countries {
		if !isValidISO3166Alpha2(v) {
			return false
		}
	}

	return true
}

func isValidISO3166Alpha2(code string) bool {
	code = strings.ToUpper(code)
	_, err := language.ParseRegion(code)
	return err == nil
}

func (h *Handler) GetOnePromo(c *gin.Context) {
	ID := c.Param("id")
	if !IsValidUUID(ID) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	IDBusiness, _ := c.Get("businessID")
	promo, err := h.service.GetOnePromo(ID, IDBusiness.(string))
	if err != nil {
		slog.Error(err.Error())
		if err.Error() == "no promo with this id" {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Промокод не найден.",
			})
			c.Abort()
			return
		} else if err.Error() == "not this business" {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Промокод не принадлежит этой компании.",
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
	log.Println("commit")
	//jsonData, _ := json.Marshal(promo)
	c.JSON(http.StatusOK, promo)
}

func IsValidUUID(u string) bool {
	_, err := uuid.Parse(u)
	return err == nil
}

func (h *Handler) ChangeOnePromo(c *gin.Context) {
	var Req validate.ChangeOnePromoRequest

	ID := c.Param("id")
	if err := c.ShouldBindJSON(&Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if Req.ActiveFrom == "" {
		Req.ActiveFrom = "1970-01-01"
		Req.ActiveFromNormal = nil
	} else {
		Req.ActiveFromNormal = &Req.ActiveFrom
	}

	if Req.ActiveUntil == "" {
		Req.ActiveUntil = "9999-12-31"
		Req.ActiveUntilNormal = nil
	} else {
		Req.ActiveUntilNormal = &Req.ActiveUntil
	}

	normal, err := checkDates(Req.ActiveFrom, Req.ActiveUntil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if !normal {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}
	if err := h.validator.Struct(Req); err != nil {
		slog.Error(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}

	if !isValidLinkA(Req.ImageUrl) {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}
	if !isValidISO3166Alpha2(Req.TargetWithoutLink.Country) && Req.TargetWithoutLink.Country != "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Ошибка в данных запроса.",
		})
		c.Abort()
		return
	}
	if Req.Target != nil {
		Req.TargetWithoutLink = *Req.Target
		if Req.TargetWithoutLink.AgeFrom > Req.TargetWithoutLink.AgeUntil && Req.TargetWithoutLink.AgeFrom != 0 && Req.TargetWithoutLink.AgeUntil != 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Ошибка в данных запроса.",
			})
			c.Abort()
			return
		}
	}
	if Req.MaxCountL == nil {
		Req.MaxCount = -1
	} else {
		Req.MaxCount = *Req.MaxCountL
	}

	businessID, _ := c.Get("businessID")
	Result, err := h.service.ChangeOnePromo(Req, ID, businessID.(string))
	if err != nil {
		slog.Error(err.Error())
		if err.Error() == "no promo with this id" {
			c.JSON(http.StatusNotFound, gin.H{
				"status":  "error",
				"message": "Промокод не найден.",
			})
			c.Abort()
			return
		} else if err.Error() == "not this business" {
			c.JSON(http.StatusForbidden, gin.H{
				"status":  "error",
				"message": "Промокод не принадлежит этой компании.",
			})
			c.Abort()
			return
		} else if err.Error() == "400" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Ошибка в данных запроса.",
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

	c.JSON(http.StatusOK, Result)
}

func checkDates(activeFrom, activeUntil string) (bool, error) {
	if activeFrom == "1970-01-01" || activeUntil == "9999-12-31" {
		return true, nil
	}
	activeUntilN, err := time.Parse("2006-01-02", activeUntil)
	if err != nil {
		return false, err
	}
	activeFromN, err := time.Parse("2006-01-02", activeFrom)
	if err != nil {
		return false, err
	}
	log.Println(activeFrom, activeUntil)
	if activeFromN.After(activeUntilN) {
		return false, nil
	} else if activeUntilN.After(activeFromN) {
		return true, nil
	} else {
		return true, nil
	}
}
