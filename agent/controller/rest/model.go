package rest

import (
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	lclient "github.com/rancher/longhorn/client"
)

type Snapshot struct {
	client.Resource
	Name string `json:"name"`
}

type SnapshotCollection struct {
	client.Collection
	Data []Snapshot `json:"data"`
}

func NewSnapshot(context *api.ApiContext, name string) *Snapshot {
	snapshot := &Snapshot{
		Resource: client.Resource{
			Id:      name,
			Type:    "snapshot",
			Actions: map[string]string{},
		},
		Name: name,
	}

	return snapshot
}

func NewSchema() *client.Schemas {
	schemas := &client.Schemas{}

	schemas.AddType("error", client.ServerApiError{})
	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})

	snapshot := schemas.AddType("snapshot", Snapshot{})
	snapshot.CollectionMethods = []string{"GET", "POST"}
	snapshot.ResourceMethods = []string{"GET", "PUT", "DELETE"}

	return schemas
}

type Server struct {
	controllerClient *lclient.ControllerClient
}

func NewServer() *Server {
	contollerClient := lclient.NewControllerClient("http://localhost:9501")
	return &Server{
		controllerClient: contollerClient,
	}
}
