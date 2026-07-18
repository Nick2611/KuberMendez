package apiserver

// import (
// 	"github.com/moby/moby/api/types/container"
// )

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

// type GetDeploymentStatusResponseDto struct {
// 	ID string							`json:"id"`
// 	Labels map[string]string			`json:"labels"`
// 	Name []string						`json:"name"`
// 	Image string						`json:"image"`
// 	Status string						`json:"status"`
// 	Port []container.PortSummary		`json:"port"`
// }

type GetDeploymentStatusResponseDto struct{
	DeploymentName	string	`json:"deploymentName"`
	Replicas		int		`json:"replicas"`
}
