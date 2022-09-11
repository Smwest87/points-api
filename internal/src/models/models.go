package models

import (
	"context"
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
	ID        int       `sql:"id"`
	Payer     string    `sql:"payer"`
	Remainder int       `sql:"remainder"`
	Timestamp time.Time `sql:"timestamp"`
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

func (h *Handler) AddPoints(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if r.Method != "POST" {
		fmt.Println("inside POST error check")
		err := errors.New("add-points only supports POST")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		jsonErr := ErrorResponse{
			Success: false,
			Error:   err.Error(),
		}
		json.NewEncoder(w).Encode(jsonErr)
		return
	}
	if err != nil {
		fmt.Println("inside Readall error check")
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
		fmt.Println("inside unmarshall error")
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
	_, err := db.Exec(ctx, "INSERT INTO points_api.point_transactions (payer, points,timestamp, remainder) VALUES ($1, $2, $3, $4)", transaction.Payer, transaction.Points, time.Now(), transaction.Points)
	if err != nil {
		return err
	}
	return nil
}
