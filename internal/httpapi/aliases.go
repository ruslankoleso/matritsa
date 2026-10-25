package httpapi

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"matritsa/internal/events"
	"matritsa/internal/store/db"
)

type (
	openapiUUID = openapi_types.UUID
	eventsEvent = events.Event
)

// parseUUID reads an identifier the database has just returned.
func parseUUID(s string) openapiUUID {
	var u openapiUUID
	_ = u.UnmarshalText([]byte(s))
	return u
}

func timeSince(t time.Time) time.Duration { return time.Since(t) }

func markReadParams(id, userID string) db.MarkNotificationReadParams {
	return db.MarkNotificationReadParams{ID: id, UserID: userID}
}
