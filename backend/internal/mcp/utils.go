package mcp

import (
	"gorm.io/gorm"
)

// gormExprAdd generates an auto-increment expression (GORM 2.0 usage)
func gormExprAdd() interface{} {
	return gorm.Expr("access_count + 1")
}
