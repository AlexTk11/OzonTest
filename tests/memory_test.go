package tests

import (
	"PostAndComment/graph/model"
	"PostAndComment/storage/memory"
	"PostAndComment/tests/testutils"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type InMemoryStorageTestSuite struct {
	suite.Suite
	storage *memory.InMemoryStorage
}

func (suite *InMemoryStorageTestSuite) SetupTest() {
	suite.storage = memory.New()
}

// Создание поста
func (suite *InMemoryStorageTestSuite) TestNewPost_Success() {
	text := "Test post text"
	commentsEnabled := true
	post, err := suite.storage.NewPost(text, commentsEnabled)

	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), text, post.Text)
	assert.Equal(suite.T(), commentsEnabled, post.CommentsEnabled)
	assert.NotEmpty(suite.T(), post.ID)
	assert.NotEmpty(suite.T(), post.CreatedAt)

	retrievedPost, err := suite.storage.GetPost(post.ID)
	require.NoError(suite.T(), err)
	testutils.AssertPostEqual(suite.T(), post, retrievedPost)
}

// Создание пустого поста
func (suite *InMemoryStorageTestSuite) TestNewPost_EmptyText() {
	post, err := suite.storage.NewPost("", true)

	require.NoError(suite.T(), err)
	assert.Empty(suite.T(), post.Text)
	assert.True(suite.T(), post.CommentsEnabled)
}

// Получение постов
func (suite *InMemoryStorageTestSuite) TestGetPost_Success() {
	originalPost, err := suite.storage.NewPost("Post text", true)
	require.NoError(suite.T(), err)

	retrievedPost, err := suite.storage.GetPost(originalPost.ID)

	require.NoError(suite.T(), err)
	testutils.AssertPostEqual(suite.T(), originalPost, retrievedPost)
}

// Поиск нессуществующего поста
func (suite *InMemoryStorageTestSuite) TestGetPost_NotFound() {
	_, err := suite.storage.GetPost("aboba")

	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "not found")
}

// Запрос постов при пустом хранилище
func (suite *InMemoryStorageTestSuite) TestGetPosts_EmptyStorage() {
	posts, err := suite.storage.GetPosts(10, 0)

	require.NoError(suite.T(), err)
	assert.Len(suite.T(), posts, 0)
}

// Запрос постов с пагинацией
func (suite *InMemoryStorageTestSuite) TestGetPosts_WithPagination() {
	// создаем 5 постов
	createdPosts := make([]*model.Post, 5)
	for i := 0; i < 5; i++ {
		post, err := suite.storage.NewPost(fmt.Sprintf("Post %d", i+1), true)
		require.NoError(suite.T(), err)
		createdPosts[i] = post

		time.Sleep(1 * time.Second) // задержка для разного времени
	}

	// получаем первые 3 поста
	retrievedPosts, err := suite.storage.GetPosts(3, 0)

	require.NoError(suite.T(), err)
	assert.Len(suite.T(), retrievedPosts, 3)

	// проверяем порядок
	assert.Equal(suite.T(), "Post 5", retrievedPosts[0].Text)
	assert.Equal(suite.T(), "Post 4", retrievedPosts[1].Text)
	assert.Equal(suite.T(), "Post 3", retrievedPosts[2].Text)

	// получаем следующие посты с offset
	remainingPosts, err := suite.storage.GetPosts(3, 3)

	require.NoError(suite.T(), err)
	assert.Len(suite.T(), remainingPosts, 2)
	assert.Equal(suite.T(), "Post 2", remainingPosts[0].Text)
	assert.Equal(suite.T(), "Post 1", remainingPosts[1].Text)
}

// Добавить комментарий к посту
func (suite *InMemoryStorageTestSuite) TestAddComment_RootComment() {
	post, err := suite.storage.NewPost("Test post text", true)
	require.NoError(suite.T(), err)

	comment, err := suite.storage.AddComment(post.ID, nil, "root comment")

	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), post.ID, comment.PostID)
	assert.Equal(suite.T(), "root comment", comment.Text)
	assert.Nil(suite.T(), comment.ParentID)
	assert.NotEmpty(suite.T(), comment.ID)
	assert.NotEmpty(suite.T(), comment.CreatedAt)
}

// Добавить ответ к комментарию
func (suite *InMemoryStorageTestSuite) TestAddComment_ReplyToComment() {
	post, err := suite.storage.NewPost("test post text", true)
	require.NoError(suite.T(), err)

	rootComment, err := suite.storage.AddComment(post.ID, nil, "root comment")
	require.NoError(suite.T(), err)

	reply, err := suite.storage.AddComment(post.ID, &rootComment.ID, "reply comment")

	require.NoError(suite.T(), err)
	assert.Equal(suite.T(), post.ID, reply.PostID)
	assert.Equal(suite.T(), "reply comment", reply.Text)
	require.NotNil(suite.T(), reply.ParentID)
	assert.Equal(suite.T(), rootComment.ID, *reply.ParentID)
}

