package pb

import timestamppb "google.golang.org/protobuf/types/known/timestamppb"

type DeployContainerRequest struct {
	ContainerId string
	ProjectPath string
	RepoUrl     string
	Branch      string
	ComposeFile string
	Service     string
	GitUsername string
	GitToken    string
	Bootstrap   bool
	RequestedBy string
}

type DeployContainerResponse struct {
	RunId string
}

type GetDeployStatusRequest struct {
	RunId string
}

type GetDeployStatusResponse struct {
	RunId       string
	ContainerId string
	State       string
	Message     string
	StartedAt   *timestamppb.Timestamp
	FinishedAt  *timestamppb.Timestamp
	ExitCode    int32
}

type GetDeployLogsRequest struct {
	RunId  string
	Offset int32
}

type GetDeployLogsResponse struct {
	RunId   string
	Offset  int32
	Lines   []string
	Next    int32
	HasMore bool
	Done    bool
}

type GetRecentDeploysRequest struct {
	ContainerId string
	Limit       int32
}

type DeployRunStatus struct {
	RunId       string
	ContainerId string
	State       string
	Message     string
	StartedAt   *timestamppb.Timestamp
	FinishedAt  *timestamppb.Timestamp
	ExitCode    int32
}

type GetRecentDeploysResponse struct {
	Items []*DeployRunStatus
}

