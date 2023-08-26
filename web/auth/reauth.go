/*
 Copyright 2020 Padduck, LLC
  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at
  	http://www.apache.org/licenses/LICENSE-2.0
  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
*/

package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"net/http"
	"time"
)

func Reauth(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ps := &services.Permission{DB: db}
	ss := &services.Session{DB: db}

	user, _ := c.MustGet("user").(*models.User)

	perms, err := ps.GetForUserAndServer(user.ID, nil)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	session, err := ss.CreateForUser(user)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	data := &LoginResponse{}
	data.Scopes = perms.Scopes

	secure := false
	if c.Request.TLS != nil {
		secure = true
	}

	maxAge := int(time.Hour / time.Second)

	c.SetCookie("puffer_auth", session, maxAge, "/", "", secure, true)
	c.SetCookie("puffer_auth_expires", "", maxAge, "/", "", secure, false)

	c.JSON(http.StatusOK, data)
}
