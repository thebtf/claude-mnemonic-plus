// Package gorm provides a GORM-based database implementation for engram.
//
// Uses PostgreSQL 17 + pgvector for persistent storage with:
//   - Type-safe query building via GORM
//   - Automatic statement caching
//   - Auto-migrations for schema management
//
// # Usage
//
//	import "github.com/thebtf/engram/internal/db/gorm"
//
//	store, err := gorm.NewStore(gorm.Config{...})
package gorm
