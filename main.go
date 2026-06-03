package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Terry-BrooksJr/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// Types
type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
}
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}
type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}
type chirpParameters struct {
	Body   string    `json:"body"`
	UserID uuid.UUID `json:"user_id"`
}

// Reciever Functons
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) middlewareResetMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Store(0)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) makeUser(w http.ResponseWriter, r *http.Request, email string) {
	result, err := cfg.dbQueries.CreateUser(r.Context(), email)
	if err != nil {
		log.Printf("error: failed to create user: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	newUser := User{
		ID:        result.ID,
		CreatedAt: result.CreatedAt,
		UpdatedAt: result.UpdatedAt,
		Email:     result.Email,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	log.Printf("info: created user with email %s", email)
	json.NewEncoder(w).Encode(newUser)
	return
}
func (cfg *apiConfig) GetAllChirps(w http.ResponseWriter, r *http.Request) {
	results, err := cfg.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		log.Printf("error: failed to get chirps: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	chrips := make([]Chirp, len(results))
	for i, result := range results {
		chrips[i] = Chirp{
			ID:        result.ID,
			CreatedAt: result.CreatedAt,
			UpdatedAt: result.UpdatedAt,
			Body:      result.Body,
			UserID:    result.UserID,
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(chrips)
	return
}
func (cfg *apiConfig) GetSingleChirp(w http.ResponseWriter, r *http.Request, id uuid.UUID) {
	results, err := cfg.dbQueries.GetSingleChirp(r.Context(), id)
	if err == sql.ErrNoRows {
		log.Printf("error: failed to find chirp with ID: %v", id)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		log.Printf("error: failed to get chirps: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	foundChirp := Chirp{
		ID:        results.ID,
		CreatedAt: results.CreatedAt,
		UpdatedAt: results.UpdatedAt,
		Body:      results.Body,
		UserID:    results.UserID,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(foundChirp)
	return
}
func (cfg *apiConfig) Chirp(w http.ResponseWriter, r *http.Request, params chirpParameters) {
	result, err := cfg.dbQueries.PostChirp(r.Context(), database.PostChirpParams{
		Body:   params.Body,
		UserID: params.UserID,
	})
	if err != nil {
		log.Printf("error: failed to create chirp: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	newChirp := Chirp{
		ID:        result.ID,
		CreatedAt: result.CreatedAt,
		UpdatedAt: result.UpdatedAt,
		Body:      result.Body,
		UserID:    result.UserID,
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newChirp)
	return
}

// Top-Level Variables
var cfg apiConfig
var serverHandler http.ServeMux
var dbURL string
var fileServer = http.FileServer(http.Dir("."))
var server = http.Server{
	Addr:    ":8080",
	Handler: &serverHandler,
}

// Route Handlers
func postHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("error: Something went wrong")
		w.WriteHeader(500)
		return
	}
	if len(params.Body) > 400 {
		log.Printf("error: Chirp is too long")
		w.WriteHeader(400)
		return
	}
	splitList := strings.Split(params.Body, " ")
	for idx := 0; idx < len(splitList); idx++ {
		word := splitList[idx]
		if strings.ToLower(word) == "kerfuffle" {
			word = "****"
		}
		if strings.ToLower(word) == "sharbert" {
			word = "****"
		}
		if strings.ToLower(word) == "fornax" {
			word = "****"
		}
		splitList[idx] = word
	}
	w.WriteHeader(http.StatusOK)
	returnValue := strings.Join(splitList, " ")
	fmt.Println(returnValue)
	w.Write([]byte(fmt.Sprintf(`{"cleaned_body": "%s"}`, returnValue)))
	return
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("error: Something went wrong")
		w.WriteHeader(500)
		return
	}
	cfg.makeUser(w, r, params.Email)

}
func getAllChrips(w http.ResponseWriter, r *http.Request) {
	cfg.GetAllChirps(w, r)
}

func getSingleChirp(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(id)
	if err != nil {
		log.Printf("error: Unable to Parse string into UUID: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	cfg.GetSingleChirp(w, r, chirpID)
}
func chirpHandler(w http.ResponseWriter, r *http.Request) {

	decoder := json.NewDecoder(r.Body)
	params := chirpParameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("error: Something went wrong: %v", err)
		w.WriteHeader(500)
		return
	}
	cfg.Chirp(w, r, params)

}

// Main Function
func main() {
	// Load Env Vars
	godotenv.Load()
	dbURL = os.Getenv("DB_URL")

	//
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("error: failed to connect to database: %v", err)
	}
	cfg.dbQueries = database.New(db)

	// Route Handlers
	serverHandler.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`OK`))
	})
	serverHandler.HandleFunc("POST /api/users", http.HandlerFunc(userHandler))
	serverHandler.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(` <html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load())))
	})
	serverHandler.Handle("GET /app/", cfg.middlewareMetricsInc(http.StripPrefix("/app", fileServer)))
	serverHandler.Handle("POST /api/validate_chirp", http.HandlerFunc(postHandler))
	serverHandler.Handle("POST /api/chirps", http.HandlerFunc(chirpHandler))
	serverHandler.Handle("GET /api/chirps", http.HandlerFunc(getAllChrips))
	serverHandler.Handle("GET /api/chirps/{chirpID}", http.HandlerFunc(getSingleChirp))
	serverHandler.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Store(0)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
	})

	// Start Server Listening
	server.ListenAndServe()
}
