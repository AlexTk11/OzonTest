package tests

import (
	"PostAndComment/graph/model"
	"PostAndComment/storage"
	"PostAndComment/storage/postgres"
	"PostAndComment/tests/testutils"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type PostgresStorageTestSuite struct {
	suite.Suite
	db      *sql.DB
	storage storage.Storage
}

func (suite *PostgresStorageTestSuite) SetupTest() {
	suite.db = testutils.SetupTestDB(suite.T())
	suite.storage = postgres.New(suite.db)
}

func (suite *PostgresStorageTestSuite) TearDownTest() {
	testutils.CleanTestDB(suite.T(), suite.db)
	suite.db.Close()
}

// Создание поста
func (suite *PostgresStorageTestSuite) TestNewPost_Success() {

	text := "Test post text"
	commentsEnabled := true

	post, err := suite.storage.NewPost(text, commentsEnabled)

	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), text, post.Text)
	assert.Equal(suite.T(), commentsEnabled, post.CommentsEnabled)
	assert.NotEmpty(suite.T(), post.ID)
	assert.NotEmpty(suite.T(), post.CreatedAt)

	// проверяем, что пост сохранился
	retrievedPost, err := suite.storage.GetPost(post.ID)
	require.NoError(suite.T(), err)
	testutils.AssertPostEqual(suite.T(), post, retrievedPost)
}

// Создание пустого поста
func (suite *PostgresStorageTestSuite) TestNewPost_EmptyText() {
	post, err := suite.storage.NewPost("", true)
	require.NoError(suite.T(), err)
	assert.Empty(suite.T(), post.Text)
	assert.True(suite.T(), post.CommentsEnabled)
}

// Добавление комментария
func (suite *PostgresStorageTestSuite) TestAddComment_Success() {
	post := testutils.CreateTestPost(suite.T(), suite.storage, "Test post", true)

	comment, err := suite.storage.AddComment(post.ID, nil, "Test comment")

	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), post.ID, comment.PostID)
	assert.Equal(suite.T(), "Test comment", comment.Text)
	assert.Nil(suite.T(), comment.ParentID)
	assert.NotEmpty(suite.T(), comment.ID)
}

// Ответ на комментарий
func (suite *PostgresStorageTestSuite) TestAddComment_WithReply() {
	post := testutils.CreateTestPost(suite.T(), suite.storage, "Test post", true)
	parentComment := testutils.CreateTestComment(suite.T(), suite.storage, post.ID, nil, "Parent comment")

	reply, err := suite.storage.AddComment(post.ID, &parentComment.ID, "Reply comment")

	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), post.ID, reply.PostID)
	assert.Equal(suite.T(), "Reply comment", reply.Text)
	require.NotNil(suite.T(), reply.ParentID)
	assert.Equal(suite.T(), parentComment.ID, *reply.ParentID)
}

// Комментарий к несуществующему посту
func (suite *PostgresStorageTestSuite) TestAddComment_NoPost() {
	_, err := suite.storage.AddComment("nonexistent-id", nil, "Test comment")
	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "not found")
}

// Комментарий к посту с выкл. комментариями
func (suite *PostgresStorageTestSuite) TestAddComment_DisabledComments() {
	post := testutils.CreateTestPost(suite.T(), suite.storage, "Post without comments", false)

	_, err := suite.storage.AddComment(post.ID, nil, "Test comment")
	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "comments are disabled")
}

// Запрос поста
func (suite *PostgresStorageTestSuite) TestGetPost_Success() {
	originalPost := testutils.CreateTestPost(suite.T(), suite.storage, "Original post", true)
	retrievedPost, err := suite.storage.GetPost(originalPost.ID)
	require.NoError(suite.T(), err)
	testutils.AssertPostEqual(suite.T(), originalPost, retrievedPost)
}

// Запрос несуществующего поста
func (suite *PostgresStorageTestSuite) TestGetPost_NoPost() {
	_, err := suite.storage.GetPost("nonexistent-id")

	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "not found")
}

