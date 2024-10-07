package dagstreamseeder

import (
	"github.com/sesanetwork/go-vassalo/gossip/basestream/basestreamseeder"
	"github.com/sesanetwork/go-vassalo/utils/cachescale"
)

type Config basestreamseeder.Config

func DefaultConfig(scale cachescale.Func) Config {
	return Config{
		SenderThreads:           8,
		MaxSenderTasks:          128,
		MaxPendingResponsesSize: scale.I64(64 * 1024 * 1024),
		MaxResponsePayloadNum:   16384,
		MaxResponsePayloadSize:  8 * 1024 * 1024,
		MaxResponseChunks:       12,
	}
}