// Комментарий к несуществующему посту
func (suite *InMemoryStorageTestSuite) TestAddComment_NonexistentPost() {
	_, err := suite.storage.AddComment("nonexistent-post", nil, "Comment")

	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "not found")
}

// Комментарий к посту с выключенными комментариями
func (suite *InMemoryStorageTestSuite) TestAddComment_DisabledComments() {
	post, err := suite.storage.NewPost("Post without comments", false)
	require.NoError(suite.T(), err)

	_, err = suite.storage.AddComment(post.ID, nil, "Test comment")

	require.Error(suite.T(), err)
	assert.Contains(suite.T(), err.Error(), "comments are disabled")
}

// Вкл./выкл. комментарии к посту
func (suite *InMemoryStorageTestSuite) TestSetCommentsEnabled() {
	post, err := suite.storage.NewPost("Test post text", true)
	require.NoError(suite.T(), err)
	assert.True(suite.T(), post.CommentsEnabled)

	updatedPost, err := suite.storage.SetCommentsEnabled(post.ID, false)

	require.NoError(suite.T(), err)
	assert.False(suite.T(), updatedPost.CommentsEnabled)

	retrievedPost, err := suite.storage.GetPost(post.ID)
	require.NoError(suite.T(), err)
	assert.False(suite.T(), retrievedPost.CommentsEnabled)
}

// Вложенные комментарии
func (suite *InMemoryStorageTestSuite) TestGetCommentsTree_SimpleStructure() {
	post, err := suite.storage.NewPost("Test post", true)
	require.NoError(suite.T(), err)

	// comment1 -> reply1, reply2
	// comment2
	// comment2
	comment1, err := suite.storage.AddComment(post.ID, nil, "Comment 1")
	require.NoError(suite.T(), err)

	comment2, err := suite.storage.AddComment(post.ID, nil, "Comment 2")
	require.NoError(suite.T(), err)

	reply1, err := suite.storage.AddComment(post.ID, &comment1.ID, "Reply 1")
	require.NoError(suite.T(), err)

	reply2, err := suite.storage.AddComment(post.ID, &comment1.ID, "Reply 2")
	require.NoError(suite.T(), err)

	_ = comment2
	_, _ = reply1, reply2

	comments, err := suite.storage.GetCommentsTree(post.ID, 10, 0)

	require.NoError(suite.T(), err)
	assert.Len(suite.T(), comments, 2) // 2 корневых комментария

	assert.Equal(suite.T(), "Comment 1", comments[0].Text)
	assert.Equal(suite.T(), "Comment 2", comments[1].Text)

	//проверяем, ответы у comment1
	require.NotNil(suite.T(), comments[0].Replies)
	assert.Len(suite.T(), comments[0].Replies, 2)

	replies := comments[0].Replies
	assert.Equal(suite.T(), "Reply 1", replies[0].Text)
	assert.Equal(suite.T(), "Reply 2", replies[1].Text)
	assert.Equal(suite.T(), comment1.ID, *replies[0].ParentID)
	assert.Equal(suite.T(), comment1.ID, *replies[1].ParentID)

	// проверяем, что у comment2 нет ответов
	if comments[1].Replies != nil {
		assert.Len(suite.T(), comments[1].Replies, 0)
	}
}

// Получение комментария по подписке
func (suite *InMemoryStorageTestSuite) TestSubscribeToComments() {
	post, err := suite.storage.NewPost("Test post", true)
	require.NoError(suite.T(), err)

	ch, unsubscribe, err := suite.storage.SubscribeToComments(post.ID)
	require.NoError(suite.T(), err)
	defer (*unsubscribe)()

	// добавляем комментарий
	go func() {
		time.Sleep(10 * time.Millisecond)
		_, err := suite.storage.AddComment(post.ID, nil, "New comment")
		require.NoError(suite.T(), err)
	}()

	//ждем комментарий через подписку
	select {
	case comment := <-ch:
		assert.Equal(suite.T(), post.ID, comment.PostID)
		assert.Equal(suite.T(), "New comment", comment.Text)
		suite.T().Log("Successfully received comment by subscription")
	case <-time.After(5 * time.Second):
		suite.T().Error("Expected to receive comment by subscription")
	}
}

// Запуск тестов
func TestMemoryStorageTestSuite(t *testing.T) {
	suite.Run(t, new(InMemoryStorageTestSuite))
}