// Запрос постов с пагинацией
func (suite *PostgresStorageTestSuite) TestGetPosts_WithPagination() {
	posts := make([]*model.Post, 5)
	for i := 0; i < 5; i++ {
		posts[i] = testutils.CreateTestPost(suite.T(), suite.storage, fmt.Sprintf("Post %d", i+1), true)
		// Добавляем небольшую задержку чтобы время создания отличалось
		time.Sleep(2 * time.Second)
	}

	// получаем первые 3 поста
	retrievedPosts, err := suite.storage.GetPosts(3, 0)
	require.NoError(suite.T(), err)
	assert.Len(suite.T(), retrievedPosts, 3)

	// проверяем порядок (новые посты первыми)
	assert.Equal(suite.T(), "Post 5", retrievedPosts[0].Text)
	assert.Equal(suite.T(), "Post 4", retrievedPosts[1].Text)
	assert.Equal(suite.T(), "Post 3", retrievedPosts[2].Text)
}

// Отключение комментариев
func (suite *PostgresStorageTestSuite) TestSetCommentsEnabled() {
	post := testutils.CreateTestPost(suite.T(), suite.storage, "Test post", true)
	assert.True(suite.T(), post.CommentsEnabled)

	// отключаем комментарии
	updatedPost, err := suite.storage.SetCommentsEnabled(post.ID, false)

	require.NoError(suite.T(), err)
	assert.False(suite.T(), updatedPost.CommentsEnabled)

	// проверяем изменения
	retrievedPost, err := suite.storage.GetPost(post.ID)
	require.NoError(suite.T(), err)
	assert.False(suite.T(), retrievedPost.CommentsEnabled)
}

// Получение вложенных комментариев
func (suite *PostgresStorageTestSuite) TestGetCommentsTree() {
	post := testutils.CreateTestPost(suite.T(), suite.storage, "Test post", true)

	// comment1 -> reply1, reply2
	// comment2
	comment1 := testutils.CreateTestComment(suite.T(), suite.storage, post.ID, nil, "Comment 1")
	comment2 := testutils.CreateTestComment(suite.T(), suite.storage, post.ID, nil, "Comment 2")
	reply1 := testutils.CreateTestComment(suite.T(), suite.storage, post.ID, &comment1.ID, "Reply 1")
	reply2 := testutils.CreateTestComment(suite.T(), suite.storage, post.ID, &comment1.ID, "Reply 2")

	_ = reply1
	_ = reply2
	_ = comment2
	comments, err := suite.storage.GetCommentsTree(post.ID, 10, 0)

	require.NoError(suite.T(), err)
	assert.Len(suite.T(), comments, 2) // Должно быть 2 корневых комментария

	assert.Equal(suite.T(), "Comment 1", comments[0].Text)
	assert.Equal(suite.T(), "Comment 2", comments[1].Text)

	// Проверяем, что у comment1 2 ответа
	require.NotNil(suite.T(), comments[0].Replies)
	assert.Len(suite.T(), comments[0].Replies, 2)

	// Проверяем ответы на comment1
	replies := comments[0].Replies
	assert.Equal(suite.T(), "Reply 1", replies[0].Text)
	assert.Equal(suite.T(), "Reply 2", replies[1].Text)

	// Проверяем, что ParentID указывает на comment1
	assert.Equal(suite.T(), comment1.ID, *replies[0].ParentID)
	assert.Equal(suite.T(), comment1.ID, *replies[1].ParentID)

	// Проверяем, что у comment2 нет ответов
	if comments[1].Replies != nil {
		assert.Len(suite.T(), comments[1].Replies, 0)
	}
}

func (suite *PostgresStorageTestSuite) TestSubscribeToComments_NonexistentPost() {
	_, _, err := suite.storage.SubscribeToComments("nonexistent-id")

	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "not found")
}

// Запуск тестов
func TestPostgresStorageTestSuite(t *testing.T) {
	testutils.SkipIfNoDatabase(t)

	suite.Run(t, new(PostgresStorageTestSuite))
}
