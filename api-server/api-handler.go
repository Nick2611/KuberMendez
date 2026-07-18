package apiserver

import (
	
	"net/http"

	"kuberMendez/utils"
	"kuberMendez/docker"

	"github.com/gin-gonic/gin"
)

func CallReconcile(eventStream chan<- ApplyRequestDto) gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var req SaveDeploymentRequestDto

		if err := ctx.ShouldBindJSON(&req); err != nil {
			ctx.JSON(http.StatusBadRequest, ApplyResponseDTO{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		stateFileName, err := utils.SaveStateFile(req.DeploymentName, utils.DefaultDeploymentsDirectory)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, ApplyResponseDTO{
				Status:  "error",
				Message: err.Error(),
			})
			return
		}
		req.DeploymentName = stateFileName //deploymentname + .yaml

		reply := make(chan ReconcileResultDto, 1)
		request := ApplyRequestDto{
			Message: req,
			Reply:   reply,
		}
		requestContext := ctx.Request.Context()

		select {
		case eventStream <- request:
		case <-requestContext.Done():
			ctx.JSON(http.StatusRequestTimeout, ApplyResponseDTO{
				Status:     "error",
				Deployment: req.DeploymentName,
				Message:    "request canceled before reconciliation started",
			})
			return
		default:
			ctx.JSON(http.StatusConflict, ApplyResponseDTO{
				Status:     "busy",
				Deployment: req.DeploymentName,
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
				Deployment: req.DeploymentName,
				Message:    "request canceled while waiting for reconciliation",
			})
		}
	}
}

func GetDeploymentStatus() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var req GetDeploymentStatusRequestDto

		if err := ctx.ShouldBindQuery(&req); err != nil {

			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

			return

		}

		requestContext := ctx.Request.Context()

		containers, err := docker.ListContainers(requestContext, req.DeploymentName)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"error": err.Error(),
			})
			return
		}
		
		//provisional until I figure out what's the best way to return current state
		//TODO
		var currentStateFileStruct GetDeploymentStatusResponseDto
		currentStateFileStruct.DeploymentName = req.DeploymentName
		currentStateFileStruct.Replicas = len(containers)


		ctx.JSON(
			http.StatusOK,
			currentStateFileStruct,
		)
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
