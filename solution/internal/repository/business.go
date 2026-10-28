package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"log"
	"os"
	"solution/validate"
	"strings"
	"time"
)

var (
	redisHost = os.Getenv("REDIS_HOST")
	redisPort = os.Getenv("REDIS_PORT")
	ctx       = context.Background()
)

type BusinessPostgresAndRedis struct {
	db  *sqlx.DB
	rdb *redis.Client
}

func NewBusiness(db *sqlx.DB, rdb *redis.Client) *BusinessPostgresAndRedis {
	return &BusinessPostgresAndRedis{
		db:  db,
		rdb: rdb,
	}
}

func (r *BusinessPostgresAndRedis) BusinessSignUp(name, password, email, id string) (string, error) {
	var exists bool
	checkQuery := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE email = $1)", businessTable)
	err := r.db.QueryRow(checkQuery, email).Scan(&exists)
	if err != nil {
		return "", err
	}

	if exists {
		return "Conflict", fmt.Errorf("пользователь с этой почтой уже существует")
	}

	query := fmt.Sprintf("INSERT INTO %s (id, name, email, password) VALUES ($1, $2, $3, $4) RETURNING id", businessTable)

	row := r.db.QueryRow(query, id, name, email, password)
	if err := row.Scan(&id); err != nil {
		return "", err
	}
	return id, nil
}

func (r *BusinessPostgresAndRedis) BusinessSignIn(email, password string) (string, error) {
	var id, hashedPassword string

	query := fmt.Sprintf("SELECT id, password FROM %s WHERE email = $1", businessTable)
	err := r.db.QueryRow(query, email).Scan(&id, &hashedPassword)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("no users with this info")
		}
		return "", err
	}

	if err = CheckPassword(hashedPassword, password); err != nil {
		log.Println(err)
		return "", errors.New("bad password")
	}

	token, err := r.rdb.Get(ctx, id).Result()
	if err != nil {
		if err.Error() != redis.Nil.Error() {
			return "", err
		}
	}

	_, err = r.rdb.Del(ctx, token).Result()
	if err != nil {
		return "", err
	}

	return id, nil
}

func (r *BusinessPostgresAndRedis) AddJWTToRedis(id, token string) (string, error) {
	if err := r.rdb.Set(ctx, token, id, 0).Err(); err != nil {
		return "", err
	}
	if err := r.rdb.Set(ctx, id, token, 0).Err(); err != nil {
		return "", err
	}

	return token, nil
}

func (r *BusinessPostgresAndRedis) GetIDByJWTFromRedis(token string) (string, error) {
	userID, err := r.rdb.Get(ctx, token).Result()
	if errors.Is(err, redis.Nil) {
		return "", errors.New("unauth")
	} else if err != nil {
		return "", err
	}

	return userID, nil
}

