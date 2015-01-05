package gce

import ()

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang/glog"
	"github.com/golang/oauth2"
	"github.com/golang/oauth2/google"
	"golang.org/x/net/context"
	"google.golang.org/cloud"
	"google.golang.org/cloud/compute/metadata"
	"google.golang.org/cloud/pubsub"
	"google.golang.org/cloud/storage"
)

// NewClient creates http.Client with a jwt service account when
// jsonFile flag is specified, otherwise by obtaining the GCE service
// account's access token.
func NewClient(jsonFile string) (*http.Client, error) {
	if jsonFile != "" {
		f, err := oauth2.New(
			google.ServiceAccountJSONKey(jsonFile),
			oauth2.Scope(pubsub.ScopePubSub),
		)
		if err != nil {
			return nil, err
		}
		return &http.Client{Transport: f.NewTransport()}, nil
	}
	if metadata.OnGCE() {
		f, err := oauth2.New(
			google.ComputeEngineAccount(""),
		)
		if err != nil {
			return nil, err
		}
		client := &http.Client{Transport: f.NewTransport()}
		return client, nil
	}
	return nil, errors.New("Could not create an authenticated client.")
}
