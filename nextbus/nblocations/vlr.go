package nblocations

import (
	"net/http"
	"time"

	"github.com/jamessynge/transit_tools/nextbus"
)

type VehicleLocationsResponse struct {
	Agency   string
	Url      string
	LastTime time.Time
	// lastTime value from previous request, which was
	// used to generate query parameter t in Url.
	LastLastTime time.Time
	// Our time just before sending request to server.
	RequestTime time.Time
	// Our time just after receiving the headers (i.e. before reading body, but
	// after we got the Date header).
	ResultTime time.Time
	Response   *http.Response
	// Time as reported by the server in its Date header (if not present, this
	// is the average of RequestTime and ResultTime).
	ServerTime time.Time
	Body       []byte
	Report     *nextbus.VehicleLocationsReport
	Error      error
}
