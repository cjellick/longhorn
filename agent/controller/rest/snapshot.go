package rest

import (
	"fmt"
	"net/http"

	"bufio"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"os/exec"
)

func (s *Server) ListSnapshots(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	snapshots, err := s.listSnapshot(apiContext)
	if err != nil {
		return err
	}
	apiContext.Write(&client.GenericCollection{
		Data: []interface{}{
			snapshots,
		},
	})
	return nil
}

func (s *Server) GetSnapshot(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	vars := mux.Vars(req)
	id, ok := vars["id"]
	if !ok {
		return fmt.Errorf("Snapshot id not supplied.")
	}

	v, err := s.getSnapshot(apiContext, id)
	if err != nil {
		return err
	}

	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	apiContext.Write(v)
	return nil
}

func (s *Server) CreateSnapshot(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	var snapshot Snapshot
	if err := apiContext.Read(&snapshot); err != nil {
		return err
	}

	name, err := s.controllerClient.Snapshot(snapshot.Name)
	if err != nil {
		return err
	}

	apiContext.Write(NewSnapshot(apiContext, name))

	return nil
}

func (s *Server) DeleteSnapshot(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	vars := mux.Vars(req)
	id, ok := vars["id"]
	if !ok {
		return fmt.Errorf("Snapshot id not supplied.")
	}

	v, err := s.getSnapshot(apiContext, id)
	if err != nil {
		return err
	}

	if v == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	cmd := exec.Command("longhorn", "snapshots", "rm", v.Id)
	if err := cmd.Run(); err != nil {
		return err
	}

	rw.WriteHeader(http.StatusNoContent)
	return nil
}

func (s *Server) listSnapshot(context *api.ApiContext) ([]*Snapshot, error) {
	cmd := exec.Command("longhorn", "snapshots")
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(output)
	first := true
	snapshots := []*Snapshot{}
	for scanner.Scan() {
		if first {
			first = false
			continue
		}

		name := scanner.Text()
		snap := NewSnapshot(context, name)
		snapshots = append(snapshots, snap)
	}

	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return snapshots, nil
}

func (s *Server) getSnapshot(context *api.ApiContext, id string) (*Snapshot, error) {
	snapshots, err := s.listSnapshot(context)
	if err != nil {
		return nil, err
	}
	for _, s := range snapshots {
		if s.Id == id {
			return s, nil
		}
	}
	return nil, nil
}
