package startup

import (
	"context"

	"github.com/flaksp/anime365-sidecar/internal/mylist"
	"github.com/flaksp/anime365-sidecar/pkg/anime365client"
)

var LoadListFromAnime365 = func(anime365Client *anime365client.Client, myListService *mylist.Service) error {
	profile, err := anime365Client.GetMe(context.Background())
	if err != nil {
		return err
	}

	return myListService.LoadFromAnime365(context.Background(), profile.ID)
}
