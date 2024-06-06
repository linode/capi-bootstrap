package client

import (
	"context"

	"github.com/linode/linodego"
	"golang.org/x/oauth2"
)

func LinodeClient(linodeToken string, ctx context.Context) linodego.Client {
	tokenSource := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: linodeToken})
	oauth2Client := oauth2.NewClient(ctx, tokenSource)

	client := linodego.NewClient(oauth2Client)
	return client
}
