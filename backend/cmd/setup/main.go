// Command setup creates the first parish user. It runs once: if any user
// already exists it refuses, so it can never mint a second admin password.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/auth"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/config"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const passwordAlphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const passwordLen = 16

func main() {
	email := os.Getenv("SETUP_EMAIL")
	if email == "" {
		log.Fatal("SETUP_EMAIL es obligatorio: el correo del párroco")
	}
	cfg := config.Load()

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("pool: %v", err)
	}
	defer pool.Close()

	password, err := randomPassword()
	if err != nil {
		log.Fatalf("password: %v", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}

	switch err := store.CreateInitialParroco(ctx, pool, email, hash); {
	case errors.Is(err, store.ErrUsersExist):
		log.Fatal("ya existen usuarios: este comando solo corre una vez")
	case err != nil:
		log.Fatalf("crear párroco: %v", err)
	}

	fmt.Printf("Usuario creado: %s\n", email)
	fmt.Printf("Contraseña (guárdala ahora, no se mostrará de nuevo): %s\n", password)
}

func randomPassword() (string, error) {
	limit := big.NewInt(int64(len(passwordAlphabet)))
	out := make([]byte, passwordLen)
	for i := range out {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		out[i] = passwordAlphabet[n.Int64()]
	}
	return string(out), nil
}
