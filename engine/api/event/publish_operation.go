package event

import (
	"fmt"
	"time"

	"github.com/fatih/structs"

	"github.com/ovh/cds/sdk"
)

// publishOperationEvent publish operation event
func PublishOperationEvent(projectKey string, payload sdk.Operation, u sdk.Identifiable) {
	event := sdk.Event{
		Timestamp:     time.Now(),
		Hostname:      hostname,
		CDSName:       cdsname,
		EventType:     fmt.Sprintf("%T", payload),
		Payload:       structs.Map(payload),
		OperationUUID: payload.UUID,
		ProjectKey:    projectKey,
	}
	if u != nil {
		event.Username = u.GetUsername()
		event.UserMail = u.GetEmail()
	}
	publishEvent(event)
}
