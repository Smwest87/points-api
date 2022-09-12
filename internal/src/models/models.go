package models

import (
	"context"
	"database/sql"
	"encoding/json"
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
		fmt.Println("Unmarshall pointsTospend Failure")
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
		fmt.Println("SpendPoints error")
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
	_, err := db.Exec(ctx, "INSERT INTO points_api.point_transactions (payer, points,created_at, remainder) VALUES ($1, $2, $3, $4)", transaction.Payer, transaction.Points, time.Now(), transaction.Points)
	if err != nil {
		return err
	}
	return nil
}

func SpendPoints(ctx context.Context, db *pgxpool.Pool, points int) ([]PointsItem, error) {
	fmt.Println("Entering spent points")
	availablePoints := 0
	var spentPoints []PointsItem
	rows, err := db.Query(ctx, "SELECT id, payer, remainder, CAST(created_at AS TEXT)  from points_api.point_transactions WHERE remainder > 0 ORDER BY created_at ASC;")
	if err != nil {
		return nil, err
	}
	fmt.Printf("Printing points %d", points)
	for availablePoints <= points {
		transaction, err := db.Begin(ctx)
		fmt.Println("Begin transaction")
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			fmt.Println("Scanning rows")
			row := TransactionResponse{}
			err := rows.Scan(&row.ID, &row.Payer, &row.Remainder, &row.CreatedAt)
			fmt.Println("After Scan")
			if err != nil {
				fmt.Println("Error scanning rows")
				return nil, err
			}

			fmt.Printf("Points: %d\nRemainder: %d\n", points, row.Remainder)

			if points-row.Remainder < 1 {
				fmt.Println("Points < 1")
				spentPoints = append(spentPoints, PointsItem{
					Payer:  row.Payer,
					Points: row.Remainder - points,
				})
				availablePoints = availablePoints + row.Remainder - points
				fmt.Println("Before transaction EXEC for updating DB")
				_, err = transaction.Exec(ctx, "UPDATE points_api.point_transactions SET remainder=$1::int, updated_at=Now() WHERE id = $2::int", row.Remainder-points, row.ID)
				if err != nil {
					transaction.Rollback(ctx)
					return nil, err
				}
				fmt.Println("Commiting transaction")
				transaction.Commit(ctx)
				return spentPoints, nil
			} else if row.Remainder-points < 1 {
				fmt.Println("entering remainder-points")
				spentPoints = append(spentPoints, PointsItem{
					Payer:  row.Payer,
					Points: row.Remainder,
				})
				availablePoints = availablePoints + row.Remainder
				_, err = transaction.Exec(ctx, "UPDATE points_api.point_transactions SET remainder=0, updated_at=Now() WHERE id = $1", row.ID)
				if err != nil {
					fmt.Println("Entering rollback")
					transaction.Rollback(ctx)
					return nil, err
				}
				continue
			}

		}

	}
	return spentPoints, nil
}
