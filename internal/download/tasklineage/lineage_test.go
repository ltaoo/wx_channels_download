package tasklineage

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/pkg/testui/assert"
	"wx_channel/pkg/testui/require"
)

func TestDownloadTaskLineage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.DownloadTask{}))

	root := model.DownloadTask{Name: "root", PlatformId: "wxchannels"}
	require.NoError(t, Apply(db, &root, nil, ""))
	require.NoError(t, db.Create(&root).Error)
	require.NoError(t, FinalizeRoot(db, &root))
	assert.Equal(t, root.Id, root.RootTaskID)
	assert.Nil(t, root.ParentTaskID)

	child := model.DownloadTask{Name: "child", PlatformId: "wxchannels"}
	require.NoError(t, Apply(db, &child, &root.Id, ""))
	require.NoError(t, db.Create(&child).Error)
	require.NoError(t, FinalizeRoot(db, &child))
	require.NotNil(t, child.ParentTaskID)
	assert.Equal(t, root.Id, *child.ParentTaskID)
	assert.Equal(t, root.Id, child.RootTaskID)
	assert.Equal(t, model.TaskRelationDiscovered, child.RelationType)

	grandchild := model.DownloadTask{Name: "grandchild", PlatformId: "wxchannels"}
	require.NoError(t, Apply(db, &grandchild, &child.Id, model.TaskRelationDerived))
	require.NoError(t, db.Create(&grandchild).Error)
	assert.Equal(t, root.Id, grandchild.RootTaskID)
	assert.Equal(t, model.TaskRelationDerived, grandchild.RelationType)
}

func TestDownloadTaskLineageRejectsInvalidParent(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.DownloadTask{}))

	task := model.DownloadTask{}
	missingParentID := 999
	assert.EqualError(t, Apply(db, &task, &missingParentID, ""), "父下载任务不存在")
	assert.EqualError(t, Apply(db, &task, nil, model.TaskRelationDerived), "relation_type 需要同时提供 parent_task_id")

	invalidParentID := 0
	assert.EqualError(t, Apply(db, &task, &invalidParentID, ""), "parent_task_id 无效")
}
