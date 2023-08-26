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

package api

import (
	"github.com/gin-gonic/gin"
	uuid "github.com/gofrs/uuid/v5"
	"github.com/pufferpanel/pufferpanel/v3"
	"github.com/pufferpanel/pufferpanel/v3/logging"
	"github.com/pufferpanel/pufferpanel/v3/middleware"
	"github.com/pufferpanel/pufferpanel/v3/models"
	"github.com/pufferpanel/pufferpanel/v3/response"
	"github.com/pufferpanel/pufferpanel/v3/services"
	"net/http"
)

func registerSelf(g *gin.RouterGroup) {
	g.Handle("GET", "", middleware.RequiresPermission(pufferpanel.ScopeLogin, false), getSelf)
	g.Handle("PUT", "", middleware.RequiresPermission(pufferpanel.ScopeSelfEdit, false), updateSelf)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "PUT"))

	g.Handle("GET", "/otp", middleware.RequiresPermission(pufferpanel.ScopeSelfEdit, false), getOtpStatus)
	g.Handle("POST", "/otp", middleware.RequiresPermission(pufferpanel.ScopeSelfEdit, false), startOtpEnroll)
	g.Handle("PUT", "/otp", middleware.RequiresPermission(pufferpanel.ScopeSelfEdit, false), validateOtpEnroll)
	g.Handle("OPTIONS", "/otp", response.CreateOptions("GET", "POST", "PUT"))

	g.Handle("DELETE", "/otp/:token", middleware.RequiresPermission(pufferpanel.ScopeSelfEdit, false), disableOtp)
	g.Handle("OPTIONS", "/otp/:token", response.CreateOptions("DELETE"))

	g.Handle("GET", "/oauth2", middleware.RequiresPermission(pufferpanel.ScopeSelfClients, false), getPersonalOAuth2Clients)
	g.Handle("POST", "/oauth2", middleware.RequiresPermission(pufferpanel.ScopeSelfClients, false), createPersonalOAuth2Client)
	g.Handle("OPTIONS", "/oauth2", response.CreateOptions("GET", "POST"))

	g.Handle("DELETE", "/oauth2/:clientId", middleware.RequiresPermission(pufferpanel.ScopeSelfClients, false), deletePersonalOAuth2Client)
	g.Handle("OPTIONS", "/oauth2/:clientId", response.CreateOptions("DELETE"))
}

// @Summary Get your user info
// @Description Gets the user information of the current user
// @Success 200 {object} models.UserView
// @Failure 400 {object} pufferpanel.ErrorResponse
// @Failure 403 {object} pufferpanel.ErrorResponse
// @Failure 404 {object} pufferpanel.ErrorResponse
// @Failure 500 {object} pufferpanel.ErrorResponse
// @Router /api/self [get]
// @Security OAuth2Application[none]
func getSelf(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	c.JSON(http.StatusOK, models.FromUser(user))
}

// @Summary Update your user
// @Description Update user information for your current user
// @Success 204 {object} nil
// @Failure 400 {object} pufferpanel.ErrorResponse
// @Failure 403 {object} pufferpanel.ErrorResponse
// @Failure 404 {object} pufferpanel.ErrorResponse
// @Failure 500 {object} pufferpanel.ErrorResponse
// @Param user body models.UserView true "User information"
// @Router /api/self [PUT]
// @Security OAuth2Application[none]
func updateSelf(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	t, exist := c.Get("user")
	user, ok := t.(*models.User)

	if !exist || !ok {
		response.HandleError(c, pufferpanel.ErrUnknownError, http.StatusInternalServerError)
		return
	}

	var viewModel models.UserView
	if err := c.BindJSON(&viewModel); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if err := viewModel.Valid(true); response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	if viewModel.Password == "" {
		response.HandleError(c, pufferpanel.ErrFieldRequired("password"), http.StatusBadRequest)
		return
	}

	if !us.IsValidCredentials(user, viewModel.Password) {
		response.HandleError(c, pufferpanel.ErrInvalidCredentials, http.StatusInternalServerError)
		return
	}

	var oldEmail string
	if user.Email != viewModel.Email {
		oldEmail = user.Email
	}

	viewModel.CopyToModel(user)

	passwordChanged := false
	if viewModel.NewPassword != "" {
		passwordChanged = true
		err := user.SetPassword(viewModel.NewPassword)
		if response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
	}

	if err := us.Update(user); response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	if oldEmail != "" {
		err := services.GetEmailService().SendEmail(oldEmail, "emailChanged", map[string]interface{}{
			"NEW_EMAIL": user.Email,
		}, true)
		if err != nil {
			logging.Error.Printf("Error sending email: %s\n", err)
		}
	}

	if passwordChanged {
		err := services.GetEmailService().SendEmail(user.Email, "passwordChanged", nil, true)
		if err != nil {
			logging.Error.Printf("Error sending email: %s\n", err)
		}
	}

	c.Status(http.StatusNoContent)
}

func getOtpStatus(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	t, exist := c.Get("user")
	user, ok := t.(*models.User)

	if !exist || !ok {
		response.HandleError(c, pufferpanel.ErrUnknownError, http.StatusInternalServerError)
		return
	}

	otpEnabled, err := us.GetOtpStatus(user.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"otpEnabled": otpEnabled,
	})
}

