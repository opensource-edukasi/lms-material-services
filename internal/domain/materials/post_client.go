package materials

import (
	"context"
	"log"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protowire"
)

// postServiceClient handles communication with lms-post-service
type postServiceClient struct {
	Log *log.Logger
}

// createPost calls lms-post-service to create a post with type MATERIAL
func (c *postServiceClient) createPost(ctx context.Context, subjectClassID, topicSubjectID, materialID, title, fileType, storageID, source, userID string) {
	postServiceAddr := os.Getenv("POST_SERVICE_ADDRESS")
	if postServiceAddr == "" {
		c.Log.Println("POST_SERVICE_ADDRESS not configured, skipping post creation")
		return
	}

	conn, err := grpc.NewClient(postServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		c.Log.Printf("error connecting to post service: %v", err)
		return
	}
	defer conn.Close()

	// Forward user metadata
	md := metadata.New(map[string]string{
		"user_id": userID,
	})
	outCtx := metadata.NewOutgoingContext(ctx, md)

	// Manually encode CreatePostRequest protobuf
	// Field 2: subject_class_id (string)
	// Field 3: topic_subject_id (string)
	// Field 4: type (int32) = 2 (MATERIAL)
	// Field 5: type_id (string) = materialID
	// Field 6: title (string)
	// Field 8: file_type (string)
	// Field 9: storage_id (string)
	// Field 10: source (string)
	// Field 12: is_published (bool) = true
	var buf []byte
	buf = protowire.AppendTag(buf, 2, protowire.BytesType)
	buf = protowire.AppendString(buf, subjectClassID)
	buf = protowire.AppendTag(buf, 3, protowire.BytesType)
	buf = protowire.AppendString(buf, topicSubjectID)
	buf = protowire.AppendTag(buf, 4, protowire.VarintType)
	buf = protowire.AppendVarint(buf, 2) // MATERIAL = 2
	buf = protowire.AppendTag(buf, 5, protowire.BytesType)
	buf = protowire.AppendString(buf, materialID)
	buf = protowire.AppendTag(buf, 6, protowire.BytesType)
	buf = protowire.AppendString(buf, title)
	if fileType != "" {
		buf = protowire.AppendTag(buf, 8, protowire.BytesType)
		buf = protowire.AppendString(buf, fileType)
	}
	if storageID != "" {
		buf = protowire.AppendTag(buf, 9, protowire.BytesType)
		buf = protowire.AppendString(buf, storageID)
	}
	if source != "" {
		buf = protowire.AppendTag(buf, 10, protowire.BytesType)
		buf = protowire.AppendString(buf, source)
	}
	buf = protowire.AppendTag(buf, 12, protowire.VarintType)
	buf = protowire.AppendVarint(buf, 1) // is_published = true

	// Use raw codec to send the request
	var resp []byte
	err = conn.Invoke(outCtx, "/posts.Posts/CreatePost", &rawMessage{data: buf}, &rawMessage{data: resp}, grpc.ForceCodec(rawCodec{}))
	if err != nil {
		c.Log.Printf("error creating post for material: %v", err)
		return
	}

	c.Log.Printf("successfully created post for material %s", materialID)
}

// rawMessage wraps raw bytes for gRPC codec
type rawMessage struct {
	data []byte
}

// rawCodec is a gRPC codec that passes raw bytes
type rawCodec struct{}

func (rawCodec) Marshal(v interface{}) ([]byte, error) {
	msg, ok := v.(*rawMessage)
	if !ok {
		return nil, nil
	}
	return msg.data, nil
}

func (rawCodec) Unmarshal(data []byte, v interface{}) error {
	msg, ok := v.(*rawMessage)
	if !ok {
		return nil
	}
	msg.data = data
	return nil
}

func (rawCodec) Name() string {
	return "proto"
}
