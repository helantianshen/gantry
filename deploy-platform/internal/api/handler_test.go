package api

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestParsePagination 覆盖默认值、合法参数和边界拒绝
func TestParsePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		query        string
		wantPage     int
		wantPageSize int
		wantErr      bool
	}{
		{wantPage: 1, wantPageSize: 20},
		{query: "?page=3&page_size=50", wantPage: 3, wantPageSize: 50},
		{query: "?page=0", wantErr: true},
		{query: "?page_size=101", wantErr: true},
	}
	for _, tt := range tests {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("GET", "/"+tt.query, nil)
		page, pageSize, err := parsePagination(ctx)
		if (err != nil) != tt.wantErr || page != tt.wantPage || pageSize != tt.wantPageSize {
			t.Fatalf("query=%q page=%d pageSize=%d err=%v", tt.query, page, pageSize, err)
		}
	}
}
