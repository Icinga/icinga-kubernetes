package notifications

import (
	"context"
	"encoding/json"
	"net/url"
	"sync"

	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/types"
	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

type Client struct {
	client    *source.Client
	webUrl    *url.URL
	db        *database.DB
	mu        sync.Mutex
	rulesInfo *source.RulesInfo
}

func NewClient(name string, config Config, db *database.DB) (*Client, error) {
	baseUrl, err := url.Parse(config.Url)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse url")
	}

	webUrl, err := url.Parse(config.KubernetesWebUrl)
	if err != nil {
		return nil, errors.Wrap(err, "unable to parse web url")
	}

	client, err := source.NewClient(source.Config{
		Url:      baseUrl.String(),
		Username: config.Username,
		Password: config.Password,
	}, name)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create notifications client")
	}

	return &Client{
		client:    client,
		webUrl:    webUrl,
		rulesInfo: &source.RulesInfo{},
		db:        db,
	}, nil
}

func (c *Client) ProcessEvent(ctx context.Context, event Marshaler) error {
	e, _ := event.MarshalEvent()
	e.URL = c.webUrl.ResolveReference(e.URL)
	ev := e.Carry()

	c.mu.Lock()
	defer c.mu.Unlock()

	for try := 0; try < 3; try++ {
		eventRuleIds, err := c.evaluateRulesForObject(
			ctx,
			e.Kind,
			e.Uuid,
			e.ClusterUuid)
		if err != nil {
			klog.Errorf("Cannot evaluate rules for event, assuming no rule matched: %v", err)
			eventRuleIds = []string{}
		}

		ev.RulesVersion = c.rulesInfo.Version
		ev.RuleIds = eventRuleIds

		newEventRules, err := c.client.ProcessEvent(ctx, ev)
		if errors.Is(err, source.ErrRulesOutdated) {
			klog.Infof("Received a rule update from Icinga Notifications, resubmitting event (old_rules_version: %q, new_rules_version: %q)",
				c.rulesInfo.Version,
				newEventRules.Version)

			c.rulesInfo = newEventRules

			continue
		} else if err != nil {
			return errors.Wrapf(err, "cannot submit event to Icinga Notifications (matched_rules: %v, rules_version: %q)", eventRuleIds, c.rulesInfo.Version)
		}

		klog.V(2).Infof("Successfully submitted event to Icinga Notifications (matched_rules: %v)", eventRuleIds)

		return nil
	}

	return errors.New("Received three rule updates from Icinga Notifications in a row")
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

func (c *Client) evaluateRulesForObject(ctx context.Context, kind string, uuid, clusterUuid types.UUID) ([]string, error) {
	const expectedRuleVersion = 1

	outRuleIds := make([]string, 0, len(c.rulesInfo.Rules))

	for id, filterExpr := range c.rulesInfo.Rules {
		if filterExpr == "" {
			// TODO(el): Why?
			outRuleIds = append(outRuleIds, id)
			continue
		}

		var r rule
		if err := json.Unmarshal([]byte(filterExpr), &r); err != nil {
			return nil, errors.Wrap(err, "cannot decode rule filter expression as JSON into struct")
		}
		if version := r.Version; version != expectedRuleVersion {
			return nil, errors.Errorf("decoded rule filter expression .Version is %d, %d expected", version, expectedRuleVersion)
		}

		if r.Kind != kind {
			continue
		}

		args := make([]any, 0, len(r.Args))
		for _, param := range r.Args {
			switch param {
			case ":uuid":
				args = append(args, uuid)
			case ":cluster_uuid":
				args = append(args, clusterUuid)
			default:
				args = append(args, param)
			}
		}

		matches, err := func() (bool, error) {
			rows, err := c.db.QueryContext(ctx, c.db.Rebind(r.Query), args...)
			if err != nil {
				return false, err
			}
			defer func() { _ = rows.Close() }()

			return rows.Next(), nil
		}()
		if err != nil {
			return nil, errors.Wrapf(err, "cannot fetch rule %q from %q", id, filterExpr)
		} else if !matches {
			continue
		}
		outRuleIds = append(outRuleIds, id)
	}

	return outRuleIds, nil
}

type rule struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Query   string `json:"query"`
	Args    []any  `json:"args"`
}