func startOtpEnroll(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	t, exist := c.Get("user")
	user, ok := t.(*models.User)

	if !exist || !ok {
		response.HandleError(c, pufferpanel.ErrUnknownError, http.StatusInternalServerError)
		return
	}

	secret, img, err := us.StartOtpEnroll(user.ID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"secret": secret,
		"img":    img,
	})
}

func validateOtpEnroll(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	t, exist := c.Get("user")
	user, ok := t.(*models.User)

	if !exist || !ok {
		response.HandleError(c, pufferpanel.ErrUnknownError, http.StatusInternalServerError)
		return
	}

	request := &ValidateOtpRequest{}

	err := c.BindJSON(request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	err = us.ValidateOtpEnroll(user.ID, request.Token)
	if err == pufferpanel.ErrInvalidCredentials {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "otpEnabled", nil, true)
	if err != nil {
		logging.Error.Printf("Error sending email: %s\n", err)
	}
	c.Status(http.StatusNoContent)
}

func disableOtp(c *gin.Context) {
	db := middleware.GetDatabase(c)
	us := &services.User{DB: db}

	t, exist := c.Get("user")
	user, ok := t.(*models.User)

	if !exist || !ok {
		response.HandleError(c, pufferpanel.ErrUnknownError, http.StatusInternalServerError)
		return
	}

	err := us.DisableOtp(user.ID, c.Param("token"))
	if err == pufferpanel.ErrInvalidCredentials {
		response.HandleError(c, err, http.StatusBadRequest)
		return
	}
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "otpDisabled", nil, true)
	if err != nil {
		logging.Error.Printf("Error sending email: %s\n", err)
	}
	c.Status(http.StatusNoContent)
}

// @Summary Gets registered OAuth2 clients
// @Description Gets known OAuth2 clients the logged-in user has registered
// @Success 200 {object} []models.Client
// @Failure 400 {object} pufferpanel.ErrorResponse
// @Failure 403 {object} pufferpanel.ErrorResponse
// @Failure 404 {object} pufferpanel.ErrorResponse
// @Failure 500 {object} pufferpanel.ErrorResponse
// @Router /api/self/oauth2 [GET]
// @Security OAuth2Application[none]
func getPersonalOAuth2Clients(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	db := middleware.GetDatabase(c)
	os := &services.OAuth2{DB: db}

	clients, err := os.GetForUserAndServer(user.ID, "")
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	data := make([]*models.Client, 0)
	for _, v := range clients {
		if v.ServerId == "" {
			data = append(data, v)
		}
	}

	c.JSON(http.StatusOK, data)
}

// @Summary Create an account-level OAuth2 client
// @Success 200 {object} models.CreatedClient
// @Failure 400 {object} pufferpanel.ErrorResponse
// @Failure 403 {object} pufferpanel.ErrorResponse
// @Failure 404 {object} pufferpanel.ErrorResponse
// @Failure 500 {object} pufferpanel.ErrorResponse
// @Param client body models.Client false "Information for the client to create"
// @Router /api/self/oauth2 [POST]
// @Security OAuth2Application[none]
func createPersonalOAuth2Client(c *gin.Context) {
	user := c.MustGet("user").(*models.User)

	db := middleware.GetDatabase(c)
	os := &services.OAuth2{DB: db}

	var request models.Client
	err := c.BindJSON(&request)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	id, err := uuid.NewV4()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	client := &models.Client{
		ClientId:    id.String(),
		UserId:      user.ID,
		Name:        request.Name,
		Description: request.Description,
	}

	secret, err := pufferpanel.GenerateRandomString(36)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = client.SetClientSecret(secret)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = os.Update(client)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "oauthCreated", nil, true)
	if err != nil {
		logging.Error.Printf("Error sending email: %s\n", err)
	}

	c.JSON(http.StatusOK, models.CreatedClient{
		ClientId:     client.ClientId,
		ClientSecret: secret,
	})
}

// @Summary Deletes an account-level OAuth2 client
// @Success 204 {object} nil
// @Failure 400 {object} pufferpanel.ErrorResponse
// @Failure 403 {object} pufferpanel.ErrorResponse
// @Failure 404 {object} pufferpanel.ErrorResponse
// @Failure 500 {object} pufferpanel.ErrorResponse
// @Param id path string true "Information for the client to create"
// @Router /api/self/oauth2/{id} [DELETE]
// @Security OAuth2Application[none]
func deletePersonalOAuth2Client(c *gin.Context) {
	user := c.MustGet("user").(*models.User)
	clientId := c.Param("clientId")

	db := middleware.GetDatabase(c)
	os := &services.OAuth2{DB: db}

	client, err := os.Get(clientId)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	//ensure the client id is specific for this server, and this user
	if client.UserId != user.ID || client.ServerId != "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	err = os.Delete(client)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	err = services.GetEmailService().SendEmail(user.Email, "oauthDeleted", nil, true)
	if err != nil {
		logging.Error.Printf("Error sending email: %s\n", err)
	}
	c.Status(http.StatusNoContent)
}

type ValidateOtpRequest struct {
	Token string `json:"token"`
}
