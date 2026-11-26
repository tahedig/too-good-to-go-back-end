package handlerinterface

import "github.com/gin-gonic/gin"

type ReservationHandler interface {
	CreateReservation(c *gin.Context)
}
