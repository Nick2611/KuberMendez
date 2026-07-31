package apiserver

import runtimecontract "kuberMendez/runtime"

type SaveDeploymentRequestDto struct {
	DeploymentName string `json:"deploymentName" binding:"required"`
}

type ApplyRequestDto struct {
	Message SaveDeploymentRequestDto
	Reply   chan ReconcileResultDto `json:"-"`
}

type ApplyResponseDTO struct {
	Status     string `json:"status"`
	Deployment string `json:"deployment,omitempty"`
	Message    string `json:"message,omitempty"`
}

type ReconcileResultDto struct {
	DeploymentName string `json:"deploymentName"`
	Created        bool   `json:"created"`
	Err            error  `json:"-"`
}

type GetDeploymentStatusRequestDto struct {
	DeploymentName string `form:"deploymentName" binding:"required"`
}

type GetDeploymentStatusResponseDto struct {
	DeploymentName string                      `json:"deploymentName"`
	Image          string                      `json:"image"`
	Ports          []runtimecontract.PortState `json:"ports"`
	Replicas       int                         `json:"replicas"`
}

type DeleteDeploymentRequestDto struct {
	DeploymentName string `json:"deploymentName" binding:"required"`
}

type DeleteDeploymentResponseDto struct {
	DeploymentName string `json:"deploymentName"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
}
