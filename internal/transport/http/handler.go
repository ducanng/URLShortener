package http

import (
	"net/http"

	grpctransport "github.com/ducanng/URLShortener/internal/transport/grpc"
	"github.com/ducanng/URLShortener/proto/urlshortenerpb"

	"github.com/gin-gonic/gin"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HandleCreateURL godoc
// @Summary      Create shortened URL
// @Description  Create a shortened URL from the original URL via grpc-gateway
// @Tags         shorten
// @Accept       json
// @Produce      json
// @Param        body body ShortenRequest true "Original URL"
// @Success      200 {object} URLResponse
// @Failure      500 {object} ErrorResponse
// @Router       /shorted [post]
func HandleCreateURL(gwMux *runtime.ServeMux) gin.HandlerFunc {
	return gin.WrapH(gwMux)
}

// HandleGetURL godoc
// @Summary      Get URL info
// @Description  Get info of a shortened URL (original URL, clicks count)
// @Tags         info
// @Produce      json
// @Param        path path string true "Short URL path" example("abc123")
// @Success      200 {object} URLResponse
// @Failure      404 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /info/{path} [get]
func HandleGetURL(gwMux *runtime.ServeMux) gin.HandlerFunc {
	return gin.WrapH(gwMux)
}

// HandleRedirect godoc
// @Summary      Redirect to original URL
// @Description  Redirect (HTTP 301) to the original URL using the short path
// @Tags         redirect
// @Param        path path string true "Short URL path" example("abc123")
// @Success      301 "Moved Permanently"
// @Failure      404 {object} ErrorResponse
// @Failure      410 {object} ErrorResponse
// @Router       /{path} [get]
func HandleRedirect(grpcClient *grpctransport.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		res, err := grpcClient.CC.GetURL(c.Request.Context(), &urlshortenerpb.GetURLRequest{URL: path})
		if err != nil {
			// Distinguish expired (410 Gone) from real not-found (404).
			// FailedPrecondition is the gRPC code chosen for "exists but
			// expired" — see the gRPC handler's GetURL.
			if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
				c.JSON(http.StatusGone, gin.H{"message": "URL expired"})
				return
			}
			c.JSON(http.StatusNotFound, gin.H{"message": "URL not found"})
			return
		}
		if res.GetStatus() == "Failed" {
			c.JSON(http.StatusNotFound, gin.H{"message": "URL not found"})
			return
		}
		c.Redirect(http.StatusMovedPermanently, res.GetUrl().GetOriginalURL())
	}
}
