package webserver

import (
	"context"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4/pgxpool"
	"net/http"
	"smwest87/points-api.com/internal/src/models"
	"time"
)

func Run(ctx context.Context, config models.Config) error {
	db, err := pgxpool.Connect(context.Background(), config.PostgresURL)
	if err != nil {
		return err
	}

	db.Config().MaxConns = int32(config.MaxDBConnections)
	db.Config().MaxConnIdleTime = time.Duration(config.MaxIdleTime)
	defer db.Close()

	h := models.Handler{
		HTTPListenerAddress: config.HttpListenAddr,
		Config:              config,
		DB:                  db,
		CTX:                 ctx,
	}

	router := mux.NewRouter()

	//specify endpoints, handler functions and HTTP method
	router.HandleFunc("/add-points", h.AddPoints).Methods("POST")
	router.HandleFunc("/spend-points", h.SpendPoints).Methods("POST")
	router.HandleFunc("/get-balance", h.GetBalance).Methods("GET")
	http.Handle("/", router)

	//start and listen to requests
	http.ListenAndServe(config.HttpListenAddr, router)

	return nil

}
