package route

import (
	"database/sql"
	"log"

	"google.golang.org/grpc"

	materialsDomain "lms-material-service/internal/domain/materials"
	"lms-material-service/internal/pkg/db/redis"
	materialsPb "lms-material-service/pb/materials"
)

// GrpcRoute func
func GrpcRoute(grpcServer *grpc.Server, db *sql.DB, log *log.Logger, cache *redis.Cache) {
	// Material service
	materialServer := materialsDomain.MaterialServiceServer{Db: db, Cache: cache, Log: log}
	materialsPb.RegisterMaterialServiceServer(grpcServer, &materialServer)

	// Student material service
	studentMaterialServer := materialsDomain.StudentMaterialServiceServer{Db: db, Cache: cache, Log: log}
	materialsPb.RegisterStudentMaterialServiceServer(grpcServer, &studentMaterialServer)
}
