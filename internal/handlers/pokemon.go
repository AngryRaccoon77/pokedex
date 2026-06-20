// Package handlers содержит HTTP-обработчики.
// Разбирают запрос, вызывают сервисный слой, мапят доменные ошибки на HTTP-статусы.
package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"pokedex-api/internal/domain"
	"pokedex-api/internal/service"
)

type PokemonHandler struct {
	svc    *service.PokemonService
	logger *slog.Logger
}

func NewPokemonHandler(svc *service.PokemonService, logger *slog.Logger) *PokemonHandler {
	return &PokemonHandler{svc: svc, logger: logger}
}

func (h *PokemonHandler) Routes() chi.Router {
	r := chi.NewRouter()

	r.Get("/", h.List)
	r.Post("/", h.Create)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.Get)
		r.Put("/", h.Update)
		r.Delete("/", h.Delete)
	})

	return r
}

// GET /pokemon?catchable=true&type=fire&search=char&limit=20&offset=0
func (h *PokemonHandler) List(w http.ResponseWriter, r *http.Request) {
	filter := domain.PokemonFilter{
		CatchableOnly: r.URL.Query().Get("catchable") == "true",
		Type:          r.URL.Query().Get("type"),
		Search:        r.URL.Query().Get("search"),
		Limit:         queryInt(r, "limit", 20),
		Offset:        queryInt(r, "offset", 0),
	}

	pokemons, err := h.svc.List(r.Context(), filter)
	if err != nil {
		h.handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, pokemons)
}

// GET /pokemon/{id}
func (h *PokemonHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid pokemon ID")
		return
	}

	pokemon, err := h.svc.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, pokemon)
}

// POST /pokemon
func (h *PokemonHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input domain.CreatePokemonInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	pokemon, err := h.svc.Create(r.Context(), input)
	if err != nil {
		h.handleError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, pokemon)
}

// PUT /pokemon/{id}
func (h *PokemonHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid pokemon ID")
		return
	}

	var input domain.UpdatePokemonInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	pokemon, err := h.svc.Update(r.Context(), id, input)
	if err != nil {
		h.handleError(w, err)
		return
	}

	respondJSON(w, http.StatusOK, pokemon)
}

// DELETE /pokemon/{id}
func (h *PokemonHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid pokemon ID")
		return
	}

	if err := h.svc.Delete(r.Context(), id); err != nil {
		h.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleError мапит доменные ошибки на HTTP-статусы.
func (h *PokemonHandler) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrDuplicateName):
		respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrNameRequired),
		errors.Is(err, domain.ErrNegativePower),
		errors.Is(err, domain.ErrInvalidType):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		h.logger.Error("internal error", slog.String("error", err.Error()))
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
		Code:    status,
	})
}

func parseID(r *http.Request) (int64, error) {
	return strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}
