package webhook

import "time"

type Message struct {
	ScheduledDelivery time.Time
	MessageId         string
}
