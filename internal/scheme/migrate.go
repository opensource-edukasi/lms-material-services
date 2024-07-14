package scheme

import (
	"database/sql"

	"github.com/GuiaBolso/darwin"
)

var migrations = []darwin.Migration{
	{
		Version:     1,
		Description: "Create uuid extension",
		Script:      `CREATE EXTENSION "uuid-ossp";`,
	},
	{
		Version:     2,
		Description: "Create Materials",
		Script: `
			CREATE TABLE materials (
				id uuid NOT NULL PRIMARY KEY DEFAULT uuid_generate_v4 (),
				subject_class_id UUID NOT NULL,
				topic_subject_id UUID NOT NULL,
				type CHAR(5) NOT NULL,
				file_type CHAR(1) NOT NULL,
				name VARCHAR(45) NOT NULL,
				storage_id UUID,
				source VARCHAR(128),
				updated_at timestamptz NOT NULL DEFAULT timezone('utc', NOW()),
				updated_by UUID,
				created_at timestamptz NOT NULL DEFAULT timezone('utc', NOW()),
				deleted_at TIMESTAMPTZ
			);
		`,
	},
	{
		Version:     3,
		Description: "Create Student Materials Table",
		Script: `
			CREATE TABLE student_materials (
				id uuid NOT NULL PRIMARY KEY DEFAULT uuid_generate_v4 (),
				material_id UUID NOT NULL,
				student_id UUID NOT NULL,
				is_downloaded BOOLEAN DEFAULT FALSE,
				progress_downloaded SMALLINT,
				created_at timestamptz NOT NULL DEFAULT timezone('utc', NOW()),
				deleted_at TIMESTAMPTZ,
				updated_by uuid, 
				FOREIGN KEY (material_id) REFERENCES materials(id)
			);
		`,
	},
}

// Migrate attempts to bring the schema for db up to date with the migrations
// defined in this package.
func Migrate(db *sql.DB) error {
	driver := darwin.NewGenericDriver(db, darwin.PostgresDialect{})

	d := darwin.New(driver, migrations, nil)

	return d.Migrate()
}
