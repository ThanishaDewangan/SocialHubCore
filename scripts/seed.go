package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"stories-service/internal/auth"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	godotenv.Load()

	db, err := sql.Open("postgres", os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	fmt.Println("Seeding database...")

	users := []struct {
		email    string
		password string
	}{
		{"alice@example.com", "password123"},
		{"bob@example.com", "password123"},
		{"charlie@example.com", "password123"},
	}

	userIDs := make([]uuid.UUID, 0)

	for _, u := range users {
		hash, _ := auth.HashPassword(u.password)
		var id uuid.UUID
		err := db.QueryRow(
			"INSERT INTO users (email, password_hash) VALUES ($1, $2) ON CONFLICT (email) DO UPDATE SET email = EXCLUDED.email RETURNING id",
			u.email, hash,
		).Scan(&id)
		if err != nil {
			log.Printf("failed to create user %s: %v", u.email, err)
			continue
		}
		userIDs = append(userIDs, id)
		fmt.Printf("Created user: %s (ID: %s)\n", u.email, id)
	}

	if len(userIDs) >= 2 {
		_, err = db.Exec("INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", userIDs[0], userIDs[1])
		if err == nil {
			fmt.Printf("Alice now follows Bob\n")
		}
	}

	if len(userIDs) >= 1 {
		text := "Hello world! This is my first story 🎉"
		expiresAt := time.Now().Add(24 * time.Hour)
		_, err = db.Exec(
			"INSERT INTO stories (author_id, text, visibility, expires_at) VALUES ($1, $2, $3, $4)",
			userIDs[0], text, "public", expiresAt,
		)
		if err == nil {
			fmt.Println("Created sample public story for Alice")
		}
	}

	fmt.Println("Seeding completed!")
}
