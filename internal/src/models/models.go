package models

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Config ...
type Config struct {
	Environment      string `config:"APP_ENV"`
	PostgresURL      string `config:"psql_URL"`
	MaxDBConnections int    `config:"max_db_connections" default:"30"`
	MaxIdleTime      int    `config:"max_idle_time" default:"15"`
	HttpListenAddr   string `config:"HTTP_LISTEN_ADDR" default:"0.0.0.0:8080"`
}

// NewConfig ...
func NewConfig() Config {
	config := Config{}
	maxDBConnections, err := strconv.Atoi(os.Getenv("max_db_connections"))
	if err != nil {
		fmt.Println("MaxDBConnections not set, using Default")
		config.MaxDBConnections = 30
	} else {
		config.MaxDBConnections = maxDBConnections
	}
	maxIdleTime, err := strconv.Atoi(os.Getenv("max_idle_time"))
	if err != nil {
		fmt.Println("MaxIdleTime not set, using default")
		config.MaxIdleTime = 15
	} else {
		config.MaxIdleTime = maxIdleTime
	}
	config.HttpListenAddr = os.Getenv("HTTP_LISTEN_ADDR")
	config.Environment = os.Getenv("APP_ENV")
	config.PostgresURL = os.Getenv("psql_URL")

	return config
}

type TransactionResponse struct {
	ID        int            `sql:"id"`
	Payer     string         `sql:"payer"`
	Remainder int            `sql:"remainder"`
	CreatedAt sql.NullString `sql:"created_at"`
	UpdatedAt sql.NullString `sql:"updated_at"`
}

type PointsPayload struct {
	Points int `json:"points"`
}

type PointsItem struct {
	Payer  string `json:"payer"`
	Points int    `json:"points"`
}

type Handler struct {
	HTTPListenerAddress string
	Config              Config
	DB                  *pgxpool.Pool
	CTX                 context.Context
}

type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

func (h *Handler) SpendPoints(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		jsonErr := ErrorResponse{
			Success: false,
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(jsonErr)
		return
	}

	var pointsToSpend PointsPayload
	err = json.Unmarshal(body, &pointsToSpend)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		jsonErr := ErrorResponse{
			Success: false,
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(jsonErr)
		return
	}

	payerPointsSpent, err := SpendPoints(h.CTX, h.DB, pointsToSpend.Points)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusForbidden)
		jsonErr := ErrorResponse{
			Success: false,
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(jsonErr)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payerPointsSpent)
}

func (h *Handler) AddPoints(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		jsonErr := ErrorResponse{
			Success: false,
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(jsonErr)
		return
	}

	var transaction PointsItem
	err = json.Unmarshal(body, &transaction)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		jsonErr := ErrorResponse{
			Success: false,
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(jsonErr)
		return
	}
	err = AddPoints(h.CTX, h.DB, transaction)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		jsonErr := ErrorResponse{
			Success: false,
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(jsonErr)
		return
	}
	return
}

func AddPoints(ctx context.Context, db *pgxpool.Pool, transaction PointsItem) error {
	var remainder int
	if transaction.Points < 0 {
		remainder = 0
	} else {
		remainder = transaction.Points
	}
	_, err := db.Exec(ctx, "INSERT INTO points_api.point_transactions (payer, points,created_at, remainder) VALUES ($1, $2, $3, $4)", transaction.Payer, transaction.Points, time.Now(), remainder)
	if err != nil {
		return err
	}
	return nil
}

func SpendPoints(ctx context.Context, db *pgxpool.Pool, points int) ([]PointsItem, error) {
	var spentPoints []PointsItem
	rows, err := db.Query(ctx, "SELECT id, payer, remainder, CAST(created_at AS TEXT)  from points_api.point_transactions WHERE remainder > 0 ORDER BY created_at ASC;")
	if err != nil {
		return nil, err
	}
	for 0 < points {
		transaction, err := db.Begin(ctx)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			row := TransactionResponse{}
			err := rows.Scan(&row.ID, &row.Payer, &row.Remainder, &row.CreatedAt)
			if err != nil {
				return nil, err
			}

			if points-row.Remainder < 1 {
				spentPoints = append(spentPoints, PointsItem{
					Payer:  row.Payer,
					Points: points - (points * 2),
				})
				_, err = transaction.Exec(ctx, "UPDATE points_api.point_transactions SET remainder=$1::int, updated_at=Now() WHERE id = $2::int", row.Remainder-points, row.ID)
				if err != nil {
					transaction.Rollback(ctx)
					return nil, err
				}
				transaction.Commit(ctx)
				return spentPoints, nil
			} else if row.Remainder-points < 1 {
				spentPoints = append(spentPoints, PointsItem{
					Payer:  row.Payer,
					Points: row.Remainder - (row.Remainder * 2),
				})
				points = points - row.Remainder
				_, err = transaction.Exec(ctx, "UPDATE points_api.point_transactions SET remainder=0, updated_at=Now() WHERE id = $1", row.ID)
				if err != nil {
					transaction.Rollback(ctx)
					return nil, err
				}
				continue
			}

		}

		err = errors.New("not enough available points")
		return nil, err

	}
	return spentPoints, nil
}

func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	payerBalances, err := GetBalance(h.CTX, h.DB)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		jsonErr := ErrorResponse{
			Success: false,
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(jsonErr)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(payerBalances)
}

func GetBalance(ctx context.Context, db *pgxpool.Pool) ([]PointsItem, error) {
	var payerBalances []PointsItem
	rows, err := db.Query(ctx, "select payer, sum(remainder) from points_api.point_transactions GROUP BY payer;")
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		balance := PointsItem{}
		err := rows.Scan(&balance.Payer, &balance.Points)
		if err != nil {
			return nil, err
		}
		payerBalances = append(payerBalances, balance)
	}
	return payerBalances, nil
}
