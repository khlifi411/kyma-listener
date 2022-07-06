package listener

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type WatcherEvent struct {
	SkrClusterID string `json:"skrClusterID"`
	Component    string `json:"body"`
	Namespace    string `json:"namespace"`
	Name         string `json:"name"`
}

type UnmarshalError struct {
	Message       string
	httpErrorCode int
}

func unmarshalSKREvent(r *http.Request) (*unstructured.Unstructured, *UnmarshalError) {
	params := mux.Vars(r)
	contractVersion, ok := params[paramContractVersion]
	if !ok {
		return nil, &UnmarshalError{"contract version could not be parsed", http.StatusBadRequest}
	}

	if contractVersion == "" {
		return nil, &UnmarshalError{"contract version cannot be empty", http.StatusBadRequest}
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, &UnmarshalError{"could not read request body", http.StatusInternalServerError}
	}

	watcherEvent := &WatcherEvent{}
	err = json.Unmarshal(body, watcherEvent)
	if err != nil {
		return nil, &UnmarshalError{"could not unmarshal watcher event", http.StatusInternalServerError}
	}

	genericEvtObject := &unstructured.Unstructured{}
	genericEvtObject.SetName(watcherEvent.Name)
	genericEvtObject.SetNamespace(watcherEvent.Namespace)

	return genericEvtObject, nil
}