func CheckPassword(hashedPassword, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

func (r *BusinessPostgresAndRedis) AddPromo(promo validate.AddPromoRequest, id, IDBusiness string) (string, error) {
	var ID string

	query := fmt.Sprintf(`INSERT INTO %s 
    							(id, mode, promo_common, promo_unique, description, 
    							 image_url, age_from, age_until, country, categories, max_count, active_from, active_until, 
    							 id_business, created_at) 

								VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id; `, promosTable)
	log.Println(promo.PromoUnique)
	//log.Println(id, promo.Mode, promo.PromoCommon, pq.Array(promo.PromoUnique), promo.Description, promo.ImageUrl, targetJSON, promo.MaxCount, promo.ActiveFrom, promo.ActiveUntil, IDBusiness)
	if promo.ActiveUntilNormal == nil && promo.ActiveFromNormal == nil {
		row := r.db.QueryRow(query, id, promo.Mode, promo.PromoCommon, pq.Array(promo.PromoUnique), promo.Description, promo.ImageUrl, promo.TargetWithoutLink.AgeFrom, promo.TargetWithoutLink.AgeUntil, promo.TargetWithoutLink.Country, pq.Array(promo.TargetWithoutLink.Categories), promo.MaxCount, nil, nil, IDBusiness, time.Now())
		if err := row.Scan(&ID); err != nil {
			log.Println(err)
			return "", errors.New("server error")
		}
		return ID, nil

	} else if promo.ActiveFromNormal == nil {
		row := r.db.QueryRow(query, id, promo.Mode, promo.PromoCommon, pq.Array(promo.PromoUnique), promo.Description, promo.ImageUrl, promo.TargetWithoutLink.AgeFrom, promo.TargetWithoutLink.AgeUntil, promo.TargetWithoutLink.Country, pq.Array(promo.TargetWithoutLink.Categories), promo.MaxCount, nil, *promo.ActiveUntilNormal, IDBusiness, time.Now())
		if err := row.Scan(&ID); err != nil {
			log.Println(err)
			return "", errors.New("server error")
		}
		return ID, nil
	} else if promo.ActiveUntilNormal == nil {
		row := r.db.QueryRow(query, id, promo.Mode, promo.PromoCommon, pq.Array(promo.PromoUnique), promo.Description, promo.ImageUrl, promo.TargetWithoutLink.AgeFrom, promo.TargetWithoutLink.AgeUntil, promo.TargetWithoutLink.Country, pq.Array(promo.TargetWithoutLink.Categories), promo.MaxCount, *promo.ActiveFromNormal, nil, IDBusiness, time.Now())
		if err := row.Scan(&ID); err != nil {
			log.Println(err)
			return "", errors.New("server error")
		}
		return ID, nil
	} else {
		row := r.db.QueryRow(query, id, promo.Mode, promo.PromoCommon, pq.Array(promo.PromoUnique), promo.Description, promo.ImageUrl, promo.TargetWithoutLink.AgeFrom, promo.TargetWithoutLink.AgeUntil, promo.TargetWithoutLink.Country, pq.Array(promo.TargetWithoutLink.Categories), promo.MaxCount, *promo.ActiveFromNormal, *promo.ActiveUntilNormal, IDBusiness, time.Now())
		if err := row.Scan(&ID); err != nil {
			log.Println(err)
			return "", errors.New("server error")
		}
		return ID, nil
	}
}

func (r *BusinessPostgresAndRedis) GetPromoList(params validate.GetListParams) ([]validate.GetPromoResponse, int, error) {
	var promos []validate.GetPromoResponse
	var countries string
	var query string
	log.Println(1)
	if len(params.Countries) != 0 {
		countryConditions := []string{}
		for _, country := range params.Countries {
			countryConditions = append(countryConditions, fmt.Sprintf("LOWER(country) = LOWER('%s')", country))
		}
		countries = "WHERE " + strings.Join(countryConditions, " OR ")
	}
	if len(countries) != 0 {
		countries += fmt.Sprintf("OR country = '' AND id_business = '%s'", params.IDBusiness)
	} else {
		countries = fmt.Sprintf("WHERE id_business = '%s'", params.IDBusiness)
	}

	var cratedAt string

	baseQuery := fmt.Sprintf(`
		SELECT p.*, b.name AS business_name, COUNT(l.user_id) AS likes_count,
			   COALESCE(p.promo_unique, '{}') AS promo_unique
		FROM %s p
		LEFT JOIN business b ON p.id_business = b.id
		LEFT JOIN likes l ON p.id = l.promo_id
		%s
		GROUP BY p.id, b.name, p.id_business, p.mode, p.promo_common, 
				 p.promo_unique, p.description, p.image_url, 
				 p.age_from, p.age_until, p.country, 
				 p.categories, p.max_count, 
				 p.active_from, p.active_until, p.active, p.created_at
	`, promosTable, countries)

	if params.SortedBy == "active_from" {
		query = fmt.Sprintf("%s ORDER BY COALESCE(TO_DATE(active_from, 'YYYY-MM-DD'), '1970-01-01'::date) ASC LIMIT %d OFFSET %d;",
			baseQuery, params.Limit, params.Offset)
	} else if params.SortedBy == "active_until" {
		query = fmt.Sprintf("%s ORDER BY COALESCE(TO_DATE(active_until, 'YYYY-MM-DD'), '9999-12-31'::date) ASC LIMIT %d OFFSET %d;",
			baseQuery, params.Limit, params.Offset)
	} else {
		query = fmt.Sprintf("%s ORDER BY created_at DESC LIMIT %d OFFSET %d;",
			baseQuery, params.Limit, params.Offset)
	}
	fmt.Println("Query:", query)

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	for rows.Next() {
		var promo validate.GetPromoResponse
		var promoUnique pq.StringArray

		if err := rows.Scan(
			&promo.PromoId,
			&promo.Mode,
			&promo.PromoCommon,
			&promoUnique,
			&promo.Description,
			&promo.ImageUrl,
			&promo.Target.AgeFrom,
			&promo.Target.AgeUntil,
			&promo.Target.Country,
			pq.Array(&promo.Target.Categories),
			&promo.MaxCount,
			&promo.ActiveFrom,
			&promo.ActiveUntil,
			&promo.CompanyId,
			&promo.Active,
			&cratedAt,
			&promo.CompanyName,
			&promo.LikeCount,
			&promoUnique,
			//&promo.UsedCount,
		); err != nil {
			return nil, 0, err
		}

		promo.PromoUnique = promoUnique
		promo.UsedCount = 0
		promo.Active, err = isPromoActive(promo.ActiveFrom, promo.ActiveUntil)
		if err != nil {
			log.Println(err)
		}
		promos = append(promos, promo)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	TotalCount, err := r.GetCountPromos(params)
	if err != nil {
		return nil, 0, err
	}

	return promos, TotalCount, nil
}

func (r *BusinessPostgresAndRedis) GetCountPromos(params validate.GetListParams) (int, error) {
	var countries string

	if len(params.Countries) != 0 {
		countryConditions := []string{}
		for _, country := range params.Countries {
			countryConditions = append(countryConditions, fmt.Sprintf("LOWER(country) = LOWER('%s')", country))
		}
		countries = "WHERE " + strings.Join(countryConditions, " OR ")
	}

	if len(countries) > 1 {
		countries += fmt.Sprintf("OR country = '' AND id_business = '%s'", params.IDBusiness)
	} else {
		countries = fmt.Sprintf("WHERE id_business = '%s'", params.IDBusiness)
	}
	var count int
	err := r.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM promos %s", countries)).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *BusinessPostgresAndRedis) GetOnePromo(ID, IDBusiness string) (validate.GetPromoResponse, error) {
	var promo validate.GetPromoResponse
	var promoUnique pq.StringArray
	var cratedAt string

	query := fmt.Sprintf(`SELECT 
							p.id,
							p.mode,
							p.promo_common,
							p.promo_unique,
							p.description,
							p.image_url,
							p.age_from,
							p.age_until,
							p.country,
							p.categories,
							p.max_count,
							p.active_from,
							p.active_until,
							p.id_business,
							p.active,
							p.created_at,
							b.name AS business_name,
							COUNT(l.promo_id) AS like_count
						FROM 
							promos p
						JOIN 
							business b ON p.id_business = b.id
						LEFT JOIN 
							likes l ON p.id = l.promo_id
						WHERE 
							p.id = '%s'
						GROUP BY 
							p.id, b.name, p.mode, p.promo_common, p.promo_unique, p.description, 
							p.image_url, p.age_from, p.age_until, p.country, p.categories, 
							p.max_count, p.active_from, p.active_until, p.active, p.created_at, p.id_business;`, ID)
	err := r.db.QueryRow(query).Scan(
		&promo.PromoId,
		&promo.Mode,
		&promo.PromoCommon,
		&promoUnique,
		&promo.Description,
		&promo.ImageUrl,
		&promo.Target.AgeFrom,
		&promo.Target.AgeUntil,
		&promo.Target.Country,
		pq.Array(&promo.Target.Categories),
		&promo.MaxCount,
		&promo.ActiveFrom,
		&promo.ActiveUntil,
		&promo.CompanyId,
		&promo.Active,
		&cratedAt,
		&promo.CompanyName,
		&promo.LikeCount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return promo, errors.New("no promo with this id")
		}
		return promo, err
	}

	if promo.CompanyId != IDBusiness {
		return promo, errors.New("not this business")
	}

	promo.PromoUnique = promoUnique
	promo.Active, err = isPromoActive(promo.ActiveFrom, promo.ActiveUntil)
	if err != nil {
		log.Println(err)
	}

	return promo, nil
}

func (r *BusinessPostgresAndRedis) ChangeOnePromo(promo validate.ChangeOnePromoRequest, ID, IDBusiness string) (validate.GetPromoResponse, error) {
	var exists bool

	err := r.db.QueryRow("SELECT EXISTS(SELECT 1 FROM promos WHERE id = $1)", ID).Scan(&exists)
	if err != nil {
		return validate.GetPromoResponse{}, err
	}
	if !exists {
		return validate.GetPromoResponse{}, errors.New("no promo with this id")
	}

	mode, err := r.GetMode(ID)
	if err != nil {
		return validate.GetPromoResponse{}, err
	}
	switch mode {
	case "UNIQUE":
		if promo.MaxCount != 1 && promo.MaxCountL != nil {
			return validate.GetPromoResponse{}, errors.New("400")
		}
	case "COMMON":
		if promo.MaxCount <= 1 && promo.MaxCount >= 100000000 && promo.MaxCountL != nil {
			return validate.GetPromoResponse{}, errors.New("400")
		}

	}

	exists, err = r.CheckIsCreator(ID, IDBusiness)
	if err != nil {
		return validate.GetPromoResponse{}, err
	}

	if !exists {
		return validate.GetPromoResponse{}, errors.New("not this business")
	}

	query := fmt.Sprintf(`UPDATE %s SET	
                  					description = COALESCE(NULLIF($1, ''), description),
                  					image_url = COALESCE(NULLIF($2, ''), image_url),
                  					max_count = COALESCE(NULLIF($3, -1), max_count),
                  					active_from = COALESCE(NULLIF($4, ''), active_from),
                  					active_until = COALESCE(NULLIF($5, ''), active_until),
                  					age_from = COALESCE(NULLIF($6, 0), age_from),
                  					age_until = COALESCE(NULLIF($7, 0), age_until),
                  					country = COALESCE(NULLIF($8, ''), country),
                  					categories = COALESCE(NULLIF($9, '{}'::text[]), categories)
								WHERE id = '%s'`, promosTable, ID)

	if promo.ActiveUntilNormal == nil && promo.ActiveFromNormal == nil {
		_, err = r.db.Exec(query, promo.Description, promo.ImageUrl, promo.MaxCount, nil, nil, promo.TargetWithoutLink.AgeFrom, promo.TargetWithoutLink.AgeUntil, promo.TargetWithoutLink.Country, pq.Array(promo.TargetWithoutLink.Categories))
		if err != nil {
			log.Println(err)
			return validate.GetPromoResponse{}, errors.New("server error")
		}

	} else if promo.ActiveFromNormal == nil {
		_, err = r.db.Exec(query, promo.Description, promo.ImageUrl, promo.MaxCount, nil, *promo.ActiveUntilNormal, promo.TargetWithoutLink.AgeFrom, promo.TargetWithoutLink.AgeUntil, promo.TargetWithoutLink.Country, pq.Array(promo.TargetWithoutLink.Categories))

		if err != nil {
			log.Println(err)
			return validate.GetPromoResponse{}, errors.New("server error")
		}
	} else if promo.ActiveUntilNormal == nil {
		_, err = r.db.Exec(query, promo.Description, promo.ImageUrl, promo.MaxCount, *promo.ActiveFromNormal, nil, promo.TargetWithoutLink.AgeFrom, promo.TargetWithoutLink.AgeUntil, promo.TargetWithoutLink.Country, pq.Array(promo.TargetWithoutLink.Categories))
		if err != nil {
			log.Println(err)
			return validate.GetPromoResponse{}, errors.New("server error")
		}
	} else {
		_, err = r.db.Exec(query, promo.Description, promo.ImageUrl, promo.MaxCount, *promo.ActiveFromNormal, *promo.ActiveUntilNormal, promo.TargetWithoutLink.AgeFrom, promo.TargetWithoutLink.AgeUntil, promo.TargetWithoutLink.Country, pq.Array(promo.TargetWithoutLink.Categories))
		if err != nil {
			log.Println(err)
			return validate.GetPromoResponse{}, errors.New("server error")
		}
	}

	promoNew, err := r.GetOnePromo(ID, IDBusiness)
	if err != nil {
		return validate.GetPromoResponse{}, err
	}

	return promoNew, nil
}

func (r *BusinessPostgresAndRedis) CheckIsCreator(promoID, businessID string) (bool, error) {
	var exists bool

	query := `
        SELECT EXISTS (
            SELECT 1 
            FROM promos 
            WHERE id = $1 AND id_business = $2
        )
    `

	err := r.db.QueryRow(query, promoID, businessID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *BusinessPostgresAndRedis) GetMode(ID string) (string, error) {
	var mode string

	err := r.db.QueryRow("SELECT mode FROM promos WHERE id = $1", ID).Scan(&mode)
	if err != nil {
		return "", err
	}

	return mode, nil
}

func isPromoActive(activeFrom *string, activeUntil *string) (bool, error) {
	var from time.Time
	var until time.Time
	var err error

	if activeFrom != nil {
		from, err = time.Parse("2006-01-02", *activeFrom)
		if err != nil {
			return false, fmt.Errorf("error parsing activeFrom: %w", err)
		}
	} else {

		from = time.Time{}
	}

	if activeUntil != nil {
		until, err = time.Parse("2006-01-02", *activeUntil)
		if err != nil {
			return false, fmt.Errorf("error parsing activeUntil: %w", err)
		}
	} else {

		until = time.Now().Add(100 * 365 * 24 * time.Hour)
	}

	now := time.Now()

	return now.After(from) && now.Before(until), nil
}
