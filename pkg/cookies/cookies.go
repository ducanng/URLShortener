package cookies

import (
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"time"
)

type Cookies struct {
	Store sessions.Store
}

func NewCookies(secret string) *Cookies {
	store := cookie.NewStore([]byte(secret))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   int(30 * time.Minute.Seconds()),
		HttpOnly: true,
	})
	return &Cookies{Store: store}
}

func (c *Cookies) SetSession(ctx *gin.Context, key string, value interface{}) {
	session := sessions.Default(ctx)
	session.Set(key, value)
	err := session.Save()
	if err != nil {
		log.Printf("Error saving session: %v", err)
		return
	}
}

func (c *Cookies) GetSession(ctx *gin.Context, key string) interface{} {
	session := sessions.Default(ctx)
	return session.Get(key)
}

func (c *Cookies) ClearSession(ctx *gin.Context) {
	session := sessions.Default(ctx)
	session.Clear()
	err := session.Save()
	if err != nil {
		log.Printf("Error saving session: %v", err)
		return
	}
}

func (c *Cookies) SetCookie(ctx *gin.Context, key string, value string, maxAge time.Duration) {
	cookie := &http.Cookie{
		Name:     key,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(maxAge.Seconds()),
	}
	http.SetCookie(ctx.Writer, cookie)
}

func (c *Cookies) GetCookie(ctx *gin.Context, key string) (*http.Cookie, error) {
	cookie, err := ctx.Request.Cookie(key)
	if err != nil {
		return nil, err
	}
	return cookie, nil
}

func (c *Cookies) ClearCookie(ctx *gin.Context, key string) {
	cookie := &http.Cookie{
		Name:     key,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
	http.SetCookie(ctx.Writer, cookie)
}
