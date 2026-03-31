package database

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

func NewPostgresWithReplicas(primaryDSN string, replicaDSNs []string) (*gorm.DB, error) {
	db, err := NewPostgres(primaryDSN)
	if err != nil {
		return nil, err
	}

	if len(replicaDSNs) == 0 {
		log.Println("No replicas configured, using primary for all queries")
		return db, nil
	}

	replicas := make([]gorm.Dialector, 0, len(replicaDSNs))
	for _, dsn := range replicaDSNs {
		replicas = append(replicas, postgres.Open(dsn))
	}

	err = db.Use(dbresolver.Register(dbresolver.Config{
		Replicas: replicas,
		Policy:   dbresolver.RandomPolicy{},
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to register db resolver: %w", err)
	}

	log.Printf("Read-write splitting enabled with %d replica(s)", len(replicaDSNs))
	return db, nil
}
