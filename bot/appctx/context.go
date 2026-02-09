package appctx

import (
	"github.com/KevinHaeusler/go-haruki/bot/clients/jellyseerr"

	"github.com/KevinHaeusler/go-haruki/bot/config"
	"github.com/KevinHaeusler/go-haruki/bot/httpx"
)

type Context struct {
	Config config.Config
	HTTP   *httpx.Client

	Jelly *jellyseerr.Client
}
