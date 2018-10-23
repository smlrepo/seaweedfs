package azuresink

import (
	"bytes"
	"context"
	"fmt"
	"net/url"

	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/chrislusf/seaweedfs/weed/filer2"
	"github.com/chrislusf/seaweedfs/weed/pb/filer_pb"
	"github.com/chrislusf/seaweedfs/weed/replication/sink"
	"github.com/chrislusf/seaweedfs/weed/replication/source"
	"github.com/chrislusf/seaweedfs/weed/util"
)

type AzureSink struct {
	containerURL azblob.ContainerURL
	container    string
	dir          string
	filerSource  *source.FilerSource
}

func init() {
	sink.Sinks = append(sink.Sinks, &AzureSink{})
}

func (g *AzureSink) GetName() string {
	return "azure"
}

func (g *AzureSink) GetSinkToDirectory() string {
	return g.dir
}

func (g *AzureSink) Initialize(configuration util.Configuration) error {
	return g.initialize(
		configuration.GetString("account_name"),
		configuration.GetString("account_key"),
		configuration.GetString("container"),
		configuration.GetString("directory"),
	)
}

func (g *AzureSink) SetSourceFiler(s *source.FilerSource) {
	g.filerSource = s
}

func (g *AzureSink) initialize(accountName, accountKey, container, dir string) error {
	g.container = container
	g.dir = dir

	// Use your Storage account's name and key to create a credential object.
	credential, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return err
	}

	// Create a request pipeline that is used to process HTTP(S) requests and responses.
	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// Create an ServiceURL object that wraps the service URL and a request pipeline.
	u, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net", accountName))
	serviceURL := azblob.NewServiceURL(*u, p)

	g.containerURL = serviceURL.NewContainerURL(g.container)

	return nil
}

func (g *AzureSink) DeleteEntry(key string, isDirectory, deleteIncludeChunks bool) error {

	if isDirectory {
		key = key + "/"
	}

	ctx := context.Background()

	if _, err := g.containerURL.NewBlobURL(key).Delete(ctx,
		azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{}); err != nil {
		return fmt.Errorf("azure delete %s/%s: %v", g.container, key, err)
	}

	return nil

}

func (g *AzureSink) CreateEntry(key string, entry *filer_pb.Entry) error {

	if entry.IsDirectory {
		return nil
	}

	totalSize := filer2.TotalSize(entry.Chunks)
	chunkViews := filer2.ViewFromChunks(entry.Chunks, 0, int(totalSize))

	ctx := context.Background()

	// Create a URL that references a to-be-created blob in your
	// Azure Storage account's container.
	appendBlobURL := g.containerURL.NewAppendBlobURL(key)

	_, err := appendBlobURL.Create(ctx, azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		return err
	}

	for _, chunk := range chunkViews {

		fileUrl, err := g.filerSource.LookupFileId(chunk.FileId)
		if err != nil {
			return err
		}

		var writeErr error
		_, readErr := util.ReadUrlAsStream(fileUrl, chunk.Offset, int(chunk.Size), func(data []byte) {
			_, writeErr = appendBlobURL.AppendBlock(ctx, bytes.NewReader(data), azblob.AppendBlobAccessConditions{}, nil)
		})

		if readErr != nil {
			return readErr
		}
		if writeErr != nil {
			return writeErr
		}

	}

	return nil

}

func (g *AzureSink) UpdateEntry(key string, oldEntry, newEntry *filer_pb.Entry, deleteIncludeChunks bool) (foundExistingEntry bool, err error) {
	// TODO improve efficiency
	return false, nil
}
