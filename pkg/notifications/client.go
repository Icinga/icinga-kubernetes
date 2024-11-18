package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"k8s.io/klog/v2"
	"net/http"
	"net/url"
)

type Client struct {
	client          http.Client
	userAgent       string
	processEventUrl string
	webUrl          *url.URL
}

func NewClient(name string, config Config) (*Client, error) {
	baseUrl, err := url.Parse(config.Url)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse url")
	}

	webUrl, err := url.Parse(config.KubernetesWebUrl)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse web url")
	}

	return &Client{
		client: http.Client{
			Transport: &basicAuthTransport{
				RoundTripper: http.DefaultTransport,
				username:     config.Username,
				password:     config.Password,
			},
		},
		userAgent:       name,
		processEventUrl: baseUrl.ResolveReference(&url.URL{Path: "/process-event"}).String(),
		webUrl:          webUrl,
	}, nil
}

func (c *Client) ProcessEvent(ctx context.Context, event Marshaler) error {
	e, _ := event.MarshalEvent()
	e.URL = c.webUrl.ResolveReference(e.URL)

	body, err := json.Marshal(e)
	if err != nil {
		return errors.Wrapf(err, "cannot marshal notifications event data of type: %T", e)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.processEventUrl, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, "cannot create new notifications http request")
	}

	req.Header.Add("Content-Type", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "cannot send notifications event")
	}

	defer func() {
		_ = res.Body.Close()
	}()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNotAcceptable {
		_, msg := io.ReadAll(res.Body)
		return errors.Errorf("received unexpected http status code from Icinga Notifications: %d: %s", res.StatusCode, msg)
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

			if err := c.ProcessEvent(ctx, entity.(Marshaler)); err != nil {
				klog.Error(err)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type basicAuthTransport struct {
	http.RoundTripper
	username string
	password string
}

func (t *basicAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.SetBasicAuth(t.username, t.password)

	return t.RoundTripper.RoundTrip(req)
}
