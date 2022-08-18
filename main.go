package main

import (
	"log"
	"net/http"
	"url-shortener/handler"

	"github.com/gorilla/mux"
)

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", handler.Home).Methods(http.MethodGet)
	r.HandleFunc("/shorten", handler.CreateShortUrl).Methods(http.MethodPost)
	r.HandleFunc("/{urlshorten}", handler.HandleShortUrlRedirect).Methods(http.MethodGet)
	r.HandleFunc("/getinfo/{urlshorten}", handler.GetURLEntry).Methods(http.MethodGet)
	r.HandleFunc("/{urlshorten}", handler.DeleteShortUrl).Methods(http.MethodDelete)
	r.HandleFunc("/{urlshorten}", handler.UpdateUrl).Methods(http.MethodPut)

	log.Fatal(http.ListenAndServe(":8080", r))
}
