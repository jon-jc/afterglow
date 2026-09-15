// migrate is a deployment step, run with a migration-owner database credential.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jon-jc/afterglow/internal/core"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Migration failed:", err)
		os.Exit(1)
	}
}
func run() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required; no implicit migration target")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := core.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("cannot connect to migration target")
	}
	defer s.DB.Close()
	if err = s.Migrate(ctx); err != nil {
		return err
	}
	fmt.Println("Schema version and checksum verified; migration complete.")
	return nil
}
