package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/pkg/errors"
	"io"
	v1 "k8s.io/api/core/v1"
	kmetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"net"
	"net/http"
	"strconv"
	"time"
)

// LogStreamApiConfig stores config for log streaming api
type LogStreamApiConfig struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

// Validate validates LogStreamApiConfig
func (c *LogStreamApiConfig) Validate() error {

	if c.Address == "" {
		return errors.New("address missing")
	}

	if c.Port < 1 || c.Port > 65536 {
		return errors.New("invalid port number")
	}

	return nil
}

// LogStreamApi streams log per http rest api
type LogStreamApi struct {
	clientset *kubernetes.Clientset
	logger    *logging.Logger
	config    *LogStreamApiConfig
}

// NewLogStreamApi creates new LogStreamApi initialized with clienset, logger and config
func NewLogStreamApi(clientset *kubernetes.Clientset, logger *logging.Logger, config *LogStreamApiConfig) *LogStreamApi {
	return &LogStreamApi{
		clientset: clientset,
		logger:    logger,
		config:    config,
	}
}

// Handle returns HandlerFunc that handles the api
func (lsa *LogStreamApi) Handle() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lsa.HandleHttp(w, r)
		lsa.logger.Debug("Finished sending response...")
	}
}

// HandleHttp handles the api
func (lsa *LogStreamApi) HandleHttp(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	namespace := query.Get("namespace")
	podName := query.Get("podName")
	containerName := query.Get("containerName")
	lastTimestampParam, err := strconv.ParseInt(query.Get("lastTimestamp"), 10, 64)
	if err != nil {
		fmt.Println("error getting last timestamp from url:", err)
	}

	lastTimestamp := kmetav1.Time{Time: time.Unix(lastTimestampParam, 0)}

	ctx := r.Context()
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, r)
		return
	}

	podLogOpts := v1.PodLogOptions{
		Container:  containerName,
		Timestamps: true,
		Follow:     true,
		SinceTime:  &lastTimestamp,
	}

	reader, err := lsa.clientset.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts).Stream(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		flusher.Flush()
		return
	}

	// Send the initial headers saying we're gonna stream the response.
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	enc := json.NewEncoder(w)

	stringReader := bufio.NewReader(reader)

	lsa.logger.Debug("Connected")

	for {
		select {
		case <-ctx.Done():
			lsa.logger.Debug("Connection closed")
			return
		default:
			msg, err := stringReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				break
			}

			// Send some data
			err = enc.Encode(msg)
			if err != nil {
				lsa.logger.Fatal(err)
			}

			flusher.Flush()
		}
	}
}

// Stream starts a local webserver and provides the api
func (lsa *LogStreamApi) Stream(ctx context.Context) (err error) {

	mux := http.NewServeMux()
	mux.Handle(lsa.config.Address, lsa.Handle())

	srv := &http.Server{
		Addr:    ":" + strconv.Itoa(lsa.config.Port),
		Handler: mux,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			lsa.logger.Fatalf("listen:%+s\n", err)
		}
	}()

	lsa.logger.Info("Starting stream")

	select {
	case <-ctx.Done():
		lsa.logger.Info("Stopped")
		//
		ctxShutDown, cancel := context.WithCancel(context.Background())
		defer cancel()

		err := srv.Shutdown(ctxShutDown)
		if err != nil {
			lsa.logger.Fatalf("Shutdown Failed:%+s", err)
		}

		lsa.logger.Info("Exited properly")

		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}

		return err
	}
}
