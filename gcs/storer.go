// Store from GCE VM to GCS using info from metadata server.

package gcs

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/golang/oauth2"
	"github.com/golang/oauth2/google"
	"google.golang.org/cloud"
	"google.golang.org/cloud/compute/metadata"
	"google.golang.org/cloud/pubsub"
	"google.golang.org/cloud/storage"

)


// newClient creates http.Client with a jwt service account when
// jsonFile flag is specified, otherwise by obtaining the GCE service
// account's access token.
func newClient(jsonFile string) (*http.Client, error) {
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

func CopyLocalFileToBucket(
		ctx context.Context,
		localPath, bucket, remotePath string,
		object *storage.Object) (storage.Object, error) {
	// TODO Open localPath first before creating a remote file.



	if object == nil {
		object = &storage.Object{}
	}
	wc := storage.NewWriter(ctx, bucket, remotePath, object
	&storage.Object{
	    ContentType: "text/plain",
	    ACL:         []storage.ACLRule{storage.ACLRule{"allUsers", storage.RoleReader}}, // Shared Publicly
	})
if _, err := wc.Write([]byte("hello world")); err != nil {
    log.Fatal(err)
}
if err := wc.Close(); err != nil {
    log.Fatal(err)
}

o, err := wc.Object()
if err != nil {
    log.Fatal(err)
}


}

// see the auth example how to initiate a context.
ctx := cloud.NewContext("project-id", &http.Client{Transport: nil})

wc := storage.NewWriter(ctx, "bucketname", "filename1", &storage.Object{
    ContentType: "text/plain",
    ACL:         []storage.ACLRule{storage.ACLRule{"allUsers", storage.RoleReader}}, // Shared Publicly
})
if _, err := wc.Write([]byte("hello world")); err != nil {
    log.Fatal(err)
}
if err := wc.Close(); err != nil {
    log.Fatal(err)
}

o, err := wc.Object()
if err != nil {
    log.Fatal(err)
}
log.Println("updated object:", o)


// createFile creates a file in Google Cloud Storage.
func (d *demo) createFile(fileName string) {
	fmt.Fprintf(d.w, "Creating file /%v/%v\n", bucket, fileName)

	wc := storage.NewWriter(d.ctx, bucket, fileName, &storage.Object{
		ContentType: "text/plain",
		Metadata: map[string]string{
			"x-goog-meta-foo": "foo",
			"x-goog-meta-bar": "bar",
		},
	})
	d.cleanUp = append(d.cleanUp, fileName)

	if _, err := wc.Write([]byte("abcde\n")); err != nil {
		d.errorf("createFile: unable to write data to bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
	if _, err := wc.Write([]byte(strings.Repeat("f", 1024*4) + "\n")); err != nil {
		d.errorf("createFile: unable to write data to bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
	if err := wc.Close(); err != nil {
		d.errorf("createFile: unable to close bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
	// Wait for the file to be fully written.
	_, err := wc.Object()
	if err != nil {
		d.errorf("createFile: unable to finalize file from bucket %q, file %q: %v", bucket, fileName, err)
		return
	}
}
