package materials

import (
	"context"
	"database/sql"
	"fmt"

	materialsPb "lms-material-service/pb/materials"
)

// MaterialRepository struct
type MaterialRepository struct {
	Db *sql.DB
}

// List materials with pagination
func (r *MaterialRepository) List(ctx context.Context, subjectClassID string, limit, offset uint32, keyword, orderBy, sort string) ([]*materialsPb.Material, uint32, error) {
	query := `
		SELECT id, subject_class_id, topic_subject_id, type, file_type, name,
			COALESCE(storage_id::text, ''), COALESCE(source, ''),
			COALESCE(updated_by::text, ''), updated_at, created_at
		FROM materials
		WHERE subject_class_id = $1 AND name ILIKE $2 AND deleted_at IS NULL
		ORDER BY ` + orderBy + ` ` + sort + `
		LIMIT $3 OFFSET $4
	`

	searchKeyword := "%" + keyword + "%"
	rows, err := r.Db.QueryContext(ctx, query, subjectClassID, searchKeyword, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var materials []*materialsPb.Material
	for rows.Next() {
		var m materialsPb.Material
		err := rows.Scan(
			&m.Id, &m.SubjectClassId, &m.TopicSubjectId, &m.Type,
			&m.FileType, &m.Name, &m.StorageId, &m.Source,
			&m.UpdatedBy, &m.UpdatedAt, &m.CreatedAt,
		)
		if err != nil {
			return nil, 0, err
		}
		materials = append(materials, &m)
	}

	countQuery := `SELECT COUNT(*) FROM materials WHERE subject_class_id = $1 AND name ILIKE $2 AND deleted_at IS NULL`
	var count uint32
	err = r.Db.QueryRowContext(ctx, countQuery, subjectClassID, searchKeyword).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return materials, count, nil
}

// Get material by ID
func (r *MaterialRepository) Get(ctx context.Context, id string) (*materialsPb.Material, error) {
	query := `
		SELECT id, subject_class_id, topic_subject_id, type, file_type, name,
			COALESCE(storage_id::text, ''), COALESCE(source, ''),
			COALESCE(updated_by::text, ''), updated_at, created_at
		FROM materials WHERE id = $1 AND deleted_at IS NULL
	`

	var m materialsPb.Material
	err := r.Db.QueryRowContext(ctx, query, id).Scan(
		&m.Id, &m.SubjectClassId, &m.TopicSubjectId, &m.Type,
		&m.FileType, &m.Name, &m.StorageId, &m.Source,
		&m.UpdatedBy, &m.UpdatedAt, &m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Create material
func (r *MaterialRepository) Create(ctx context.Context, m *materialsPb.Material) error {
	query := `
		INSERT INTO materials (subject_class_id, topic_subject_id, type, file_type, name, storage_id, source, updated_by)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, '')::uuid, $7, $8)
		RETURNING id, updated_at, created_at
	`

	return r.Db.QueryRowContext(ctx, query,
		m.SubjectClassId, m.TopicSubjectId, m.Type, m.FileType,
		m.Name, m.StorageId, m.Source, m.UpdatedBy,
	).Scan(&m.Id, &m.UpdatedAt, &m.CreatedAt)
}

// Update material
func (r *MaterialRepository) Update(ctx context.Context, m *materialsPb.Material) error {
	query := `
		UPDATE materials SET
			subject_class_id = $1, topic_subject_id = $2, type = $3, file_type = $4,
			name = $5, storage_id = NULLIF($6, '')::uuid, source = $7,
			updated_by = $8, updated_at = NOW()
		WHERE id = $9 AND deleted_at IS NULL
	`

	result, err := r.Db.ExecContext(ctx, query,
		m.SubjectClassId, m.TopicSubjectId, m.Type, m.FileType,
		m.Name, m.StorageId, m.Source, m.UpdatedBy, m.Id,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("material not found")
	}

	return nil
}

// Delete material (soft delete)
func (r *MaterialRepository) Delete(ctx context.Context, id string) error {
	query := `UPDATE materials SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.Db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("material not found")
	}

	return nil
}

// StudentMaterialRepository struct
type StudentMaterialRepository struct {
	Db *sql.DB
}

// Exists checks if a student_material record already exists for the given material and student
func (r *StudentMaterialRepository) Exists(ctx context.Context, materialID, studentID string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM student_materials WHERE material_id = $1 AND student_id = $2 AND deleted_at IS NULL)`
	var exists bool
	err := r.Db.QueryRowContext(ctx, query, materialID, studentID).Scan(&exists)
	return exists, err
}

// List student materials
func (r *StudentMaterialRepository) List(ctx context.Context, materialID string, limit, offset uint32, orderBy, sort string) ([]*materialsPb.StudentMaterial, uint32, error) {
	query := `
		SELECT id, material_id, student_id, is_downloaded, COALESCE(progress_downloaded, 0),
			created_at, COALESCE(updated_by::text, '')
		FROM student_materials
		WHERE material_id = $1 AND deleted_at IS NULL
		ORDER BY ` + orderBy + ` ` + sort + `
		LIMIT $2 OFFSET $3
	`

	rows, err := r.Db.QueryContext(ctx, query, materialID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var studentMaterials []*materialsPb.StudentMaterial
	for rows.Next() {
		var sm materialsPb.StudentMaterial
		err := rows.Scan(
			&sm.Id, &sm.MaterialId, &sm.StudentId, &sm.IsDownloaded,
			&sm.ProgressDownloaded, &sm.CreatedAt, &sm.UpdatedBy,
		)
		if err != nil {
			return nil, 0, err
		}
		studentMaterials = append(studentMaterials, &sm)
	}

	countQuery := `SELECT COUNT(*) FROM student_materials WHERE material_id = $1 AND deleted_at IS NULL`
	var count uint32
	err = r.Db.QueryRowContext(ctx, countQuery, materialID).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return studentMaterials, count, nil
}

// Create student material
func (r *StudentMaterialRepository) Create(ctx context.Context, sm *materialsPb.StudentMaterial) error {
	query := `
		INSERT INTO student_materials (material_id, student_id, updated_by)
		VALUES ($1, $2, $3)
		RETURNING id, is_downloaded, COALESCE(progress_downloaded, 0), created_at
	`

	return r.Db.QueryRowContext(ctx, query,
		sm.MaterialId, sm.StudentId, sm.UpdatedBy,
	).Scan(&sm.Id, &sm.IsDownloaded, &sm.ProgressDownloaded, &sm.CreatedAt)
}

// UpdateProgress updates download progress
func (r *StudentMaterialRepository) UpdateProgress(ctx context.Context, sm *materialsPb.StudentMaterial) error {
	query := `
		UPDATE student_materials SET
			is_downloaded = $1, progress_downloaded = $2, updated_by = $3
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING material_id, student_id, created_at
	`

	return r.Db.QueryRowContext(ctx, query,
		sm.IsDownloaded, sm.ProgressDownloaded, sm.UpdatedBy, sm.Id,
	).Scan(&sm.MaterialId, &sm.StudentId, &sm.CreatedAt)
}

// Delete student material (soft delete)
func (r *StudentMaterialRepository) Delete(ctx context.Context, id string) error {
	query := `UPDATE student_materials SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`

	result, err := r.Db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return fmt.Errorf("student material not found")
	}

	return nil
}
