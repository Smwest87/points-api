package main

import (
	"context"
	"fmt"
	"os"
	"smwest87/points-api.com/internal/src/models"
	"smwest87/points-api.com/internal/src/webserver"
)

func main() {
	ctx := context.Background()
	cfg := models.NewConfig()
	fmt.Println(cfg.PostgresURL)
	err := webserver.Run(ctx, cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

}
