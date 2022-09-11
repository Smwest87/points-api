package main

import (
	"context"
	"fmt"
	"smwest87/points-api.com/internal/src/models"
	"smwest87/points-api.com/internal/src/webserver"
)

func main() {
	ctx := context.Background()
	cfg := models.NewConfig()
	err := webserver.Run(ctx, cfg)
	if err != nil {
		fmt.Println(err)
	}

}
