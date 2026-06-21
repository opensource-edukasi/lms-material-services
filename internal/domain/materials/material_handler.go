package materials

import (
	"context"
	"database/sql"
	"log"
	"time"

	"lms-material-service/internal/pkg/app"
	"lms-material-service/internal/pkg/db/redis"
	genericPb "lms-material-service/pb/generic"
	materialsPb "lms-material-service/pb/materials"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MaterialServiceServer struct
type MaterialServiceServer struct {
	Db    *sql.DB
	Cache *redis.Cache
	Log   *log.Logger
	materialsPb.UnimplementedMaterialServiceServer
}

// List materials
func (s *MaterialServiceServer) List(ctx context.Context, in *materialsPb.MaterialListInput) (*materialsPb.MaterialList, error) {
	repo := MaterialRepository{Db: s.Db}

	pagination := in.GetPagination()
	limit := uint32(10)
	offset := uint32(0)
	keyword := ""
	orderBy := "created_at"
	sort := "DESC"

	if pagination != nil {
		if pagination.Limit > 0 {
			limit = pagination.Limit
		}
		offset = pagination.Offset
		if pagination.Keyword != "" {
			keyword = pagination.Keyword
		}
		if pagination.Order != "" {
			orderBy = pagination.Order
		}
		if pagination.Sort != "" {
			sort = pagination.Sort
		}
	}

	materials, count, err := repo.List(ctx, in.GetSubjectClassId(), limit, offset, keyword, orderBy, sort)
	if err != nil {
		s.Log.Printf("error listing materials: %v", err)
		return nil, status.Error(codes.Internal, "failed to list materials")
	}

	return &materialsPb.MaterialList{
		Materials: materials,
		Count:     count,
	}, nil
}

// Get material by ID (Issue #3)
// If the viewer is a student, insert into student_materials with is_downloaded = false
func (s *MaterialServiceServer) Get(ctx context.Context, in *genericPb.Id) (*materialsPb.Material, error) {
	repo := MaterialRepository{Db: s.Db}

	if in.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	material, err := repo.Get(ctx, in.GetId())
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.NotFound, "material not found")
		}
		s.Log.Printf("error getting material: %v", err)
		return nil, status.Error(codes.Internal, "failed to get material")
	}

	// If the viewer is a student, insert into student_materials with is_downloaded = false
	userID := ctx.Value(app.Ctx("user_id")).(string)
	studentRepo := StudentMaterialRepository{Db: s.Db}

	// Check if student_material already exists
	exists, err := studentRepo.Exists(ctx, in.GetId(), userID)
	if err != nil {
		s.Log.Printf("error checking student material: %v", err)
	}

	if !exists {
		sm := &materialsPb.StudentMaterial{
			MaterialId: in.GetId(),
			StudentId:  userID,
			UpdatedBy:  userID,
		}
		err = studentRepo.Create(ctx, sm)
		if err != nil {
			s.Log.Printf("error creating student material on view: %v", err)
			// Don't fail the request, just log the error
		}
	}

	return material, nil
}

// Create material (Issue #1)
// Validates input and after successful creation, calls gRPC to create post in lms-post-service
func (s *MaterialServiceServer) Create(ctx context.Context, in *materialsPb.MaterialInput) (*materialsPb.Material, error) {
	repo := MaterialRepository{Db: s.Db}

	userID := ctx.Value(app.Ctx("user_id")).(string)

	// Validation
	if in.GetSubjectClassId() == "" {
		return nil, status.Error(codes.InvalidArgument, "subject_class_id is required")
	}
	if in.GetTopicSubjectId() == "" {
		return nil, status.Error(codes.InvalidArgument, "topic_subject_id is required")
	}
	if in.GetType() == "" {
		return nil, status.Error(codes.InvalidArgument, "type is required")
	}
	if in.GetFileType() == "" {
		return nil, status.Error(codes.InvalidArgument, "file_type is required")
	}
	if in.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	material := &materialsPb.Material{
		SubjectClassId: in.GetSubjectClassId(),
		TopicSubjectId: in.GetTopicSubjectId(),
		Type:           in.GetType(),
		FileType:       in.GetFileType(),
		Name:           in.GetName(),
		StorageId:      in.GetStorageId(),
		Source:         in.GetSource(),
		UpdatedBy:      userID,
	}

	err := repo.Create(ctx, material)
	if err != nil {
		s.Log.Printf("error creating material: %v", err)
		return nil, status.Error(codes.Internal, "failed to create material")
	}

	// Call gRPC to create post in lms-post-service
	go s.createPostForMaterial(ctx, material, userID)

	return material, nil
}

// createPostForMaterial calls lms-post-service to create a post with type MATERIAL
func (s *MaterialServiceServer) createPostForMaterial(ctx context.Context, material *materialsPb.Material, userID string) {
	client := &postServiceClient{Log: s.Log}
	client.createPost(
		ctx,
		material.SubjectClassId,
		material.TopicSubjectId,
		material.Id,
		material.Name,
		material.FileType,
		material.StorageId,
		material.Source,
		userID,
	)
}

