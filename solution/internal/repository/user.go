package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"log"
	"solution/validate"
)

var conflictErr = errors.New("conflict")

type UserPostgresAndRedis struct {
	db  *sqlx.DB
	rdb *redis.Client
}

func NewUser(db *sqlx.DB, rdb *redis.Client) *UserPostgresAndRedis {
	return &UserPostgresAndRedis{
		db:  db,
		rdb: rdb,
	}
}

func (r *UserPostgresAndRedis) UserSignUp(ID string, userInfo validate.SignUpUserRequest) (string, error) {
	var exists bool
	checkQuery := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE email = $1)", usersTable)
	err := r.db.QueryRow(checkQuery, userInfo.Email).Scan(&exists)
	if err != nil {
		return "", err
	}

	if exists {
		return "", conflictErr
	}

	query := fmt.Sprintf("INSERT INTO %s (id, name, email, password, surname, avatar_url, age, country) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id;", usersTable)

	row := r.db.QueryRow(query, ID, userInfo.Name, userInfo.Email, userInfo.Password, userInfo.Surname, &userInfo.AvatarUrl, userInfo.Other.Age, userInfo.Other.Country)
	if err = row.Scan(&ID); err != nil {
		return "", err
	}
	return ID, nil
}

func (r *UserPostgresAndRedis) AddJWTToRedis(id, token string) (string, error) {
	if err := r.rdb.Set(ctx, token, id, 0).Err(); err != nil {
		return "", err
	}

	return token, nil
}

func (r *UserPostgresAndRedis) UserSignIn(email, password string) (string, error) {
	var ID, hashedPassword string
	fmt.Println(email, password)
	query := fmt.Sprintf("SELECT id, password FROM %s WHERE email = $1", usersTable)
	err := r.db.QueryRow(query, email).Scan(&ID, &hashedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("unauth")
		}
		return "", err
	}
	log.Println(hashedPassword, password)
	if err = CheckPassword(hashedPassword, password); err != nil {
		log.Println(err)
		return "", errors.New("unauth")
	}

	token, err := r.rdb.Get(ctx, ID).Result()
	if err != nil {
		if err.Error() != redis.Nil.Error() {
			return "", err
		}
	}

	_, err = r.rdb.Del(ctx, token).Result()
	if err != nil {
		return "", err
	}

	return ID, nil
}

func (r *UserPostgresAndRedis) GetUserProfile(ID string) (validate.UserProfileResponse, error) {
	var result validate.UserProfileResponse

	query := fmt.Sprintf("SELECT name, email, surname, avatar_url, age, country FROM %s WHERE id = $1", usersTable)

	err := r.db.QueryRow(query, ID).Scan(&result.Name, &result.Email, &result.Surname, &result.AvatarURL, &result.Other.Age, &result.Other.Country)
	if err != nil {
		log.Println(err)
		if errors.Is(err, sql.ErrNoRows) {
			return result, errors.New("no user with this info")
		}
		return result, errors.New("server error")
	}

	return result, nil
}

func (r *UserPostgresAndRedis) ChangeProfile(UserInfo validate.ChangeUserProfileRequest, ID string) (validate.UserProfileResponse, error) {
	var result validate.UserProfileResponse

	query := fmt.Sprintf(`UPDATE %s SET 
                        name = COALESCE(NULLIF($1, ''), name),
                        surname = COALESCE(NULLIF($2, ''), surname),
                        avatar_url = COALESCE(NULLIF($3, ''), avatar_url),
                        password = COALESCE(NULLIF($4, ''), password)
                      WHERE id = $5
                      RETURNING name, email, surname, avatar_url, age, country`, usersTable)

	err := r.db.QueryRow(query, &UserInfo.Name, &UserInfo.Surname, &UserInfo.AvatarUrl, &UserInfo.Password, ID).Scan(&result.Name, &result.Email, &result.Surname, &result.AvatarURL, &result.Other.Age, &result.Other.Country)
	if err != nil {
		log.Println(err.Error())
		if errors.Is(err, sql.ErrNoRows) {
			return result, errors.New("no user with this info")
		}
		return result, errors.New("server error")
	}

	return result, nil
}
