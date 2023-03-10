package models

import "time"

// ShortenedUrl Encapsulates the data of a shortened URL
type ShortenedUrl struct {
	Id          string `json:"id"`
	OriginalUrl string `json:"originalUrl"`
	//Alias       string    `json:"alias"`
	ShortUrl  string    `json:"shortUrl"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *ShortenedUrl) GetId() string {
	return s.Id
}

func (s *ShortenedUrl) SetId(id string) {
	s.Id = id
}

func (s *ShortenedUrl) GetOriginalUrl() string {
	return s.OriginalUrl
}

func (s *ShortenedUrl) SetOriginalUrl(originalUrl string) {
	s.OriginalUrl = originalUrl
}

//
//func (s *ShortenedUrl) GetAlias() string {
//	return s.Alias
//}
//
//func (s *ShortenedUrl) SetAlias(alias string) {
//	s.Alias = alias
//}

func (s *ShortenedUrl) GetShortUrl() string {
	return s.ShortUrl
}

func (s *ShortenedUrl) SetShortUrl(shortUrl string) {
	s.ShortUrl = shortUrl
}

func (s *ShortenedUrl) GetCreatedAt() time.Time {
	return s.CreatedAt
}

func (s *ShortenedUrl) SetCreatedAt(createdAt time.Time) {
	s.CreatedAt = createdAt
}
