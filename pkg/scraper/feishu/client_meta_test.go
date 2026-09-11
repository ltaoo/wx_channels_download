package feishu

import (
	"encoding/json"
	"testing"
)

// Sample of the /space/api/meta/ response for a wiki-mounted docx document.
const meta_response_sample = `{"code":0,"msg":"Success","data":{"type":22,"create_time":1785733837,"title":"WMS低代码迁移至源码2","owner_user_name":"李涛 Tao Li","obj_type":"docx"}}`

func TestDocumentMetaResponseDecoding(t *testing.T) {
	var meta document_meta_response
	if err := json.Unmarshal([]byte(meta_response_sample), &meta); err != nil {
		t.Fatalf("decode meta response: %v", err)
	}
	if meta.Code != 0 {
		t.Fatalf("code = %d, want 0", meta.Code)
	}
	if meta.Data.OwnerUserName != "李涛 Tao Li" {
		t.Fatalf("owner_user_name = %q", meta.Data.OwnerUserName)
	}
	if meta.Data.CreateTime != 1785733837 {
		t.Fatalf("create_time = %d", meta.Data.CreateTime)
	}
}
