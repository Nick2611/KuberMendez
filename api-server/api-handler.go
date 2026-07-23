package apiserver

import (
	"net/http"
	"strconv"

	"kuberMendez/docker"
	"kuberMendez/utils"

	"github.com/gin-gonic/gin"
	"github.com/moby/moby/api/types/container"
)

const (
	defaultLogLines = 50
	maxLogLines     = 500
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
			return
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

		if len(containers) == 0{
			ctx.JSON(
				http.StatusOK,
				gin.H{
					"Status":"No running containers",
				},
			)

			return
		}

		//provisional until I figure out what's the best way to return current state
		//TODO
		var currentStateFileStruct GetDeploymentStatusResponseDto
		var ports []container.PortSummary

		for _, c := range containers{
			ports = append(ports, c.Ports...)
		}

		currentStateFileStruct.DeploymentName = req.DeploymentName
		currentStateFileStruct.Image = containers[0].Image
		currentStateFileStruct.Port = ports
		currentStateFileStruct.Replicas = len(containers)

		ctx.JSON(
			http.StatusOK,
			currentStateFileStruct,
		)
	}
}

func DeleteDeployment() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req DeleteDeploymentRequestDto

		if err := ctx.ShouldBindJSON(&req); err != nil {

			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})

			return

		}

		status, err := utils.DeleteFile(req.DeploymentName)
		if err != nil {
			switch status {
			case "File does not exist":
				ctx.JSON(http.StatusNotFound, DeleteDeploymentResponseDto{
					DeploymentName: req.DeploymentName,
					Status:         status,
					Error:          err,
				})
			case "Unexpected error":
				ctx.JSON(http.StatusNotFound, DeleteDeploymentResponseDto{
					DeploymentName: req.DeploymentName,
					Status:         status,
					Error:          err,
				})
			}

		}

		requestContext := ctx.Request.Context()

		err = docker.RemoveContainers(requestContext, req.DeploymentName)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, DeleteDeploymentResponseDto{
				DeploymentName: req.DeploymentName,
				Status:         "Error while removing container",
				Error:          err,
			})

			return
		}

		ctx.JSON(http.StatusOK, DeleteDeploymentResponseDto{
			DeploymentName: req.DeploymentName,
			Status:         "Deployment deleted successfully",
		})

	}
}

func StreamLogs() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		lines, err := requestedLogLines(ctx.Query("lines"))
		if err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx.Header("Content-Type", "text/plain; charset=utf-8")
		ctx.Status(http.StatusAccepted)
		if err := utils.StreamFileTail(ctx.Writer, ".kubermendez/daemon.log", lines); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
}

func requestedLogLines(value string) (int, error) {
	if value == "" {
		return defaultLogLines, nil
	}

	lines, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	if lines < 0 {
		return 0, nil
	}
	if lines > maxLogLines {
		return maxLogLines, nil
	}

	return lines, nil
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