// Update material (Issue #2)
func (s *MaterialServiceServer) Update(ctx context.Context, in *materialsPb.Material) (*materialsPb.Material, error) {
	repo := MaterialRepository{Db: s.Db}

	userID := ctx.Value(app.Ctx("user_id")).(string)

	// Validation
	if in.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}
	if in.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	in.UpdatedBy = userID
	in.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	err := repo.Update(ctx, in)
	if err != nil {
		s.Log.Printf("error updating material: %v", err)
		return nil, status.Error(codes.Internal, "failed to update material")
	}

	return in, nil
}

// Delete material (Issue #4) - soft delete
func (s *MaterialServiceServer) Delete(ctx context.Context, in *genericPb.Id) (*genericPb.BoolMessage, error) {
	repo := MaterialRepository{Db: s.Db}

	if in.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	err := repo.Delete(ctx, in.GetId())
	if err != nil {
		s.Log.Printf("error deleting material: %v", err)
		return &genericPb.BoolMessage{IsTrue: false}, status.Error(codes.Internal, "failed to delete material")
	}

	return &genericPb.BoolMessage{IsTrue: true}, nil
}

// StudentMaterialServiceServer struct
type StudentMaterialServiceServer struct {
	Db    *sql.DB
	Cache *redis.Cache
	Log   *log.Logger
	materialsPb.UnimplementedStudentMaterialServiceServer
}

// List student materials
func (s *StudentMaterialServiceServer) List(ctx context.Context, in *materialsPb.StudentMaterialListInput) (*materialsPb.StudentMaterialList, error) {
	repo := StudentMaterialRepository{Db: s.Db}

	pagination := in.GetPagination()
	limit := uint32(10)
	offset := uint32(0)
	orderBy := "created_at"
	sort := "DESC"

	if pagination != nil {
		if pagination.Limit > 0 {
			limit = pagination.Limit
		}
		offset = pagination.Offset
		if pagination.Order != "" {
			orderBy = pagination.Order
		}
		if pagination.Sort != "" {
			sort = pagination.Sort
		}
	}

	studentMaterials, count, err := repo.List(ctx, in.GetMaterialId(), limit, offset, orderBy, sort)
	if err != nil {
		s.Log.Printf("error listing student materials: %v", err)
		return nil, status.Error(codes.Internal, "failed to list student materials")
	}

	return &materialsPb.StudentMaterialList{
		StudentMaterials: studentMaterials,
		Count:            count,
	}, nil
}

// Create student material
func (s *StudentMaterialServiceServer) Create(ctx context.Context, in *materialsPb.StudentMaterialInput) (*materialsPb.StudentMaterial, error) {
	repo := StudentMaterialRepository{Db: s.Db}

	userID := ctx.Value(app.Ctx("user_id")).(string)

	sm := &materialsPb.StudentMaterial{
		MaterialId: in.GetMaterialId(),
		StudentId:  in.GetStudentId(),
		UpdatedBy:  userID,
	}

	err := repo.Create(ctx, sm)
	if err != nil {
		s.Log.Printf("error creating student material: %v", err)
		return nil, status.Error(codes.Internal, "failed to create student material")
	}

	return sm, nil
}

// UpdateProgress - Download File (Issue #5)
// When a student downloads a file, update is_downloaded = true
func (s *StudentMaterialServiceServer) UpdateProgress(ctx context.Context, in *materialsPb.UpdateProgressInput) (*materialsPb.StudentMaterial, error) {
	repo := StudentMaterialRepository{Db: s.Db}

	userID := ctx.Value(app.Ctx("user_id")).(string)

	if in.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id is required")
	}

	sm := &materialsPb.StudentMaterial{
		Id:                 in.GetId(),
		IsDownloaded:       in.GetIsDownloaded(),
		ProgressDownloaded: in.GetProgressDownloaded(),
		UpdatedBy:          userID,
	}

	err := repo.UpdateProgress(ctx, sm)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, status.Error(codes.NotFound, "student material not found")
		}
		s.Log.Printf("error updating progress: %v", err)
		return nil, status.Error(codes.Internal, "failed to update progress")
	}

	return sm, nil
}

// Delete student material (soft delete)
func (s *StudentMaterialServiceServer) Delete(ctx context.Context, in *genericPb.Id) (*genericPb.BoolMessage, error) {
	repo := StudentMaterialRepository{Db: s.Db}

	err := repo.Delete(ctx, in.GetId())
	if err != nil {
		s.Log.Printf("error deleting student material: %v", err)
		return &genericPb.BoolMessage{IsTrue: false}, status.Error(codes.Internal, "failed to delete student material")
	}

	return &genericPb.BoolMessage{IsTrue: true}, nil
}
