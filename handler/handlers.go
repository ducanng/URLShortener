package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"url-shortener/base62"
	"url-shortener/shorten"
	"url-shortener/store"

	"github.com/gorilla/mux"
)

var prefixLink string = "http://localhost:8080/"

type UrlCreationRequest struct {
	LongUrl string `json:"long_url"`
}
//Home Page
func Home(w http.ResponseWriter, _ *http.Request)  {
	sendResponse(w, http.StatusOK, map[string]string{"message": "Welcome to URL shortener"})
}
//Create short link
func CreateShortUrl(w http.ResponseWriter, r *http.Request) {
	var myurl UrlCreationRequest
	var urlshortener shorten.URLEntry

	err := json.NewDecoder(r.Body).Decode(&myurl)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if !isValidURL(myurl.LongUrl) {
		respondWithError(w, http.StatusBadRequest, "Invalid url")
		return
	}

	if store.CheckURLinDB(myurl.LongUrl) == true {
		urlshortener = store.GetURLEntry(myurl.LongUrl)
	} else {
		shorurl := shorten.GenerateShortLink()
		urlshortener.ShortenURL = prefixLink + shorurl
		urlshortener.Id = base62.Decode(shorurl)
		urlshortener.OriginalURL = myurl.LongUrl
		//returns the current local time.
		loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
		urlshortener.CreateAt = time.Now().In(loc)
		store.SaveURL(urlshortener)
	}
	sendResponse(w, http.StatusOK, urlshortener)

}
// Redirect link
func HandleShortUrlRedirect(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	shortPath := params["urlshorten"]

	var urlCreationRequest UrlCreationRequest

	urlCreationRequest.LongUrl = store.GetLongURL(shortPath)
	if urlCreationRequest.LongUrl == "" {
		respondWithError(w, http.StatusNotFound, "Not found")
		return
	}
	http.Redirect(w, r, urlCreationRequest.LongUrl, http.StatusSeeOther)
}
// Delete short link
func DeleteShortUrl(w http.ResponseWriter, r *http.Request)  {
	params := mux.Vars(r)
	shortPath := params["urlshorten"]

	key := base62.Decode(shortPath)
	check := store.DeleteShortURL(key)
	if check == true {
		sendResponse(w, http.StatusOK, map[string]string{"message": "delete successfully"})
	} else {
		sendResponse(w, http.StatusBadRequest, map[string]string{"message": "delete failed"})
	}
}
//Update short link
func UpdateUrl(w http.ResponseWriter, r *http.Request)  {
	params := mux.Vars(r)
	shortUrl := params["urlshorten"]

	var urlCreationRequest UrlCreationRequest
	var updateUrlEntry shorten.URLEntry

	err := json.NewDecoder(r.Body).Decode(&urlCreationRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	updateUrlEntry.OriginalURL = urlCreationRequest.LongUrl
	updateUrlEntry.ShortenURL = prefixLink + shortUrl
	updateUrlEntry.Id = base62.Decode(shortUrl)
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	updateUrlEntry.CreateAt = time.Now().In(loc)

	check := store.UpdateURL(updateUrlEntry.Id, updateUrlEntry)
	if check == true {
		sendResponse(w, http.StatusOK, map[string]string{"message": "update successful"})
	} else {
		sendResponse(w, http.StatusBadRequest, map[string]string{"message": "update failed"})
	}
}
// Check url
func isValidURL(toTest string) bool {
	_, err := url.ParseRequestURI(toTest)
	if err != nil {
		return false
	}
	return true
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	sendResponse(w, code, map[string]string{"error": message})
}

func sendResponse(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}