package spanner

import (
	"context"
	"io"

	cspanner "cloud.google.com/go/spanner"
	database "cloud.google.com/go/spanner/admin/database/apiv1"
)

func Client(db string) error {
	ctx := context.Background()

	adminClient, err := database.NewDatabaseAdminClient(ctx)
	if err != nil {
		return err
	}
	defer adminClient.Close()

	dataClient, err := spanner.NewClient(ctx, db)
	if err != nil {
		return err
	}
	defer dataClient.Close()

	_ = adminClient
	_ = dataClient

	return nil
}
