package admin

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"time"

	"github.com/im-kulikov/docker-dns/internal/cacher"
	"github.com/miekg/dns"
)

type ResponseItem struct {
	Domain string        `json:"domain"`
	Record []string      `json:"record"`
	Expire time.Duration `json:"expire"`
}

type ResponseList struct {
	List []ResponseItem `json:"list"`
}

type ErrorResponse struct {
	Code        string `json:"code"`
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
}

type Response struct {
	*ErrorResponse
	*ResponseItem
	*ResponseList
}

type Storage interface {
	Get(string) (*cacher.CacheItem, bool)
	Delete(string)
	Set(string, *cacher.CacheItem) bool
	List() map[string]*cacher.CacheItem
}

type ErrorHandler func(http.ResponseWriter, *http.Request) error

//go:embed frontend/*
var root embed.FS

var content, _ = fs.Sub(root, "frontend")

func validateDomain(domain string) error {
	if domain == "" {
		return errors.New("domain is required")
	}

	_, err := dns.Exchange(&dns.Msg{Question: []dns.Question{{Name: domain + ".", Qtype: dns.TypeA}}}, "1.1.1.1:53")

	return err
}

func (s *server) listCacheItems(w http.ResponseWriter, _ *http.Request) error {
	var result ResponseList
	for _, item := range s.rec.List() {
		result.List = append(result.List, ResponseItem{
			Domain: item.Domain,
			Record: item.Record,
			Expire: time.Second * time.Duration(item.Expire),
		})

	}

	return json.NewEncoder(w).Encode(Response{ResponseList: &result})
}

func (s *server) createCacheItem(w http.ResponseWriter, r *http.Request) error {
	var item ResponseItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		return err
	}

	if err := validateDomain(item.Domain); err != nil {
		w.WriteHeader(http.StatusBadRequest)

		return json.NewEncoder(w).Encode(Response{
			ErrorResponse: &ErrorResponse{
				Code:        "400",
				Message:     "Invalid domain",
				Description: err.Error(),
			},
		})
	}

	if !s.rec.Set(item.Domain, cacher.NewItem(item.Domain)) {
		w.WriteHeader(http.StatusBadRequest)

		return json.NewEncoder(w).Encode(Response{
			ErrorResponse: &ErrorResponse{
				Code:    "400",
				Message: "something went wrong",
			},
		})
	}

	w.WriteHeader(http.StatusCreated)

	return nil
}

func (s *server) getCacheItem(w http.ResponseWriter, r *http.Request) error {
	domain := r.PathValue("domain")

	item, exists := s.rec.Get(domain)
	if !exists {
		w.WriteHeader(http.StatusNotFound)

		return json.NewEncoder(w).Encode(Response{
			ErrorResponse: &ErrorResponse{
				Code:    "404",
				Message: "Domain not found",
			},
		})
	}

	out := &ResponseItem{
		Domain: item.Domain,
		Record: item.Record,
		Expire: time.Second * time.Duration(item.Expire),
	}

	if err := json.NewEncoder(w).Encode(Response{ResponseItem: out}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	return nil
}

func (s *server) updateCacheItem(w http.ResponseWriter, r *http.Request) error {
	oldDomain := r.PathValue("domain")

	var item ResponseItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		return err

	} else if err = validateDomain(item.Domain); err != nil {
		w.WriteHeader(http.StatusBadRequest)

		return json.NewEncoder(w).Encode(Response{
			ErrorResponse: &ErrorResponse{
				Code:        "400",
				Message:     "Invalid domain",
				Description: err.Error(),
			},
		})
	}

	if _, exists := s.rec.Get(oldDomain); !exists {
		w.WriteHeader(http.StatusNotFound)

		return json.NewEncoder(w).Encode(Response{
			ErrorResponse: &ErrorResponse{
				Code:    "404",
				Message: "Domain not found",
			},
		})
	}

	if !s.rec.Set(item.Domain, cacher.NewItem(item.Domain)) {
		w.WriteHeader(http.StatusBadRequest)

		return json.NewEncoder(w).Encode(Response{
			ErrorResponse: &ErrorResponse{
				Code:    "400",
				Message: "something went wrong",
			},
		})
	}

	s.rec.Delete(oldDomain)

	w.WriteHeader(http.StatusAccepted)

	return nil
}

func (s *server) deleteCacheItem(w http.ResponseWriter, r *http.Request) error {
	domain := r.PathValue("domain")

	if _, exists := s.rec.Get(domain); !exists {
		w.WriteHeader(http.StatusNotFound)

		return json.NewEncoder(w).Encode(Response{
			ErrorResponse: &ErrorResponse{
				Code:    "404",
				Message: "Domain not found",
			},
		})
	}

	s.rec.Delete(domain)
	w.WriteHeader(http.StatusAccepted)

	return nil
}

// wrapErrorHandler оборачивает ErrorHandler, чтобы обрабатывать ошибки
func wrapErrorHandler(handler ErrorHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var err error
		if err = handler(w, r); err == nil {
			return
		}

		// Преобразовать ошибку в ErrorResponse и отправить её клиенту
		w.WriteHeader(http.StatusInternalServerError)

		errorResponse := ErrorResponse{
			Code:    "500",
			Message: err.Error(),
		}

		if jsonErr := json.NewEncoder(w).Encode(errorResponse); jsonErr != nil {
			http.Error(w, jsonErr.Error(), http.StatusInternalServerError)
		}
	}
}

func (s *server) router() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("GET /", http.FileServer(http.FS(content)))
	mux.HandleFunc("GET /api", wrapErrorHandler(s.listCacheItems))
	mux.HandleFunc("POST /api", wrapErrorHandler(s.createCacheItem))
	mux.HandleFunc("GET /api/{domain}/", wrapErrorHandler(s.getCacheItem))
	mux.HandleFunc("PUT /api/{domain}/", wrapErrorHandler(s.updateCacheItem))
	mux.HandleFunc("DELETE /api/{domain}/", wrapErrorHandler(s.deleteCacheItem))

	return mux
}
