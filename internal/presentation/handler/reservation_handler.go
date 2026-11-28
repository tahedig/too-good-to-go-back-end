package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/tahedig/too-good-to-go/backend/internal/application/dto"
	serviceinterface "github.com/tahedig/too-good-to-go/backend/internal/application/service/interface"
	"github.com/tahedig/too-good-to-go/backend/internal/bootstrap"
	handlerinterface "github.com/tahedig/too-good-to-go/backend/internal/presentation/handler/interface"
)

const idempotencyKeyHeader = "Idempotency-Key"

type reservationHandler struct {
	svc serviceinterface.ReservationService
}

// NewReservationHandler creates a ReservationHandler for TEAM-49 and TEAM-51.
func NewReservationHandler(svc serviceinterface.ReservationService) handlerinterface.ReservationHandler {
	return &reservationHandler{svc: svc}
}

// CreateReservation godoc
// @Summary      Create a reservation (checkout)
// @Description  Creates a customer reservation with atomic inventory decrement. Supply an Idempotency-Key header to make repeated submissions safe; replaying the same key with the same payload returns the existing reservation (200), while reusing it with different parameters returns 409.
// @Tags         reservations
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        Idempotency-Key header string                       false "Optional idempotency key for safe retries"
// @Param        body            body   dto.CreateReservationRequest true  "Reservation checkout payload"
// @Success      201 {object} dto.ReservationResponse "Reservation created"
// @Success      200 {object} dto.ReservationResponse "Idempotent replay of an existing reservation"
// @Failure      400 {object} dto.ErrorResponse "BAD_REQUEST: invalid offer_id, quantity, terms_accepted or Idempotency-Key"
// @Failure      401 {object} dto.ErrorResponse "UNAUTHORIZED: missing or invalid bearer token"
// @Failure      404 {object} dto.ErrorResponse "NOT_FOUND: offer does not exist"
// @Failure      409 {object} dto.ErrorResponse "CONFLICT: offer unavailable, insufficient stock, or idempotency key parameter mismatch"
// @Failure      500 {object} dto.ErrorResponse
// @Router       /reservations [post]
func (h *reservationHandler) CreateReservation(c *gin.Context) {
	var req dto.CreateReservationRequest
	if err := bindJSON(c, &req); err != nil {
		abortWithError(c, err)
		return
	}

	out, err := h.svc.CreateReservation(
		c.Request.Context(),
		c.GetString(bootstrap.CtxKeyUserID),
		serviceinterface.CreateReservationInput{
			OfferID:        req.OfferID,
			Quantity:       req.Quantity,
			TermsAccepted:  req.TermsAccepted,
			IdempotencyKey: c.GetHeader(idempotencyKeyHeader),
		},
	)
	if err != nil {
		abortWithError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	if out.Replayed {
		c.Header("Idempotency-Replayed", "true")
		c.JSON(http.StatusOK, out.Reservation)
		return
	}
	c.JSON(http.StatusCreated, out.Reservation)
}
