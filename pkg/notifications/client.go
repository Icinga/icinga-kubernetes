package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-kubernetes/internal"
	"github.com/pkg/errors"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
)

// Notifiable can be implemented by all k8s types that want to submit notification events to Icinga Notifications.
type Notifiable interface {
	// GetNotificationsEvent returns the event data of this type that will be transmitted to Icinga Notifications.
	GetNotificationsEvent(baseUrl *url.URL) map[string]any
}

type Client struct {
	db     *database.DB
	client http.Client
	Config
}

func NewClient(db *database.DB, c Config) *Client {
	return &Client{db: db, client: http.Client{}, Config: c}
}

func (c *Client) ProcessEvent(notifiable Notifiable) error {
	baseUrl, err := url.Parse(c.Config.KubernetesWebUrl)
	if err != nil {
		return errors.Wrapf(err, "cannot parse Icinga for Kubernetes Web URL: %q", c.Config.KubernetesWebUrl)
	}

	body, err := json.Marshal(notifiable.GetNotificationsEvent(baseUrl))
	if err != nil {
		return errors.Wrapf(err, "cannot marshal notifications event data of type: %T", notifiable)
	}

	r, err := http.NewRequest(http.MethodPost, c.Config.Url+"/process-event", bytes.NewBuffer(body))
	if err != nil {
		return errors.Wrap(err, "cannot create new notifications http request")
	}

	r.SetBasicAuth(c.Config.Username, c.Config.Password)
	r.Header.Set("User-Agent", "icinga-kubernetes/"+internal.Version.Version)
	r.Header.Add("Content-Type", "application/json")

	res, err := c.client.Do(r)
	if err != nil {
		return errors.Wrap(err, "cannot send notifications event")
	}
	defer func() {
		_, _ = io.Copy(io.Discard, res.Body)
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusAlreadyReported {
		return errors.Errorf("received unexpected http status code from Icinga Notifications: %d", res.StatusCode)
	}

	return nil
}

// Stream consumes the items from the given `entities` chan and triggers a notifications event for each of them.
func (c *Client) Stream(ctx context.Context, entities <-chan any) error {
	for {
		select {
		case entity, more := <-entities:
			if !more {
				return nil
			}

			if err := c.ProcessEvent(entity.(Notifiable)); err != nil {
				klog.Error(err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
