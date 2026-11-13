package notifications

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/icinga/icinga-go-library/database"
	"github.com/icinga/icinga-go-library/logging"
	"github.com/icinga/icinga-go-library/notifications/event"
	"github.com/icinga/icinga-go-library/notifications/source"
	"github.com/icinga/icinga-go-library/types"
	"github.com/icinga/icinga-go-library/utils"
	"github.com/icinga/icinga-kubernetes/internal/cluster"
	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Client struct {
	client *source.Client
	config source.Config

	db     *database.DB
	logger *logging.Logger

	clusterName string
}

func NewClient(name string, config source.Config, db *database.DB, logger *logging.Logger, cluster string) (*Client, error) {
	client, err := source.NewClient(config, name)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create notifications client")
	}

	return &Client{
		client:      client,
		config:      config,
		db:          db,
		logger:      logger,
		clusterName: cluster,
	}, nil
}

// Stream consumes the items from the given `entities` chan and triggers
// a notifications event for each of them.
func (c *Client) Stream(ctx context.Context, entities <-chan database.Entity) error {
	clusterUuid := cluster.ClusterUuidFromContext(ctx)
	if clusterUuid.IsZero() {
		return errors.New("cannot get cluster UUID from context")
	}

	for {
		select {
		case entity, more := <-entities:
			if !more {
				return nil
			}

			ev := entity.(Marshaler).MarshalEvent()
			if suppressWhileDeleting(entity, ev) {
				c.logger.Debugw(
					"Skipping non-OK event of an object that is being deleted",
					zap.String("name", ev.Name),
					zap.Stringer("severity", ev.Severity),
				)

				continue
			}

			if ev.Severity == event.SeverityOK {
				ev.Close = types.MakeBool(true)
			}

			if c.config.Icingaweb2UrlParsed != nil {
				evURL, err := c.config.Icingaweb2UrlParsed.Parse(strings.TrimLeft(ev.URL, "/"))
				if err != nil {
					c.logger.Errorw(
						"BUG: generated notifications event has invalid URL, skipping",
						zap.Any("event", ev),
						zap.Any("entity", entity),
						logging.Error(err),
					)

					continue
				}
				ev.URL = evURL.String()
			} else {
				ev.URL = "" // We can't resolve it to a full URL, so we don't send it to Icinga Notifications.
			}

			// We don't have extra information that we can re-send to Icinga Notifications,
			// so mark all the relations as complete. This will ensure that Icinga Notifications
			// doesn't request any additional attributes.
			ev.CompleteRelations = []string{"object", "cluster", "owners", "node"}
			ev.Relations["cluster"] = map[string]any{
				"uuid": clusterUuid,
				"name": c.clusterName,
			}

			if ev.Summary == "" {
				// If the summary is empty, we take the first line of the message as the summary.
				ev.Summary, _, _ = strings.Cut(strings.TrimLeft(ev.Message, "\n\t "), "\n")
				ev.Summary = utils.EllipsizeRunes(strings.TrimSpace(ev.Summary), event.MaxSummaryLen)
			}

			if err := ev.Validate(); err != nil {
				c.logger.Errorw(
					"BUG: generated notifications event is invalid, skipping",
					zap.Any("event", ev),
					zap.Any("entity", entity),
					logging.Error(err),
				)

				continue
			}

			if ev.CloseIncident() {
				// Before sending a closing event, we need to first make sure there's
				// an open incident for this entity in Icinga Notifications. Otherwise,
				// we'll end up creating a new incident and immediately closing it,
				// potentially causing a bunch of notifications to be sent out.
				incidents, err := c.client.GetIncidents(ctx, ev.Tags)
				if err != nil {
					c.logger.Warnw(
						"Unable to check for existing incidents in Icinga Notifications, skipping closing event",
						zap.Any("event", ev),
						zap.Any("entity", entity),
						logging.Error(err),
					)

					continue
				}

				if len(incidents) == 0 {
					continue // No open incidents, so we don't need to send a closing event.
				}
			}

			attributes, err := c.client.ProcessEvent(ctx, ev, false)
			if errors.Is(err, source.ErrAttrsNegotiation) {
				c.logger.Errorw(
					"Cannot complete requested missing attributes",
					zap.Strings("attributes", attributes),
					logging.Error(err),
				)
			} else if err != nil {
				c.logger.Errorw("Cannot process event", logging.Error(err))
			} else {
				c.logger.Debugw("Successfully submitted event to Icinga Notifications")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// suppressWhileDeleting reports whether ev, the event of the given entity, must
// not be sent because the object is being deleted. Of such objects, only OK
// events are sent. Failures during shutdown, such as containers exiting
// non-zero on SIGTERM, must not open or worsen an incident. An incident that
// was already open before the deletion started stays open, so a deletion that
// never finishes still shows up, and an OK event can still close it.
func suppressWhileDeleting(entity any, ev *event.Event) bool {
	d, ok := entity.(DeletionReporter)

	return ok && d.IsDeleting() && ev.Severity != event.SeverityOK
}

// CloseOrphanedIncidents closes all incidents in Icinga Notifications that are
// associated with the given clusterUuid but do not have a corresponding resource
// in the database.
func (c *Client) CloseOrphanedIncidents(ctx context.Context, clusterUuid types.UUID) {
	incidentsCh, errCh := c.client.YieldIncidents(ctx, map[string]string{
		"cluster_uuid": clusterUuid.String(),
	})

	var filter []map[string]string
	uuidByResource := make(map[string][]types.UUID)
	for incident := range incidentsCh {
		id, err := uuid.Parse(incident.ObjectTags["uuid"])
		if err != nil {
			c.logger.Warnw(
				"Invalid UUID in incident object tags, skipping",
				zap.Any("incident", incident),
				logging.Error(err),
			)

			continue
		}

		switch resource := incident.ObjectTags["resource"]; resource {
		case "node", "pod", "deployment", "stateful_set", "daemon_set", "replica_set":
			uuidByResource[resource] = append(uuidByResource[resource], types.MakeUUID(id))
			filter = append(filter, incident.ObjectTags)
		default:
			c.logger.Warnw(
				"Unknown resource type in incident object tags, skipping",
				zap.Any("incident", incident),
				zap.String("resource", resource),
			)

			continue
		}
	}

	if err := <-errCh; err != nil {
		c.logger.Errorw("Error while fetching incidents from Icinga Notifications", logging.Error(err))

		return
	}

	ng, nctx := errgroup.WithContext(ctx)

	var mu sync.Mutex
	live := make(map[string]struct{})
	for kind, uuids := range uuidByResource {
		ng.Go(func() error {
			q, args, err := sqlx.In(fmt.Sprintf(`SELECT "uuid" FROM %q WHERE "uuid" IN (?) AND "deleted" IS NULL`, kind), uuids)
			if err != nil {
				return err
			}

			rows, err := c.db.QueryxContext(nctx, c.db.Rebind(q), args...)
			if err != nil {
				return err
			}
			defer func() { _ = rows.Close() }()

			for rows.Next() {
				var id types.UUID
				if err := rows.Scan(&id); err != nil {
					return err
				}

				mu.Lock()
				live[id.String()] = struct{}{}
				mu.Unlock()
			}

			return rows.Err()
		})
	}

	if err := ng.Wait(); err != nil {
		c.logger.Errorw("Error while fetching live resources from database", logging.Error(err))

		return
	}

	filter = slices.DeleteFunc(filter, func(tags map[string]string) bool {
		_, ok := live[tags["uuid"]]

		return ok
	})
	if len(filter) == 0 {
		c.logger.Debugw("No orphaned incidents found to close in Icinga Notifications")

		return
	}

	modifyAttrs := source.ModifiableIncidentAttrs{
		Close:   types.MakeBool(true),
		Message: types.MakeString("Resource no longer exists in Kubernetes cluster."),
	}

	if err := c.client.ModifyIncidents(ctx, modifyAttrs, filter); err != nil {
		c.logger.Errorw("Error while closing orphaned incidents in Icinga Notifications", logging.Error(err))
	}
}
