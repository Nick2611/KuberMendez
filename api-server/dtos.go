package apiserver

type ChannelMessageDto struct {
	DeploymentName string `json:"deploymentName" binding:"required"`
}

type ApplyRequestDto struct {
	Message ChannelMessageDto
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
