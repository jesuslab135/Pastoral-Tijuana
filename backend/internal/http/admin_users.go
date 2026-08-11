package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jesuslab135/pastoral-tijuana/backend/internal/auth"
	"github.com/jesuslab135/pastoral-tijuana/backend/internal/store"
)

const uniqueViolation = "23505"

var validRoles = map[string]bool{"parroco": true, "secretaria": true}

type userInput struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Password    string `json:"password"` // optional: empty = magic-link-only
}

func writeUserError(w http.ResponseWriter, err error, fallback string) {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		writeError(w, http.StatusNotFound, "no_encontrado", "No existe esa persona.")
	case errors.As(err, &pgErr) && pgErr.Code == uniqueViolation:
		writeError(w, http.StatusConflict, "correo_duplicado", "Ese correo ya está registrado.")
	default:
		writeError(w, http.StatusInternalServerError, "internal", fallback)
	}
}

func listUsersHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		us, err := store.ListUsers(r.Context(), pool)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", "No se pudo cargar el equipo.")
			return
		}
		out := make([]map[string]any, 0, len(us))
		for _, u := range us {
			out = append(out, userJSON(u))
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": out})
	}
}

func createUserHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in userInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "Cuerpo JSON inválido.")
			return
		}
		in.Email = strings.TrimSpace(in.Email)
		if in.Email == "" || !strings.Contains(in.Email, "@") {
			writeError(w, http.StatusBadRequest, "bad_request", "Escribe un correo válido.")
			return
		}
		if !validRoles[in.Role] {
			writeError(w, http.StatusBadRequest, "bad_request", "El rol debe ser parroco o secretaria.")
			return
		}
		var hash string
		if in.Password != "" {
			h, err := auth.HashPassword(in.Password)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal", "No se pudo crear la cuenta.")
				return
			}
			hash = h
		}
		u := store.User{
			ID: uuid.New(), Email: in.Email, PasswordHash: hash,
			DisplayName: in.DisplayName, Role: in.Role, IsActive: true,
		}
		if err := store.CreateUser(r.Context(), pool, u); err != nil {
			writeUserError(w, err, "No se pudo crear la cuenta.")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"user": userJSON(u)})
	}
}

func updateUserHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "no_encontrado", "No existe esa persona.")
			return
		}
		var in userInput
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			writeError(w, http.StatusBadRequest, "bad_request", "Cuerpo JSON inválido.")
			return
		}
		if !validRoles[in.Role] {
			writeError(w, http.StatusBadRequest, "bad_request", "El rol debe ser parroco o secretaria.")
			return
		}
		if _, err := store.GetUserByID(r.Context(), pool, id); err != nil {
			writeUserError(w, err, "No se pudo cargar la cuenta.")
			return
		}
		if err := store.UpdateUser(r.Context(), pool, id, in.DisplayName, in.Role); err != nil {
			writeUserError(w, err, "No se pudo guardar la cuenta.")
			return
		}
		u, err := store.GetUserByID(r.Context(), pool, id)
		if err != nil {
			writeUserError(w, err, "No se pudo cargar la cuenta.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": userJSON(u)})
	}
}

// setUserActiveHandler backs both activate and deactivate.
func setUserActiveHandler(pool *pgxpool.Pool, active bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			writeError(w, http.StatusNotFound, "no_encontrado", "No existe esa persona.")
			return
		}
		// Locking yourself out would leave the parish with no way back in.
		if !active && id == currentUser(r).ID {
			writeError(w, http.StatusBadRequest, "no_permitido",
				"No puedes desactivar tu propia cuenta.")
			return
		}
		if _, err := store.GetUserByID(r.Context(), pool, id); err != nil {
			writeUserError(w, err, "No se pudo cargar la cuenta.")
			return
		}
		if err := store.SetUserActive(r.Context(), pool, id, active); err != nil {
			writeUserError(w, err, "No se pudo actualizar la cuenta.")
			return
		}
		u, err := store.GetUserByID(r.Context(), pool, id)
		if err != nil {
			writeUserError(w, err, "No se pudo cargar la cuenta.")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": userJSON(u)})
	}
}
