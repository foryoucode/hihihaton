package repository

import (
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"solution/validate"
)

const (
	businessTable = "business"
	promosTable   = "promos"
	usersTable    = "users"
)

type Business interface {
	BusinessSignUp(name, password, email, id string) (string, error)
	BusinessSignIn(email, password string) (string, error)
	GetIDByJWTFromRedis(token string) (string, error)
	AddPromo(info validate.AddPromoRequest, id, IDBusiness string) (string, error)
	AddJWTToRedis(id, token string) (string, error)
	GetPromoList(Params validate.GetListParams) ([]validate.GetPromoResponse, int, error)
	GetCountPromos(params validate.GetListParams) (int, error)
	GetOnePromo(ID, IDBusiness string) (validate.GetPromoResponse, error)
	ChangeOnePromo(info validate.ChangeOnePromoRequest, ID, IDBusiness string) (validate.GetPromoResponse, error)
}

type User interface {
	UserSignUp(ID string, userInfo validate.SignUpUserRequest) (string, error)
	AddJWTToRedis(id, token string) (string, error)
	UserSignIn(email, HashPassword string) (string, error)
	GetUserProfile(ID string) (validate.UserProfileResponse, error)
	ChangeProfile(UserInfo validate.ChangeUserProfileRequest, ID string) (validate.UserProfileResponse, error)
}

type Repository struct {
	Business
	User
}

func NewRepository(db *sqlx.DB, rdb *redis.Client) *Repository {
	return &Repository{
		Business: NewBusiness(db, rdb),
		User:     NewUser(db, rdb),
	}
}
