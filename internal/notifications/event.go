package notifications

import (
	"encoding/json"
	"net/url"

	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/types"
)

type Event struct {
	Uuid        types.UUID
	ClusterUuid types.UUID
	Kind        string
	Name        string
	Severity    string
	Message     string
	URL         *url.URL
	Tags        map[string]string
	ExtraTags   map[string]string
}

func (e Event) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Name      string            `json:"name"`
		Severity  string            `json:"severity"`
		Message   string            `json:"message"`
		URL       string            `json:"json"`
		Tags      map[string]string `json:"tags"`
		ExtraTags map[string]string `json:"extra_tags"`
	}{
		Name:      e.Name,
		Severity:  e.Severity,
		Message:   e.Message,
		URL:       e.URL.String(),
		Tags:      e.Tags,
		ExtraTags: e.ExtraTags,
	})
}

func (e Event) Carry() *event.Event {
	severity, _ := event.ParseSeverity(e.Severity)

	return &event.Event{
		Name:      e.Name,
		URL:       e.URL.String(),
		Tags:      e.Tags,
		ExtraTags: e.ExtraTags,
		Type:      event.TypeState,
		Severity:  severity,
		Message:   e.Message,
	}
}
