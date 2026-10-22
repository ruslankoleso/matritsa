package httpapi

import (
	openapi_types "github.com/oapi-codegen/runtime/types"

	"matritsa/internal/events"
	"matritsa/internal/store/db"
)

type (
	openapiUUID = openapi_types.UUID
	eventsEvent = events.Event
)

func markReadParams(id, userID string) db.MarkNotificationReadParams {
	return db.MarkNotificationReadParams{ID: id, UserID: userID}
}
