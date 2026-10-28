package handler

import (
	"errors"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	authorizationHeader = "Authorization"
)

var secret = os.Getenv("RANDOM_SECRET")

func (h *Handler) AuthBusinessMiddleware(c *gin.Context) {
	authHeader := c.Request.Header.Get(authorizationHeader)
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Пользователь не авторизован."})
		c.Abort()
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Пользователь не авторизован.",
		})
		c.Abort()
		return
	}
	token := parts[1]

	tokenJWT, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil //TODO: ЗАТЕСТИТЬ!!!
	})

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Пользователь не авторизован.",
		})
		c.Abort()
		return
	}

	if claims, ok := tokenJWT.Claims.(jwt.MapClaims); ok && tokenJWT.Valid {
		isBusiness := claims["isBusiness"].(bool)
		if !isBusiness {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Пользователь не авторизован.",
			})
			c.Abort()
			return
		}
		if exp, ok := claims["exp"].(float64); ok {
			expirationTime := time.Unix(int64(exp), 0)
			if expirationTime.Before(time.Now()) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Token has expired"})
				c.Abort()
				return
			}
		}
	} else {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Пользователь не авторизован.",
		})
		c.Abort()
		return
	}

	businessID, err := h.service.JWTAuthMiddleware(token)
	if err != nil {
		if err.Error() == unauthorizedErr.Error() {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Пользователь не авторизован.",
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

	c.Set("businessID", businessID)

	c.Next()
}

func (h *Handler) AuthUserMiddleware(c *gin.Context) {
	authHeader := c.Request.Header.Get(authorizationHeader)
	if authHeader == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Пользователь не авторизован."})
		c.Abort()
		return
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Пользователь не авторизован."})
		c.Abort()
		return
	}
	token := parts[1]

	tokenJWT, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Пользователь не авторизован.",
		})
		c.Abort()
		return
	}

	if claims, ok := tokenJWT.Claims.(jwt.MapClaims); ok && tokenJWT.Valid {
		if exp, ok := claims["exp"].(float64); ok {
			expirationTime := time.Unix(int64(exp), 0)
			if expirationTime.Before(time.Now()) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"status":  "error",
					"message": "Пользователь не авторизован.",
				})
				c.Abort()
				return
			}
		}
		isBusiness := claims["isBusiness"].(bool)
		if isBusiness {
			c.JSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Пользователь не авторизован.",
			})
			c.Abort()
			return
		}

	} else {
		c.JSON(http.StatusUnauthorized, gin.H{
			"status":  "error",
			"message": "Пользователь не авторизован.",
		})
		c.Abort()
		return
	}

	userID, err := h.service.JWTAuthMiddleware(token)
	if err != nil {
		if errors.Is(err, errors.New("unauth")) {
			c.JSON(http.StatusUnauthorized, gin.H{"status": "error", "message": "Пользователь не авторизован."})
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": err.Error()})
		c.Abort()
		return
	}

	c.Set("userID", userID)

	c.Next()
}
