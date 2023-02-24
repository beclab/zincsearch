/* Copyright 2022 Zinc Labs Inc. and Contributors
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package routes

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/zinclabs/zincsearch/docs" // docs is generated by Swag CLI

	"github.com/zinclabs/zincsearch"
	"github.com/zinclabs/zincsearch/pkg/handlers/auth"
	"github.com/zinclabs/zincsearch/pkg/handlers/document"
	"github.com/zinclabs/zincsearch/pkg/handlers/index"
	"github.com/zinclabs/zincsearch/pkg/handlers/search"
	"github.com/zinclabs/zincsearch/pkg/meta"
	"github.com/zinclabs/zincsearch/pkg/meta/elastic"
	"github.com/zinclabs/zincsearch/pkg/zutils"
)

// SetRoutes sets up all gin HTTP API endpoints that can be called by front end
func SetRoutes(r *gin.Engine) {
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "PUT", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "authorization", "content-type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	r.GET("/", meta.GUI)
	r.GET("/version", meta.GetVersion)
	r.GET("/healthz", meta.GetHealthz)

	// use ginSwagger middleware to serve the API docs
	r.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	front, err := zincsearch.GetFrontendAssets()
	if err != nil {
		log.Err(err)
	}

	// UI
	HTTPCacheForUI(r)
	r.StaticFS("/ui/", http.FS(front))
	r.NoRoute(func(c *gin.Context) {
		log.Error().
			Str("method", c.Request.Method).
			Int("code", 404).
			Int("took", 0).
			Msg(c.Request.RequestURI)

		if strings.HasPrefix(c.Request.RequestURI, "/ui/") {
			path := strings.TrimPrefix(c.Request.RequestURI, "/ui/")
			locationPath := strings.Repeat("../", strings.Count(path, "/"))
			c.Status(http.StatusFound)
			c.Writer.Header().Set("Location", "./"+locationPath)
		}
	})

	// auth
	r.POST("/api/login", auth.Login)
	r.POST("/api/user", AuthMiddleware("auth.CreateUpdateUser"), auth.CreateUpdateUser)
	r.PUT("/api/user", AuthMiddleware("auth.CreateUpdateUser"), auth.CreateUpdateUser)
	r.DELETE("/api/user/:id", AuthMiddleware("auth.DeleteUser"), auth.DeleteUser)
	r.GET("/api/user", AuthMiddleware("auth.ListUser"), auth.ListUser)
	r.GET("/api/permissions", AuthMiddleware("auth.ListPermissions"), auth.ListPermissions)
	r.GET("/api/role", AuthMiddleware("auth.ListRole"), auth.ListRole)
	r.POST("/api/role", AuthMiddleware("auth.CreateUpdateRole"), auth.CreateUpdateRole)
	r.PUT("/api/role", AuthMiddleware("auth.CreateUpdateRole"), auth.CreateUpdateRole)
	r.DELETE("/api/role/:id", AuthMiddleware("auth.DeleteRole"), auth.DeleteRole)

	// index
	r.GET("/api/index", AuthMiddleware("index.List"), index.List)
	r.GET("/api/index_name", AuthMiddleware("index.IndexNameList"), index.IndexNameList)
	r.POST("/api/index", AuthMiddleware("index.Create"), index.Create)
	r.PUT("/api/index", AuthMiddleware("index.Create"), index.Create)
	r.PUT("/api/index/:target", AuthMiddleware("index.Create"), index.Create)
	r.DELETE("/api/index/:target", AuthMiddleware("index.Delete"), index.Delete)
	r.GET("/api/index/:target", AuthMiddleware("index.Get"), index.Get)
	r.HEAD("/api/index/:target", AuthMiddleware("index.Exists"), index.Exists)
	r.POST("/api/index/:target/refresh", AuthMiddleware("index.Refresh"), index.Refresh)
	// index settings
	r.GET("/api/:target/_mapping", AuthMiddleware("index.GetMapping"), index.GetMapping)
	r.PUT("/api/:target/_mapping", AuthMiddleware("index.SetMapping"), index.SetMapping)
	r.GET("/api/:target/_settings", AuthMiddleware("index.GetSettings"), index.GetSettings)
	r.PUT("/api/:target/_settings", AuthMiddleware("index.SetSettings"), index.SetSettings)
	// analyze
	r.POST("/api/_analyze", AuthMiddleware("index.Analyze"), index.Analyze)
	r.POST("/api/:target/_analyze", AuthMiddleware("index.Analyze"), index.Analyze)

	// search
	r.POST("/api/:target/_search", AuthMiddleware("search.SearchV1"), search.SearchV1)

	// document
	// Document Bulk update/insert
	r.POST("/api/_bulk", AuthMiddleware("document.Bulk"), document.Bulk)
	r.POST("/api/:target/_bulk", AuthMiddleware("document.Bulk"), document.Bulk)
	r.POST("/api/:target/_multi", AuthMiddleware("document.Multi"), document.Multi)
	r.POST("/api/_bulkv2", AuthMiddleware("document.Bulk"), document.Bulkv2)         // New JSON format
	r.POST("/api/:target/_bulkv2", AuthMiddleware("document.Bulk"), document.Bulkv2) // New JSON format
	// Document CRUD APIs. Update is same as create.
	r.POST("/api/:target/_doc", AuthMiddleware("document.Create"), document.CreateUpdate)    // create
	r.PUT("/api/:target/_doc", AuthMiddleware("document.Create"), document.CreateUpdate)     // create
	r.PUT("/api/:target/_doc/:id", AuthMiddleware("document.Create"), document.CreateUpdate) // create or update
	r.HEAD("/api/:target/_doc/:id", AuthMiddleware("document.Get"), document.Get)            // get
	r.GET("/api/:target/_doc/:id", AuthMiddleware("document.Get"), document.Get)             // get
	r.POST("/api/:target/_update/:id", AuthMiddleware("document.Update"), document.Update)   // update
	r.DELETE("/api/:target/_doc/:id", AuthMiddleware("document.Delete"), document.Delete)    // delete

	/**
	 * elastic compatible APIs
	 */

	r.GET("/es/", ESMiddleware, func(c *gin.Context) {
		zutils.GinRenderJSON(c, http.StatusOK, elastic.NewESInfo(c))
	})
	r.HEAD("/es/", ESMiddleware, func(c *gin.Context) {
		zutils.GinRenderJSON(c, http.StatusOK, elastic.NewESInfo(c))
	})
	r.GET("/es/_license", ESMiddleware, func(c *gin.Context) {
		zutils.GinRenderJSON(c, http.StatusOK, elastic.NewESLicense(c))
	})
	r.GET("/es/_xpack", ESMiddleware, func(c *gin.Context) {
		zutils.GinRenderJSON(c, http.StatusOK, elastic.NewESXPack(c))
	})

	r.POST("/es/_search", AuthMiddleware("search.SearchDSL"), ESMiddleware, IndexAliasMiddleware, search.SearchDSL)
	r.POST("/es/_msearch", AuthMiddleware("search.MultipleSearch"), ESMiddleware, IndexAliasMiddleware, search.MultipleSearch)
	r.POST("/es/:target/_search", AuthMiddleware("search.SearchDSL"), ESMiddleware, IndexAliasMiddleware, search.SearchDSL)
	r.POST("/es/:target/_msearch", AuthMiddleware("search.MultipleSearch"), ESMiddleware, IndexAliasMiddleware, search.MultipleSearch)
	r.POST("/es/:target/_delete_by_query", AuthMiddleware("search.DeleteByQuery"), IndexAliasMiddleware, search.DeleteByQuery)

	r.GET("/es/_index_template", AuthMiddleware("index.ListTemplate"), ESMiddleware, index.ListTemplate)
	r.POST("/es/_index_template", AuthMiddleware("index.CreateTemplate"), ESMiddleware, index.CreateTemplate)
	r.PUT("/es/_index_template/:target", AuthMiddleware("index.CreateTemplate"), ESMiddleware, index.CreateTemplate)
	r.GET("/es/_index_template/:target", AuthMiddleware("index.GetTemplate"), ESMiddleware, index.GetTemplate)
	r.HEAD("/es/_index_template/:target", AuthMiddleware("index.GetTemplate"), ESMiddleware, index.GetTemplate)
	r.DELETE("/es/_index_template/:target", AuthMiddleware("index.DeleteTemplate"), ESMiddleware, index.DeleteTemplate)
	// ES Compatible data stream
	r.PUT("/es/_data_stream/:target", AuthMiddleware("elastic.PutDataStream"), ESMiddleware, elastic.PutDataStream)
	r.GET("/es/_data_stream/:target", AuthMiddleware("elastic.GetDataStream"), ESMiddleware, elastic.GetDataStream)
	r.HEAD("/es/_data_stream/:target", AuthMiddleware("elastic.GetDataStream"), ESMiddleware, elastic.GetDataStream)

	r.PUT("/es/:target", AuthMiddleware("index.CreateES"), ESMiddleware, index.CreateES)
	r.HEAD("/es/:target", AuthMiddleware("index.Exists"), ESMiddleware, index.Exists)

	r.GET("/es/:target/_mapping", AuthMiddleware("index.GetESMapping"), ESMiddleware, index.GetESMapping)
	r.PUT("/es/:target/_mapping", AuthMiddleware("index.SetMapping"), ESMiddleware, index.SetMapping)

	r.GET("/es/:target/_settings", AuthMiddleware("index.GetSettings"), ESMiddleware, index.GetSettings)
	r.PUT("/es/:target/_settings", AuthMiddleware("index.SetSettings"), ESMiddleware, index.SetSettings)

	r.POST("/es/_analyze", AuthMiddleware("index.Analyze"), ESMiddleware, index.Analyze)
	r.POST("/es/:target/_analyze", AuthMiddleware("index.Analyze"), ESMiddleware, index.Analyze)

	r.POST("/es/_aliases", AuthMiddleware("index.AddOrRemoveESAlias"), ESMiddleware, index.AddOrRemoveESAlias)

	r.GET("/es/_alias", AuthMiddleware("index.GetESAliases"), ESMiddleware, index.GetESAliases)
	r.GET("/es/:target/_alias", AuthMiddleware("index.GetESAliases"), ESMiddleware, index.GetESAliases)
	r.GET("/es/_alias/:target_alias", AuthMiddleware("index.GetESAliases"), ESMiddleware, index.GetESAliases)

	// ES Bulk update/insert
	r.POST("/es/_bulk", AuthMiddleware("document.ESBulk"), ESMiddleware, document.ESBulk)
	r.POST("/es/:target/_bulk", AuthMiddleware("document.ESBulk"), ESMiddleware, document.ESBulk)
	r.PUT("/es/:target/_bulk", AuthMiddleware("document.ESBulk"), ESMiddleware, document.ESBulk)
	r.POST("/es/:target/_refresh", AuthMiddleware("index.Refresh"), index.Refresh)
	// ES Document
	r.POST("/es/:target/_doc", AuthMiddleware("document.CreateUpdate"), ESMiddleware, document.CreateUpdate)        // create
	r.PUT("/es/:target/_doc/:id", AuthMiddleware("document.CreateUpdate"), ESMiddleware, document.CreateUpdate)     // create or update
	r.PUT("/es/:target/_create/:id", AuthMiddleware("document.CreateUpdate"), ESMiddleware, document.CreateUpdate)  // create
	r.POST("/es/:target/_create/:id", AuthMiddleware("document.CreateUpdate"), ESMiddleware, document.CreateUpdate) // create
	r.POST("/es/:target/_update/:id", AuthMiddleware("document.Update"), ESMiddleware, document.Update)             // update part of document
	r.DELETE("/es/:target/_doc/:id", AuthMiddleware("document.Delete"), ESMiddleware, document.Delete)              // delete
}
