package testutils

import (
	"PostAndComment/graph/model"
	"PostAndComment/storage"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func CreateTestPost(t *testing.T, s storage.Storage, text string, commentsEnabled bool) *model.Post {
	post, err := s.NewPost(text, commentsEnabled)
	require.NoError(t, err)
	return post
}

func CreateTestComment(t *testing.T, s storage.Storage, postID string, parentID *string, text string) *model.Comment {
	comment, err := s.AddComment(postID, parentID, text)
	require.NoError(t, err)
	return comment
}

// Проверяет совпадение комментариев
func AssertCommentEqual(t *testing.T, expected, actual *model.Comment) {
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.PostID, actual.PostID)
	assert.Equal(t, expected.Text, actual.Text)
	assert.Equal(t, expected.ParentID, actual.ParentID)
}

// Проверяет совпадение постов
func AssertPostEqual(t *testing.T, expected, actual *model.Post) {
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.Text, actual.Text)
	assert.Equal(t, expected.CommentsEnabled, actual.CommentsEnabled)
}

// Пропускает тест если нет БД
func SkipIfNoDatabase(t *testing.T) {
	t.Helper()

	db := SetupTestDB(t)
	if db == nil {
		t.Skip("Test database not available")
	}
	db.Close()
}
