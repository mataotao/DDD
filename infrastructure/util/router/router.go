package router

import (
	"DDD/infrastructure/util/router/middleware"
	"DDD/infrastructure/util/router/middleware/limiter"
	"DDD/interfaces/facade/sd"

	"github.com/gin-contrib/pprof"
	"github.com/gin-gonic/gin"

	"net/http"
)

// Load loads the middlewares, routes, handlers.
func Load(g *gin.Engine, mw ...gin.HandlerFunc) *gin.Engine {
	// Middlewares.
	g.Use(gin.Recovery())
	g.Use(mw...)
	g.Use(middleware.RequestId())

	pprof.Register(g)
	// 404 Handler.
	g.NoRoute(func(c *gin.Context) {
		c.String(http.StatusNotFound, "The incorrect API route.")
	})

	svcd := g.Group("/sd")
	svcd.Use(limiter.TBIP())
	{
		svcd.GET("/health", sd.HealthCheck)
		svcd.GET("/disk", sd.DiskCheck)
		svcd.GET("/cpu", sd.CPUCheck)
		svcd.GET("/ram", sd.RAMCheck)
	}
	return g
}
