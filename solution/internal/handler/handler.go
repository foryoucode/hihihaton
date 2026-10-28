package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"log/slog"
	"solution/internal/service"
	"solution/validate"
)

type Handler struct {
	service   *service.Service
	validator *validator.Validate
}

func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service, validator: validate.CreateValidator()}
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.New()

	api := router.Group("/api")
	{
		api.GET("/ping", h.Ping)
		business := api.Group("/business")
		{
			auth := business.Group("/auth")
			{
				auth.POST("/sign-up", h.BusinessSignUp)
				auth.POST("/sign-in", h.BusinessSignIn)
			}
			promo := business.Group("/promo", h.AuthBusinessMiddleware)
			{
				promo.POST("", h.AddPromo)
				promo.GET("", h.GetList)
				promo.GET(":id", h.GetOnePromo)
				promo.PATCH(":id", h.ChangeOnePromo)
			}
		}
		user := api.Group("/user")
		{
			auth := user.Group("/auth")
			{
				auth.POST("/sign-up", h.UserSignUp)
				auth.POST("/sign-in", h.UserSignIn)
			}
			profile := user.Group("/profile", h.AuthUserMiddleware)
			{
				profile.GET("", h.GetProfile)
				profile.PATCH("", h.ChangeProfile)
			}
		}
	}
	slog.Info("Routes initialized!")

	return router
}
