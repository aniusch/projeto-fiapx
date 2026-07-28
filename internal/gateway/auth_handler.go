package gateway

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/aniusch/projeto-fiapx/internal/auth"
	"github.com/aniusch/projeto-fiapx/internal/domain"
)

// handleRegister creates an account and returns a token so the client is
// immediately logged in.
func (s *Server) handleRegister(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("could not process password"))
		return
	}

	user := &domain.User{Email: req.Email, PasswordHash: hash}
	err = s.users.Create(c.Request.Context(), user)
	if errors.Is(err, domain.ErrDuplicate) {
		c.JSON(http.StatusConflict, errorResponse("email already registered"))
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("could not create user"))
		return
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("could not issue token"))
		return
	}

	c.JSON(http.StatusCreated, authResponse{Token: token})
}

// handleLogin verifies credentials and returns a token. It deliberately returns
// the same 401 whether the email is unknown or the password is wrong, so an
// attacker cannot use the response to discover which emails are registered.
func (s *Server) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	user, err := s.users.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, errorResponse("invalid credentials"))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse("could not look up user"))
		return
	}

	if !auth.CheckPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, errorResponse("invalid credentials"))
		return
	}

	token, err := s.tokens.Issue(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("could not issue token"))
		return
	}

	c.JSON(http.StatusOK, authResponse{Token: token})
}
