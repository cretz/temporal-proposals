package converter

import (
	"fmt"

	commonpb "go.temporal.io/api/common/v1"
)

// TODO(cretz): Converter which supports this
type PassthroughPayload struct {
	Payload *commonpb.Payload
}

func (*PassthroughPayload) MarshalJSON() ([]byte, error) {
	return nil, fmt.Errorf("JSON marshalling not supported on passthrough payload")
}

func (*PassthroughPayload) UnmarshalJSON([]byte) error {
	return fmt.Errorf("JSON unmarshalling not supported on passthrough payload")
}
