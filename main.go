package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

type AuthCodeRequest struct {
	Code string `json:"code"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

var (
	cfg    Config
	client *http.Client
)

func main() {
	cfg = Config {
		ClientID: os.Getenv("DROPBOX_CLIENT_ID"),
		ClientSecret: os.Getenv("DROPBOX_CLIENT_SECRET"),
		RedirectURI: os.Getenv("DROPBOX_REDIRECT_URI"),
	}

	if cfg.ClientID == "" {
		panic("Missing required environment variables: DROPBOX_CLIENT_ID")
	}

	if (cfg.ClientSecret == "") {
		panic("Missing required environment variable: DROPBOX_CLIENT_SECRET")
	}

	if (cfg.RedirectURI == "") {
		panic("Missing required environment variable: DROPBOX_REDIRECT_URI")
	}

	client = &http.Client{Timeout: 10 * time.Second }

	mux := http.NewServeMux()
	mux.HandleFunc("/api/dropbox/exchange", exchangeHanlder)
	mux.HandleFunc("/api/dropbox/refresh", refreshHandler)

	srv := &http.Server{
		Addr: ":3000",
		Handler: withCORS(mux),
	}

	go func ()  {
		fmt.Println("Server running on http://localhost:3000")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	fmt.Println("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		panic(err)
	}

	fmt.Println("Server stopped")
}

func exchangeHanlder(w http.ResponseWriter, r *http.Request) {
	var req AuthCodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	data := url.Values {
		"code": {req.Code},
		"grant_type": {"authorization_code"},
		"client_id": {cfg.ClientID},
		"client_secret": {cfg.ClientSecret},
		"redirect_uri": {cfg.RedirectURI},
	}

	callDropbox(w, data)
}

func refreshHandler(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	data := url.Values{
		"refresh_token": {req.RefreshToken},
		"grant_type": {"refresh_token"},
		"client_id": {cfg.ClientID},
		"client_secret" : {cfg.ClientSecret},
	}

	callDropbox(w, data)
}


func writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

func callDropbox(w http.ResponseWriter, data url.Values) {
	resp, err := client.PostForm("https://api.dropboxapi.com/oauth2/token", data)
	if err != nil {
		writeError(w, "failed to contact dropbox", http.StatusBadGateway)
		return
	}

	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	w.Header().Set("Content-Type", "application_json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:4200")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return 
		}

		next.ServeHTTP(w, r)
	})
}