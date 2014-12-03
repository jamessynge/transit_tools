// Store from GCE VM to GCS using info from metadata server.

package gcs

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

	f, err := os.Open(localPath)


/*
	// If not provided, create a default Object.
	if object == nil {
		//	&storage.Object{ContentType: "text/plain", ACL: []storage.ACLRule{storage.ACLRule{"allUsers", storage.RoleReader}}}, // Shared Publicly
		object = &storage.Object{}
		// TODO Use http.DetectContentType to set the ContentType field.
	}
*/

	wc := storage.NewWriter(ctx, bucket, remotePath, object)
	written, err := io.Copy(wc, rc)
	err2 := wc.Close()
	if err2 != nil && err == nil {
		err = err2
	}
	result, err2 := wc.Object()
	if err2 != nil && err == nil {
		err = err2
	}
	if err != nil {
		glog.Warningf(`Error while copying local file to bucket.\n
 Local path: %s
     Bucket: %s
Remote path: %s
	    Error: %s`, localPath, bucket, remotePath, err)
	}
	return result, err
}

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
