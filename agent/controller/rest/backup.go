package rest

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"

	"bytes"
	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"
	"github.com/rancher/go-rancher/api"
	"strings"
)

var restoreMutex = &sync.RWMutex{}
var restoreMap = make(map[string]*status)

var backupMutex = &sync.RWMutex{}
var backupMap = make(map[string]*status)

func (s *Server) CreateBackup(rw http.ResponseWriter, req *http.Request) error {
	logrus.Infof("Creating backup")

	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	snapshot, err := s.getSnapshot(apiContext, id)
	if err != nil {
		return err
	}

	if snapshot == nil {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	var input backupInput
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	if input.Target == "" || input.UUID == "" {
		rw.WriteHeader(http.StatusBadRequest)
		return nil
	}

	status, err := backup(input.UUID, snapshot.Id, input.Target)
	if err != nil {
		return err
	}

	return apiContext.WriteResource(status)
}

func (s *Server) RemoveBackup(rw http.ResponseWriter, req *http.Request) error {
	logrus.Infof("Removing backup")
	apiContext := api.GetApiContext(req)

	var input locationInput
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	if input.Location == "" {
		rw.WriteHeader(http.StatusBadRequest)
		return nil
	}

	if backupNotExists(input.Location) {
		rw.WriteHeader(http.StatusNoContent)
		return nil
	}

	cmd := exec.Command("longhorn", "backup", "rm", input.Location)
	if err := cmd.Run(); err != nil {
		return err
	}

	rw.WriteHeader(http.StatusNoContent)
	return nil
}

func backupNotExists(backupLocation string) bool {
	if !strings.HasPrefix(backupLocation, "vfs://") {
		return true
	}

	loc := strings.TrimPrefix(backupLocation, "vfs://")
	_, err := os.Stat(loc)
	return os.IsNotExist(err)

}

func (s *Server) RestoreFromBackup(rw http.ResponseWriter, req *http.Request) error {
	logrus.Infof("Restoring from backup")

	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	if id != "1" {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	var input locationInput
	if err := apiContext.Read(&input); err != nil {
		return err
	}

	if input.Location == "" || input.UUID == "" {
		rw.WriteHeader(http.StatusBadRequest)
		return nil
	}

	restoreStatus, err := restore(input.UUID, input.Location)
	if err != nil {
		return err
	}

	return apiContext.WriteResource(restoreStatus)
}

func restore(uuid, location string) (*status, error) {
	cmd := exec.Command("longhorn", "backup", "restore", location)
	return doStatusBackedCommand(uuid, "restorestatus", cmd, restoreMap, restoreMutex)
}

func backup(uuid, snapshot, destination string) (*status, error) {
	cmd := exec.Command("longhorn", "backup", "create", snapshot, "--dest", destination)
	return doStatusBackedCommand(uuid, "backupstatus", cmd, backupMap, backupMutex)
}

func doStatusBackedCommand(id, resourceType string, command *exec.Cmd, statusMap map[string]*status, statusMutex *sync.RWMutex) (*status, error) {
	output := new(bytes.Buffer)
	command.Stdout = output
	command.Stderr = os.Stderr
	err := command.Start()
	if err != nil {
		return &status{}, err
	}

	statusMutex.Lock()
	defer statusMutex.Unlock()
	status := newStatus(id, "running", "", resourceType)
	statusMap[id] = status

	go func(id string, c *exec.Cmd) {
		var message string
		var state string

		err := c.Wait()
		if err != nil {
			state = "error"
			message = fmt.Sprintf("Error: %v", err)
		} else {
			state = "done"
			message = output.String()
		}

		statusMutex.Lock()
		defer statusMutex.Unlock()
		status, ok := statusMap[id]
		if !ok {
			status = newStatus(id, "", "", resourceType)
		}

		status.State = state
		status.Message = message
		restoreMap[id] = status
	}(id, command)

	return status, nil
}

func (s *Server) GetBackupStatus(rw http.ResponseWriter, req *http.Request) error {
	logrus.Infof("Getting backup status")
	return getStatus(backupMap, backupMutex, rw, req)
}

func (s *Server) GetRestoreStatus(rw http.ResponseWriter, req *http.Request) error {
	logrus.Infof("Getting restore status")
	return getStatus(restoreMap, restoreMutex, rw, req)
}

func getStatus(statusMap map[string]*status, statusMutex *sync.RWMutex, rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	id := mux.Vars(req)["id"]

	statusMutex.RLock()
	defer statusMutex.RUnlock()

	status, ok := statusMap[id]
	if !ok {
		rw.WriteHeader(http.StatusNotFound)
		return nil
	}

	return apiContext.WriteResource(status)
}
