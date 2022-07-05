package listener

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const paramContractVersion = "version"

func StartListenerComponent(ctx context.Context, addr string, componentName string) *source.Channel {

	var log logr.Logger
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("failed to initialize zap logger: %v?", err))
	}
	log = zapr.NewLogger(zapLog)

	skrEventsListener := &SKREventsListener{
		Addr:           addr,
		Logger:         log,
		ReceivedEvents: make(chan event.GenericEvent),
		ComponentName:  componentName,
	}
	go func() {
		if err := skrEventsListener.start(ctx); err != nil {
			log.Error(err, "Failed to start SKR listener...")
		}
	}()
	return &source.Channel{Source: skrEventsListener.ReceivedEvents}
}

type SKREventsListener struct {
	Addr           string
	Logger         logr.Logger
	ComponentName  string
	ReceivedEvents chan event.GenericEvent
}

func (l *SKREventsListener) start(ctx context.Context) error {
	//routing
	mainRouter := mux.NewRouter()
	apiRouter := mainRouter.PathPrefix("/").Subrouter()

	apiRouter.HandleFunc(
		fmt.Sprintf("/v{%s}/%s/event", paramContractVersion, l.ComponentName),
		l.handleSKREvent(),
	).Methods(http.MethodPost)

	//start web server
	server := &http.Server{Addr: l.Addr, Handler: mainRouter}
	go func() {
		l.Logger.Info("SKR events listener is starting up...")
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			l.Logger.Error(err, "Webserver startup failed")
		}
	}()
	<-ctx.Done()
	l.Logger.Info("SKR events listener is shutting down: context got closed")
	return server.Shutdown(ctx)
}

func (l *SKREventsListener) handleSKREvent() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l.Logger.Info("received event from SKR")

		//unmarshal received event
		genericEvtObject, unmarshalErr := unmarshalSKREvent(r)
		if unmarshalErr != nil {
			l.Logger.Error(nil, unmarshalErr.Message)
			w.WriteHeader(unmarshalErr.httpErrorCode)
			w.Write([]byte(unmarshalErr.Message))
			return
		}

		//add event to the channel
		l.ReceivedEvents <- event.GenericEvent{Object: genericEvtObject}
		l.Logger.Info("dispatched event object into channel", "resource", genericEvtObject.GetName())
		w.WriteHeader(http.StatusOK)
	}
}

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

func unmarshalSKREvent(r *http.Request) (client.Object, *UnmarshalError) {
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
