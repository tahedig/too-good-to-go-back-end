package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type authHandler struct {
	svc      serviceinterface.AuthService
	sessions serviceinterface.SessionService
}

// NewAuthHandler wires the OTP endpoints. A successful verify logs the user
// in through sessions and returns an access token.
func NewAuthHandler(svc serviceinterface.AuthService, sessions serviceinterface.SessionService) handlerinterface.AuthHandler {
	return &authHandler{svc: svc, sessions: sessions}
}

// RequestOTP godoc
// @Summary      Request a one-time password
// @Description  Sends an OTP by SMS to the phone number. The code is never returned, except as debug_code when OTP_EXPOSE_CODE is enabled outside production.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body dto.RequestOTPRequest true "Phone number"
// @Success      200 {object} dto.RequestOTPResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      413 {object} dto.ErrorResponse
// @Failure      429 {object} dto.ErrorResponse "OTP_RESEND_TOO_SOON or TOO_MANY_REQUESTS; see Retry-After"
// @Router       /auth/otp/request [post]
func (h *authHandler) RequestOTP(c *gin.Context) {
	var req dto.RequestOTPRequest
	if err := bindJSON(c, &req); err != nil {
		abortWithError(c, err)
		return
	}

	res, err := h.svc.RequestOTP(c.Request.Context(), req.Phone)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, dto.RequestOTPResponse{
		Message:          "verification code sent",
		ExpiresInSeconds: int(res.TTL.Seconds()),
		DebugCode:        res.DebugCode,
	})
}

// VerifyOTP godoc
// @Summary      Verify a one-time password and log in
// @Description  Checks the code and consumes it on success, then logs the user in (creating the account on first login) and returns a JWT access token. A malformed code is rejected without using up an attempt.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body dto.VerifyOTPRequest true "Phone number and code"
// @Success      200 {object} dto.VerifyOTPResponse
// @Failure      400 {object} dto.ErrorResponse
// @Failure      403 {object} dto.ErrorResponse "FORBIDDEN: the account is suspended or deleted"
// @Failure      413 {object} dto.ErrorResponse
// @Failure      429 {object} dto.ErrorResponse "OTP_ATTEMPTS_EXCEEDED or TOO_MANY_REQUESTS"
// @Router       /auth/otp/verify [post]
func (h *authHandler) VerifyOTP(c *gin.Context) {
	var req dto.VerifyOTPRequest
	if err := bindJSON(c, &req); err != nil {
		abortWithError(c, err)
		return
	}

	res, err := h.svc.VerifyOTP(c.Request.Context(), req.Phone, req.Code)
	if err != nil {
		abortWithError(c, err)
		return
	}

	// The phone is now proven, so open a session for it.
	session, err := h.sessions.Login(c.Request.Context(), res.Phone)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, dto.VerifyOTPResponse{
		Verified:    true,
		Phone:       res.Phone,
		AccessToken: session.AccessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(session.ExpiresIn.Seconds()),
		User: dto.AuthUserInfo{
			ID:    session.User.ID,
			Phone: session.User.Phone,
			Role:  string(session.User.Role),
		},
	})
}
