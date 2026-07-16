package apiserver

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CallReconcile(eventStream chan<- ApplyRequestDto) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var message ChannelMessageDto
		if err := ctx.ShouldBindJSON(&message); err != nil {
			ctx.JSON(http.StatusBadRequest, ApplyResponseDTO{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}

		reply := make(chan ReconcileResultDto, 1)
		request := ApplyRequestDto{
			Message: message,
			Reply:   reply,
		}
		requestContext := ctx.Request.Context()

		select {
		case eventStream <- request:
		case <-requestContext.Done():
			ctx.JSON(http.StatusRequestTimeout, ApplyResponseDTO{
				Status:     "error",
				Deployment: message.DeploymentName,
				Message:    "request canceled before reconciliation started",
			})
			return
		default:
			ctx.JSON(http.StatusConflict, ApplyResponseDTO{
				Status:     "busy",
				Deployment: message.DeploymentName,
				Message:    "reconcile request is already queued",
			})
			return
		}

		select {
		case result := <-reply:
			writeReconcileResult(ctx, result)
		case <-requestContext.Done():
			ctx.JSON(http.StatusRequestTimeout, ApplyResponseDTO{
				Status:     "error",
				Deployment: message.DeploymentName,
				Message:    "request canceled while waiting for reconciliation",
			})
		}
	}
}

func writeReconcileResult(ctx *gin.Context, result ReconcileResultDto) {
	if result.Err != nil {
		ctx.JSON(http.StatusInternalServerError, ApplyResponseDTO{
			Status:     "error",
			Deployment: result.DeploymentName,
			Message:    result.Err.Error(),
		})
		return
	}

	if !result.Created {
		ctx.JSON(http.StatusConflict, ApplyResponseDTO{
			Status:     "not_reconciled",
			Deployment: result.DeploymentName,
			Message:    "reconcile finished without creating the deployment",
		})
		return
	}

	ctx.JSON(http.StatusOK, ApplyResponseDTO{
		Status:     "reconciled",
		Deployment: result.DeploymentName,
		Message:    "deployment reconciled",
	})
}
