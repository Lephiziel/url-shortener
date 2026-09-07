package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"url-shortener/internal/domain"
	"url-shortener/internal/service"
)

type Handler struct {
	service service.Service
	baseURL string
}

type CreateLinkRequest struct {
	URL string `json:"url"`
}

type CreateLinkResponse struct {
	ShortURL string `json:"short_url"`
}

func NewHandler(s service.Service, baseURL string) *Handler {
	return &Handler{
		service: s,
		baseURL: baseURL}
}

func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	var req CreateLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "incorrect json format: "+err.Error(), http.StatusBadRequest)
		return
	}

	res, linkErr := h.service.CreateShortLink(r.Context(), req.URL)
	if linkErr != nil {
		if errors.Is(linkErr, domain.ErrInvalidURL) {
			http.Error(w, "invalid url: "+linkErr.Error(), http.StatusBadRequest)
			return
		}
		slog.Error("failed to create short link", "error", linkErr)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	shortURL, joinErr := url.JoinPath(h.baseURL, res.Code)
	if joinErr != nil {
		slog.Error("failed to build short url", "error", joinErr)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	response := CreateLinkResponse{
		ShortURL: shortURL,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		slog.Error("failed to encode response", "error", err)
		return
	}
}

func (h *Handler) RedirectLink(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")

	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	link, linkErr := h.service.GetOriginalLink(r.Context(), code)
	if linkErr != nil {
		if errors.Is(linkErr, domain.ErrNotFound) {
			http.Error(w, "the url was not found", http.StatusNotFound)
			return
		}
		slog.Error("failed to get original link", "error", linkErr)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, link.URL, http.StatusFound)
}
