package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

type authHandler struct {
	svc serviceinterface.AuthService
}

func NewAuthHandler(svc serviceinterface.AuthService) handlerinterface.AuthHandler {
	return &authHandler{svc: svc}
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
// @Summary      Verify a one-time password
// @Description  Checks the code and consumes it on success. A malformed code is rejected without using up an attempt.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        body body dto.VerifyOTPRequest true "Phone number and code"
// @Success      200 {object} dto.VerifyOTPResponse
// @Failure      400 {object} dto.ErrorResponse
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

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, dto.VerifyOTPResponse{Verified: true, Phone: res.Phone})
}
